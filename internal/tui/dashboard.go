package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/fairy-pitta/portree/internal/state"
)

// Fixed column widths for SERVICE, PORT, STATUS, PID.
const (
	colServiceWidth = 12
	colPortWidth    = 8
	colStatusWidth  = 14
	colPIDWidth     = 10
	colSeparators   = 4 * 2 // 4 separators × 2 chars ("  ")
	colCursorPrefix = 2     // "▸ " or "  "
	colMinWorktree  = 18
)

// fixedColumnsWidth is the sum of all non-WORKTREE columns plus separators and cursor.
const fixedColumnsWidth = colServiceWidth + colPortWidth + colStatusWidth + colPIDWidth + colSeparators + colCursorPrefix

// worktreeColumnWidth computes the dynamic WORKTREE column width.
func worktreeColumnWidth(termWidth int) int {
	// Account for border padding (lipgloss RoundedBorder + Padding(1,2) adds ~6 chars on each side).
	const borderOverhead = 6
	available := termWidth - fixedColumnsWidth - borderOverhead
	if available < colMinWorktree {
		return colMinWorktree
	}
	return available
}

// renderTable renders the dashboard table with the given rows and cursor position.
func renderTable(rows []ServiceRow, cursor int, termWidth int) string {
	wtWidth := worktreeColumnWidth(termWidth)

	var b strings.Builder

	// Header
	headerCells := []string{
		lipgloss.NewStyle().Width(wtWidth).Bold(true).Foreground(colorWhite).Render("WORKTREE"),
		lipgloss.NewStyle().Width(colServiceWidth).Bold(true).Foreground(colorWhite).Render("SERVICE"),
		lipgloss.NewStyle().Width(colPortWidth).Bold(true).Foreground(colorWhite).Render("PORT"),
		lipgloss.NewStyle().Width(colStatusWidth).Bold(true).Foreground(colorWhite).Render("STATUS"),
		lipgloss.NewStyle().Width(colPIDWidth).Bold(true).Foreground(colorWhite).Render("PID"),
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
		if row.Status == state.StatusRunning {
			statusStr = statusRunning
		}

		pidStr := "—"
		if row.PID > 0 {
			pidStr = fmt.Sprintf("%d", row.PID)
		}

		cells := []string{
			lipgloss.NewStyle().Width(wtWidth).Render(row.Branch),
			lipgloss.NewStyle().Width(colServiceWidth).Render(row.Service),
			lipgloss.NewStyle().Width(colPortWidth).Render(portStr),
			lipgloss.NewStyle().Width(colStatusWidth).Render(statusStr),
			lipgloss.NewStyle().Width(colPIDWidth).Render(pidStr),
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
