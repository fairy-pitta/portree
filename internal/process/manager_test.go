package process

import (
	"testing"
	"time"

	"github.com/fairy-pitta/portree/internal/config"
	"github.com/fairy-pitta/portree/internal/git"
	"github.com/fairy-pitta/portree/internal/port"
	"github.com/fairy-pitta/portree/internal/state"
)

func TestTargetServices(t *testing.T) {
	cfg := &config.Config{
		Services: map[string]config.ServiceConfig{
			"web":    {Command: "npm start"},
			"api":    {Command: "go run ."},
			"worker": {Command: "python worker.py"},
		},
	}
	store, _ := state.NewFileStore(t.TempDir())
	m := NewManager(cfg, store, nil)

	t.Run("no filter returns sorted", func(t *testing.T) {
		got := m.targetServices("")
		if len(got) != 3 {
			t.Fatalf("targetServices() returned %d, want 3", len(got))
		}
		if got[0] != "api" || got[1] != "web" || got[2] != "worker" {
			t.Errorf("targetServices() = %v, want [api, web, worker]", got)
		}
	})

	t.Run("filter exists", func(t *testing.T) {
		got := m.targetServices("web")
		if len(got) != 1 || got[0] != "web" {
			t.Errorf("targetServices(web) = %v, want [web]", got)
		}
	})

	t.Run("filter not found", func(t *testing.T) {
		got := m.targetServices("nonexistent")
		if got != nil {
			t.Errorf("targetServices(nonexistent) = %v, want nil", got)
		}
	})
}

func TestMutexHelpers(t *testing.T) {
	cfg := &config.Config{
		Services: map[string]config.ServiceConfig{},
	}
	store, _ := state.NewFileStore(t.TempDir())
	m := NewManager(cfg, store, nil)

	// Initially no runner.
	_, ok := m.getRunner("main:web")
	if ok {
		t.Error("expected getRunner to return false for missing key")
	}

	// Set a runner.
	r := NewRunner(RunnerConfig{ServiceName: "web"})
	m.setRunner("main:web", r)

	got, ok := m.getRunner("main:web")
	if !ok {
		t.Error("expected getRunner to return true after setRunner")
	}
	if got != r {
		t.Error("expected getRunner to return the same runner")
	}

	// Delete.
	m.deleteRunner("main:web")
	_, ok = m.getRunner("main:web")
	if ok {
		t.Error("expected getRunner to return false after deleteRunner")
	}

	// Delete non-existent key should not panic.
	m.deleteRunner("nonexistent")
}

func newTestManager(t *testing.T) (*Manager, *state.FileStore) {
	t.Helper()
	dir := t.TempDir()
	store, err := state.NewFileStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Services: map[string]config.ServiceConfig{
			"web": {
				Command:   "sleep 60",
				PortRange: config.PortRange{Min: 19100, Max: 19199},
				ProxyPort: 3000,
			},
		},
		Env:       map[string]string{},
		Worktrees: map[string]config.WTOverride{},
	}

	registry := port.NewRegistry(store, cfg)
	mgr := NewManager(cfg, store, registry)
	return mgr, store
}

