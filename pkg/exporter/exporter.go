package exporter

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// LogExporter отвечает за дамп и автосохранение логов
type LogExporter struct {
	logFilePath string
}

func NewLogExporter() (*LogExporter, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home dir: %w", err)
	}

	dir := filepath.Join(homeDir, ".wunschpunsch", "logs")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log dir: %w", err)
	}

	return &LogExporter{
		logFilePath: filepath.Join(dir, "app.log"),
	}, nil
}

// AppendLog записывает новую строку лога в файл ~/.wunschpunsch/logs/app.log
func (e *LogExporter) AppendLog(line string) error {
	f, err := os.OpenFile(e.logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	_, err = f.WriteString(fmt.Sprintf("[%s] %s\n", timestamp, line))
	return err
}

// MetricsSnapshot структрура snapshot метрик для экспорта
type MetricsSnapshot struct {
	Timestamp     time.Time `json:"timestamp"`
	ActiveConns   int       `json:"active_connections"`
	IdleConns     int       `json:"idle_connections"`
	CacheHitRatio float64   `json:"cache_hit_ratio"`
	TPS           float64   `json:"tps"`
}

// ExportMetricsJSON экспортирует метрики в JSON файл
func ExportMetricsJSON(filename string, metrics MetricsSnapshot) error {
	data, err := json.MarshalIndent(metrics, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filename, data, 0644)
}

// ExportMetricsCSV экспортирует метрики в CSV файл
func ExportMetricsCSV(filename string, metrics []MetricsSnapshot) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Заголовок
	_ = writer.Write([]string{"Timestamp", "Active Connections", "Idle Connections", "Cache Hit Ratio (%)", "TPS"})

	for _, m := range metrics {
		record := []string{
			m.Timestamp.Format(time.RFC3339),
			fmt.Sprintf("%d", m.ActiveConns),
			fmt.Sprintf("%d", m.IdleConns),
			fmt.Sprintf("%.2f", m.CacheHitRatio),
			fmt.Sprintf("%.2f", m.TPS),
		}
		if err := writer.Write(record); err != nil {
			return err
		}
	}
	return nil
}
