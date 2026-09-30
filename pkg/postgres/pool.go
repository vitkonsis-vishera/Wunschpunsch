package postgres

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type NodeMetrics struct {
	NodeHost          string
	IsInRecovery      bool
	ActiveConnections int
	MaxConnections    int
	TransactionsSec   float64
	CacheHitRatio     float64
	PGVersion         string
	Error             error
	IdleConnections   int
	TPS               float64
}

type Config struct {
	User     string
	Password string
	Database string
	Port     int
}

type PGPoolManager struct {
	cfg   Config
	pools map[string]*pgxpool.Pool // key: host
	mu    sync.RWMutex
}

func NewPGPoolManager(cfg Config) *PGPoolManager {
	return &PGPoolManager{
		cfg:   cfg,
		pools: make(map[string]*pgxpool.Pool),
	}
}

// GetOrCreatePool возвращает или создает пул подключений к конкретной ноде
func (m *PGPoolManager) GetOrCreatePool(ctx context.Context, host string) (*pgxpool.Pool, error) {
	m.mu.RLock()
	pool, exists := m.pools[host]
	m.mu.RUnlock()

	if exists {
		return pool, nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Двойная проверка на случай race condition
	if pool, exists := m.pools[host]; exists {
		return pool, nil
	}

	// Формируем DSN без логина и пароля
	connString := fmt.Sprintf("postgres://%s:%d/%s?sslmode=prefer&connect_timeout=3",
		host, m.cfg.Port, m.cfg.Database)

	poolConfig, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config for %s: %w", host, err)
	}

	// Задаем учетные данные напрямую
	poolConfig.ConnConfig.User = m.cfg.User
	poolConfig.ConnConfig.Password = m.cfg.Password

	poolConfig.MaxConns = 3
	poolConfig.MinConns = 1
	poolConfig.MaxConnIdleTime = 30 * time.Second

	newPool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create pool for %s: %w", host, err)
	}

	m.pools[host] = newPool
	return newPool, nil
}

// FetchNodeMetrics собирает системные метрики с указанного узла PG
func (m *PGPoolManager) FetchNodeMetrics(ctx context.Context, host string) NodeMetrics {
	metrics := NodeMetrics{NodeHost: host}

	pool, err := m.GetOrCreatePool(ctx, host)
	if err != nil {
		metrics.Error = err
		return metrics
	}

	// 1. Проверка роли (Primary/Standby) и версии
	err = pool.QueryRow(ctx, "SELECT pg_is_in_recovery(), version()").Scan(&metrics.IsInRecovery, &metrics.PGVersion)
	if err != nil {
		metrics.Error = err
		return metrics
	}

	// 2. Статистика подключений
	connQuery := `
		SELECT 
			count(*) AS active_conns,
			setting::int AS max_conns
		FROM pg_stat_activity, pg_settings 
		WHERE name = 'max_connections'
		GROUP BY setting;
	`
	_ = pool.QueryRow(ctx, connQuery).Scan(&metrics.ActiveConnections, &metrics.MaxConnections)

	// 3. Cache Hit Ratio
	cacheQuery := `
		SELECT 
			CASE WHEN (sum(heap_blks_read) + sum(heap_blks_hit)) = 0 THEN 0.0
			ELSE sum(heap_blks_hit) * 100.0 / (sum(heap_blks_read) + sum(heap_blks_hit))
			END AS cache_hit_ratio
		FROM pg_statio_user_tables;
	`
	_ = pool.QueryRow(ctx, cacheQuery).Scan(&metrics.CacheHitRatio)

	return metrics
}

// Close закрывает все установленные соединения
func (m *PGPoolManager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for host, pool := range m.pools {
		pool.Close()
		delete(m.pools, host)
	}
}
