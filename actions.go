package main

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type RiskLevel string

const (
	RiskLow    RiskLevel = "LOW"
	RiskMedium RiskLevel = "MEDIUM"
	RiskHigh   RiskLevel = "HIGH"
)

type ActionItem struct {
	ID          string
	Name        string
	Description string
	Query       string
	Risk        RiskLevel
}

func GetDefaultActions() []ActionItem {
	return []ActionItem{
		{
			ID:          "checkpoint",
			Name:        "Force CHECKPOINT",
			Description: "Принудительный сброс грязных страниц из shared_buffers на диск",
			Query:       "CHECKPOINT;",
			Risk:        RiskLow,
		},
		{
			ID:          "analyze",
			Name:        "ANALYZE Database",
			Description: "Сбор свежей статистики для планировщика запросов",
			Query:       "ANALYZE;",
			Risk:        RiskLow,
		},
		{
			ID:          "vacuum_analyze",
			Name:        "VACUUM ANALYZE",
			Description: "Очистка мертвых строк и обновление статистики",
			Query:       "VACUUM (ANALYZE);",
			Risk:        RiskMedium,
		},
		{
			ID:          "terminate_idle",
			Name:        "Kill Idle Sessions (>5m)",
			Description: "Принудительно закрыть соединения в состоянии idle > 5 минут",
			Query:       "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE state = 'idle' AND state_change < clock_timestamp() - interval '5 minutes' AND pid <> pg_backend_pid();",
			Risk:        RiskMedium,
		},
		{
			ID:          "reset_stats",
			Name:        "Reset Query Statistics",
			Description: "Сбросить счетчики pg_stat_statements",
			Query:       "SELECT pg_stat_statements_reset();",
			Risk:        RiskMedium,
		},
		{
			ID:          "vacuum_full",
			Name:        "VACUUM FULL (Blocking!)",
			Description: "Полная перезапись таблиц для возврата места ОС. БЛОКИРУЕТ ТАБЛИЦЫ!",
			Query:       "VACUUM FULL;",
			Risk:        RiskHigh,
		},
	}
}

func ExecuteAction(ctx context.Context, db *sql.DB, action ActionItem) (string, error) {
	start := time.Now()
	_, err := db.ExecContext(ctx, action.Query)
	duration := time.Since(start)

	if err != nil {
		return "", fmt.Errorf("ошибка выполнения [%s]: %w", action.Name, err)
	}
	return fmt.Sprintf("[%s] успешно выполнено за %s", action.Name, duration.Round(time.Millisecond)), nil
}

// --- TUI State Management for Actions ---

type ActionsModel struct {
	items         []ActionItem
	selectedIndex int
	showModal     bool
	executing     bool
	lastResult    string
	lastErr       error
}

func NewActionsModel() ActionsModel {
	return ActionsModel{
		items: GetDefaultActions(),
	}
}

func (m *ActionsModel) HandleKey(key string, db *sql.DB) {
	if m.showModal {
		switch key {
		case "y", "Y", "enter":
			m.showModal = false
			m.executing = true

			selected := m.items[m.selectedIndex]
			go func() {
				res, err := ExecuteAction(context.Background(), db, selected)
				m.executing = false
				if err != nil {
					m.lastErr = err
					m.lastResult = ""
				} else {
					m.lastResult = res
					m.lastErr = nil
				}
			}()

		case "n", "N", "esc":
			m.showModal = false
		}
		return
	}

	switch key {
	case "up", "k":
		if m.selectedIndex > 0 {
			m.selectedIndex--
		}
	case "down", "j":
		if m.selectedIndex < len(m.items)-1 {
			m.selectedIndex++
		}
	case "enter":
		m.showModal = true
	}
}
