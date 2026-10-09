package main

import (
	"time"

	"wunschpunsch/pkg/patroni"
	"wunschpunsch/pkg/postgres"

	"github.com/charmbracelet/lipgloss"
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

type AlertLevel int

const (
	AlertLevelOK AlertLevel = iota
	AlertLevelWarn
	AlertLevelCrit
)

type AlertItem struct {
	Node    string
	Level   AlertLevel
	Message string
}

func (a AlertLevel) String() string {
	switch a {
	case AlertLevelCrit:
		return "CRIT"
	case AlertLevelWarn:
		return "WARN"
	default:
		return "OK"
	}
}

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
	SSHUser         string
	SSHPort         int
	SSHKeyPath      string
	SSHPassword     string
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
	tabAnalytics
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
	logSubTabSSH
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
	case logSubTabSSH:
		return "SSH / Journal"
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

// --- Структуры логов и сообщений ---
type LogEntry struct {
	Timestamp string
	Node      string
	Component string // HA Engine, Postgres, System, SSH
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

type topQueriesLoadedMsg struct {
	stats []postgres.QueryStat
	err   error
}

type locksLoadedMsg struct {
	locks []postgres.LockInfo
	err   error
}

type terminateResultMsg struct {
	pid   int
	force bool
	err   error
}

type dcsLoadedMsg struct {
	config string
	err    error
}

type nodeActionResultMsg struct {
	action string
	node   string
	err    error
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

type sshLogsLoadedMsg struct {
	node string
	logs []LogEntry
	err  error
}
