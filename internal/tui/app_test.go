package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/fairy-pitta/portree/internal/config"
	"github.com/fairy-pitta/portree/internal/git"
	"github.com/fairy-pitta/portree/internal/state"
)

// testModel creates a minimal Model for testing without git/filesystem dependencies.
func testModel(t *testing.T, rows []ServiceRow) *Model {
	t.Helper()
	dir := t.TempDir()
	store, err := state.NewFileStore(dir)
	if err != nil {
		t.Fatalf("creating test store: %v", err)
	}
	return &Model{
		keys:       DefaultKeyMap(),
		rows:       rows,
		cursor:     0,
		width:      100,
		height:     40,
		proxyPorts: []int{3000, 8000},
		store:      store,
	}
}

func TestModelView_Empty(t *testing.T) {
	m := testModel(t, nil)
	view := m.View()

	if !strings.Contains(view, "portree dashboard") {
		t.Error("view should contain title")
	}
	if !strings.Contains(view, "WORKTREE") {
		t.Error("view should contain table header")
	}
}

func TestModelView_WithRows(t *testing.T) {
	rows := []ServiceRow{
		{Branch: "main", Service: "frontend", Port: 3100, Status: state.StatusRunning, PID: 123},
	}
	m := testModel(t, rows)
	view := m.View()

	if !strings.Contains(view, "main") {
		t.Error("view should contain worktree name")
	}
	if !strings.Contains(view, "frontend") {
		t.Error("view should contain service name")
	}
}

func TestModelView_TerminalTooSmall(t *testing.T) {
	m := testModel(t, nil)
	m.width = 40
	m.height = 5
	view := m.View()

	if !strings.Contains(view, "Terminal too small") {
		t.Error("should show terminal too small message")
	}
}

func TestModelView_WithStatusMessage(t *testing.T) {
	m := testModel(t, nil)
	m.statusMsg = "Started frontend for main"
	view := m.View()

	if !strings.Contains(view, "Started frontend for main") {
		t.Error("view should contain status message")
	}
}

func TestModelUpdate_WindowSizeMsg(t *testing.T) {
	m := testModel(t, nil)

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 50})
	model := updated.(*Model)

	if model.width != 120 {
		t.Errorf("width = %d, want 120", model.width)
	}
	if model.height != 50 {
		t.Errorf("height = %d, want 50", model.height)
	}
}

func TestModelUpdate_CursorMovement(t *testing.T) {
	rows := []ServiceRow{
		{Branch: "main", Service: "frontend"},
		{Branch: "main", Service: "backend"},
		{Branch: "feature", Service: "frontend"},
	}
	m := testModel(t, rows)

	// Move down
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	model := updated.(*Model)
	if model.cursor != 1 {
		t.Errorf("after 'j', cursor = %d, want 1", model.cursor)
	}

	// Move down again
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	model = updated.(*Model)
	if model.cursor != 2 {
		t.Errorf("after second 'j', cursor = %d, want 2", model.cursor)
	}

	// Move down at bottom (should stay)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	model = updated.(*Model)
	if model.cursor != 2 {
		t.Errorf("at bottom 'j', cursor = %d, want 2", model.cursor)
	}

	// Move up
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	model = updated.(*Model)
	if model.cursor != 1 {
		t.Errorf("after 'k', cursor = %d, want 1", model.cursor)
	}
}

func TestModelUpdate_CursorUpAtTop(t *testing.T) {
	rows := []ServiceRow{
		{Branch: "main", Service: "frontend"},
	}
	m := testModel(t, rows)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	model := updated.(*Model)
	if model.cursor != 0 {
		t.Errorf("at top 'k', cursor = %d, want 0", model.cursor)
	}
}

func TestModelUpdate_ArrowKeys(t *testing.T) {
	rows := []ServiceRow{
		{Branch: "main", Service: "frontend"},
		{Branch: "main", Service: "backend"},
	}
	m := testModel(t, rows)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	model := updated.(*Model)
	if model.cursor != 1 {
		t.Errorf("after down arrow, cursor = %d, want 1", model.cursor)
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyUp})
	model = updated.(*Model)
	if model.cursor != 0 {
		t.Errorf("after up arrow, cursor = %d, want 0", model.cursor)
	}
}

func TestModelUpdate_QuitKey(t *testing.T) {
	m := testModel(t, nil)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Error("'q' should return a quit command")
	}
}

