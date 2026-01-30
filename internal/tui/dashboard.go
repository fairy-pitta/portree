package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// columns defines the table layout.
var columns = []struct {
	Title string
	Width int
}{
	{"WORKTREE", 18},
	{"SERVICE", 12},
	{"PORT", 8},
	{"STATUS", 14},
	{"PID", 10},
}

// renderTable renders the dashboard table with the given rows and cursor position.
func renderTable(rows []ServiceRow, cursor int) string {
	var b strings.Builder

	// Header
	var headerCells []string
	for _, col := range columns {
		headerCells = append(headerCells, lipgloss.NewStyle().
			Width(col.Width).
			Bold(true).
			Foreground(colorWhite).
			Render(col.Title))
	}
	header := strings.Join(headerCells, "  ")
	b.WriteString(headerStyle.Render(header))
	b.WriteString("\n")

	// Rows
	for i, row := range rows {
		portStr := "—"
		if row.Port > 0 {
			portStr = fmt.Sprintf("%d", row.Port)
		}

		statusStr := statusStopped
		if row.Status == "running" {
			statusStr = statusRunning
		}

		pidStr := "—"
		if row.PID > 0 {
			pidStr = fmt.Sprintf("%d", row.PID)
		}

		cells := []string{
			lipgloss.NewStyle().Width(columns[0].Width).Render(row.Branch),
			lipgloss.NewStyle().Width(columns[1].Width).Render(row.Service),
			lipgloss.NewStyle().Width(columns[2].Width).Render(portStr),
			lipgloss.NewStyle().Width(columns[3].Width).Render(statusStr),
			lipgloss.NewStyle().Width(columns[4].Width).Render(pidStr),
		}

		line := strings.Join(cells, "  ")

		if i == cursor {
			// Prepend cursor indicator
			line = "▸ " + line
			b.WriteString(selectedRowStyle.Render(line))
		} else {
			line = "  " + line
			b.WriteString(rowStyle.Render(line))
		}
		b.WriteString("\n")
	}

	return b.String()
}

// renderProxyStatus renders the proxy status line.
func renderProxyStatus(running bool, ports []int) string {
	if running {
		portStrs := make([]string, len(ports))
		for i, p := range ports {
			portStrs[i] = fmt.Sprintf(":%d", p)
		}
		return proxyRunningStyle.Render(
			fmt.Sprintf("Proxy: ● running (%s)", strings.Join(portStrs, ", ")))
	}
	return proxyStoppedStyle.Render("Proxy: ○ stopped")
}

// renderHelp renders the key binding help bar.
func renderHelp(keys KeyMap) string {
	parts := []string{
		"[s] start", "[x] stop", "[r] restart", "[o] open in browser",
		"[a] start all", "[X] stop all", "[p] toggle proxy",
		"[l] view logs", "[q] quit",
	}
	return helpStyle.Render(strings.Join(parts, "  "))
}
