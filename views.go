package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m model) View() string {
	if m.width == 0 {
		return "Initializing TUI..."
	}

	if m.showConfirmationModal {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, m.renderModal())
	}

	if m.showModal {
		return m.renderModalView()
	}

	if m.showNodeMenu {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, m.renderNodeMenuModal())
	}

	if m.showDCSModal {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, m.renderDCSModal())
	}

	containerWidth := m.width - 4
	if containerWidth < 40 {
		containerWidth = 40
	}

	// --- Вычисление индикатора алертов для шапки ---
	var alertBadge string
	warnCount := 0
	critCount := 0

	for _, a := range m.activeAlerts {
		if a.Level == AlertLevelCrit {
			critCount++
		} else if a.Level == AlertLevelWarn {
			warnCount++
		}
	}

	if critCount > 0 {
		alertBadge = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#11111B")).
			Background(dangerColor).
			Padding(0, 1).
			Render(fmt.Sprintf("🚨 %d CRIT", critCount))
	} else if warnCount > 0 {
		alertBadge = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#11111B")).
			Background(warningColor).
			Padding(0, 1).
			Render(fmt.Sprintf("⚠️ %d WARN", warnCount))
	} else {
		alertBadge = lipgloss.NewStyle().
			Foreground(successColor).
			Render("🟢 OK")
	}

	header := headerStyle.Render(" Wunschpunsch ") + "  " +
		lipgloss.NewStyle().Foreground(primaryColor).Bold(true).Render("Engine: "+m.detectedEngine.String()) +
		statusLineStyle.Render(" | Target: "+m.pgFlavor) + "  " + alertBadge

	tabs := []string{"0: Connect", "1: Topology", "2: Metrics", "3: Analytics", "4: Actions", "5: Event Logs", "6: Settings"}
	var renderedTabs []string
	for i, t := range tabs {
		if activeTab(i) == m.tab {
			renderedTabs = append(renderedTabs, activeTabStyle.Render(t))
		} else {
			renderedTabs = append(renderedTabs, inactiveTabStyle.Render(t))
		}
	}

	tabRow := tabBorderStyle.Width(containerWidth + 2).Render(lipgloss.JoinHorizontal(lipgloss.Top, renderedTabs...))
	currentBoxStyle := boxStyle.Width(containerWidth)

	var body string

	if m.tab == tabConnect {
		var statusMsg string
		if m.connecting {
			statusMsg = lipgloss.NewStyle().Foreground(warningColor).Render("Detecting HA Engine / Standalone PG & Connecting...")
		} else if m.lastError != nil {
			errText := fmt.Sprintf("Connection Error: %v", m.lastError)
			statusMsg = lipgloss.NewStyle().
				Foreground(dangerColor).
				Width(containerWidth - 4).
				Render(errText)
		}

		form := lipgloss.JoinVertical(
			lipgloss.Left,
			lipgloss.NewStyle().Bold(true).Foreground(secondaryColor).Render("Connect to PostgreSQL Cluster or Single Database"),
			"\n\n",
			m.inputs[0].View(),
			"\n",
			m.inputs[1].View(),
			"\n",
			m.inputs[2].View(),
			"\n",
			statusMsg,
			"\n\n",
			lipgloss.NewStyle().Foreground(subtleColor).Render("Press [ENTER] to Connect  •  [Up/Down] Move Focus"),
		)
		body = currentBoxStyle.Render(form)
	} else {
		switch m.tab {
		case tabTopology:
			summary := badgeStandalone.Render("SINGLE NODE / STANDALONE MODE")
			if m.detectedEngine != EngineSingleNode {
				primaryNode := "Unknown"
				totalNodes := 0
				isPause := false

				if m.lastStatus != nil {
					totalNodes = len(m.lastStatus.Members)
					isPause = m.lastStatus.Pause
					for _, mem := range m.lastStatus.Members {
						if mem.Role == "leader" {
							primaryNode = mem.Name
							break
						}
					}
				}

				maintStr := lipgloss.NewStyle().Foreground(successColor).Render("Maintenance: OFF")
				if isPause {
					maintStr = lipgloss.NewStyle().Foreground(warningColor).Render("Maintenance: ON (Paused)")
				}

				summary = lipgloss.JoinHorizontal(
					lipgloss.Left,
					badgePrimary.Render("PRIMARY: "+primaryNode),
					"  ",
					badgeReplica.Render(fmt.Sprintf("NODES: %d", totalNodes)),
					"  ",
					maintStr,
				)
			}

			body = currentBoxStyle.Render(
				lipgloss.JoinVertical(
					lipgloss.Left,
					summary,
					"\n",
					m.table.View(),
				),
			)

		case tabEngineStats:
			if m.showMetricsConfig {
				body = m.renderMetricsConfigView(containerWidth)
			} else {
				body = currentBoxStyle.Render(
					lipgloss.JoinVertical(
						lipgloss.Left,
						lipgloss.NewStyle().Bold(true).Foreground(secondaryColor).Render("PostgreSQL Performance & HA Metrics"),
						"\n",
						m.renderMetricsTabContent(),
					),
				)
			}

		case tabAnalytics:
			body = currentBoxStyle.Render(m.renderAnalyticsTab())

		case tabActions:
			body = currentBoxStyle.Render(m.renderActionsView())

		case tabLogs:
			subTabs := []logSubTab{logSubTabAll, logSubTabPostgres, logSubTabHA, logSubTabSystem, logSubTabSSH}
			var renderedSubTabs []string

			for _, st := range subTabs {
				label := fmt.Sprintf(" %s ", st.String())
				if st == m.activeLogSubTab {
					renderedSubTabs = append(renderedSubTabs, lipgloss.NewStyle().
						Bold(true).
						Foreground(lipgloss.Color("#11111B")).
						Background(primaryColor).
						Render(label))
				} else {
					renderedSubTabs = append(renderedSubTabs, lipgloss.NewStyle().
						Foreground(subtleColor).
						Background(lipgloss.Color("#313244")).
						Render(label))
				}
			}

			subTabHeader := lipgloss.JoinHorizontal(lipgloss.Top, renderedSubTabs...)

			scrollStatus := lipgloss.NewStyle().Foreground(successColor).Render("[AUTOSCROLL: ON]")
			if !m.logAutoScroll {
				scrollStatus = lipgloss.NewStyle().Foreground(warningColor).Render("[PAUSED - 'a' to resume]")
			}

			var headerLine string
			if m.showLogSearch {
				headerLine = m.logSearchInput.View() + "  " + scrollStatus
			} else if m.logSearchQuery != "" {
				searchBadge := lipgloss.NewStyle().Foreground(warningColor).Render("🔍 Filter: " + m.logSearchQuery + " [/ to edit]")
				headerLine = lipgloss.JoinHorizontal(lipgloss.Left, subTabHeader, "  ", searchBadge, "  ", scrollStatus)
			} else {
				headerLine = lipgloss.JoinHorizontal(lipgloss.Left, subTabHeader, "  ", scrollStatus)
			}

			body = currentBoxStyle.Render(
				lipgloss.JoinVertical(
					lipgloss.Left,
					lipgloss.JoinHorizontal(lipgloss.Left,
						lipgloss.NewStyle().Bold(true).Foreground(secondaryColor).Render("Stream Logs:"),
						"  ",
						headerLine,
					),
					"\n",
					m.logsViewport.View(),
				),
			)

		case tabSettings:
			body = currentBoxStyle.Render(m.renderSettingsView())
		}
	}

	footerHint := "Tab/Arrow: Move / Switch Tab  •  e/E: Export JSON/CSV  •  Ctrl+C: Quit"
	if m.tab == tabEngineStats {
		if m.showMetricsConfig {
			footerHint = "Up/Down: Navigate  •  Space: Toggle Metric  •  Esc/Ctrl+D: Save"
		} else {
			footerHint = "Ctrl+D: Metrics Settings  •  e/E: Export JSON/CSV  •  Tab: Switch Tab  •  Ctrl+C: Quit"
		}
	} else if m.tab == tabAnalytics {
		footerHint = "Up/Down: Выбор PID  •  k: Cancel Backend  •  K: Terminate Backend  •  r: Обновить  •  Tab: Switch Tab"
	} else if m.tab == tabLogs {
		footerHint = "[ / ] Filter  •  / Search  •  Up/Down Scroll  •  'a' Resume Autoscroll  •  e/E Export  •  Ctrl+C Quit"
	} else if m.tab == tabSettings {
		footerHint = "Up/Down: Navigate  •  Left/Right/Space: Change Option  •  Esc/q: Back  •  Ctrl+C: Quit"
	}

	footer := statusLineStyle.Render(footerHint)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		tabRow,
		"\n",
		body,
		"\n",
		footer,
	)
}

