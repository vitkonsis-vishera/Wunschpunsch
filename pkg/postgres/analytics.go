package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
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

	explainSQL := "EXPLAIN (ANALYZE, BUFFERS, VERBOSE, FORMAT TEXT) " + queryText
	rows, err := db.QueryContext(ctx, explainSQL)
	if err != nil {
		return "", fmt.Errorf("ошибка выполнения EXPLAIN ANALYZE: %w", err)
	}
	defer rows.Close()

	var planLines []string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err == nil {
			planLines = append(planLines, line)
		}
	}

	return fmt.Sprintf("EXPLAIN ANALYZE plan for query:\n%s\n\nExecution Breakdown & Buffers:\n%s", queryText, strings.Join(planLines, "\n")), nil
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

type IndexRecommendation struct {
	Schemaname string `json:"schemaname"`
	Tablename  string `json:"tablename"`
	SeqScan    int64  `json:"seq_scan"`
	SeqTupRead int64  `json:"seq_tup_read"`
	IdxScan    int64  `json:"idx_scan"`
	TableSize  string `json:"table_size"`
	SuggestedSQL string `json:"suggested_sql"`
}

func (m *PGPoolManager) GetIndexRecommendations(ctx context.Context, host string) ([]IndexRecommendation, error) {
	db, err := m.GetDB(host)
	if err != nil {
		return nil, fmt.Errorf("ошибка подключения к %s: %w", host, err)
	}

	query := `
		SELECT 
			schemaname,
			relname,
			seq_scan,
			seq_tup_read,
			COALESCE(idx_scan, 0) AS idx_scan,
			pg_size_pretty(pg_total_relation_size(relid)) AS table_size
		FROM pg_stat_user_tables
		WHERE seq_scan > 1000 AND (seq_scan > idx_scan OR idx_scan IS NULL)
		ORDER BY seq_tup_read DESC
		LIMIT 10;
	`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения статистики таблиц: %w", err)
	}
	defer rows.Close()

	var recs []IndexRecommendation
	for rows.Next() {
		var r IndexRecommendation
		if err := rows.Scan(&r.Schemaname, &r.Tablename, &r.SeqScan, &r.SeqTupRead, &r.IdxScan, &r.TableSize); err == nil {
			r.SuggestedSQL = fmt.Sprintf("CREATE INDEX idx_%s_auto ON %s.%s (...); -- Seq scans: %d, Seq tup read: %d", r.Tablename, r.Schemaname, r.Tablename, r.SeqScan, r.SeqTupRead)
			recs = append(recs, r)
		}
	}

	return recs, nil
}

type DeadlockInfo struct {
	BlockedPID     int     `json:"blocked_pid"`
	BlockedUser    string  `json:"blocked_user"`
	BlockedQuery   string  `json:"blocked_query"`
	BlockingPID    int     `json:"blocking_pid"`
	BlockingUser   string  `json:"blocking_user"`
	BlockingQuery  string  `json:"blocking_query"`
	DurationSec    float64 `json:"duration_sec"`
}

func (m *PGPoolManager) GetDeadlocks(ctx context.Context, host string) ([]DeadlockInfo, error) {
	db, err := m.GetDB(host)
	if err != nil {
		return nil, fmt.Errorf("ошибка подключения к %s: %w", host, err)
	}

	query := `
		SELECT 
			blocked_locks.pid AS blocked_pid,
			ka.usename AS blocked_user,
			blocked_activity.query AS blocked_query,
			blocking_locks.pid AS blocking_pid,
			kb.usename AS blocking_user,
			blocking_activity.query AS blocking_query,
			EXTRACT(EPOCH FROM (clock_timestamp() - blocked_activity.xact_start)) AS duration_sec
		FROM pg_catalog.pg_locks blocked_locks
		JOIN pg_catalog.pg_stat_activity blocked_activity ON blocked_activity.pid = blocked_locks.pid
		JOIN pg_catalog.pg_stat_user_tables blocked_rels ON blocked_rels.relid = blocked_locks.relation
		JOIN pg_catalog.pg_locks blocking_locks 
			ON blocking_locks.locktype = blocked_locks.locktype
			AND blocking_locks.database IS NOT DISTINCT FROM blocked_locks.database
			AND blocking_locks.relation IS NOT DISTINCT FROM blocked_locks.relation
			AND blocking_locks.page IS NOT DISTINCT FROM blocked_locks.page
			AND blocking_locks.tuple IS NOT DISTINCT FROM blocked_locks.tuple
			AND blocking_locks.virtualxid IS NOT DISTINCT FROM blocked_locks.virtualxid
			AND blocking_locks.transactionid IS NOT DISTINCT FROM blocked_locks.transactionid
			AND blocking_locks.classid IS NOT DISTINCT FROM blocked_locks.classid
			AND blocking_locks.objid IS NOT DISTINCT FROM blocked_locks.objid
			AND blocking_locks.objsubid IS NOT DISTINCT FROM blocked_locks.objsubid
			AND blocking_locks.pid != blocked_locks.pid
		JOIN pg_catalog.pg_stat_activity blocking_activity ON blocking_activity.pid = blocking_locks.pid
		JOIN pg_catalog.pg_stat_user_tables blocking_rels ON blocking_rels.relid = blocking_locks.relation
		LEFT JOIN pg_user ka ON ka.usesysid = blocked_activity.usesysid
		LEFT JOIN pg_user kb ON kb.usesysid = blocking_activity.usesysid
		WHERE NOT blocked_locks.granted
		  AND blocking_locks.granted;
	`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения информации о блокировках: %w", err)
	}
	defer rows.Close()

	var deadlocks []DeadlockInfo
	for rows.Next() {
		var d DeadlockInfo
		if err := rows.Scan(&d.BlockedPID, &d.BlockedUser, &d.BlockedQuery, &d.BlockingPID, &d.BlockingUser, &d.BlockingQuery, &d.DurationSec); err == nil {
			deadlocks = append(deadlocks, d)
		}
	}

	return deadlocks, nil
}

