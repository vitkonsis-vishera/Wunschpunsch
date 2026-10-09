package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"wunschpunsch/pkg/exporter"
	"wunschpunsch/pkg/pacemaker"
	"wunschpunsch/pkg/patroni"
	"wunschpunsch/pkg/postgres"
	"wunschpunsch/pkg/ssh"

	tea "github.com/charmbracelet/bubbletea"
)

func (m model) fetchTopQueriesCmd(host string) tea.Cmd {
	return func() tea.Msg {
		if m.pgManager == nil {
			return topQueriesLoadedMsg{err: fmt.Errorf("pgManager не инициализирован")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		stats, err := m.pgManager.GetTopQueries(ctx, host)
		return topQueriesLoadedMsg{stats: stats, err: err}
	}
}

func (m model) fetchActiveLocksCmd(host string) tea.Cmd {
	return func() tea.Msg {
		if m.pgManager == nil {
			return locksLoadedMsg{err: fmt.Errorf("pgManager не инициализирован")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		locks, err := m.pgManager.GetActiveLocks(ctx, host)
		return locksLoadedMsg{locks: locks, err: err}
	}
}

func (m model) terminateBackendCmd(host string, pid int, force bool) tea.Cmd {
	return func() tea.Msg {
		if m.pgManager == nil {
			return terminateResultMsg{pid: pid, force: force, err: fmt.Errorf("pgManager не инициализирован")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		err := m.pgManager.TerminateBackend(ctx, host, pid, force)
		return terminateResultMsg{pid: pid, force: force, err: err}
	}
}

func (m model) executeDBActionCmd(action ActionItem) tea.Cmd {
	return func() tea.Msg {
		if m.pgManager == nil {
			return actionResultMsg{err: fmt.Errorf("PGPoolManager не инициализирован")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		db, err := m.pgManager.GetDB(m.targetHost)
		if err != nil {
			return actionResultMsg{err: fmt.Errorf("ошибка соединения с %s: %w", m.targetHost, err)}
		}

		res, err := ExecuteAction(ctx, db, action)
		return actionResultMsg{message: res, err: err}
	}
}

func (m model) fetchDCSConfigCmd() tea.Cmd {
	return func() tea.Msg {
		if m.patroniClient == nil {
			return nil
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		cfg, err := m.patroniClient.GetDCSConfig(ctx)
		if err != nil {
			return dcsLoadedMsg{err: err}
		}
		return dcsLoadedMsg{config: cfg}
	}
}

func (m model) restartNodeCmd(nodeName string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		err := m.patroniClient.RestartNode(ctx, nodeName)
		return nodeActionResultMsg{action: "Restart", node: nodeName, err: err}
	}
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

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		var status *patroni.ClusterStatus
		detected := EnginePatroni

		if m.patroniClient != nil {
			st, err := m.patroniClient.GetClusterState(ctx)
			if err == nil && st != nil {
				status = st
			}
		}

		if status == nil {
			metrics := m.pgManager.FetchNodeMetrics(ctx, m.targetHost)

			nodeState := "running"
			if metrics.Error != nil {
				nodeState = "UNREACHABLE"
			}

			detected = EngineSingleNode
			status = &patroni.ClusterStatus{
				Members: []patroni.Member{
					{
						Name:     m.targetHost,
						Host:     m.targetHost,
						Role:     "standalone",
						State:    nodeState,
						Timeline: 1,
						Lag:      0,
					},
				},
			}
		}

		metricsMap := make(map[string]postgres.NodeMetrics)
		var wg sync.WaitGroup
		var mu sync.Mutex

		for _, member := range status.Members {
			wg.Add(1)
			go func(mem patroni.Member) {
				defer wg.Done()

				nodeCtx, nodeCancel := context.WithTimeout(ctx, 1500*time.Millisecond)
				defer nodeCancel()

				metrics := m.pgManager.FetchNodeMetrics(nodeCtx, mem.Host)

				mu.Lock()
				metricsMap[mem.Host] = metrics
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

func (m model) fetchLogsCmd() tea.Cmd {
	return func() tea.Msg {
		if m.pgManager == nil {
			return nil
		}

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		var newLogs []LogEntry
		now := time.Now().Format("15:04:05")

		// 1. HA Engine Logs (Patroni / Standalone)
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
		}

		// 3. Postgres Logs (Health & Connection checks)
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

func (m model) fetchSSHLogsCmd(host string, service string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		defer cancel()

		user := m.settings.SSHUser
		if user == "" {
			user = "postgres"
		}
		port := m.settings.SSHPort
		if port <= 0 {
			port = 22
		}

		sshClient := ssh.NewClient(user, port, m.settings.SSHKeyPath, m.settings.SSHPassword)
		rawLines, err := sshClient.FetchRecentLogs(ctx, host, service, 25)
		if err != nil {
			return sshLogsLoadedMsg{node: host, err: err}
		}

		var entries []LogEntry
		now := time.Now().Format("15:04:05")

		for _, line := range rawLines {
			entries = append(entries, LogEntry{
				Timestamp: now,
				Node:      host,
				Component: "SSH",
				Level:     parseLogLevel(line),
				Message:   line,
			})
		}

		return sshLogsLoadedMsg{node: host, logs: entries}
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

func (m model) executePacemakerActionCmd(actionType string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if m.detectedEngine != EnginePacemaker {
			return actionResultMsg{
				message: "Действие недоступно: текущий движок не Pacemaker",
				err:     fmt.Errorf("unsupported engine"),
			}
		}

		pmClient := pacemaker.NewClient()
		var err error
		var msg string

		selectedNode := m.targetHost
		if len(m.table.SelectedRow()) > 0 {
			selectedNode = m.table.SelectedRow()[0]
		}

		switch actionType {
		case "move":
			err = pmClient.MoveResource(ctx, "pg_resource", selectedNode)
			msg = fmt.Sprintf("Перемещение ресурса Pacemaker выполнено для узла: %s", selectedNode)

		case "cleanup":
			err = pmClient.CleanupResource(ctx, "pg_resource")
			msg = "Очистка состояний ресурсов Pacemaker выполнена"

		case "standby":
			err = pmClient.ToggleNodeStandby(ctx, selectedNode, true)
			msg = fmt.Sprintf("Узел %s переведен в режим Standby", selectedNode)
		}

		return actionResultMsg{message: msg, err: err}
	}
}