func (m model) renderAnalyticsTab() string {
	s := strings.Builder{}

	if m.statusMsg != "" {
		s.WriteString(lipgloss.NewStyle().Foreground(warningColor).Render(m.statusMsg + "\n\n"))
	}

	if m.analyticsErr != "" {
		s.WriteString(lipgloss.NewStyle().Foreground(dangerColor).Render("⚠️  " + m.analyticsErr + "\n\n"))
	}

	s.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205")).Render("🔥 Top Slow Queries (pg_stat_statements)\n\n"))

	if len(m.topQueries) == 0 {
		s.WriteString(" Запросы не найдены или расширение pg_stat_statements не активировано.\n\n")
	} else {
		s.WriteString(fmt.Sprintf("%-60s | %-10s | %-12s | %-10s\n", "Query", "Calls", "Total Time", "Mean Time"))
		s.WriteString(strings.Repeat("-", 100) + "\n")
		for _, q := range m.topQueries {
			queryText := q.Query
			if len(queryText) > 58 {
				queryText = queryText[:55] + "..."
			}
			s.WriteString(fmt.Sprintf("%-60s | %-10d | %-10.2fms | %-8.2fms\n",
				queryText, q.Calls, q.TotalTime, q.MeanTime))
		}
		s.WriteString("\n")
	}

	s.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214")).Render("🔒 Active Locks & Hung Transactions (> 2s)\n\n"))

	if len(m.activeLocks) == 0 {
		s.WriteString(" 🟢 Нет зависших транзакций и блокировок.\n\n")
	} else {
		s.WriteString(fmt.Sprintf("%-8s | %-12s | %-10s | %-10s | %-40s\n", "PID", "User", "State", "Duration", "Query"))
		s.WriteString(strings.Repeat("-", 95) + "\n")
		for _, l := range m.activeLocks {
			prefix := "  "
			if l.PID == m.selectedPID {
				prefix = "👉"
			}
			qText := l.Query
			if len(qText) > 38 {
				qText = qText[:35] + "..."
			}
			s.WriteString(fmt.Sprintf("%s %-6d | %-12s | %-10s | %-8.1fs | %-40s\n",
				prefix, l.PID, l.User, l.State, l.DurationSec, qText))
		}
		s.WriteString("\n")
		s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("243")).Render(
			"Подсказка: [k] Мягкая отмена (pg_cancel_backend) | [K] Принудительное завершение (pg_terminate_backend) | [r] Обновить",
		))
	}

	return s.String()
}

