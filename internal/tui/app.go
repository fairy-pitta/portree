package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/fairy-pitta/portree/internal/browser"
	"github.com/fairy-pitta/portree/internal/config"
	"github.com/fairy-pitta/portree/internal/git"
	"github.com/fairy-pitta/portree/internal/port"
	"github.com/fairy-pitta/portree/internal/process"
	"github.com/fairy-pitta/portree/internal/state"
)

const pollInterval = 2 * time.Second

// Model is the top-level Bubble Tea model for the dashboard.
type Model struct {
	cfg      *config.Config
	repoRoot string
	store    *state.FileStore
	registry *port.Registry
	manager  *process.Manager
	keys     KeyMap

	rows         []ServiceRow
	cursor       int
	proxyRunning bool
	proxyPorts   []int
	statusMsg    string
	width        int
	height       int
}

// NewModel creates a new dashboard model.
func NewModel(cfg *config.Config, repoRoot string) (*Model, error) {
	stateDir := filepath.Join(repoRoot, ".portree")
	store, err := state.NewFileStore(stateDir)
	if err != nil {
		return nil, err
	}

	registry := port.NewRegistry(store, cfg)
	mgr := process.NewManager(cfg, store, registry)

	// Collect proxy ports.
	var proxyPorts []int
	seen := map[int]bool{}
	for _, svc := range cfg.Services {
		if !seen[svc.ProxyPort] {
			seen[svc.ProxyPort] = true
			proxyPorts = append(proxyPorts, svc.ProxyPort)
		}
	}
	sort.Ints(proxyPorts)

	return &Model{
		cfg:        cfg,
		repoRoot:   repoRoot,
		store:      store,
		registry:   registry,
		manager:    mgr,
		keys:       DefaultKeyMap(),
		proxyPorts: proxyPorts,
	}, nil
}

// Init implements tea.Model.
func (m *Model) Init() tea.Cmd {
	return tea.Batch(
		m.refreshStatus,
		tickCmd(),
	)
}

// Update implements tea.Model.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case TickMsg:
		return m, tea.Batch(m.refreshStatus, tickCmd())

	case StatusUpdateMsg:
		m.rows = msg.Rows
		// Refresh proxy status from state.
		_ = m.store.WithLock(func() error {
			st, e := m.store.Load()
			if e != nil {
				return e
			}
			m.proxyRunning = st.Proxy.Status == "running" && st.Proxy.PID > 0 && process.IsProcessRunning(st.Proxy.PID)
			return nil
		})
		if m.cursor >= len(m.rows) && len(m.rows) > 0 {
			m.cursor = len(m.rows) - 1
		}
		return m, nil

	case ActionResultMsg:
		m.statusMsg = msg.Message
		return m, m.refreshStatus

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

// View implements tea.Model.
func (m *Model) View() string {
	title := titleStyle.Render(" portree dashboard ")

	table := renderTable(m.rows, m.cursor)
	proxyLine := renderProxyStatus(m.proxyRunning, m.proxyPorts)
	help := renderHelp(m.keys)

	content := fmt.Sprintf("%s\n\n%s\n%s\n%s", title, table, proxyLine, help)

	if m.statusMsg != "" {
		content += "\n\n" + m.statusMsg
	}

	return borderStyle.Render(content) + "\n"
}

func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit

	case key.Matches(msg, m.keys.Up):
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil

	case key.Matches(msg, m.keys.Down):
		if m.cursor < len(m.rows)-1 {
			m.cursor++
		}
		return m, nil

	case key.Matches(msg, m.keys.Start):
		return m, m.startSelected

	case key.Matches(msg, m.keys.Stop):
		return m, m.stopSelected

	case key.Matches(msg, m.keys.Restart):
		return m, m.restartSelected

	case key.Matches(msg, m.keys.Open):
		return m, m.openSelected

	case key.Matches(msg, m.keys.StartAll):
		return m, m.startAll

	case key.Matches(msg, m.keys.StopAll):
		return m, m.stopAll

	case key.Matches(msg, m.keys.ToggleProxy):
		m.statusMsg = "Proxy toggle not supported in dashboard (use 'portree proxy start' in a separate terminal)"
		return m, nil

	case key.Matches(msg, m.keys.ViewLogs):
		return m, m.viewLogs
	}

	return m, nil
}

func (m *Model) selectedRow() *ServiceRow {
	if m.cursor >= 0 && m.cursor < len(m.rows) {
		return &m.rows[m.cursor]
	}
	return nil
}

func (m *Model) refreshStatus() tea.Msg {
	cwd, err := os.Getwd()
	if err != nil {
		return StatusUpdateMsg{}
	}

	trees, err := git.ListWorktrees(cwd)
	if err != nil {
		return StatusUpdateMsg{}
	}

	serviceNames := make([]string, 0, len(m.cfg.Services))
	for name := range m.cfg.Services {
		serviceNames = append(serviceNames, name)
	}
	sort.Strings(serviceNames)

	var st *state.State
	_ = m.store.WithLock(func() error {
		st, err = m.store.Load()
		return err
	})
	if st == nil {
		return StatusUpdateMsg{}
	}

	var rows []ServiceRow
	for _, tree := range trees {
		if tree.IsBare {
			continue
		}
		for _, svcName := range serviceNames {
			row := ServiceRow{
				Branch:  tree.Branch,
				Slug:    tree.Slug(),
				Service: svcName,
			}

			ss := state.GetServiceState(st, tree.Branch, svcName)
			if ss != nil {
				row.Port = ss.Port
				row.PID = ss.PID
				if ss.PID > 0 && process.IsProcessRunning(ss.PID) {
					row.Status = "running"
				} else {
					row.Status = "stopped"
				}
			} else {
				row.Status = "stopped"
			}

			rows = append(rows, row)
		}
	}

	return StatusUpdateMsg{Rows: rows}
}

