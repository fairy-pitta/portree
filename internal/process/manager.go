package process

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/fairy-pitta/portree/internal/config"
	"github.com/fairy-pitta/portree/internal/git"
	"github.com/fairy-pitta/portree/internal/logging"
	"github.com/fairy-pitta/portree/internal/port"
	"github.com/fairy-pitta/portree/internal/state"
)

// Manager coordinates starting and stopping services across worktrees.
type Manager struct {
	cfg      *config.Config
	store    *state.FileStore
	registry *port.Registry
	mu       sync.RWMutex
	runners  map[string]*Runner // key: "branch:service"
}

// NewManager creates a new process Manager.
func NewManager(cfg *config.Config, store *state.FileStore, registry *port.Registry) *Manager {
	return &Manager{
		cfg:      cfg,
		store:    store,
		registry: registry,
		runners:  map[string]*Runner{},
	}
}

func (m *Manager) setRunner(key string, r *Runner) {
	m.mu.Lock()
	m.runners[key] = r
	m.mu.Unlock()
}

func (m *Manager) getRunner(key string) (*Runner, bool) {
	m.mu.RLock()
	r, ok := m.runners[key]
	m.mu.RUnlock()
	return r, ok
}

func (m *Manager) deleteRunner(key string) {
	m.mu.Lock()
	delete(m.runners, key)
	m.mu.Unlock()
}

// ServiceResult describes the outcome of starting or stopping a service.
type ServiceResult struct {
	Branch  string
	Service string
	Port    int
	PID     int
	Err     error
	// AlreadyRunning is true when the service was found healthy and running,
	// so no new process was started.
	AlreadyRunning bool
	// WasRunning is set by StopServices and reports whether a live process was
	// actually stopped, as opposed to the service already being idle.
	WasRunning bool
}

// defaultStartupGrace is how long a freshly started service is watched for an
// early exit (e.g. EADDRINUSE) before it is reported as started.
const defaultStartupGrace = 3 * time.Second

// startupGraceWindow returns the startup watch duration, overridable via
// PORTREE_STARTUP_GRACE (a Go duration string, e.g. "500ms").
func startupGraceWindow() time.Duration {
	if v := os.Getenv("PORTREE_STARTUP_GRACE"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
	}
	return defaultStartupGrace
}

