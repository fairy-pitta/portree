package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/fairy-pitta/portree/internal/logging"
)

const (
	// StatusRunning indicates a running service or proxy.
	StatusRunning = "running"
	// StatusStopped indicates a stopped service or proxy.
	StatusStopped = "stopped"
)

const lockTimeout = 10 * time.Second

// ServiceState represents the runtime state of a single service in a worktree.
type ServiceState struct {
	Port      int    `json:"port"`
	PID       int    `json:"pid"`
	Status    string `json:"status"` // StatusRunning, StatusStopped
	StartedAt string `json:"started_at"`
}

// ProxyState represents the runtime state of the reverse proxy.
type ProxyState struct {
	PID    int    `json:"pid"`
	Status string `json:"status"`
}

// State represents the full persisted state.
type State struct {
	// Services maps branch -> service name -> ServiceState.
	Services map[string]map[string]*ServiceState `json:"services"`
	Proxy    ProxyState                           `json:"proxy"`
	// PortAssignments maps "branch:service" -> port.
	PortAssignments map[string]int `json:"port_assignments"`
}

// FileStore manages reading and writing state to a JSON file with file locking.
type FileStore struct {
	dir      string
	filePath string
	lockPath string
}

// NewFileStore creates a new FileStore. The dir is typically .portree/ under the main worktree root.
func NewFileStore(dir string) (*FileStore, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("creating state directory: %w", err)
	}
	return &FileStore{
		dir:      dir,
		filePath: filepath.Join(dir, "state.json"),
		lockPath: filepath.Join(dir, "state.lock"),
	}, nil
}

// Load reads the state from disk. Returns an empty state if the file doesn't exist.
func (s *FileStore) Load() (*State, error) {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return emptyState(), nil
		}
		return nil, fmt.Errorf("reading state: %w", err)
	}

	var st State
	if err := json.Unmarshal(data, &st); err != nil {
		logging.Warn("corrupt state file, starting fresh: %v", err)
		return emptyState(), nil
	}
	if st.Services == nil {
		st.Services = map[string]map[string]*ServiceState{}
	}
	if st.PortAssignments == nil {
		st.PortAssignments = map[string]int{}
	}
	return &st, nil
}

// Save writes the state to disk.
func (s *FileStore) Save(st *State) error {
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling state: %w", err)
	}
	return os.WriteFile(s.filePath, data, 0600)
}

// WithLock executes fn while holding an exclusive file lock.
func (s *FileStore) WithLock(fn func() error) error {
	f, err := os.OpenFile(s.lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return fmt.Errorf("opening lock file: %w", err)
	}
	defer func() { _ = f.Close() }()

	deadline := time.Now().Add(lockTimeout)
	for {
		err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			break
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) {
			return fmt.Errorf("acquiring lock: %w", err)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("acquiring lock: timed out after %v", lockTimeout)
		}
		time.Sleep(50 * time.Millisecond)
	}
	defer func() { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN) }()

	return fn()
}

// Dir returns the state directory path.
func (s *FileStore) Dir() string {
	return s.dir
}

// SetServiceState updates the state for a specific branch and service.
func SetServiceState(st *State, branch, service string, ss *ServiceState) {
	if st.Services[branch] == nil {
		st.Services[branch] = map[string]*ServiceState{}
	}
	st.Services[branch][service] = ss
}

// GetServiceState returns the state for a specific branch and service, or nil.
func GetServiceState(st *State, branch, service string) *ServiceState {
	if m, ok := st.Services[branch]; ok {
		return m[service]
	}
	return nil
}

// PortKey returns the state key for a branch+service port assignment.
func PortKey(branch, service string) string {
	return branch + ":" + service
}

// ParsePortKey splits a port key back into branch and service.
// Returns the original key as branch with an empty service if no separator is found.
func ParsePortKey(key string) (branch, service string) {
	if idx := strings.Index(key, ":"); idx >= 0 {
		return key[:idx], key[idx+1:]
	}
	return key, ""
}

// SetPortAssignment records a port assignment.
func SetPortAssignment(st *State, branch, service string, port int) {
	st.PortAssignments[PortKey(branch, service)] = port
}

// GetPortAssignment returns the assigned port, or 0 if not found.
func GetPortAssignment(st *State, branch, service string) int {
	return st.PortAssignments[PortKey(branch, service)]
}

// RunningServiceState creates a running ServiceState.
func RunningServiceState(port, pid int) *ServiceState {
	return &ServiceState{
		Port:      port,
		PID:       pid,
		Status:    StatusRunning,
		StartedAt: time.Now().Format(time.RFC3339),
	}
}

// StoppedServiceState creates a stopped ServiceState.
func StoppedServiceState(port int) *ServiceState {
	return &ServiceState{
		Port:   port,
		Status: StatusStopped,
	}
}

// OrphanedBranches returns branches present in state but not in the given set of active branches.
func OrphanedBranches(st *State, activeBranches map[string]bool) []string {
	var orphaned []string
	for branch := range st.Services {
		if !activeBranches[branch] {
			orphaned = append(orphaned, branch)
		}
	}
	return orphaned
}

func emptyState() *State {
	return &State{
		Services:        map[string]map[string]*ServiceState{},
		PortAssignments: map[string]int{},
	}
}
