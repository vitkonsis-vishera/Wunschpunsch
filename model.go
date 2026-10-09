package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"wunschpunsch/pkg/config"
	"wunschpunsch/pkg/exporter"
	"wunschpunsch/pkg/patroni"
	"wunschpunsch/pkg/postgres"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type model struct {
	tab               activeTab
	activeLogSubTab   logSubTab
	table             table.Model
	logsViewport      viewport.Model
	detectedEngine    engineType
	pgFlavor          string
	targetHost        string
	targetPort        int
	width             int
	height            int
	availableMetrics  []MetricItem
	metricsCursor     int
	showMetricsConfig bool
	hostCPUHistory    []float64
	activeAlerts      []AlertItem

	// Settings state
	settings       appSettings
	settingsCursor int
	settingsInputs []textinput.Model

	// Form Inputs (Connect)
	inputs     []textinput.Model
	focusIndex int
	connecting bool
	connected  bool

	patroniClient *patroni.Client
	pgManager     *postgres.PGPoolManager
	logExporter   *exporter.LogExporter

	// Поиск и управление логированием
	logSearchInput textinput.Model
	logSearchQuery string
	showLogSearch  bool
	logAutoScroll  bool

	lastStatus  *patroni.ClusterStatus
	lastMetrics map[string]postgres.NodeMetrics
	lastError   error

	showModal  bool
	modalMsg   string
	modalIsErr bool
	logs       []LogEntry

	showNodeMenu    bool
	selectedNode    string
	dcsConfigText   string
	showDCSModal    bool
	actionStatusMsg string

	// Поля для аналитики и блокировок
	topQueries   []postgres.QueryStat
	activeLocks  []postgres.LockInfo
	selectedPID  int
	analyticsErr string
	statusMsg    string

	actionsSelectedIndex  int
	showConfirmationModal bool
}

func initialModel() model {
	inputs := make([]textinput.Model, 3)
	inputs[0] = textinput.New()
	inputs[0].Placeholder = "localhost:5432 или http://127.0.0.1:8008"
	inputs[0].Focus()
	inputs[0].CharLimit = 128
	inputs[0].Width = 60
	inputs[0].Prompt = "Endpoint / Host: "

	inputs[1] = textinput.New()
	inputs[1].Placeholder = "postgres"
	inputs[1].CharLimit = 64
	inputs[1].Width = 60
	inputs[1].Prompt = "PG User:         "

	inputs[2] = textinput.New()
	inputs[2].Placeholder = "password"
	inputs[2].EchoMode = textinput.EchoPassword
	inputs[2].EchoCharacter = '•'
	inputs[2].CharLimit = 64
	inputs[2].Width = 60
	inputs[2].Prompt = "PG Password:     "

	columns := []table.Column{
		{Title: "Node Name", Width: 16},
		{Title: "Host", Width: 14},
		{Title: "Role", Width: 14},
		{Title: "State", Width: 12},
		{Title: "TL", Width: 5},
		{Title: "Lag (MB)", Width: 10},
		{Title: "Conns", Width: 10},
		{Title: "Cache Hit", Width: 10},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows([]table.Row{}),
		table.WithFocused(true),
		table.WithHeight(7),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#45475A")).
		BorderBottom(true).
		Bold(true)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("#11111B")).
		Background(primaryColor).
		Bold(false)
	t.SetStyles(s)

	vp := viewport.New(80, 15)

	logExp, _ := exporter.NewLogExporter()

	searchInput := textinput.New()
	searchInput.Placeholder = "Введите ключевое слово..."
	searchInput.Prompt = "🔍 Поиск: "
	searchInput.CharLimit = 64
	searchInput.Width = 40

	savedCfg := config.LoadConfig()
	initialSettings := appSettings{
		Language:        language(savedCfg.Language),
		Theme:           themeType(savedCfg.Theme),
		PollIntervalSec: savedCfg.PollIntervalSec,
		SSHUser:         savedCfg.SSHUser,
		SSHPort:         savedCfg.SSHPort,
		SSHKeyPath:      savedCfg.SSHKeyPath,
	}
	if initialSettings.SSHUser == "" {
		initialSettings.SSHUser = "postgres"
	}
	if initialSettings.SSHPort <= 0 {
		initialSettings.SSHPort = 22
	}

	// Инициализация текстовых инпутов для экрана настроек с поддержкой курсора
	sInputs := make([]textinput.Model, 4)
	sInputs[0] = textinput.New()
	sInputs[0].SetValue(initialSettings.SSHUser)
	sInputs[0].Prompt = "SSH User: "
	sInputs[0].Width = 30

	sInputs[1] = textinput.New()
	sInputs[1].SetValue(strconv.Itoa(initialSettings.SSHPort))
	sInputs[1].Prompt = "SSH Port: "
	sInputs[1].Width = 10

	sInputs[2] = textinput.New()
	sInputs[2].SetValue(initialSettings.SSHKeyPath)
	sInputs[2].Placeholder = "~/.ssh/id_rsa (опционально)"
	sInputs[2].Prompt = "SSH Key:  "
	sInputs[2].Width = 40

	sInputs[3] = textinput.New()
	sInputs[3].SetValue(initialSettings.SSHPassword)
	sInputs[3].Placeholder = "пароль (опционально)"
	sInputs[3].EchoMode = textinput.EchoPassword
	sInputs[3].EchoCharacter = '•'
	sInputs[3].Prompt = "SSH Pass: "
	sInputs[3].Width = 30

	m := model{
		tab:              tabConnect,
		activeLogSubTab:  logSubTabAll,
		table:            t,
		logsViewport:     vp,
		inputs:           inputs,
		settingsInputs:   sInputs,
		focusIndex:       0,
		detectedEngine:   EngineAutoDetect,
		pgFlavor:         "PostgreSQL / Postgres Pro",
		lastMetrics:      make(map[string]postgres.NodeMetrics),
		logs:             []LogEntry{},
		availableMetrics: defaultMetrics(),
		settings:         initialSettings,
		logExporter:      logExp,
		logSearchInput:   searchInput,
		logAutoScroll:    true,
		activeAlerts:     []AlertItem{},
	}

	m.applyTheme(initialSettings.Theme)

	return m
}

