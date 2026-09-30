package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"cluster-tui/pkg/exporter"
	"cluster-tui/pkg/patroni"
	"cluster-tui/pkg/postgres"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
)

// --- Styles ---
var (
	primaryColor   = lipgloss.Color("#89B4FA")
	secondaryColor = lipgloss.Color("#F5C2E7")
	successColor   = lipgloss.Color("#A6E3A1")
	warningColor   = lipgloss.Color("#F9E2AF")
	dangerColor    = lipgloss.Color("#F38BA8")
	subtleColor    = lipgloss.Color("#6C7086")

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#11111B")).
			Background(primaryColor).
			Padding(0, 1)

	statusLineStyle = lipgloss.NewStyle().
			Foreground(subtleColor).
			Padding(0, 1)

	activeTabStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#11111B")).
			Background(secondaryColor).
			Padding(0, 2)

	inactiveTabStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#CDD6F4")).
				Background(lipgloss.Color("#313244")).
				Padding(0, 2)

	tabBorderStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(lipgloss.Color("#45475A"))

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(primaryColor).
			Padding(1)

	badgePrimary = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#11111B")).
			Background(successColor).
			Padding(0, 1)

	badgeReplica = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#11111B")).
			Background(primaryColor).
			Padding(0, 1)

	badgeStandalone = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#11111B")).
			Background(warningColor).
			Padding(0, 1)
)

type language int

const (
	LangRU language = iota
	LangEN
)

type themeType int

const (
	ThemeCatppuccin themeType = iota
	ThemeNord
	ThemeMonokai
)

func (t themeType) String() string {
	switch t {
	case ThemeNord:
		return "Nord"
	case ThemeMonokai:
		return "Monokai"
	default:
		return "Catppuccin"
	}
}

type appSettings struct {
	Language        language
	Theme           themeType
	PollIntervalSec int
}

var defaultSettings = appSettings{
	Language:        LangRU,
	Theme:           ThemeCatppuccin,
	PollIntervalSec: 2,
}

type activeTab int

const (
	tabConnect activeTab = iota
	tabTopology
	tabEngineStats
	tabActions
	tabLogs
	tabSettings
)

type logSubTab int

const (
	logSubTabAll logSubTab = iota
	logSubTabPostgres
	logSubTabHA
	logSubTabSystem
)

func (l logSubTab) String() string {
	switch l {
	case logSubTabAll:
		return "All Logs"
	case logSubTabPostgres:
		return "Postgres"
	case logSubTabHA:
		return "HA Engine"
	case logSubTabSystem:
		return "System"
	default:
		return "Unknown"
	}
}

type engineType int

const (
	EngineAutoDetect engineType = iota
	EnginePatroni
	EnginePacemaker
	EngineSingleNode
)

func (e engineType) String() string {
	switch e {
	case EnginePatroni:
		return "Patroni (etcd/consul)"
	case EnginePacemaker:
		return "Corosync + Pacemaker"
	case EngineSingleNode:
		return "Single Node (Standalone PG)"
	default:
		return "Auto-Detecting..."
	}
}

// --- Структуры логов ---
type LogEntry struct {
	Timestamp string
	Node      string
	Component string // HA Engine, Postgres, System
	Level     string
	Message   string
}

type logsUpdateMsg struct {
	logs []LogEntry
	err  error
}

type actionResultMsg struct {
	message string
	err     error
}

type exportFinishedMsg struct {
	filePath string
	err      error
}

type tickMsg time.Time
type clusterUpdateMsg struct {
	detectedEngine engineType
	status         *patroni.ClusterStatus
	metrics        map[string]postgres.NodeMetrics
	err            error
}

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

	// Settings state
	settings       appSettings
	settingsCursor int

	// Form Inputs
	inputs     []textinput.Model
	focusIndex int
	connecting bool
	connected  bool

	patroniClient *patroni.Client
	pgManager     *postgres.PGPoolManager
	logExporter   *exporter.LogExporter

	lastStatus  *patroni.ClusterStatus
	lastMetrics map[string]postgres.NodeMetrics
	lastError   error

	logs []LogEntry
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

	return model{
		tab:              tabConnect,
		activeLogSubTab:  logSubTabAll,
		table:            t,
		logsViewport:     vp,
		inputs:           inputs,
		focusIndex:       0,
		detectedEngine:   EngineAutoDetect,
		pgFlavor:         "PostgreSQL / Postgres Pro",
		lastMetrics:      make(map[string]postgres.NodeMetrics),
		logs:             []LogEntry{},
		availableMetrics: defaultMetrics(),
		settings:         defaultSettings,
		logExporter:      logExp,
	}
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func parseHostAndPort(rawInput string) (string, int) {
	cleaned := strings.TrimPrefix(rawInput, "http://")
	cleaned = strings.TrimPrefix(cleaned, "https://")
	if idx := strings.Index(cleaned, "/"); idx != -1 {
		cleaned = cleaned[:idx]
	}

	host, portStr, err := net.SplitHostPort(cleaned)
	if err != nil {
		host = cleaned
		portStr = "5432"
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		port = 5432
	}

	if host == "" {
		host = "127.0.0.1"
	}

	return host, port
}