func (m *model) renderActionsView() string {
	var s strings.Builder

	s.WriteString(lipgloss.NewStyle().Bold(true).Foreground(secondaryColor).Render("🔧 Maintenance & DBA Operations\n\n"))

	for i, action := range GetDefaultActions() {
		cursor := " "
		if i == m.actionsSelectedIndex {
			cursor = ">"
		}
		riskBadge := fmt.Sprintf("[%s]", action.Risk)
		s.WriteString(fmt.Sprintf(" %s %d. %-28s %-10s %s\n",
			cursor, i+1, action.Name, riskBadge, action.Description))
	}

	switch m.detectedEngine {
	case EnginePatroni:
		s.WriteString("\n" + strings.Repeat("-", 60) + "\n\n")
		s.WriteString(lipgloss.NewStyle().Bold(true).Foreground(dangerColor).Render("⚡ Cluster Management Actions (Patroni / HA)\n\n"))
		s.WriteString("  [S] Switchover (Graceful Leader Transfer)\n")
		s.WriteString("  [F] Failover (Force Leader Selection)\n")
		s.WriteString("  [R] Reinitialize Replica\n")
		s.WriteString("  [P] Pause/Resume Auto-failover (Maintenance Mode)\n")

	case EnginePacemaker:
		s.WriteString("\n" + strings.Repeat("-", 60) + "\n\n")
		s.WriteString(lipgloss.NewStyle().Bold(true).Foreground(warningColor).Render("⚡ Cluster Management Actions (Corosync / Pacemaker)\n\n"))
		s.WriteString("  [M] Move Resource (pcs resource move)\n")
		s.WriteString("  [C] Cleanup Resource (pcs resource cleanup)\n")
		s.WriteString("  [S] Standby Node (pcs node standby)\n")
	}

	return s.String()
}