func (m *model) evaluateAlerts() {
	var alerts []AlertItem

	if m.lastStatus != nil {
		for _, member := range m.lastStatus.Members {
			if strings.Contains(strings.ToUpper(member.State), "UNREACHABLE") || strings.Contains(strings.ToUpper(member.State), "DOWN") {
				alerts = append(alerts, AlertItem{
					Node:    member.Name,
					Level:   AlertLevelCrit,
					Message: fmt.Sprintf("Узел %s недоступен!", member.Name),
				})
			}

			if member.Role != "leader" && member.Lag > 100*1024*1024 {
				lagMb := member.Lag / (1024 * 1024)
				alerts = append(alerts, AlertItem{
					Node:    member.Name,
					Level:   AlertLevelWarn,
					Message: fmt.Sprintf("Высокое отставание репликации на %s: %d MB", member.Name, lagMb),
				})
			}
		}
	}

	if len(m.hostCPUHistory) > 0 {
		lastCPU := m.hostCPUHistory[len(m.hostCPUHistory)-1]
		if lastCPU > 85.0 {
			alerts = append(alerts, AlertItem{
				Node:    m.targetHost,
				Level:   AlertLevelWarn,
				Message: fmt.Sprintf("Высокая загрузка CPU: %.1f%%", lastCPU),
			})
		}
	}

	for _, alert := range alerts {
		if alert.Level == AlertLevelCrit || alert.Level == AlertLevelWarn {
			levelStr := "WARN"
			if alert.Level == AlertLevelCrit {
				levelStr = "ERROR"
			}
			if !m.isLogDuplicate(alert.Node, "AlertSystem", alert.Message) {
				m.logs = append(m.logs, LogEntry{
					Timestamp: time.Now().Format("15:04:05"),
					Node:      alert.Node,
					Component: "AlertSystem",
					Level:     levelStr,
					Message:   alert.Message,
				})
			}
		}
	}

	m.activeAlerts = alerts
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.table.SetWidth(msg.Width - 6)
		vpHeight := msg.Height - 10
		if vpHeight < 5 {
			vpHeight = 5
		}
		m.logsViewport.Width = msg.Width - 8
		m.logsViewport.Height = vpHeight

	case tickMsg:
		if m.connected || m.pgManager != nil {
			cmds := []tea.Cmd{m.fetchClusterDataCmd(), tickCmd(m.settings.PollIntervalSec)}
			if m.connected {
				if m.tab == tabLogs {
					cmds = append(cmds, m.fetchLogsCmd())
					if m.targetHost != "" {
						targetSvc := "patroni"
						if m.detectedEngine == EnginePacemaker {
							targetSvc = "pacemaker"
						} else if m.detectedEngine == EngineSingleNode {
							targetSvc = "postgresql"
						}
						cmds = append(cmds, m.fetchSSHLogsCmd(m.targetHost, targetSvc))
					}
				}
				if m.tab == tabAnalytics {
					cmds = append(cmds, m.fetchTopQueriesCmd(m.targetHost), m.fetchActiveLocksCmd(m.targetHost))
				}
			}
			return m, tea.Batch(cmds...)
		}

	case dcsLoadedMsg:
		if msg.err != nil {
			m.dcsConfigText = fmt.Sprintf("Ошибка загрузки DCS: %v", msg.err)
		} else {
			m.dcsConfigText = msg.config
		}

	case nodeActionResultMsg:
		if msg.err != nil {
			m.actionStatusMsg = fmt.Sprintf("❌ Ошибка %s для %s: %v", msg.action, msg.node, msg.err)
		} else {
			m.actionStatusMsg = fmt.Sprintf("✅ %s успешно выполнен для %s", msg.action, msg.node)
			m.showNodeMenu = false
		}

	case topQueriesLoadedMsg:
		if msg.err != nil {
			m.analyticsErr = fmt.Sprintf("Ошибка pg_stat_statements: %v", msg.err)
		} else {
			m.topQueries = msg.stats
		}

	case locksLoadedMsg:
		if msg.err != nil {
			m.analyticsErr = fmt.Sprintf("Ошибка получения блокировок: %v", msg.err)
		} else {
			m.activeLocks = msg.locks
			if len(m.activeLocks) > 0 {
				m.selectedPID = m.activeLocks[0].PID
			} else {
				m.selectedPID = 0
			}
		}

	case terminateResultMsg:
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("❌ Ошибка завершения PID %d: %v", msg.pid, msg.err)
		} else {
			action := "остановлен (cancel)"
			if msg.force {
				action = "убит (terminate)"
			}
			m.statusMsg = fmt.Sprintf("✅ Процесс %d успешно %s", msg.pid, action)
			return m, m.fetchActiveLocksCmd(m.targetHost)
		}

	case logsUpdateMsg:
		if msg.err == nil && len(msg.logs) > 0 {
			m.logs = append(m.logs, msg.logs...)
			if m.logExporter != nil {
				for _, l := range msg.logs {
					_ = m.logExporter.AppendLog(fmt.Sprintf("[%s] [%s] [%s] %s", l.Node, l.Component, l.Level, l.Message))
				}
			}
			m.updateLogsViewport()
		}

	case sshLogsLoadedMsg:
		if msg.err == nil && len(msg.logs) > 0 {
			for _, entry := range msg.logs {
				if !m.isLogDuplicate(entry.Node, entry.Component, entry.Message) {
					m.logs = append(m.logs, entry)
				}
			}
			m.updateLogsViewport()
		}

	case actionResultMsg:
		entry := LogEntry{
			Timestamp: time.Now().Format("15:04:05"),
			Node:      "TUI Console",
			Component: "HA Engine",
			Level:     "WARN",
			Message:   msg.message,
		}
		if msg.err != nil {
			entry.Level = "ERROR"
			entry.Message = fmt.Sprintf("Action failed: %v", msg.err)
		}
		m.logs = append(m.logs, entry)
		if m.logExporter != nil {
			_ = m.logExporter.AppendLog(fmt.Sprintf("[%s] [%s] [%s] %s", entry.Node, entry.Component, entry.Level, entry.Message))
		}
		m.updateLogsViewport()

	case exportFinishedMsg:
		entry := LogEntry{
			Timestamp: time.Now().Format("15:04:05"),
			Node:      "TUI Console",
			Component: "System",
			Level:     "INFO",
			Message:   fmt.Sprintf("Metrics snapshot saved to: %s", msg.filePath),
		}

		if msg.err != nil {
			entry.Level = "ERROR"
			entry.Message = fmt.Sprintf("Metrics export failed: %v", msg.err)
			m.modalMsg = msg.err.Error()
			m.modalIsErr = true
		} else {
			m.modalMsg = msg.filePath
			m.modalIsErr = false
		}

		m.showModal = true

		m.logs = append(m.logs, entry)
		if m.logExporter != nil {
			_ = m.logExporter.AppendLog(fmt.Sprintf("[%s] [%s] [%s] %s", entry.Node, entry.Component, entry.Level, entry.Message))
		}
		m.updateLogsViewport()
		return m, nil

	case clusterUpdateMsg:
		m.connecting = false
		if msg.err != nil {
			m.lastError = msg.err
			m.connected = false
			m.evaluateAlerts()
			return m, nil
		}

		m.lastError = nil
		m.connected = true
		m.detectedEngine = msg.detectedEngine
		m.lastStatus = msg.status
		m.lastMetrics = msg.metrics

		if m.tab == tabConnect {
			m.tab = tabTopology
		}

		rows := []table.Row{}
		for _, member := range msg.status.Members {
			metrics, hasMetrics := msg.metrics[member.Host]

			stateStr := strings.ToUpper(member.State)
			if stateStr == "" {
				stateStr = "UNKNOWN"
			}

			connsStr := "N/A"
			cacheStr := "N/A"

			if hasMetrics {
				if metrics.Error != nil {
					if stateStr == "RUNNING" {
						stateStr = "UNREACHABLE (PG)"
					}
					connsStr = "DOWN"
					cacheStr = "DOWN"
				} else {
					connsStr = fmt.Sprintf("%d/%d", metrics.ActiveConnections, metrics.MaxConnections)
					cacheStr = fmt.Sprintf("%.1f%%", metrics.CacheHitRatio)
				}
			} else {
				stateStr = "TIMEOUT"
			}

			lagMb := "-"
			if member.Lag >= 0 {
				lagMb = fmt.Sprintf("%d MB", member.Lag/(1024*1024))
			}

			tlStr := fmt.Sprintf("%d", member.Timeline)
			if member.Timeline == 0 {
				tlStr = "-"
			}

			rows = append(rows, table.Row{
				member.Name,
				member.Host,
				member.Role,
				stateStr,
				tlStr,
				lagMb,
				connsStr,
				cacheStr,
			})
		}
		m.table.SetRows(rows)
		m.evaluateAlerts()

	case tea.KeyMsg:
		if m.showConfirmationModal {
			switch strings.ToLower(msg.String()) {
			case "y", "enter":
				m.showConfirmationModal = false
				actions := GetDefaultActions()
				if m.actionsSelectedIndex >= 0 && m.actionsSelectedIndex < len(actions) {
					selectedAction := actions[m.actionsSelectedIndex]
					return m, m.executeDBActionCmd(selectedAction)
				}
			case "n", "esc":
				m.showConfirmationModal = false
				return m, nil
			}
			return m, nil
		}

		if m.showNodeMenu {
			switch strings.ToLower(msg.String()) {
			case "r":
				return m, m.restartNodeCmd(m.selectedNode)
			case "esc":
				m.showNodeMenu = false
				return m, nil
			}
			return m, nil
		}

		if m.showDCSModal {
			if msg.String() == "esc" {
				m.showDCSModal = false
				return m, nil
			}
			return m, nil
		}

		if m.showMetricsConfig {
			switch msg.String() {
			case "esc", "ctrl+d", "q":
				m.showMetricsConfig = false
				return m, nil
			case "up", "k":
				if m.metricsCursor > 0 {
					m.metricsCursor--
				}
			case "down", "j":
				if m.metricsCursor < len(m.availableMetrics)-1 {
					m.metricsCursor++
				}
			case " ", "enter":
				m.availableMetrics[m.metricsCursor].Enabled = !m.availableMetrics[m.metricsCursor].Enabled
			}
			return m, nil
		}

		if m.showModal {
			switch msg.String() {
			case "enter", "esc", "space", "q":
				m.showModal = false
				return m, nil
			}
			return m, nil
		}

		if m.showLogSearch {
			switch msg.String() {
			case "esc", "enter":
				m.showLogSearch = false
				m.logSearchInput.Blur()
				return m, nil
			}
			var cmd tea.Cmd
			m.logSearchInput, cmd = m.logSearchInput.Update(msg)
			m.logSearchQuery = m.logSearchInput.Value()
			m.updateLogsViewport()
			return m, cmd
		}

		if msg.String() == "ctrl+c" {
			if m.pgManager != nil {
				m.pgManager.Close()
			}
			return m, tea.Quit
		}

		if msg.String() == "q" || msg.String() == "esc" {
			if m.tab == tabLogs || m.tab == tabSettings || m.tab == tabAnalytics {
				m.tab = tabTopology
				return m, nil
			}
			if m.tab != tabConnect {
				if m.pgManager != nil {
					m.pgManager.Close()
				}
				return m, tea.Quit
			}
		}

		// Логика переключения и ввода в окне Settings
		if m.tab == tabSettings {
			switch msg.String() {
			case "up":
				if m.settingsCursor > 0 {
					m.settingsCursor--
					m.updateSettingsFocus()
				}
				return m, nil
			case "down":
				if m.settingsCursor < 6 { // Максимальный индекс теперь 6
					m.settingsCursor++
					m.updateSettingsFocus()
				}
				return m, nil
			case "left":
				if m.settingsCursor <= 2 {
					m.adjustSetting(-1)
					return m, nil
				}
			case "right":
				if m.settingsCursor <= 2 {
					m.adjustSetting(1)
					return m, nil
				}
			case "enter":
				m.saveSettingsFromInputs()
			}

			// Если курсор на инпутах SSH (3..6)
			if m.settingsCursor >= 3 && m.settingsCursor <= 6 {
				idx := m.settingsCursor - 3
				var cmd tea.Cmd
				m.settingsInputs[idx], cmd = m.settingsInputs[idx].Update(msg)
				m.saveSettingsFromInputs()
				return m, cmd
			}
		}

		if m.tab == tabLogs && m.connected {
			switch msg.String() {
			case "/":
				m.showLogSearch = true
				return m, m.logSearchInput.Focus()
			case "a":
				m.logAutoScroll = true
				m.logsViewport.GotoBottom()
				return m, nil
			case "up", "k", "pgup":
				m.logAutoScroll = false
			case "down", "j", "pgdown":
				if m.logsViewport.AtBottom() {
					m.logAutoScroll = true
				}
			case "[", "h", "left":
				if m.activeLogSubTab > logSubTabAll {
					m.activeLogSubTab--
					m.updateLogsViewport()
					return m, nil
				}
			case "]", "l", "right":
				if m.activeLogSubTab < logSubTabSSH {
					m.activeLogSubTab++
					m.updateLogsViewport()
					return m, nil
				}
			}
		}

		if m.tab == tabEngineStats && msg.String() == "ctrl+d" {
			m.showMetricsConfig = true
			return m, nil
		}

		if m.tab == tabTopology && m.connected {
			switch msg.String() {
			case "enter":
				if len(m.table.Rows()) > 0 {
					selectedRow := m.table.SelectedRow()
					if len(selectedRow) > 0 {
						m.selectedNode = selectedRow[0]
						m.showNodeMenu = true
					}
				}
			case "c":
				m.showDCSModal = true
				return m, m.fetchDCSConfigCmd()
			}
		}

		if m.tab == tabAnalytics && m.connected {
			switch msg.String() {
			case "k":
				if m.selectedPID > 0 {
					m.statusMsg = fmt.Sprintf("Отправка pg_cancel_backend(%d)...", m.selectedPID)
					return m, m.terminateBackendCmd(m.targetHost, m.selectedPID, false)
				}
			case "K":
				if m.selectedPID > 0 {
					m.statusMsg = fmt.Sprintf("Принудительное завершение pg_terminate_backend(%d)...", m.selectedPID)
					return m, m.terminateBackendCmd(m.targetHost, m.selectedPID, true)
				}
			case "r":
				return m, tea.Batch(
					m.fetchTopQueriesCmd(m.targetHost),
					m.fetchActiveLocksCmd(m.targetHost),
				)
			case "up":
				if len(m.activeLocks) > 0 {
					for i, l := range m.activeLocks {
						if l.PID == m.selectedPID && i > 0 {
							m.selectedPID = m.activeLocks[i-1].PID
							break
						}
					}
				}
			case "down":
				if len(m.activeLocks) > 0 {
					for i, l := range m.activeLocks {
						if l.PID == m.selectedPID && i < len(m.activeLocks)-1 {
							m.selectedPID = m.activeLocks[i+1].PID
							break
						}
					}
				}
			}
		}

		if m.connected {
			switch msg.String() {
			case "e":
				return m, m.exportMetricsCmd("json")
			case "E":
				return m, m.exportMetricsCmd("csv")
			case "tab", "l", "right":
				if m.tab > tabConnect && m.tab != tabSettings {
					m.tab = (m.tab % 6) + 1
					if m.tab == tabAnalytics {
						return m, tea.Batch(
							m.fetchTopQueriesCmd(m.targetHost),
							m.fetchActiveLocksCmd(m.targetHost),
						)
					}
				}
			case "shift+tab", "h", "left":
				if m.tab > tabConnect && m.tab != tabSettings {
					m.tab = (m.tab-2+6)%6 + 1
					if m.tab == tabAnalytics {
						return m, tea.Batch(
							m.fetchTopQueriesCmd(m.targetHost),
							m.fetchActiveLocksCmd(m.targetHost),
						)
					}
				}
			case "1":
				m.tab = tabTopology
			case "2":
				m.tab = tabEngineStats
			case "3":
				m.tab = tabAnalytics
				return m, tea.Batch(
					m.fetchTopQueriesCmd(m.targetHost),
					m.fetchActiveLocksCmd(m.targetHost),
				)
			case "4":
				m.tab = tabActions
			case "5":
				m.tab = tabLogs
				return m, m.fetchLogsCmd()
			case "6":
				m.tab = tabSettings
				m.updateSettingsFocus()
			}
		}

		if m.tab == tabActions && m.connected {
			switch msg.String() {
			case "up", "k":
				if m.actionsSelectedIndex > 0 {
					m.actionsSelectedIndex--
				}
			case "down", "j":
				actions := GetDefaultActions()
				if m.actionsSelectedIndex < len(actions)-1 {
					m.actionsSelectedIndex++
				}
			case "enter":
				m.showConfirmationModal = true
			}

			if m.detectedEngine == EnginePatroni {
				switch strings.ToLower(msg.String()) {
				case "s":
					return m, m.executeActionCmd("switchover")
				case "f":
					return m, m.executeActionCmd("failover")
				case "r":
					return m, m.executeActionCmd("reinit")
				case "p":
					return m, m.executeActionCmd("pause")
				}
			}
			if m.detectedEngine == EnginePacemaker {
				switch strings.ToLower(msg.String()) {
				case "m":
					return m, m.executePacemakerActionCmd("move")
				case "c":
					return m, m.executePacemakerActionCmd("cleanup")
				case "s":
					return m, m.executePacemakerActionCmd("standby")
				}
			}
		}

		if m.tab == tabConnect {
			switch msg.String() {
			case "up":
				m.focusIndex--
				if m.focusIndex < 0 {
					m.focusIndex = len(m.inputs) - 1
				}
				return m, m.updateFocus()

			case "down":
				m.focusIndex++
				if m.focusIndex >= len(m.inputs) {
					m.focusIndex = 0
				}
				return m, m.updateFocus()

			case "enter":
				rawEndpoint := m.inputs[0].Value()
				if rawEndpoint == "" {
					rawEndpoint = m.inputs[0].Placeholder
				}

				host, port := parseHostAndPort(rawEndpoint)
				m.targetHost = host
				m.targetPort = port

				pgUser := m.inputs[1].Value()
				if pgUser == "" {
					pgUser = m.inputs[1].Placeholder
				}

				pgPass := m.inputs[2].Value()
				if pgPass == "" {
					pgPass = m.inputs[2].Placeholder
				}

				httpEndpoints := []string{}

				httpEndpoint := rawEndpoint
				if !strings.HasPrefix(httpEndpoint, "http://") && !strings.HasPrefix(httpEndpoint, "https://") {
					httpEndpoint = "http://" + httpEndpoint
				}
				httpEndpoints = append(httpEndpoints, httpEndpoint)

				patroniDefaultEndpoint := fmt.Sprintf("http://%s:8008", host)
				if patroniDefaultEndpoint != httpEndpoint {
					httpEndpoints = append(httpEndpoints, patroniDefaultEndpoint)
				}

				m.patroniClient = patroni.NewClient(httpEndpoints, 1500*time.Millisecond)
				m.pgManager = postgres.NewPGPoolManager(postgres.Config{
					User:     pgUser,
					Password: pgPass,
					Database: "postgres",
					Port:     port,
				})

				m.connecting = true
				m.lastError = nil
				return m, tea.Batch(m.fetchClusterDataCmd(), tickCmd(m.settings.PollIntervalSec))
			}

			cmd := m.updateInputs(msg)
			return m, cmd
		}
	}

	if m.tab == tabLogs && m.connected {
		var cmd tea.Cmd
		m.logsViewport, cmd = m.logsViewport.Update(msg)
		cmds = append(cmds, cmd)
	}

	if m.tab == tabTopology && m.connected {
		var cmd tea.Cmd
		m.table, cmd = m.table.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *model) updateSettingsFocus() {
	for i := range m.settingsInputs {
		if m.settingsCursor >= 3 && (m.settingsCursor-3) == i {
			m.settingsInputs[i].Focus()
		} else {
			m.settingsInputs[i].Blur()
		}
	}
}