func (m model) exportMetricsCmd(format string) tea.Cmd {
	return func() tea.Msg {
		home, err := os.UserHomeDir()
		if err != nil {
			return exportFinishedMsg{err: err}
		}
		exportDir := filepath.Join(home, ".wunschpunsch", "exports")
		if err := os.MkdirAll(exportDir, 0755); err != nil {
			return exportFinishedMsg{err: err}
		}

		filename := fmt.Sprintf("metrics_%s.%s", time.Now().Format("20060102_150405"), format)
		fullPath := filepath.Join(exportDir, filename)

		now := time.Now()
		var snapshots []exporter.MetricsSnapshot

		if len(m.lastMetrics) > 0 {
			for _, nm := range m.lastMetrics {
				snapshots = append(snapshots, exporter.MetricsSnapshot{
					Timestamp:     now,
					ActiveConns:   nm.ActiveConnections,
					IdleConns:     nm.IdleConnections,
					CacheHitRatio: nm.CacheHitRatio,
					TPS:           nm.TPS,
				})
			}
		} else {
			snapshots = append(snapshots, exporter.MetricsSnapshot{
				Timestamp: now,
			})
		}

		if format == "json" {
			if len(snapshots) > 0 {
				err = exporter.ExportMetricsJSON(fullPath, snapshots[0])
			}
		} else if format == "csv" {
			err = exporter.ExportMetricsCSV(fullPath, snapshots)
		}

		return exportFinishedMsg{filePath: fullPath, err: err}
	}
}

