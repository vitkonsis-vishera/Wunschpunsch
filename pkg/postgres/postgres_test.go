package postgres

import (
	"context"
	"testing"
	"time"
)

func TestPGPoolManager_UnreachableNode(t *testing.T) {
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

	// Проверяем обработку заведомо недоступного хоста (не должна быть паника)
	metrics := mgr.FetchNodeMetrics(ctx, "127.0.0.254")
	if metrics.Error == nil {
		t.Error("Expected error when connecting to unreachable host, got nil")
	}
}