type ReplicationLagStat struct {
	ClientAddr      string `json:"client_addr"`
	ApplicationName string `json:"application_name"`
	State           string `json:"state"`
	SyncState       string `json:"sync_state"`
	SentLSN         string `json:"sent_lsn"`
	WriteLSN        string `json:"write_lsn"`
	FlushLSN        string `json:"flush_lsn"`
	ReplayLSN       string `json:"replay_lsn"`
	WriteLag        string `json:"write_lag"`
	FlushLag        string `json:"flush_lag"`
	ReplayLag       string `json:"replay_lag"`
	ByteLag         int64  `json:"byte_lag"`
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

type DDLHistoryEntry struct {
	Timestamp  string `json:"timestamp"`
	Database   string `json:"database"`
	CommandTag string `json:"command_tag"`
	ObjectName string `json:"object_name"`
	QueryText  string `json:"query_text"`
}

func (m *PGPoolManager) GetDDLAuditLog(ctx context.Context, host string) ([]DDLHistoryEntry, error) {
	db, err := m.GetDB(host)
	if err != nil {
		return nil, fmt.Errorf("ошибка подключения к %s: %w", host, err)
	}

	query := `
		SELECT 
			to_char(now(), 'YYYY-MM-DD HH24:MI:SS') AS ts,
			current_database() AS db,
			tag AS command_tag,
			object_identity AS object_name,
			statement AS query_text
		FROM pg_stat_statements
		WHERE query ILIKE 'CREATE %' 
		   OR query ILIKE 'ALTER %' 
		   OR query ILIKE 'DROP %'
		LIMIT 20;
	`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения DDL аудита: %w", err)
	}
	defer rows.Close()

	var entries []DDLHistoryEntry
	for rows.Next() {
		var e DDLHistoryEntry
		if err := rows.Scan(&e.Timestamp, &e.Database, &e.CommandTag, &e.ObjectName, &e.QueryText); err == nil {
			entries = append(entries, e)
		}
	}

	return entries, nil
}

type StandbyHealthMatrix struct {
	StandbyHost  string  `json:"standby_host"`
	Status       string  `json:"status"`
	LagBytes     int64   `json:"lag_bytes"`
	LatencyMs    float64 `json:"latency_ms"`
}

func (m *PGPoolManager) GetStandbyHealthMatrix(ctx context.Context, host string) ([]StandbyHealthMatrix, error) {
	db, err := m.GetDB(host)
	if err != nil {
		return nil, fmt.Errorf("ошибка подключения к %s: %w", host, err)
	}

	query := `
		SELECT 
			COALESCE(client_addr::text, 'unknown') AS standby_host,
			state AS status,
			COALESCE(pg_wal_lsn_diff(pg_current_wal_lsn(), replay_lsn), 0) AS lag_bytes,
			EXTRACT(EPOCH FROM (COALESCE(write_lag, '0s'::interval))) * 1000 AS latency_ms
		FROM pg_stat_replication;
	`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения матричного здоровья реплик: %w", err)
	}
	defer rows.Close()

	var matrix []StandbyHealthMatrix
	for rows.Next() {
		var sh StandbyHealthMatrix
		if err := rows.Scan(&sh.StandbyHost, &sh.Status, &sh.LagBytes, &sh.LatencyMs); err == nil {
			matrix = append(matrix, sh)
		}
	}

	return matrix, nil
}

func (m *PGPoolManager) GetReplicationLag(ctx context.Context, host string) ([]ReplicationLagStat, error) {
	db, err := m.GetDB(host)
	if err != nil {
		return nil, fmt.Errorf("ошибка подключения к %s: %w", host, err)
	}

	query := `
		SELECT 
			COALESCE(client_addr::text, 'local') AS client_addr,
			COALESCE(application_name, 'N/A') AS application_name,
			COALESCE(state, 'N/A') AS state,
			COALESCE(sync_state, 'async') AS sync_state,
			COALESCE(sent_lsn::text, '0/0000000') AS sent_lsn,
			COALESCE(write_lsn::text, '0/0000000') AS write_lsn,
			COALESCE(flush_lsn::text, '0/0000000') AS flush_lsn,
			COALESCE(replay_lsn::text, '0/0000000') AS replay_lsn,
			COALESCE(write_lag::text, '0s') AS write_lag,
			COALESCE(flush_lag::text, '0s') AS flush_lag,
			COALESCE(replay_lag::text, '0s') AS replay_lag,
			COALESCE(pg_wal_lsn_diff(pg_current_wal_lsn(), replay_lsn), 0) AS byte_lag
		FROM pg_stat_replication;
	`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("pg_stat_replication недоступен (возможно, нода не лидер): %w", err)
	}
	defer rows.Close()

	var stats []ReplicationLagStat
	for rows.Next() {
		var r ReplicationLagStat
		if err := rows.Scan(&r.ClientAddr, &r.ApplicationName, &r.State, &r.SyncState, &r.SentLSN, &r.WriteLSN, &r.FlushLSN, &r.ReplayLSN, &r.WriteLag, &r.FlushLag, &r.ReplayLag, &r.ByteLag); err == nil {
			stats = append(stats, r)
		}
	}

	return stats, nil
}
