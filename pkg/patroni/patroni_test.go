package patroni

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestPatroniClient_GetClusterState(t *testing.T) {
	// Мок-сервер Patroni REST API
	mockStatus := ClusterStatus{
		Scope: "postgres-cluster",
		Pause: false,
		Members: []Member{
			{Name: "pg-node-1", Host: "192.168.1.10", Role: "leader", State: "running", Timeline: 1, Lag: 0},
			{Name: "pg-node-2", Host: "192.168.1.11", Role: "replica", State: "running", Timeline: 1, Lag: 1048576},
		},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/cluster" || r.URL.Path == "/patroni" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(mockStatus)
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	client := NewClient([]string{ts.URL}, 1*time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	status, err := client.GetClusterState(ctx)
	if err != nil {
		t.Fatalf("GetClusterState returned error: %v", err)
	}

	if status.Scope != "postgres-cluster" {
		t.Errorf("Expected scope 'postgres-cluster', got '%s'", status.Scope)
	}

	if len(status.Members) != 2 {
		t.Errorf("Expected 2 cluster members, got %d", len(status.Members))
	}
}
