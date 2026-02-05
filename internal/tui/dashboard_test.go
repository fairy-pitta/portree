package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/key"

	"github.com/fairy-pitta/portree/internal/state"
)

func TestWorktreeColumnWidth(t *testing.T) {
	tests := []struct {
		name      string
		termWidth int
		wantMin   int
	}{
		{"wide terminal", 120, colMinWorktree},
		{"narrow terminal", 40, colMinWorktree},
		{"minimum terminal", 0, colMinWorktree},
		{"exact minimum", 80, colMinWorktree},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := worktreeColumnWidth(tt.termWidth)
			if got < tt.wantMin {
				t.Errorf("worktreeColumnWidth(%d) = %d, want >= %d", tt.termWidth, got, tt.wantMin)
			}
		})
	}
}

func TestWorktreeColumnWidthGrowsWithTerminal(t *testing.T) {
	narrow := worktreeColumnWidth(80)
	wide := worktreeColumnWidth(200)
	if wide <= narrow {
		t.Errorf("wider terminal should give wider column: 80->%d, 200->%d", narrow, wide)
	}
}

func TestRenderTableEmpty(t *testing.T) {
	result := renderTable(nil, 0, 100)
	if !strings.Contains(result, "WORKTREE") {
		t.Error("empty table should still contain header")
	}
	if !strings.Contains(result, "SERVICE") {
		t.Error("empty table should still contain SERVICE header")
	}
}

func TestRenderTableWithRows(t *testing.T) {
	rows := []ServiceRow{
		{Branch: "main", Slug: "main", Service: "frontend", Port: 3100, Status: state.StatusRunning, PID: 12345},
		{Branch: "main", Slug: "main", Service: "backend", Port: 8100, Status: state.StatusStopped, PID: 0},
		{Branch: "feature/auth", Slug: "feature-auth", Service: "frontend", Port: 3117, Status: state.StatusRunning, PID: 12346},
	}

	result := renderTable(rows, 0, 100)

	// Header present
	if !strings.Contains(result, "WORKTREE") {
		t.Error("table should contain WORKTREE header")
	}

	// Rows present
	if !strings.Contains(result, "main") {
		t.Error("table should contain 'main' worktree")
	}
	if !strings.Contains(result, "frontend") {
		t.Error("table should contain 'frontend' service")
	}
	if !strings.Contains(result, "backend") {
		t.Error("table should contain 'backend' service")
	}
	if !strings.Contains(result, "3100") {
		t.Error("table should contain port 3100")
	}

	// Status indicators
	if !strings.Contains(result, "● running") {
		t.Error("table should contain running indicator")
	}
	if !strings.Contains(result, "○ stopped") {
		t.Error("table should contain stopped indicator")
	}

	// Cursor on first row
	if !strings.Contains(result, "▸") {
		t.Error("table should contain cursor indicator")
	}
}

func TestRenderTableCursorPosition(t *testing.T) {
	rows := []ServiceRow{
		{Branch: "main", Service: "frontend", Status: state.StatusStopped},
		{Branch: "main", Service: "backend", Status: state.StatusStopped},
	}

	// Cursor on second row
	result := renderTable(rows, 1, 100)
	lines := strings.Split(result, "\n")

	// Find which line has the cursor
	cursorLine := -1
	for i, line := range lines {
		if strings.Contains(line, "▸") {
			cursorLine = i
			break
		}
	}
	if cursorLine < 0 {
		t.Fatal("no cursor found in output")
	}
	// The cursor line should contain "backend" (second row)
	if !strings.Contains(lines[cursorLine], "backend") {
		t.Errorf("cursor should be on 'backend' row, got: %s", lines[cursorLine])
	}
}

func TestRenderTableDashForZeroValues(t *testing.T) {
	rows := []ServiceRow{
		{Branch: "main", Service: "frontend", Port: 0, Status: state.StatusStopped, PID: 0},
	}

	result := renderTable(rows, 0, 100)

	// Port 0 and PID 0 should be rendered as "—"
	if !strings.Contains(result, "—") {
		t.Error("zero port/PID should be rendered as em dash")
	}
}

func TestRenderProxyStatus(t *testing.T) {
	t.Run("running with ports", func(t *testing.T) {
		result := renderProxyStatus(true, []int{3000, 8000})
		if !strings.Contains(result, "● running") {
			t.Error("running proxy should show running indicator")
		}
		if !strings.Contains(result, ":3000") {
			t.Error("should show port 3000")
		}
		if !strings.Contains(result, ":8000") {
			t.Error("should show port 8000")
		}
	})

	t.Run("stopped", func(t *testing.T) {
		result := renderProxyStatus(false, nil)
		if !strings.Contains(result, "○ stopped") {
			t.Error("stopped proxy should show stopped indicator")
		}
	})

	t.Run("running with single port", func(t *testing.T) {
		result := renderProxyStatus(true, []int{3000})
		if !strings.Contains(result, ":3000") {
			t.Error("should show single port")
		}
	})
}

func TestRenderHelp(t *testing.T) {
	keys := DefaultKeyMap()
	result := renderHelp(keys, 120)

	expectedParts := []string{
		"[s] start", "[x] stop", "[r] restart", "[o] open",
		"[a] all start", "[X] all stop", "[p] proxy",
		"[l] logs", "[q] quit",
	}

	for _, part := range expectedParts {
		if !strings.Contains(result, part) {
			t.Errorf("help should contain %q", part)
		}
	}
}

func TestRenderHelpWrapping(t *testing.T) {
	keys := DefaultKeyMap()

	// Narrow width should produce multiple lines
	narrow := renderHelp(keys, 50)
	lines := strings.Split(narrow, "\n")
	if len(lines) < 2 {
		t.Errorf("narrow help should wrap to multiple lines, got %d", len(lines))
	}

	// Wide width should fit on fewer lines
	wide := renderHelp(keys, 200)
	wideLines := strings.Split(wide, "\n")
	if len(wideLines) >= len(lines) {
		t.Errorf("wide help should have fewer lines than narrow")
	}
}

func TestDefaultKeyMap(t *testing.T) {
	km := DefaultKeyMap()

	// Verify all key bindings are set
	bindings := []struct {
		name    string
		binding key.Binding
	}{
		{"Up", km.Up},
		{"Down", km.Down},
		{"Start", km.Start},
		{"Stop", km.Stop},
		{"Restart", km.Restart},
		{"Open", km.Open},
		{"StartAll", km.StartAll},
		{"StopAll", km.StopAll},
		{"ToggleProxy", km.ToggleProxy},
		{"ViewLogs", km.ViewLogs},
		{"Quit", km.Quit},
	}

	for _, b := range bindings {
		if !b.binding.Enabled() {
			t.Errorf("key binding %s should be enabled", b.name)
		}
	}
}

func TestKeyMapShortHelp(t *testing.T) {
	km := DefaultKeyMap()
	bindings := km.ShortHelp()
	if len(bindings) == 0 {
		t.Error("ShortHelp() should return non-empty slice")
	}
}

func TestKeyMapFullHelp(t *testing.T) {
	km := DefaultKeyMap()
	groups := km.FullHelp()
	if len(groups) == 0 {
		t.Error("FullHelp() should return non-empty slice")
	}
	for i, group := range groups {
		if len(group) == 0 {
			t.Errorf("FullHelp() group %d is empty", i)
		}
	}
}