func (m *model) renderStandaloneActions() string {
	var s string
	s += "  🔧 Maintenance & DBA Operations (Single Node)\n\n"

	for i, action := range GetDefaultActions() {
		cursor := " "
		if i == m.actionsSelectedIndex {
			cursor = ">"
		}

		riskBadge := fmt.Sprintf("[%s]", action.Risk)
		s += fmt.Sprintf(" %s %d. %-28s %-10s %s\n",
			cursor, i+1, action.Name, riskBadge, action.Description)
	}

	return s
}

func (m *model) renderClusterActions() string {
	s := "  ⚡ Cluster Management Actions (Patroni / HA)\n\n"
	s += "  [S] Switchover (Graceful Leader Transfer)\n"
	s += "  [F] Failover (Force Leader Selection)\n"
	s += "  [R] Reinitialize Replica\n"
	s += "  [P] Pause/Resume Auto-failover (Maintenance Mode)\n"
	return s
}

func (m *model) renderModal() string {
	actions := GetDefaultActions()
	if m.actionsSelectedIndex < 0 || m.actionsSelectedIndex >= len(actions) {
		return ""
	}
	action := actions[m.actionsSelectedIndex]

	title := lipgloss.NewStyle().Bold(true).Foreground(warningColor).Render("⚠️  ПОДТВЕРЖДЕНИЕ ОПЕРАЦИИ DBA")
	info := fmt.Sprintf("Действие: %s\nЗапрос:    %s\nРиск:      [%s]\n\n%s",
		action.Name, action.Query, action.Risk, action.Description)

	buttons := lipgloss.NewStyle().Foreground(primaryColor).Bold(true).Render("[Y / Enter] Да, выполнить   •   [N / Esc] Отмена")

	modalContent := lipgloss.JoinVertical(lipgloss.Center, title, "\n", info, "\n\n", buttons)

	return lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(warningColor).
		Padding(1, 4).
		Align(lipgloss.Center).
		Render(modalContent)
}

func (m model) renderNodeMenuModal() string {
	if !m.showNodeMenu {
		return ""
	}

	content := fmt.Sprintf(
		"🔧  Управление узлом: %s\n\n"+
			" [R] 🔄 Перезапустить узел (Restart)\n"+
			" [Esc] ❌ Отмена",
		m.selectedNode,
	)

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("63")).
		Padding(1, 3).
		Align(lipgloss.Center).
		Render(content)
}

func (m model) renderDCSModal() string {
	if !m.showDCSModal {
		return ""
	}

	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205")).Render("⚙️  Patroni Dynamic Configuration (DCS)\n\n")

	body := m.dcsConfigText
	if body == "" {
		body = "Загрузка конфигурации..."
	}

	return lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(lipgloss.Color("212")).
		Padding(1, 2).
		Width(70).
		Render(title + body + "\n\n[Esc] Закрыть")
}

