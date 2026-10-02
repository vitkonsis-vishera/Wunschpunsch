package postgres

import (
	"context"
	"fmt"
)

// QueryStat содержит информацию о тяжелых запросах из pg_stat_statements
type QueryStat struct {
	Query     string
	Calls     int64
	TotalTime float64 // в миллисекундах
	MeanTime  float64 // в миллисекундах
	Rows      int64
}

// LockInfo содержит информацию о блокировках и зависших сессиях
type LockInfo struct {
	PID          int
	User         string
	Database     string
	ClientAddr   string
	State        string
	DurationSec  float64
	BlockedByPID int
	Query        string
}

// GetTopQueries запрашивает топ-10 самых медленных запросов по общему времени выполнения
func (p *PGPoolManager) GetTopQueries(ctx context.Context, host string) ([]QueryStat, error) {
	conn, err := p.GetOrCreatePool(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("connection error: %w", err)
	}

	sql := `
		SELECT 
			LEFT(query, 100) AS query_truncated,
			calls,
			total_exec_time,
			mean_exec_time,
			rows
		FROM pg_stat_statements
		ORDER BY total_exec_time DESC
		LIMIT 10;
	`

	rows, err := conn.Query(ctx, sql)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch pg_stat_statements (extension enabled?): %w", err)
	}
	defer rows.Close()

	var stats []QueryStat
	for rows.Next() {
		var q QueryStat
		if err := rows.Scan(&q.Query, &q.Calls, &q.TotalTime, &q.MeanTime, &q.Rows); err != nil {
			continue
		}
		stats = append(stats, q)
	}

	return stats, nil
}

// GetActiveLocks запрашивает процессы, находящиеся в состоянии ожидания/блокировки или выполняющиеся более 2 секунд
func (p *PGPoolManager) GetActiveLocks(ctx context.Context, host string) ([]LockInfo, error) {
	conn, err := p.GetOrCreatePool(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("connection error: %w", err)
	}

	sql := `
		SELECT 
			pid,
			usename,
			datname,
			COALESCE(host(client_addr), 'local'),
			state,
			EXTRACT(EPOCH FROM (clock_timestamp() - query_start)) AS duration_sec,
			LEFT(query, 80) AS query
		FROM pg_stat_activity
		WHERE state != 'idle' 
		  AND pid != pg_backend_pid()
		  AND (clock_timestamp() - query_start) > INTERVAL '2 seconds'
		ORDER BY duration_sec DESC
		LIMIT 15;
	`

	rows, err := conn.Query(ctx, sql)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch lock info: %w", err)
	}
	defer rows.Close()

	var locks []LockInfo
	for rows.Next() {
		var l LockInfo
		if err := rows.Scan(&l.PID, &l.User, &l.Database, &l.ClientAddr, &l.State, &l.DurationSec, &l.Query); err != nil {
			continue
		}
		locks = append(locks, l)
	}

	return locks, nil
}

// TerminateBackend завершает процесс в PostgreSQL (pg_cancel_backend или pg_terminate_backend)
func (p *PGPoolManager) TerminateBackend(ctx context.Context, host string, pid int, force bool) error {
	conn, err := p.GetOrCreatePool(ctx, host)
	if err != nil {
		return fmt.Errorf("connection error: %w", err)
	}

	funcName := "pg_cancel_backend"
	if force {
		funcName = "pg_terminate_backend"
	}

	sql := fmt.Sprintf("SELECT %s($1)", funcName)

	var success bool
	err = conn.QueryRow(ctx, sql, pid).Scan(&success)
	if err != nil {
		return fmt.Errorf("failed to execute %s(%d): %w", funcName, pid, err)
	}

	if !success {
		return fmt.Errorf("process %d could not be terminated or already finished", pid)
	}

	return nil
}
