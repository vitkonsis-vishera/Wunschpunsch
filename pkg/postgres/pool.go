package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
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

// GetPostgresLogs запрашивает последние записи логов напрямую из PostgreSQL
// с помощью системных функций pg_ls_logdir() и pg_read_file()
func (m *PGPoolManager) GetPostgresLogs(ctx context.Context, host string, limit int) ([]string, error) {
	db, err := m.GetDB(host)
	if err != nil {
		return nil, fmt.Errorf("ошибка подключения к %s: %w", host, err)
	}

	if limit <= 0 {
		limit = 20
	}

	// Чтение последних строк из самого свежего лог-файла в pg_log/log
	query := `
		SELECT line FROM (
			SELECT unnest(string_to_array(
				pg_read_file(
					(SELECT name FROM pg_ls_logdir() ORDER BY modification DESC LIMIT 1),
					0, 200000
				), E'\n'
			)) AS line
		) sub
		WHERE line <> ''
		LIMIT $1;
	`

	rows, err := db.QueryContext(ctx, query, limit)
	if err != nil {
		// Если у пользователя PG нет прав на чтение системных файлов или выключен logging_collector
		return []string{"[Postgres Logger] Чтение логов через SQL недоступно (требуются права pg_read_all_server_files или утилита journalctl)."}, nil
	}
	defer rows.Close()

	var logs []string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err == nil {
			logs = append(logs, line)
		}
	}

	if len(logs) == 0 {
		return []string{"[Postgres Logger] Файлы логов PG пусты или не заполняются."}, nil
	}

	return logs, nil
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

	if pool, exists := m.pools[host]; exists {
		return pool, nil
	}

	connString := fmt.Sprintf("postgres://%s:%d/%s?sslmode=prefer&connect_timeout=3",
		host, m.cfg.Port, m.cfg.Database)

	poolConfig, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config for %s: %w", host, err)
	}

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

// GetDB возвращает стандартное подключение *sql.DB для указанного хоста
func (m *PGPoolManager) GetDB(host string) (*sql.DB, error) {
	pool, err := m.GetOrCreatePool(context.Background(), host)
	if err != nil {
		return nil, err
	}
	return stdlib.OpenDBFromPool(pool), nil
}

// FetchNodeMetrics собирает системные метрики с указанного узла PG
func (m *PGPoolManager) FetchNodeMetrics(ctx context.Context, host string) NodeMetrics {
	metrics := NodeMetrics{NodeHost: host}

	pool, err := m.GetOrCreatePool(ctx, host)
	if err != nil {
		metrics.Error = err
		return metrics
	}

	err = pool.QueryRow(ctx, "SELECT pg_is_in_recovery(), version()").Scan(&metrics.IsInRecovery, &metrics.PGVersion)
	if err != nil {
		metrics.Error = err
		return metrics
	}

	connQuery := `
		SELECT 
			count(*) AS active_conns,
			setting::int AS max_conns
		FROM pg_stat_activity, pg_settings 
		WHERE name = 'max_connections'
		GROUP BY setting;
	`
	_ = pool.QueryRow(ctx, connQuery).Scan(&metrics.ActiveConnections, &metrics.MaxConnections)

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

func (m *PGPoolManager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for host, pool := range m.pools {
		pool.Close()
		delete(m.pools, host)
	}
}