func (m model) renderModalView() string {
	var icon, title string
	var borderCol lipgloss.Color
	var btnStyle lipgloss.Style

	if m.modalIsErr {
		borderCol = dangerColor
		icon = lipgloss.NewStyle().Foreground(dangerColor).Bold(true).Render(`
 ███████╗██████╗ ██╗██╗
 ██╔════╝██╔══██╗██║██║
 █████╗  ██████╔╝██║██║
 ██╔══╝  ██╔══██╗██║██║
 ██║     ██║  ██║██║███████╗
 ╚═╝     ╚═╝  ╚═╝╚═╝╚══════╝`)
		title = lipgloss.NewStyle().Bold(true).Foreground(dangerColor).Render("Ошибка экспорта метрик")
		btnStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#11111B")).Background(dangerColor)
	} else {
		borderCol = successColor
		icon = lipgloss.NewStyle().Foreground(successColor).Bold(true).Render(`
 ██████╗ ██╗  ██╗
██╔═══██╗██║ ██╔╝
██║   ██║█████═╝ 
██║   ██║██╔═██╗ 
╚██████╔╝██║  ██╗
 ╚═════╝ ╚═╝  ╚═╝`)
		title = lipgloss.NewStyle().Bold(true).Foreground(successColor).Render("Метрики успешно экспортированы!")
		btnStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#11111B")).Background(successColor)
	}

	pathLabel := lipgloss.NewStyle().Foreground(subtleColor).Render("Файл сохранен по пути:")
	pathValue := lipgloss.NewStyle().Foreground(secondaryColor).Bold(true).Render(m.modalMsg)
	okButton := btnStyle.Padding(0, 3).Render(" [ OK ] (Enter) ")

	content := lipgloss.JoinVertical(
		lipgloss.Center,
		icon,
		"",
		title,
		"",
		pathLabel,
		pathValue,
		"\n",
		okButton,
	)

	modalBox := lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(borderCol).
		Padding(1, 5).
		Align(lipgloss.Center).
		Render(content)

	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		modalBox,
	)
}

func (m model) renderSettingsView() string {
	var lines []string
	isRU := m.settings.Language == LangRU

	title := "Глобальные настройки (Global Settings)"
	if !isRU {
		title = "Global App Settings"
	}
	lines = append(lines, lipgloss.NewStyle().Bold(true).Foreground(secondaryColor).Render(title))
	lines = append(lines, "")

	langVal := "Русский (RU)"
	if m.settings.Language == LangEN {
		langVal = "English (EN)"
	}
	langLine := fmt.Sprintf("  Язык интерфейса / Language : < %s >", langVal)
	themeLine := fmt.Sprintf("  Цветовая схема / Theme     : < %s >", m.settings.Theme.String())
	pollLine := fmt.Sprintf("  Интервал опроса / Poll Rate : < %d sec >", m.settings.PollIntervalSec)

	options := []string{langLine, themeLine, pollLine}

	for i, opt := range options {
		cursor := " "
		if i == m.settingsCursor {
			cursor = ">"
			opt = lipgloss.NewStyle().Foreground(primaryColor).Bold(true).Render(cursor + opt)
		} else {
			opt = lipgloss.NewStyle().Foreground(subtleColor).Render(cursor + opt)
		}
		lines = append(lines, opt)
	}

	lines = append(lines, "")
	lines = append(lines, lipgloss.NewStyle().Bold(true).Foreground(secondaryColor).Render("Настройки подключения для логов (SSH / Localhost):"))
	lines = append(lines, "")

	// Отрисовка текстовых инпутов с отображением фокуса и мигающего курсора
	for i, input := range m.settingsInputs {
		cursor := " "
		lineIdx := i + 3
		var lineStr string
		if lineIdx == m.settingsCursor {
			cursor = "> "
			lineStr = lipgloss.NewStyle().Foreground(primaryColor).Bold(true).Render(cursor) + input.View()
		} else {
			lineStr = lipgloss.NewStyle().Foreground(subtleColor).Render(cursor) + input.View()
		}
		lines = append(lines, lineStr)
	}

	lines = append(lines, "")
	hint := "Up/Down: Выбор поля  •  Буквы/Цифры/Стрелки: Редактирование текста  •  Enter: Сохранить"
	if !isRU {
		hint = "Up/Down: Select Field  •  Type to edit  •  Enter: Save"
	}
	lines = append(lines, lipgloss.NewStyle().Foreground(subtleColor).Render(hint))

	return strings.Join(lines, "\n")
}