func (m *Model) startSelected() tea.Msg {
	row := m.selectedRow()
	if row == nil {
		return ActionResultMsg{Message: "No service selected"}
	}

	tree := &git.Worktree{Path: m.worktreePath(row.Branch), Branch: row.Branch}
	results := m.manager.StartServices(tree, row.Service)
	for _, r := range results {
		if r.Err != nil {
			return ActionResultMsg{Message: fmt.Sprintf("Error: %v", r.Err), IsError: true}
		}
	}
	return ActionResultMsg{Message: fmt.Sprintf("Started %s for %s", row.Service, row.Branch)}
}

func (m *Model) stopSelected() tea.Msg {
	row := m.selectedRow()
	if row == nil {
		return ActionResultMsg{Message: "No service selected"}
	}

	tree := &git.Worktree{Path: m.worktreePath(row.Branch), Branch: row.Branch}
	results := m.manager.StopServices(tree, row.Service)
	for _, r := range results {
		if r.Err != nil {
			return ActionResultMsg{Message: fmt.Sprintf("Error: %v", r.Err), IsError: true}
		}
	}
	return ActionResultMsg{Message: fmt.Sprintf("Stopped %s for %s", row.Service, row.Branch)}
}

func (m *Model) restartSelected() tea.Msg {
	row := m.selectedRow()
	if row == nil {
		return ActionResultMsg{Message: "No service selected"}
	}

	tree := &git.Worktree{Path: m.worktreePath(row.Branch), Branch: row.Branch}
	m.manager.StopServices(tree, row.Service)
	results := m.manager.StartServices(tree, row.Service)
	for _, r := range results {
		if r.Err != nil {
			return ActionResultMsg{Message: fmt.Sprintf("Error: %v", r.Err), IsError: true}
		}
	}
	return ActionResultMsg{Message: fmt.Sprintf("Restarted %s for %s", row.Service, row.Branch)}
}

func (m *Model) openSelected() tea.Msg {
	row := m.selectedRow()
	if row == nil {
		return ActionResultMsg{Message: "No service selected"}
	}

	svc, ok := m.cfg.Services[row.Service]
	if !ok {
		return ActionResultMsg{Message: "Unknown service", IsError: true}
	}

	url := browser.BuildURL(row.Slug, svc.ProxyPort)
	if err := browser.Open(url); err != nil {
		return ActionResultMsg{Message: fmt.Sprintf("Error opening browser: %v", err), IsError: true}
	}
	return ActionResultMsg{Message: fmt.Sprintf("Opening %s", url)}
}

func (m *Model) startAll() tea.Msg {
	cwd, _ := os.Getwd()
	trees, err := git.ListWorktrees(cwd)
	if err != nil {
		return ActionResultMsg{Message: fmt.Sprintf("Error: %v", err), IsError: true}
	}

	count := 0
	for _, tree := range trees {
		if tree.IsBare {
			continue
		}
		results := m.manager.StartServices(&tree, "")
		for _, r := range results {
			if r.Err == nil {
				count++
			}
		}
	}
	return ActionResultMsg{Message: fmt.Sprintf("Started %d services", count)}
}

func (m *Model) stopAll() tea.Msg {
	cwd, _ := os.Getwd()
	trees, err := git.ListWorktrees(cwd)
	if err != nil {
		return ActionResultMsg{Message: fmt.Sprintf("Error: %v", err), IsError: true}
	}

	count := 0
	for _, tree := range trees {
		if tree.IsBare {
			continue
		}
		results := m.manager.StopServices(&tree, "")
		for _, r := range results {
			if r.Err == nil {
				count++
			}
		}
	}
	return ActionResultMsg{Message: fmt.Sprintf("Stopped %d services", count)}
}

func (m *Model) viewLogs() tea.Msg {
	row := m.selectedRow()
	if row == nil {
		return ActionResultMsg{Message: "No service selected"}
	}

	logPath := filepath.Join(m.store.Dir(), "logs",
		fmt.Sprintf("%s.%s.log", row.Slug, row.Service))

	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		return ActionResultMsg{Message: "No log file found"}
	}

	return ActionResultMsg{Message: fmt.Sprintf("Log file: %s", logPath)}
}

// worktreePath looks up the worktree path from known worktrees.
func (m *Model) worktreePath(branch string) string {
	cwd, _ := os.Getwd()
	trees, err := git.ListWorktrees(cwd)
	if err != nil {
		return m.repoRoot
	}
	for _, t := range trees {
		if t.Branch == branch {
			return t.Path
		}
	}
	return m.repoRoot
}

// Run launches the Bubble Tea program.
func Run(cfg *config.Config, repoRoot string) error {
	model, err := NewModel(cfg, repoRoot)
	if err != nil {
		return err
	}

	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err = p.Run()
	return err
}
