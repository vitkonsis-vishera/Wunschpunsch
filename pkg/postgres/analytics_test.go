package postgres

import (
	"context"
	"testing"
	"time"
)

func TestAnalyticsMethods_Unreachable(t *testing.T) {
	cfg := Config{
		User:     "postgres",
		Password: "invalid_password",
		Database: "postgres",
		Port:     5432,
	}

	mgr := NewPGPoolManager(cfg)
	defer mgr.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// 1. Проверка обработки недоступности хоста для pg_stat_statements
	_, err := mgr.GetTopQueries(ctx, "127.0.0.254")
	if err == nil {
		t.Error("Expected error when fetching queries from unreachable host")
	}

	// 2. Проверка обработки недоступности для блокировок
	_, err = mgr.GetActiveLocks(ctx, "127.0.0.254")
	if err == nil {
		t.Error("Expected error when fetching locks from unreachable host")
	}

	// 3. Проверка обработки для terminate backend
	err = mgr.TerminateBackend(ctx, "127.0.0.254", 12345, false)
	if err == nil {
		t.Error("Expected error when terminating backend on unreachable host")
	}
}
