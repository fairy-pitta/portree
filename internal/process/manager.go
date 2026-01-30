package process

import (
	"path/filepath"
	"sort"

	"github.com/shuna/gws/internal/config"
	"github.com/shuna/gws/internal/git"
	"github.com/shuna/gws/internal/port"
	"github.com/shuna/gws/internal/state"
)

// Manager coordinates starting and stopping services across worktrees.
type Manager struct {
	cfg      *config.Config
	store    *state.FileStore
	registry *port.Registry
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

// StartResult describes the outcome of starting a service.
type StartResult struct {
	Branch  string
	Service string
	Port    int
	PID     int
	Err     error
}

// StartServices starts services for the given worktree.
// If serviceFilter is non-empty, only that service is started.
func (m *Manager) StartServices(tree *git.Worktree, serviceFilter string) []StartResult {
	var results []StartResult

	services := m.targetServices(serviceFilter)

	// First allocate all ports so cross-service env vars are available.
	portMap := map[string]int{}
	for _, svcName := range services {
		p, err := m.registry.AssignPort(tree.Branch, svcName)
		if err != nil {
			results = append(results, StartResult{
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

	slug := tree.Slug()

	for _, svcName := range services {
		p, ok := portMap[svcName]
		if !ok {
			continue // port allocation failed, already reported
		}

		// Clean up stale processes.
		m.cleanStale(tree.Branch, svcName)

		svc := m.cfg.Services[svcName]
		command := m.cfg.CommandForBranch(svcName, tree.Branch)
		env := m.cfg.EnvForBranch(svcName, tree.Branch)

		dir := tree.Path
		if svc.Dir != "" {
			dir = filepath.Join(tree.Path, svc.Dir)
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
		})

		pid, err := runner.Start()
		result := StartResult{
			Branch: tree.Branch, Service: svcName, Port: p, PID: pid, Err: err,
		}
		results = append(results, result)

		if err == nil {
			key := tree.Branch + ":" + svcName
			m.runners[key] = runner

			m.store.WithLock(func() error {
				st, e := m.store.Load()
				if e != nil {
					return e
				}
				state.SetServiceState(st, tree.Branch, svcName, state.RunningServiceState(p, pid))
				return m.store.Save(st)
			})
		}
	}

	return results
}

// StopServices stops services for the given worktree.
func (m *Manager) StopServices(tree *git.Worktree, serviceFilter string) []StartResult {
	var results []StartResult
	services := m.targetServices(serviceFilter)

	for _, svcName := range services {
		key := tree.Branch + ":" + svcName
		result := StartResult{Branch: tree.Branch, Service: svcName}

		// Try runner first.
		if runner, ok := m.runners[key]; ok {
			result.Err = runner.Stop()
			delete(m.runners, key)
		} else {
			// Fall back to PID from state.
			m.store.WithLock(func() error {
				st, e := m.store.Load()
				if e != nil {
					return e
				}
				ss := state.GetServiceState(st, tree.Branch, svcName)
				if ss != nil && ss.PID > 0 && IsProcessRunning(ss.PID) {
					result.PID = ss.PID
					result.Err = StopPID(ss.PID)
				}
				return nil
			})
		}

		// Update state to stopped.
		m.store.WithLock(func() error {
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
		})

		results = append(results, result)
	}

	return results
}

// cleanStale checks if a previously recorded process is dead and cleans up state.
func (m *Manager) cleanStale(branch, service string) {
	m.store.WithLock(func() error {
		st, err := m.store.Load()
		if err != nil {
			return err
		}
		ss := state.GetServiceState(st, branch, service)
		if ss != nil && ss.Status == "running" && ss.PID > 0 && !IsProcessRunning(ss.PID) {
			state.SetServiceState(st, branch, service, state.StoppedServiceState(ss.Port))
			return m.store.Save(st)
		}
		return nil
	})
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