// StartServices starts services for the given worktree.
// If serviceFilter is non-empty, only that service is started.
// Services named in skip get ports allocated (so cross-service env vars like
// PT_<SVC>_PORT stay correct for the services that do run) but are not started.
func (m *Manager) StartServices(tree *git.Worktree, serviceFilter string, skip ...string) []ServiceResult {
	var results []ServiceResult

	services := m.targetServices(serviceFilter)
	skipped := make(map[string]bool, len(skip))
	for _, s := range skip {
		skipped[s] = true
	}

	// First allocate all ports so cross-service env vars are available.
	portMap := map[string]int{}
	for _, svcName := range services {
		p, err := m.registry.AssignPort(tree.Branch, svcName)
		if err != nil {
			// A skipped service is not being started, so a failed allocation is
			// not fatal: it only means its PT_<SVC>_PORT env var is unavailable
			// to the services that do run (harmless unless one references it).
			// Don't fail `up` over a port we were never going to use — e.g. a
			// worker range already exhausted by sibling worktrees.
			if skipped[svcName] {
				logging.Warn("skipping port allocation for %s/%s: %v", tree.Branch, svcName, err)
				continue
			}
			results = append(results, ServiceResult{
				Branch: tree.Branch, Service: svcName, Err: err,
			})
			continue
		}
		portMap[svcName] = p
	}

	// Build proxy port map for cross-service URLs.
	proxyPorts := map[string]int{}
	for svcName, svc := range m.cfg.Services {
		proxyPorts[svcName] = svc.ProxyPort
	}

	// Determine proxy scheme from state.
	proxyScheme := "http"
	if err := m.store.WithLock(func() error {
		st, e := m.store.Load()
		if e != nil {
			return e
		}
		if st.Proxy.HTTPS {
			proxyScheme = "https"
		}
		return nil
	}); err != nil {
		logging.Warn("failed to load proxy state for scheme: %v", err)
	}

	slug := tree.Slug()

	var watches []startWatch
	for _, svcName := range services {
		p, ok := portMap[svcName]
		if !ok {
			continue // port allocation failed, already reported
		}

		if skipped[svcName] {
			continue
		}

		// Clean up stale processes.
		m.cleanStale(tree.Branch, svcName)

		// If our own service is already running on this port, leave it alone.
		if pid, running := m.runningServicePID(tree.Branch, svcName, p); running {
			results = append(results, ServiceResult{
				Branch: tree.Branch, Service: svcName, Port: p, PID: pid,
				AlreadyRunning: true,
			})
			continue
		}

		// Check if port is available. If not, the port might be held by an orphan process.
		if !IsPortAvailable(p) {
			results = append(results, ServiceResult{
				Branch: tree.Branch, Service: svcName, Port: p,
				Err: fmt.Errorf("port %d is already in use (orphan process?)", p),
			})
			continue
		}

		svc := m.cfg.Services[svcName]
		command := m.cfg.CommandForBranch(svcName, tree.Branch)
		env := m.cfg.EnvForBranch(svcName, tree.Branch)

		dir := tree.Path
		if svc.Dir != "" {
			dir = filepath.Join(tree.Path, svc.Dir)
		}

		// Validate the resolved directory stays within the worktree root.
		cleanDir := filepath.Clean(dir)
		cleanRoot := filepath.Clean(tree.Path)
		if cleanDir != cleanRoot && !strings.HasPrefix(cleanDir, cleanRoot+string(filepath.Separator)) {
			results = append(results, ServiceResult{
				Branch: tree.Branch, Service: svcName,
				Err: fmt.Errorf("service directory %q resolves outside worktree root", svc.Dir),
			})
			continue
		}

		runner := NewRunner(RunnerConfig{
			ServiceName:          svcName,
			Branch:               tree.Branch,
			BranchSlug:           slug,
			Command:              command,
			Dir:                  dir,
			Port:                 p,
			Env:                  env,
			LogDir:               filepath.Join(m.store.Dir(), "logs"),
			AllServicePorts:      portMap,
			AllServiceProxyPorts: proxyPorts,
			ProxyScheme:          proxyScheme,
		})

		pid, err := runner.Start()
		result := ServiceResult{
			Branch: tree.Branch, Service: svcName, Port: p, PID: pid, Err: err,
		}
		results = append(results, result)

		if err == nil {
			key := tree.Branch + ":" + svcName
			m.setRunner(key, runner)

			if err := m.store.WithLock(func() error {
				st, e := m.store.Load()
				if e != nil {
					return e
				}
				state.SetServiceState(st, tree.Branch, svcName, state.RunningServiceState(p, pid))
				return m.store.Save(st)
			}); err != nil {
				logging.Warn("failed to save state after starting %s/%s: %v", tree.Branch, svcName, err)
			}

			watches = append(watches, startWatch{
				resultIdx: len(results) - 1,
				runner:    runner,
				logPath:   filepath.Join(m.store.Dir(), "logs", fmt.Sprintf("%s.%s.log", slug, svcName)),
			})
		}
	}

	m.watchStartups(tree.Branch, watches, results)

	return results
}

// startWatch tracks a freshly started service during the startup grace window.
type startWatch struct {
	resultIdx int
	runner    *Runner
	logPath   string
}

// watchStartups waits up to startupGraceWindow for each freshly started
// service and converts an early exit into a loud failure. Without this, a
// service that crashes right after spawn (e.g. its port is taken by a process
// the bind probe could not see) is reported as started.
func (m *Manager) watchStartups(branch string, watches []startWatch, results []ServiceResult) {
	var wg sync.WaitGroup
	for _, w := range watches {
		wg.Add(1)
		go func(w startWatch) {
			defer wg.Done()
			grace := startupGraceWindow()
			select {
			case <-w.runner.Done():
				r := &results[w.resultIdx]
				r.Err = fmt.Errorf("exited within %s of starting; last log lines:\n%s",
					grace, tailFile(w.logPath, 10))
				key := branch + ":" + r.Service
				m.deleteRunner(key)
				if err := m.store.WithLock(func() error {
					st, e := m.store.Load()
					if e != nil {
						return e
					}
					state.SetServiceState(st, branch, r.Service, state.StoppedServiceState(r.Port))
					return m.store.Save(st)
				}); err != nil {
					logging.Warn("failed to save state after %s/%s died at startup: %v", branch, r.Service, err)
				}
			case <-time.After(grace):
			}
		}(w)
	}
	wg.Wait()
}