func (m *model) saveSettingsFromInputs() {
	user := strings.TrimSpace(m.settingsInputs[0].Value())
	if user != "" {
		m.settings.SSHUser = user
	}

	if portVal, err := strconv.Atoi(strings.TrimSpace(m.settingsInputs[1].Value())); err == nil && portVal > 0 {
		m.settings.SSHPort = portVal
	}

	m.settings.SSHKeyPath = strings.TrimSpace(m.settingsInputs[2].Value())
	m.settings.SSHPassword = strings.TrimSpace(m.settingsInputs[3].Value())

	go func() {
		_ = config.SaveConfig(config.AppConfig{
			Language:        int(m.settings.Language),
			Theme:           int(m.settings.Theme),
			PollIntervalSec: m.settings.PollIntervalSec,
			SSHUser:         m.settings.SSHUser,
			SSHPort:         m.settings.SSHPort,
			SSHKeyPath:      m.settings.SSHKeyPath,
			SSHPassword:     m.settings.SSHPassword,
		})
	}()
}

func (m model) tr(ruStr, enStr string) string {
	if m.settings.Language == LangEN {
		return enStr
	}
	return ruStr
}

func (m *model) adjustSetting(delta int) {
	switch m.settingsCursor {
	case 0:
		if m.settings.Language == LangRU {
			m.settings.Language = LangEN
		} else {
			m.settings.Language = LangRU
		}
	case 1:
		next := (int(m.settings.Theme) + delta + 3) % 3
		m.settings.Theme = themeType(next)
		m.applyTheme(m.settings.Theme)
	case 2:
		m.settings.PollIntervalSec += delta
		if m.settings.PollIntervalSec < 1 {
			m.settings.PollIntervalSec = 1
		} else if m.settings.PollIntervalSec > 10 {
			m.settings.PollIntervalSec = 10
		}
	}

	go func() {
		_ = config.SaveConfig(config.AppConfig{
			Language:        int(m.settings.Language),
			Theme:           int(m.settings.Theme),
			PollIntervalSec: m.settings.PollIntervalSec,
			SSHUser:         m.settings.SSHUser,
			SSHPort:         m.settings.SSHPort,
			SSHKeyPath:      m.settings.SSHKeyPath,
		})
	}()
}