func TestManagerStartStopServices(t *testing.T) {
	mgr, store := newTestManager(t)

	tree := &git.Worktree{
		Path:   t.TempDir(),
		Branch: "main",
	}

	// Start services.
	results := mgr.StartServices(tree, "web")
	if len(results) != 1 {
		t.Fatalf("StartServices returned %d results, want 1", len(results))
	}
	r := results[0]
	if r.Err != nil {
		t.Fatalf("StartServices error: %v", r.Err)
	}
	if r.PID <= 0 {
		t.Errorf("expected positive PID, got %d", r.PID)
	}
	if r.Port < 19100 || r.Port > 19199 {
		t.Errorf("port %d out of expected range [19100, 19199]", r.Port)
	}
	if r.Branch != "main" {
		t.Errorf("branch = %q, want %q", r.Branch, "main")
	}
	if r.Service != "web" {
		t.Errorf("service = %q, want %q", r.Service, "web")
	}

	// Verify state was persisted.
	var st *state.State
	_ = store.WithLock(func() error {
		var e error
		st, e = store.Load()
		return e
	})
	ss := state.GetServiceState(st, "main", "web")
	if ss == nil {
		t.Fatal("expected service state to be persisted")
	}
	if ss.Status != "running" {
		t.Errorf("state status = %q, want %q", ss.Status, "running")
	}
	if ss.PID != r.PID {
		t.Errorf("state PID = %d, want %d", ss.PID, r.PID)
	}

	// Verify runner is tracked.
	_, ok := mgr.getRunner("main:web")
	if !ok {
		t.Error("expected runner to be tracked in manager")
	}

	// Stop services.
	stopResults := mgr.StopServices(tree, "web")
	if len(stopResults) != 1 {
		t.Fatalf("StopServices returned %d results, want 1", len(stopResults))
	}
	if stopResults[0].Err != nil {
		t.Fatalf("StopServices error: %v", stopResults[0].Err)
	}

	// Give OS time to clean up.
	time.Sleep(200 * time.Millisecond)

	// Verify runner was removed.
	_, ok = mgr.getRunner("main:web")
	if ok {
		t.Error("expected runner to be removed after stop")
	}

	// Verify state was updated to stopped.
	_ = store.WithLock(func() error {
		var e error
		st, e = store.Load()
		return e
	})
	ss = state.GetServiceState(st, "main", "web")
	if ss == nil {
		t.Fatal("expected service state after stop")
	}
	if ss.Status != "stopped" {
		t.Errorf("state status after stop = %q, want %q", ss.Status, "stopped")
	}
}

func TestManagerCleanStale(t *testing.T) {
	dir := t.TempDir()
	store, err := state.NewFileStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Services: map[string]config.ServiceConfig{
			"web": {
				Command:   "sleep 60",
				PortRange: config.PortRange{Min: 19200, Max: 19299},
				ProxyPort: 3000,
			},
		},
		Env:       map[string]string{},
		Worktrees: map[string]config.WTOverride{},
	}

	// Write stale state with a non-existent PID.
	st := &state.State{
		Services:        map[string]map[string]*state.ServiceState{},
		PortAssignments: map[string]int{},
	}
	state.SetServiceState(st, "main", "web", state.RunningServiceState(19200, 99999999))
	if err := store.Save(st); err != nil {
		t.Fatal(err)
	}

	mgr := NewManager(cfg, store, port.NewRegistry(store, cfg))

	// cleanStale should detect the dead PID and update state.
	mgr.cleanStale("main", "web")

	_ = store.WithLock(func() error {
		var e error
		st, e = store.Load()
		return e
	})
	ss := state.GetServiceState(st, "main", "web")
	if ss == nil {
		t.Fatal("expected service state after cleanStale")
	}
	if ss.Status != "stopped" {
		t.Errorf("state status after cleanStale = %q, want %q", ss.Status, "stopped")
	}
}

func TestManagerStatusAll(t *testing.T) {
	dir := t.TempDir()
	store, err := state.NewFileStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Services: map[string]config.ServiceConfig{},
	}
	mgr := NewManager(cfg, store, nil)

	st, err := mgr.StatusAll()
	if err != nil {
		t.Fatalf("StatusAll error: %v", err)
	}
	if st == nil {
		t.Fatal("StatusAll returned nil state")
	}
}

func TestManagerStopWithoutStart(t *testing.T) {
	mgr, _ := newTestManager(t)

	tree := &git.Worktree{
		Path:   t.TempDir(),
		Branch: "nonexistent",
	}

	// Stopping services that were never started should not error.
	results := mgr.StopServices(tree, "web")
	if len(results) != 1 {
		t.Fatalf("StopServices returned %d results, want 1", len(results))
	}
	if results[0].Err != nil {
		t.Errorf("StopServices error: %v", results[0].Err)
	}
}