func (m model) fetchClusterDataCmd() tea.Cmd {
	return func() tea.Msg {
		if m.pgManager == nil {
			return nil
		}

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		var status *patroni.ClusterStatus
		detected := EnginePatroni

		if m.patroniClient != nil {
			st, err := m.patroniClient.GetClusterState(ctx)
			if err == nil {
				status = st
			}
		}

		if status == nil {
			metrics := m.pgManager.FetchNodeMetrics(ctx, m.targetHost)
			if metrics.Error == nil {
				detected = EngineSingleNode
				status = &patroni.ClusterStatus{
					Members: []patroni.Member{
						{
							Name:     m.targetHost,
							Host:     m.targetHost,
							Role:     "standalone",
							State:    "running",
							Timeline: 1,
							Lag:      0,
						},
					},
				}
			} else {
				return clusterUpdateMsg{
					err: fmt.Errorf("unable to connect to PG at %s:%d: %v", m.targetHost, m.targetPort, metrics.Error),
				}
			}
		}

		metricsMap := make(map[string]postgres.NodeMetrics)
		var wg sync.WaitGroup
		var mu sync.Mutex

		for _, member := range status.Members {
			wg.Add(1)
			go func(mem patroni.Member) {
				defer wg.Done()

				nodeHost := mem.Host
				metrics := m.pgManager.FetchNodeMetrics(ctx, nodeHost)

				mu.Lock()
				metricsMap[nodeHost] = metrics
				mu.Unlock()
			}(member)
		}

		wg.Wait()

		return clusterUpdateMsg{
			detectedEngine: detected,
			status:         status,
			metrics:        metricsMap,
		}
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

func (m model) fetchLogsCmd() tea.Cmd {
	return func() tea.Msg {
		if m.pgManager == nil {
			return nil
		}

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		var newLogs []LogEntry
		now := time.Now().Format("15:04:05")

		// 1. HA Engine Logs
		switch m.detectedEngine {
		case EnginePatroni:
			if m.patroniClient != nil {
				history, err := m.patroniClient.GetHistory(ctx)
				if err == nil {
					for _, h := range history {
						msg := fmt.Sprintf("Timeline %d (LSN %d): %s", h.TL, h.LSN, h.Reason)
						if !m.isLogDuplicate("Patroni Cluster", "HA Engine", msg) {
							newLogs = append(newLogs, LogEntry{
								Timestamp: h.Timestamp,
								Node:      "Patroni Cluster",
								Component: "HA Engine",
								Level:     "WARN",
								Message:   msg,
							})
						}
					}
				}

				if m.lastStatus != nil {
					leaderName := "Unknown"
					nodesCount := len(m.lastStatus.Members)
					for _, mem := range m.lastStatus.Members {
						if mem.Role == "leader" {
							leaderName = mem.Name
						}
						nodeMsg := fmt.Sprintf("Node status: role=%s, state=%s, timeline=%d, lag=%d MB",
							mem.Role, mem.State, mem.Timeline, mem.Lag/(1024*1024))
						if !m.isLogDuplicate(mem.Name, "HA Engine", nodeMsg) {
							newLogs = append(newLogs, LogEntry{
								Timestamp: now,
								Node:      mem.Name,
								Component: "HA Engine",
								Level:     "INFO",
								Message:   nodeMsg,
							})
						}
					}

					maintStatus := "OFF"
					if m.lastStatus.Pause {
						maintStatus = "ON (Paused)"
					}
					clusterMsg := fmt.Sprintf("Cluster Scope: %s | Leader: %s | Nodes: %d | Maintenance: %s",
						m.lastStatus.Scope, leaderName, nodesCount, maintStatus)
					if !m.isLogDuplicate("Patroni Cluster", "HA Engine", clusterMsg) {
						newLogs = append(newLogs, LogEntry{
							Timestamp: now,
							Node:      "Patroni Cluster",
							Component: "HA Engine",
							Level:     "INFO",
							Message:   clusterMsg,
						})
					}
				}
			}

		case EngineSingleNode:
			msg := "Initialized in Standalone PG mode (No HA Engine)."
			if !m.isLogDuplicate(m.targetHost, "HA Engine", msg) {
				newLogs = append(newLogs, LogEntry{
					Timestamp: now,
					Node:      m.targetHost,
					Component: "HA Engine",
					Level:     "INFO",
					Message:   msg,
				})
			}
		}

		// 2. System / Host Logs
		hostMetrics, err := FetchHostMetrics(m.hostCPUHistory)
		if err == nil {
			sysMsg := fmt.Sprintf("CPU: %.1f%% | RAM: %.1f%% (%.1f/%.1f GB) | Disk: %.1f%% (%.1f/%.1f GB)",
				hostMetrics.CPUUsage, hostMetrics.RAMUsage, hostMetrics.RAMUsedGB, hostMetrics.RAMTotalGB,
				hostMetrics.DiskUsage, hostMetrics.DiskUsedGB, hostMetrics.DiskTotalGB)

			if !m.isLogDuplicate(m.targetHost, "System", sysMsg) {
				level := "INFO"
				if hostMetrics.CPUUsage > 85 || hostMetrics.RAMUsage > 85 || hostMetrics.DiskUsage > 90 {
					level = "WARN"
				}
				newLogs = append(newLogs, LogEntry{
					Timestamp: now,
					Node:      m.targetHost,
					Component: "System",
					Level:     level,
					Message:   sysMsg,
				})
			}
		} else {
			errMsg := fmt.Sprintf("Failed to fetch host metrics: %v", err)
			if !m.isLogDuplicate(m.targetHost, "System", errMsg) {
				newLogs = append(newLogs, LogEntry{
					Timestamp: now,
					Node:      m.targetHost,
					Component: "System",
					Level:     "ERROR",
					Message:   errMsg,
				})
			}
		}

		// 3. Postgres Logs
		nodes := []string{m.targetHost}
		if m.lastStatus != nil && len(m.lastStatus.Members) > 0 {
			nodes = nil
			for _, mem := range m.lastStatus.Members {
				nodes = append(nodes, mem.Host)
			}
		}

		for _, host := range nodes {
			metrics := m.pgManager.FetchNodeMetrics(ctx, host)
			if metrics.Error != nil {
				errMsg := fmt.Sprintf("Health Check Failed: %v", metrics.Error)
				if !m.isLogDuplicate(host, "Postgres", errMsg) {
					newLogs = append(newLogs, LogEntry{
						Timestamp: now,
						Node:      host,
						Component: "Postgres",
						Level:     "ERROR",
						Message:   errMsg,
					})
				}
			} else {
				pgMsg := fmt.Sprintf("Connections: %d/%d | Cache Hit: %.1f%%",
					metrics.ActiveConnections, metrics.MaxConnections, metrics.CacheHitRatio)
				if !m.isLogDuplicate(host, "Postgres", pgMsg) {
					newLogs = append(newLogs, LogEntry{
						Timestamp: now,
						Node:      host,
						Component: "Postgres",
						Level:     "INFO",
						Message:   pgMsg,
					})
				}
			}
		}

		return logsUpdateMsg{logs: newLogs}
	}
}

func (m model) executeActionCmd(actionType string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		var err error
		var msg string

		if m.detectedEngine != EnginePatroni || m.patroniClient == nil {
			return actionResultMsg{
				message: "Action unavailable for non-Patroni cluster",
				err:     fmt.Errorf("unsupported engine"),
			}
		}

		var leaderName, leaderHost string
		var candidateName, candidateHost string

		if m.lastStatus != nil {
			for _, mem := range m.lastStatus.Members {
				if mem.Role == "leader" {
					leaderName = mem.Name
					leaderHost = mem.Host
				}
			}

			selectedRow := m.table.SelectedRow()
			if len(selectedRow) > 0 {
				selName := selectedRow[0]
				for _, mem := range m.lastStatus.Members {
					if mem.Name == selName && mem.Role != "leader" {
						candidateName = mem.Name
						candidateHost = mem.Host
						break
					}
				}
			}

			if candidateName == "" {
				for _, mem := range m.lastStatus.Members {
					if mem.Role != "leader" {
						candidateName = mem.Name
						candidateHost = mem.Host
						break
					}
				}
			}
		}

		switch actionType {
		case "switchover":
			if leaderName == "" {
				err = fmt.Errorf("leader node not found in cluster state")
			} else if candidateName == "" {
				err = fmt.Errorf("no candidate replica node found for switchover")
			} else {
				err = m.patroniClient.Switchover(ctx, leaderName, candidateName, leaderHost)
				msg = fmt.Sprintf("Switchover initiated (Leader: %s [%s] -> Candidate: %s [%s])",
					leaderName, leaderHost, candidateName, candidateHost)
			}

		case "failover":
			if candidateName == "" && len(m.table.SelectedRow()) > 0 {
				candidateName = m.table.SelectedRow()[0]
			}
			err = m.patroniClient.Failover(ctx, candidateName, leaderHost)
			msg = fmt.Sprintf("Forced Failover executed for candidate: %s", candidateName)

		case "reinit":
			targetNode := m.targetHost
			targetHost := m.targetHost
			if len(m.table.SelectedRow()) > 0 {
				targetNode = m.table.SelectedRow()[0]
				for _, mem := range m.lastStatus.Members {
					if mem.Name == targetNode {
						targetHost = mem.Host
						break
					}
				}
			}
			err = m.patroniClient.Reinitialize(ctx, targetNode, targetHost)
			msg = fmt.Sprintf("Reinitialize triggered for node: %s [%s]", targetNode, targetHost)

		case "pause":
			paused, e := m.patroniClient.TogglePause(ctx, leaderHost)
			err = e
			if paused {
				msg = "Maintenance mode ENABLED (Paused auto-failover)"
			} else {
				msg = "Maintenance mode DISABLED (Resumed auto-failover)"
			}
		}

		return actionResultMsg{message: msg, err: err}
	}
}

func tickCmd(intervalSec int) tea.Cmd {
	if intervalSec <= 0 {
		intervalSec = 2
	}
	return tea.Every(time.Duration(intervalSec)*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
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
		if m.connected {
			cmds := []tea.Cmd{m.fetchClusterDataCmd(), tickCmd(m.settings.PollIntervalSec)}
			if m.tab == tabLogs {
				cmds = append(cmds, m.fetchLogsCmd())
			}
			return m, tea.Batch(cmds...)
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
		}
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

			connsStr := "N/A"
			cacheStr := "N/A"
			if hasMetrics && metrics.Error == nil {
				connsStr = fmt.Sprintf("%d/%d", metrics.ActiveConnections, metrics.MaxConnections)
				cacheStr = fmt.Sprintf("%.1f%%", metrics.CacheHitRatio)
			}

			lagMb := fmt.Sprintf("%d", member.Lag/(1024*1024))

			rows = append(rows, table.Row{
				member.Name,
				member.Host,
				member.Role,
				member.State,
				fmt.Sprintf("%d", member.Timeline),
				lagMb,
				connsStr,
				cacheStr,
			})
		}
		m.table.SetRows(rows)

	case tea.KeyMsg:
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

		if m.tab == tabSettings {
			switch msg.String() {
			case "up", "k":
				if m.settingsCursor > 0 {
					m.settingsCursor--
				}
			case "down", "j":
				if m.settingsCursor < 2 {
					m.settingsCursor++
				}
			case "left", "h":
				m.adjustSetting(-1)
			case "right", "l", "enter", " ":
				m.adjustSetting(1)
			}
		}

		if m.tab == tabLogs && m.connected {
			switch msg.String() {
			case "[", "h", "left":
				if m.activeLogSubTab > logSubTabAll {
					m.activeLogSubTab--
					m.updateLogsViewport()
					return m, nil
				}
			case "]", "l", "right":
				if m.activeLogSubTab < logSubTabSystem {
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

		if msg.String() == "ctrl+c" {
			if m.pgManager != nil {
				m.pgManager.Close()
			}
			return m, tea.Quit
		}

		if msg.String() == "q" || msg.String() == "esc" {
			if m.tab == tabLogs || m.tab == tabSettings {
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

		if m.connected {
			switch msg.String() {
			case "e":
				return m, m.exportMetricsCmd("json")
			case "E":
				return m, m.exportMetricsCmd("csv")
			case "tab", "l", "right":
				if m.tab > tabConnect && m.tab != tabSettings {
					m.tab = (m.tab % 5) + 1
				}
			case "shift+tab", "h", "left":
				if m.tab > tabConnect && m.tab != tabSettings {
					m.tab = (m.tab-2+5)%5 + 1
				}
			case "1":
				m.tab = tabTopology
			case "2":
				m.tab = tabEngineStats
			case "3":
				m.tab = tabActions
			case "4":
				m.tab = tabLogs
				return m, m.fetchLogsCmd()
			case "5":
				m.tab = tabSettings
			}
		}

		if m.tab == tabActions && m.connected && m.detectedEngine != EngineSingleNode {
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

	items := []string{langLine, themeLine, pollLine}

	for i, item := range items {
		cursor := " "
		if i == m.settingsCursor {
			cursor = ">"
			item = lipgloss.NewStyle().Foreground(primaryColor).Bold(true).Render(cursor + item)
		} else {
			item = lipgloss.NewStyle().Foreground(subtleColor).Render(cursor + item)
		}
		lines = append(lines, item)
	}

	lines = append(lines, "")
	hint := "Up/Down: Выбор параметра  •  Left/Right/Space: Изменить"
	if !isRU {
		hint = "Up/Down: Select Option  •  Left/Right/Space: Toggle"
	}
	lines = append(lines, lipgloss.NewStyle().Foreground(subtleColor).Render(hint))

	return strings.Join(lines, "\n")
}

func (m *model) updateLogsViewport() {
	var logLines []string

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

		line := fmt.Sprintf("%s %s %s %s %s", timeStr, nodeStr, compStr, levelStr, entry.Message)
		logLines = append(logLines, line)
	}

	if len(logLines) == 0 {
		logLines = append(logLines, lipgloss.NewStyle().Foreground(subtleColor).Render("No log entries for this filter."))
	}

	m.logsViewport.SetContent(strings.Join(logLines, "\n"))
	m.logsViewport.GotoBottom()
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

func (m model) View() string {
	if m.width == 0 {
		return "Initializing TUI..."
	}

	containerWidth := m.width - 4
	if containerWidth < 40 {
		containerWidth = 40
	}

	header := headerStyle.Render(" Wunschpunsch ") + "  " +
		lipgloss.NewStyle().Foreground(primaryColor).Bold(true).Render("Engine: "+m.detectedEngine.String()) +
		statusLineStyle.Render(" | Target: "+m.pgFlavor)

	tabs := []string{"0: Connect", "1: Topology", "2: Metrics", "3: Actions", "4: Event Logs", "5: Settings"}
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

		case tabActions:
			if m.detectedEngine == EngineSingleNode {
				body = currentBoxStyle.Render(
					lipgloss.JoinVertical(
						lipgloss.Left,
						lipgloss.NewStyle().Bold(true).Foreground(warningColor).Render("Cluster Management Actions Disabled"),
						"\n",
						lipgloss.NewStyle().Foreground(subtleColor).Render(
							"HA operations (Switchover, Failover, Reinitialization) are unavailable for Single-Node / Standalone PostgreSQL instances.",
						),
					),
				)
			} else {
				body = currentBoxStyle.Render(
					lipgloss.JoinVertical(
						lipgloss.Left,
						lipgloss.NewStyle().Bold(true).Foreground(dangerColor).Render("Cluster Management Actions"),
						"\n",
						"[S] Switchover (Graceful Leader Transfer)",
						"[F] Failover (Force Leader Selection)",
						"[R] Reinitialize Replica",
						"[P] Pause/Resume Auto-failover (Maintenance Mode)",
					),
				)
			}

		case tabLogs:
			subTabs := []logSubTab{logSubTabAll, logSubTabPostgres, logSubTabHA, logSubTabSystem}
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

			body = currentBoxStyle.Render(
				lipgloss.JoinVertical(
					lipgloss.Left,
					lipgloss.JoinHorizontal(lipgloss.Left,
						lipgloss.NewStyle().Bold(true).Foreground(secondaryColor).Render("Stream Logs:"),
						"  ",
						subTabHeader,
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
	} else if m.tab == tabLogs {
		footerHint = "[ / ] or Left/Right: Switch Log Filter  •  Up/Down: Scroll  •  e/E: Export JSON/CSV  •  Esc/q: Back  •  Ctrl+C: Quit"
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

type HostMetrics struct {
	CPUUsage    float64
	RAMUsage    float64
	RAMUsedGB   float64
	RAMTotalGB  float64
	DiskUsage   float64
	DiskUsedGB  float64
	DiskTotalGB float64
	CPUHistory  []float64
}

func FetchHostMetrics(history []float64) (HostMetrics, error) {
	var hm HostMetrics

	cpuPercents, err := cpu.Percent(200*time.Millisecond, false)
	if err == nil && len(cpuPercents) > 0 {
		hm.CPUUsage = cpuPercents[0]
	}

	vMem, err := mem.VirtualMemory()
	if err == nil {
		hm.RAMUsage = vMem.UsedPercent
		hm.RAMUsedGB = float64(vMem.Used) / (1024 * 1024 * 1024)
		hm.RAMTotalGB = float64(vMem.Total) / (1024 * 1024 * 1024)
	}

	diskStat, err := disk.Usage("/")
	if err == nil {
		hm.DiskUsage = diskStat.UsedPercent
		hm.DiskUsedGB = float64(diskStat.Used) / (1024 * 1024 * 1024)
		hm.DiskTotalGB = float64(diskStat.Total) / (1024 * 1024 * 1024)
	}

	hm.CPUHistory = append(history, hm.CPUUsage)
	if len(hm.CPUHistory) > 20 {
		hm.CPUHistory = hm.CPUHistory[len(hm.CPUHistory)-20:]
	}

	return hm, nil
}

func drawProgressBar(percent float64, width int) string {
	if percent > 100 {
		percent = 100
	}
	filledWidth := int((percent / 100.0) * float64(width))
	if filledWidth > width {
		filledWidth = width
	}

	filled := strings.Repeat("█", filledWidth)
	empty := strings.Repeat("░", width-filledWidth)

	var color lipgloss.Color
	switch {
	case percent > 85:
		color = dangerColor
	case percent > 65:
		color = warningColor
	default:
		color = successColor
	}

	barStyle := lipgloss.NewStyle().Foreground(color)
	return barStyle.Render(filled+empty) + fmt.Sprintf(" %5.1f%%", percent)
}

func drawSparkline(history []float64, maxVal float64) string {
	bars := []rune{' ', ' ', '▂', '▃', '▄', '▅', '▆', '▇', '█'}
	var result strings.Builder

	for _, val := range history {
		if maxVal <= 0 {
			maxVal = 100
		}
		idx := int((val / maxVal) * float64(len(bars)-1))
		if idx < 0 {
			idx = 0
		}
		if idx >= len(bars) {
			idx = len(bars) - 1
		}
		result.WriteRune(bars[idx])
	}

	return lipgloss.NewStyle().Foreground(primaryColor).Bold(true).Render(result.String())
}

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running TUI: %v\n", err)
		os.Exit(1)
	}
}