func (m *model) applyTheme(t themeType) {
	switch t {
	case ThemeNord:
		primaryColor = lipgloss.Color("#88C0D0")
		secondaryColor = lipgloss.Color("#81A1C1")
		successColor = lipgloss.Color("#A3BE8C")
		warningColor = lipgloss.Color("#EBCB8B")
		dangerColor = lipgloss.Color("#BF616A")
		subtleColor = lipgloss.Color("#4C566A")
	case ThemeMonokai:
		primaryColor = lipgloss.Color("#66D9EF")
		secondaryColor = lipgloss.Color("#AE81FF")
		successColor = lipgloss.Color("#A6E22E")
		warningColor = lipgloss.Color("#E6DB74")
		dangerColor = lipgloss.Color("#F92672")
		subtleColor = lipgloss.Color("#75715E")
	default:
		primaryColor = lipgloss.Color("#89B4FA")
		secondaryColor = lipgloss.Color("#F5C2E7")
		successColor = lipgloss.Color("#A6E3A1")
		warningColor = lipgloss.Color("#F9E2AF")
		dangerColor = lipgloss.Color("#F38BA8")
		subtleColor = lipgloss.Color("#6C7086")
	}

	headerStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#11111B")).Background(primaryColor).Padding(0, 1)
	statusLineStyle = lipgloss.NewStyle().Foreground(subtleColor).Padding(0, 1)
	activeTabStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#11111B")).Background(secondaryColor).Padding(0, 2)
	boxStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(primaryColor).Padding(1)
}