// tailFile returns up to n trailing lines of the file at path.
func tailFile(path string, n int) string {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Sprintf("(could not read log %s: %v)", path, err)
	}
	defer func() { _ = f.Close() }()

	const tailBytes = 8 * 1024
	info, err := f.Stat()
	if err != nil {
		return fmt.Sprintf("(could not stat log %s: %v)", path, err)
	}
	offset := info.Size() - tailBytes
	if offset < 0 {
		offset = 0
	}
	buf := make([]byte, info.Size()-offset)
	if _, err := f.ReadAt(buf, offset); err != nil && err != io.EOF {
		return fmt.Sprintf("(could not read log %s: %v)", path, err)
	}
	lines := strings.Split(strings.TrimRight(string(buf), "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

// runningServicePID reports whether the branch's service is recorded as
// running on the given port with a live PID.
func (m *Manager) runningServicePID(branch, service string, port int) (int, bool) {
	var pid int
	var running bool
	if err := m.store.WithLock(func() error {
		st, e := m.store.Load()
		if e != nil {
			return e
		}
		ss := state.GetServiceState(st, branch, service)
		if ss != nil && ss.Status == state.StatusRunning && ss.Port == port && ss.PID > 0 && IsProcessRunning(ss.PID) {
			pid = ss.PID
			running = true
		}
		return nil
	}); err != nil {
		logging.Warn("failed to check running state for %s/%s: %v", branch, service, err)
	}
	return pid, running
}

// StopServices stops services for the given worktree.
func (m *Manager) StopServices(tree *git.Worktree, serviceFilter string) []ServiceResult {
	var results []ServiceResult
	services := m.targetServices(serviceFilter)

	for _, svcName := range services {
		key := tree.Branch + ":" + svcName
		result := ServiceResult{Branch: tree.Branch, Service: svcName}

		// Try runner first.
		if runner, ok := m.getRunner(key); ok {
			result.WasRunning = runner.IsRunning()
			result.Err = runner.Stop()
			m.deleteRunner(key)
		} else {
			// Fall back to PID from state.
			if err := m.store.WithLock(func() error {
				st, e := m.store.Load()
				if e != nil {
					return e
				}
				ss := state.GetServiceState(st, tree.Branch, svcName)
				if ss != nil && ss.PID > 0 && IsProcessRunning(ss.PID) {
					result.PID = ss.PID
					result.WasRunning = true
					result.Err = StopPID(ss.PID)
				}
				return nil
			}); err != nil {
				result.Err = err
			}
		}

		// Update state to stopped.
		if err := m.store.WithLock(func() error {
			st, e := m.store.Load()
			if e != nil {
				return e
			}
			ss := state.GetServiceState(st, tree.Branch, svcName)
			portVal := 0
			if ss != nil {
				portVal = ss.Port
			}
			state.SetServiceState(st, tree.Branch, svcName, state.StoppedServiceState(portVal))
			return m.store.Save(st)
		}); err != nil {
			logging.Warn("failed to update state after stopping %s/%s: %v", tree.Branch, svcName, err)
		}

		results = append(results, result)
	}

	return results
}

// cleanStale checks if a previously recorded process is dead and cleans up state.
func (m *Manager) cleanStale(branch, service string) {
	if err := m.store.WithLock(func() error {
		st, err := m.store.Load()
		if err != nil {
			return err
		}
		ss := state.GetServiceState(st, branch, service)
		if ss != nil && ss.Status == state.StatusRunning && ss.PID > 0 && !IsProcessRunning(ss.PID) {
			state.SetServiceState(st, branch, service, state.StoppedServiceState(ss.Port))
			return m.store.Save(st)
		}
		return nil
	}); err != nil {
		logging.Warn("failed to clean stale state for %s/%s: %v", branch, service, err)
	}
}

// targetServices returns sorted service names, optionally filtered.
func (m *Manager) targetServices(filter string) []string {
	if filter != "" {
		if _, ok := m.cfg.Services[filter]; ok {
			return []string{filter}
		}
		return nil
	}
	names := make([]string, 0, len(m.cfg.Services))
	for name := range m.cfg.Services {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// StatusAll returns the full state for display.
func (m *Manager) StatusAll() (*state.State, error) {
	var st *state.State
	err := m.store.WithLock(func() error {
		var e error
		st, e = m.store.Load()
		return e
	})
	return st, err
}