func TestModelUpdate_StatusUpdateMsg(t *testing.T) {
	m := testModel(t, nil)

	newRows := []ServiceRow{
		{Branch: "main", Service: "frontend", Port: 3100, Status: state.StatusRunning, PID: 123},
	}

	updated, _ := m.Update(StatusUpdateMsg{Rows: newRows})
	model := updated.(*Model)

	if len(model.rows) != 1 {
		t.Fatalf("rows len = %d, want 1", len(model.rows))
	}
	if model.rows[0].Port != 3100 {
		t.Errorf("row port = %d, want 3100", model.rows[0].Port)
	}
}

func TestModelUpdate_StatusUpdateMsg_CursorClamp(t *testing.T) {
	rows := []ServiceRow{
		{Branch: "main", Service: "frontend"},
		{Branch: "main", Service: "backend"},
		{Branch: "main", Service: "worker"},
	}
	m := testModel(t, rows)
	m.cursor = 2 // on last row

	// Update with fewer rows — cursor should clamp
	newRows := []ServiceRow{
		{Branch: "main", Service: "frontend"},
	}
	updated, _ := m.Update(StatusUpdateMsg{Rows: newRows})
	model := updated.(*Model)

	if model.cursor != 0 {
		t.Errorf("cursor = %d, want 0 (clamped to new last row)", model.cursor)
	}
}

func TestModelUpdate_ActionResultMsg(t *testing.T) {
	m := testModel(t, nil)

	updated, _ := m.Update(ActionResultMsg{Message: "Started frontend", IsError: false})
	model := updated.(*Model)

	if model.statusMsg != "Started frontend" {
		t.Errorf("statusMsg = %q, want %q", model.statusMsg, "Started frontend")
	}
}

func TestModelUpdate_ToggleProxy(t *testing.T) {
	m := testModel(t, nil)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	model := updated.(*Model)

	if model.statusMsg == "" {
		t.Error("toggle proxy should set a status message")
	}
}

func TestSelectedRow(t *testing.T) {
	t.Run("valid cursor", func(t *testing.T) {
		rows := []ServiceRow{
			{Branch: "main", Service: "frontend"},
			{Branch: "main", Service: "backend"},
		}
		m := testModel(t, rows)
		m.cursor = 1

		row := m.selectedRow()
		if row == nil {
			t.Fatal("selectedRow() returned nil")
		}
		if row.Service != "backend" {
			t.Errorf("selectedRow().Service = %q, want %q", row.Service, "backend")
		}
	})

	t.Run("empty rows", func(t *testing.T) {
		m := testModel(t, nil)
		row := m.selectedRow()
		if row != nil {
			t.Error("selectedRow() should return nil for empty rows")
		}
	})

	t.Run("cursor out of bounds", func(t *testing.T) {
		rows := []ServiceRow{
			{Branch: "main", Service: "frontend"},
		}
		m := testModel(t, rows)
		m.cursor = 5

		row := m.selectedRow()
		if row != nil {
			t.Error("selectedRow() should return nil when cursor out of bounds")
		}
	})
}

func TestModelView_ProxyStatus(t *testing.T) {
	t.Run("proxy running", func(t *testing.T) {
		m := testModel(t, nil)
		m.proxyRunning = true
		m.proxyPorts = []int{3000, 8000}
		view := m.View()
		if !strings.Contains(view, "● running") {
			t.Error("should show proxy running")
		}
	})

	t.Run("proxy stopped", func(t *testing.T) {
		m := testModel(t, nil)
		m.proxyRunning = false
		view := m.View()
		if !strings.Contains(view, "○ stopped") {
			t.Error("should show proxy stopped")
		}
	})
}

func TestModelView_DefaultWidth(t *testing.T) {
	m := testModel(t, nil)
	m.width = 0 // Before first WindowSizeMsg
	m.height = 0
	view := m.View()

	// Should not panic, should use default width
	if view == "" {
		t.Error("view should not be empty even with zero dimensions")
	}
}

func TestTickCmd(t *testing.T) {
	cmd := tickCmd()
	if cmd == nil {
		t.Error("tickCmd() should return non-nil command")
	}
}

func TestModelUpdate_TickMsg(t *testing.T) {
	m := testModel(t, nil)
	// TickMsg should return a batch of commands (refresh + tick)
	_, cmd := m.Update(TickMsg{})
	if cmd == nil {
		t.Error("TickMsg should return a command")
	}
}