func (m *model) updateLogsViewport() {
	var logLines []string
	searchQueryLower := strings.ToLower(m.logSearchQuery)

	for _, entry := range m.logs {
		switch m.activeLogSubTab {
		case logSubTabPostgres:
			if entry.Component != "Postgres" {
				continue
			}
		case logSubTabHA:
			if entry.Component != "HA Engine" && entry.Component != "Patroni" && entry.Component != "Pacemaker" {
				continue
			}
		case logSubTabSystem:
			if entry.Component != "System" && entry.Component != "Host" {
				continue
			}
		case logSubTabSSH:
			if entry.Component != "SSH" {
				continue
			}
		}

		msgText := entry.Message
		if searchQueryLower != "" {
			if !strings.Contains(strings.ToLower(msgText), searchQueryLower) &&
				!strings.Contains(strings.ToLower(entry.Node), searchQueryLower) {
				continue
			}

			msgText = highlightSearchMatch(msgText, m.logSearchQuery)
		}

		timeStr := lipgloss.NewStyle().Foreground(subtleColor).Render("[" + entry.Timestamp + "]")
		nodeStr := lipgloss.NewStyle().Foreground(primaryColor).Bold(true).Render("[" + entry.Node + "]")
		compStr := lipgloss.NewStyle().Foreground(secondaryColor).Render("[" + entry.Component + "]")

		levelColor := successColor
		if entry.Level == "WARN" {
			levelColor = warningColor
		} else if entry.Level == "ERROR" {
			levelColor = dangerColor
		}
		levelStr := lipgloss.NewStyle().Foreground(levelColor).Render(entry.Level + ":")

		line := fmt.Sprintf("%s %s %s %s %s", timeStr, nodeStr, compStr, levelStr, msgText)
		logLines = append(logLines, line)
	}

	if len(logLines) == 0 {
		logLines = append(logLines, lipgloss.NewStyle().Foreground(subtleColor).Render("Записи логов по данному фильтру/запросу не найдены."))
	}

	m.logsViewport.SetContent(strings.Join(logLines, "\n"))

	if m.logAutoScroll {
		m.logsViewport.GotoBottom()
	}
}

func (m model) isLogDuplicate(node, component, message string) bool {
	if len(m.logs) == 0 {
		return false
	}
	start := len(m.logs) - 10
	if start < 0 {
		start = 0
	}
	for i := len(m.logs) - 1; i >= start; i-- {
		l := m.logs[i]
		if l.Node == node && l.Component == component && l.Message == message {
			return true
		}
	}
	return false
}

func (m *model) updateFocus() tea.Cmd {
	cmds := make([]tea.Cmd, len(m.inputs))
	for i := 0; i < len(m.inputs); i++ {
		if i == m.focusIndex {
			cmds[i] = m.inputs[i].Focus()
		} else {
			m.inputs[i].Blur()
		}
	}
	return tea.Batch(cmds...)
}

func (m *model) updateInputs(msg tea.Msg) tea.Cmd {
	cmds := make([]tea.Cmd, len(m.inputs))
	for i := range m.inputs {
		m.inputs[i], cmds[i] = m.inputs[i].Update(msg)
	}
	return tea.Batch(cmds...)
}
