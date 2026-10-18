package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type QueryStat struct {
	Query     string  `json:"query"`
	Calls     int64   `json:"calls"`
	TotalTime float64 `json:"total_time_ms"`
	MeanTime  float64 `json:"mean_time_ms"`
}

type LockInfo struct {
	PID         int     `json:"pid"`
	User        string  `json:"user"`
	State       string  `json:"state"`
	DurationSec float64 `json:"duration_sec"`
	Query       string  `json:"query"`
}

type AutovacuumWorker struct {
	PID         int     `json:"pid"`
	Database    string  `json:"database"`
	Table       string  `json:"table"`
	Phase       string  `json:"phase"`
	DurationSec float64 `json:"duration_sec"`
}

type TableBloatStat struct {
	SchemaName      string     `json:"schema_name"`
	TableName       string     `json:"table_name"`
	LiveTuples      int64      `json:"live_tuples"`
	DeadTuples      int64      `json:"dead_tuples"`
	DeadTuplesPct   float64    `json:"dead_tuples_pct"`
	LastAutovacuum  *time.Time `json:"last_autovacuum"`
	LastAutoanalyze *time.Time `json:"last_autoanalyze"`
}

func (m *PGPoolManager) GetTopQueries(ctx context.Context, host string) ([]QueryStat, error) {
	db, err := m.GetDB(host)
	if err != nil {
		return nil, fmt.Errorf("ошибка подключения к %s: %w", host, err)
	}

	query := `
		SELECT 
			query, 
			calls, 
			total_exec_time AS total_time, 
			mean_exec_time AS mean_time 
		FROM pg_stat_statements 
		ORDER BY total_exec_time DESC 
		LIMIT 10;
	`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("pg_stat_statements недоступен: %w", err)
	}
	defer rows.Close()

	var stats []QueryStat
	for rows.Next() {
		var q QueryStat
		if err := rows.Scan(&q.Query, &q.Calls, &q.TotalTime, &q.MeanTime); err == nil {
			stats = append(stats, q)
		}
	}

	return stats, nil
}

func (m *PGPoolManager) GetActiveLocks(ctx context.Context, host string) ([]LockInfo, error) {
	db, err := m.GetDB(host)
	if err != nil {
		return nil, fmt.Errorf("ошибка подключения к %s: %w", host, err)
	}

	query := `
		SELECT 
			pid, 
			usename, 
			state, 
			EXTRACT(EPOCH FROM (clock_timestamp() - xact_start)) AS duration_sec, 
			query 
		FROM pg_stat_activity 
		WHERE state != 'idle' 
		  AND xact_start IS NOT NULL 
		  AND EXTRACT(EPOCH FROM (clock_timestamp() - xact_start)) > 2 
		ORDER BY duration_sec DESC 
		LIMIT 10;
	`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения блокировок: %w", err)
	}
	defer rows.Close()

	var locks []LockInfo
	for rows.Next() {
		var l LockInfo
		if err := rows.Scan(&l.PID, &l.User, &l.State, &l.DurationSec, &l.Query); err == nil {
			locks = append(locks, l)
		}
	}

	return locks, nil
}

func (m *PGPoolManager) TerminateBackend(ctx context.Context, host string, pid int, force bool) error {
	db, err := m.GetDB(host)
	if err != nil {
		return fmt.Errorf("ошибка подключения к %s: %w", host, err)
	}

	query := "SELECT pg_cancel_backend($1)"
	if force {
		query = "SELECT pg_terminate_backend($1)"
	}

	var res bool
	err = db.QueryRowContext(ctx, query, pid).Scan(&res)
	if err != nil {
		return fmt.Errorf("ошибка выполнения функции: %w", err)
	}
	if !res {
		return fmt.Errorf("процесс с PID %d не существует или недоступен для завершения", pid)
	}

	return nil
}

func (m *PGPoolManager) ExplainQuery(ctx context.Context, host string, queryText string) (string, error) {
	db, err := m.GetDB(host)
	if err != nil {
		return "", fmt.Errorf("ошибка подключения к %s: %w", host, err)
	}

	explainSQL := "EXPLAIN " + queryText
	rows, err := db.QueryContext(ctx, explainSQL)
	if err != nil {
		return "", fmt.Errorf("ошибка выполнения EXPLAIN: %w", err)
	}
	defer rows.Close()

	var planLines []string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err == nil {
			planLines = append(planLines, line)
		}
	}

	return fmt.Sprintf("EXPLAIN plan for query:\n%s\n\nPlan:\n%s", queryText, fmt.Sprint(planLines)), nil
}