func TestWorktreePath(t *testing.T) {
	m := testModel(t, nil)
	m.repoRoot = "/tmp/repo"
	m.trees = []git.Worktree{
		{Path: "/tmp/repo", Branch: "main"},
		{Path: "/tmp/repo-feature", Branch: "feature/auth"},
	}

	t.Run("known branch", func(t *testing.T) {
		got := m.worktreePath("feature/auth")
		if got != "/tmp/repo-feature" {
			t.Errorf("worktreePath(feature/auth) = %q, want %q", got, "/tmp/repo-feature")
		}
	})

	t.Run("main branch", func(t *testing.T) {
		got := m.worktreePath("main")
		if got != "/tmp/repo" {
			t.Errorf("worktreePath(main) = %q, want %q", got, "/tmp/repo")
		}
	})

	t.Run("unknown branch", func(t *testing.T) {
		got := m.worktreePath("nonexistent")
		if got != "/tmp/repo" {
			t.Errorf("worktreePath(nonexistent) = %q, want %q (should fall back to repoRoot)", got, "/tmp/repo")
		}
	})
}

func TestModelInit(t *testing.T) {
	m := testModel(t, nil)
	cmd := m.Init()
	if cmd == nil {
		t.Error("Init() should return a command")
	}
}

func TestModelUpdate_StartSelectedNoRows(t *testing.T) {
	m := testModel(t, nil)
	// Press 's' with no rows should not panic
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	_ = updated
}

func TestModelUpdate_StopSelectedNoRows(t *testing.T) {
	m := testModel(t, nil)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	_ = updated
}

func TestModelUpdate_RestartSelectedNoRows(t *testing.T) {
	m := testModel(t, nil)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	_ = updated
}

func TestModelUpdate_OpenSelectedNoRows(t *testing.T) {
	m := testModel(t, nil)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	_ = updated
}

func TestModelUpdate_ViewLogsNoRows(t *testing.T) {
	m := testModel(t, nil)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	_ = updated
}

func TestModelUpdate_StartAll(t *testing.T) {
	m := testModel(t, nil)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	_ = updated
}

func TestModelUpdate_StopAll(t *testing.T) {
	m := testModel(t, nil)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'X'}})
	_ = updated
}

func TestOpenSelected_NotRunning(t *testing.T) {
	rows := []ServiceRow{
		{Branch: "main", Slug: "main", Service: "frontend", Port: 3100, Status: state.StatusStopped},
	}
	m := testModel(t, rows)
	m.cfg = &config.Config{
		Services: map[string]config.ServiceConfig{
			"frontend": {Command: "echo", PortRange: config.PortRange{Min: 3100, Max: 3199}, ProxyPort: 3000},
		},
	}

	msg := m.openSelected()
	result, ok := msg.(ActionResultMsg)
	if !ok {
		t.Fatalf("expected ActionResultMsg, got %T", msg)
	}
	if !result.IsError {
		t.Error("opening stopped service should return error")
	}
	if !strings.Contains(result.Message, "not running") {
		t.Errorf("message should contain 'not running', got: %s", result.Message)
	}
}

func TestViewLogs_NoLogFile(t *testing.T) {
	rows := []ServiceRow{
		{Branch: "main", Slug: "main", Service: "frontend"},
	}
	m := testModel(t, rows)

	msg := m.viewLogs()
	result, ok := msg.(ActionResultMsg)
	if !ok {
		t.Fatalf("expected ActionResultMsg, got %T", msg)
	}
	if !strings.Contains(result.Message, "No log file") {
		t.Errorf("message should contain 'No log file', got: %s", result.Message)
	}
}

func TestViewLogs_WithLogFile(t *testing.T) {
	rows := []ServiceRow{
		{Branch: "main", Slug: "main", Service: "frontend"},
	}
	m := testModel(t, rows)

	// Create the log file
	logDir := filepath.Join(m.store.Dir(), "logs")
	if err := os.MkdirAll(logDir, 0700); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(logDir, "main.frontend.log")
	if err := os.WriteFile(logPath, []byte("test log"), 0600); err != nil {
		t.Fatal(err)
	}

	msg := m.viewLogs()
	result, ok := msg.(ActionResultMsg)
	if !ok {
		t.Fatalf("expected ActionResultMsg, got %T", msg)
	}
	if !strings.Contains(result.Message, "Log file:") {
		t.Errorf("message should contain 'Log file:', got: %s", result.Message)
	}
}