func (m *model) renderMetricsConfigView(containerWidth int) string {
	var lines []string
	lines = append(lines, lipgloss.NewStyle().Bold(true).Foreground(secondaryColor).Render("Настройка отображаемых метрик (Space — вкл/выкл, Esc — выход)"))
	lines = append(lines, "")

	for i, item := range m.availableMetrics {
		cursor := " "
		if i == m.metricsCursor {
			cursor = ">"
		}

		checked := "[ ]"
		if item.Enabled {
			checked = "[x]"
		}

		line := fmt.Sprintf("%s %s %-22s | %-15s | %s", cursor, checked, item.Name, item.Category, item.Description)
		if i == m.metricsCursor {
			line = lipgloss.NewStyle().Foreground(primaryColor).Bold(true).Render(line)
		} else {
			line = lipgloss.NewStyle().Foreground(subtleColor).Render(line)
		}
		lines = append(lines, line)
	}

	return boxStyle.Width(containerWidth).Render(strings.Join(lines, "\n"))
}

func (m model) renderMetricsTabContent() string {
	var lines []string

	hostMetrics, _ := FetchHostMetrics(m.hostCPUHistory)

	lines = append(lines, lipgloss.NewStyle().Bold(true).Foreground(primaryColor).Render("Host System Metrics ("+m.targetHost+")"))
	lines = append(lines, "----------------------------------------------------------------")

	for _, item := range m.availableMetrics {
		if !item.Enabled {
			continue
		}
		switch item.Name {
		case "CPU Load (%)":
			chart := drawSparkline(hostMetrics.CPUHistory, 100)
			bar := drawProgressBar(hostMetrics.CPUUsage, 25)
			lines = append(lines, fmt.Sprintf("  • CPU Usage:  %s  Trend: [%-20s]", bar, chart))

		case "RAM Usage (%)":
			bar := drawProgressBar(hostMetrics.RAMUsage, 25)
			ramDetails := fmt.Sprintf("(%.1f / %.1f GB)", hostMetrics.RAMUsedGB, hostMetrics.RAMTotalGB)
			lines = append(lines, fmt.Sprintf("  • RAM Usage:  %s  %s", bar, lipgloss.NewStyle().Foreground(subtleColor).Render(ramDetails)))

		case "Disk Usage (%)":
			bar := drawProgressBar(hostMetrics.DiskUsage, 25)
			diskDetails := fmt.Sprintf("(%.1f / %.1f GB)", hostMetrics.DiskUsedGB, hostMetrics.DiskTotalGB)
			lines = append(lines, fmt.Sprintf("  • Disk Usage: %s  %s", bar, lipgloss.NewStyle().Foreground(subtleColor).Render(diskDetails)))
		}
	}

	lines = append(lines, "")
	lines = append(lines, lipgloss.NewStyle().Bold(true).Foreground(primaryColor).Render("PostgreSQL Instance Metrics"))
	lines = append(lines, "----------------------------------------------------------------")

	if len(m.lastMetrics) == 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(subtleColor).Render("  Метрики БД не собраны или узел недоступен."))
	} else {
		for host, metrics := range m.lastMetrics {
			if metrics.Error != nil {
				lines = append(lines, lipgloss.NewStyle().Foreground(dangerColor).Render(fmt.Sprintf("  [%s] Ошибка: %v", host, metrics.Error)))
			} else {
				for _, item := range m.availableMetrics {
					if !item.Enabled {
						continue
					}
					switch item.Name {
					case "Active Connections":
						pct := (float64(metrics.ActiveConnections) / float64(metrics.MaxConnections)) * 100
						bar := drawProgressBar(pct, 20)
						lines = append(lines, fmt.Sprintf("  • Active Conns: %d / %d  %s", metrics.ActiveConnections, metrics.MaxConnections, bar))

					case "Cache Hit Ratio":
						bar := drawProgressBar(metrics.CacheHitRatio, 20)
						lines = append(lines, fmt.Sprintf("  • Cache Hit:    %s", bar))
					}
				}
			}
		}
	}

	return strings.Join(lines, "\n")
}