func (m *PGPoolManager) GetAutovacuumWorkers(ctx context.Context, host string) ([]AutovacuumWorker, error) {
	db, err := m.GetDB(host)
	if err != nil {
		return nil, fmt.Errorf("ошибка подключения к %s: %w", host, err)
	}

	query := `
		SELECT 
			a.pid,
			a.datname,
			COALESCE(c.relname, 'N/A') AS relname,
			COALESCE(p.phase, 'running') AS phase,
			EXTRACT(EPOCH FROM (clock_timestamp() - a.xact_start)) AS duration_sec
		FROM pg_stat_activity a
		LEFT JOIN pg_stat_progress_vacuum p ON a.pid = p.pid
		LEFT JOIN pg_class c ON p.relid = c.oid
		WHERE a.query LIKE 'autovacuum:%'
		ORDER BY duration_sec DESC;
	`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения процессов autovacuum: %w", err)
	}
	defer rows.Close()

	var workers []AutovacuumWorker
	for rows.Next() {
		var w AutovacuumWorker
		if err := rows.Scan(&w.PID, &w.Database, &w.Table, &w.Phase, &w.DurationSec); err == nil {
			workers = append(workers, w)
		}
	}

	return workers, nil
}

func (m *PGPoolManager) GetTableBloatStats(ctx context.Context, host string) ([]TableBloatStat, error) {
	db, err := m.GetDB(host)
	if err != nil {
		return nil, fmt.Errorf("ошибка подключения к %s: %w", host, err)
	}

	query := `
		SELECT 
			schemaname,
			relname,
			n_live_tup,
			n_dead_tup,
			CASE WHEN (n_live_tup + n_dead_tup) > 0 
				THEN ROUND((n_dead_tup::numeric / (n_live_tup + n_dead_tup)::numeric) * 100, 1)
				ELSE 0.0 
			END AS dead_pct,
			last_autovacuum,
			last_autoanalyze
		FROM pg_stat_user_tables
		WHERE n_dead_tup > 100
		ORDER BY n_dead_tup DESC
		LIMIT 8;
	`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения статистики bloat: %w", err)
	}
	defer rows.Close()

	var stats []TableBloatStat
	for rows.Next() {
		var s TableBloatStat
		var lastVac, lastAnl sql.NullTime
		if err := rows.Scan(&s.SchemaName, &s.TableName, &s.LiveTuples, &s.DeadTuples, &s.DeadTuplesPct, &lastVac, &lastAnl); err == nil {
			if lastVac.Valid {
				s.LastAutovacuum = &lastVac.Time
			}
			if lastAnl.Valid {
				s.LastAutoanalyze = &lastAnl.Time
			}
			stats = append(stats, s)
		}
	}

	return stats, nil
}

type PgBouncerPoolStat struct {
	Database    string `json:"database"`
	User        string `json:"user"`
	ClActive    int64  `json:"cl_active"`
	ClWaiting   int64  `json:"cl_waiting"`
	SvActive    int64  `json:"sv_active"`
	SvIdle      int64  `json:"sv_idle"`
	SvUsed      int64  `json:"sv_used"`
	SvTested    int64  `json:"sv_tested"`
	SvLogin     int64  `json:"sv_login"`
	Maxwait     int64  `json:"maxwait"`
	MaxwaitUs   int64  `json:"maxwait_us"`
	PoolMode    string `json:"pool_mode"`
}

func (m *PGPoolManager) GetPgBouncerPools(ctx context.Context, host string) ([]PgBouncerPoolStat, error) {
	db, err := m.GetDB(host)
	if err != nil {
		return nil, fmt.Errorf("ошибка подключения к %s: %w", host, err)
	}

	query := `SHOW POOLS;`
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("ошибка SHOW POOLS: %w", err)
	}
	defer rows.Close()

	var pools []PgBouncerPoolStat
	for rows.Next() {
		var p PgBouncerPoolStat
		if err := rows.Scan(&p.Database, &p.User, &p.ClActive, &p.ClWaiting, &p.SvActive, &p.SvIdle, &p.SvUsed, &p.SvTested, &p.SvLogin, &p.Maxwait, &p.MaxwaitUs, &p.PoolMode); err == nil {
			pools = append(pools, p)
		}
	}

	return pools, nil
}
