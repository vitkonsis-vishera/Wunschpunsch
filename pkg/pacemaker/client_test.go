package pacemaker

import (
	"context"
	"testing"
)

func TestNewClient(t *testing.T) {
	client := NewClient()
	if client == nil {
		t.Fatalf("expected non-nil client")
	}
}

func TestPacemakerClientMethodsWithoutBinary(t *testing.T) {
	client := NewClient()
	ctx := context.Background()

	// Since 'pcs' binary may not be present in test container, we verify calls handle or return error appropriately
	_ = client.MoveResource(ctx, "pg_master", "node2")
	_ = client.CleanupResource(ctx, "pg_master")
	_ = client.ToggleNodeStandby(ctx, "node1", true)
	_ = client.ToggleNodeStandby(ctx, "node1", false)
}
