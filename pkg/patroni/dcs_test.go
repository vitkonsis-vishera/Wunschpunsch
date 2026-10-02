package patroni

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDCSAndNodeControl(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/config":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"loop_wait":10,"ttl":30}`))
		case "/restart":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"message":"restart scheduled"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	client := NewClient([]string{ts.URL}, 1*time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// 1. Тест получения DCS Конфига
	cfg, err := client.GetDCSConfig(ctx)
	if err != nil {
		t.Fatalf("GetDCSConfig failed: %v", err)
	}
	if cfg == "" {
		t.Error("Expected non-empty DCS config")
	}

	// 2. Тест команды перезапуска ноды
	err = client.RestartNode(ctx, "pg-node-1")
	if err != nil {
		t.Fatalf("RestartNode failed: %v", err)
	}
}
