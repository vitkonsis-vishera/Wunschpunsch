package main

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
)

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

func highlightSearchMatch(text, query string) string {
	if query == "" {
		return text
	}
	idx := strings.Index(strings.ToLower(text), strings.ToLower(query))
	if idx == -1 {
		return text
	}

	matchedStr := text[idx : idx+len(query)]
	highlighted := lipgloss.NewStyle().
		Background(warningColor).
		Foreground(lipgloss.Color("#11111B")).
		Bold(true).
		Render(matchedStr)

	return text[:idx] + highlighted + text[idx+len(query):]
}

func parseLogLevel(line string) string {
	upper := strings.ToUpper(line)
	switch {
	case strings.Contains(upper, "ERROR"), strings.Contains(upper, "FATAL"), strings.Contains(upper, "PANIC"):
		return "ERROR"
	case strings.Contains(upper, "WARN"), strings.Contains(upper, "WARNING"):
		return "WARN"
	default:
		return "INFO"
	}
}
