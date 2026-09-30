package exporter

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestExportMetricsJSONAndCSV(t *testing.T) {
	tempDir := t.TempDir()

	snapshot := MetricsSnapshot{
		Timestamp:     time.Now(),
		ActiveConns:   15,
		IdleConns:     5,
		CacheHitRatio: 99.4,
		TPS:           1250.5,
	}

	// 1. Тест экспорта JSON
	jsonPath := filepath.Join(tempDir, "test_metrics.json")
	err := ExportMetricsJSON(jsonPath, snapshot)
	if err != nil {
		t.Fatalf("ExportMetricsJSON failed: %v", err)
	}

	if _, err := os.Stat(jsonPath); os.IsNotExist(err) {
		t.Errorf("Expected JSON file to be created at %s", jsonPath)
	}

	// 2. Тест экспорта CSV
	csvPath := filepath.Join(tempDir, "test_metrics.csv")
	err = ExportMetricsCSV(csvPath, []MetricsSnapshot{snapshot})
	if err != nil {
		t.Fatalf("ExportMetricsCSV failed: %v", err)
	}

	if _, err := os.Stat(csvPath); os.IsNotExist(err) {
		t.Errorf("Expected CSV file to be created at %s", csvPath)
	}
}

func TestLogExporter(t *testing.T) {
	exp, err := NewLogExporter()
	if err != nil {
		t.Fatalf("Failed to create LogExporter: %v", err)
	}

	testMessage := "Test log entry for unit test"
	err = exp.AppendLog(testMessage)
	if err != nil {
		t.Errorf("AppendLog returned error: %v", err)
	}
}
