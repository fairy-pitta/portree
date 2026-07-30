package port

import (
	"os"
	"syscall"

	"github.com/fairy-pitta/portree/internal/config"
	"github.com/fairy-pitta/portree/internal/state"
)

// Registry manages port assignments backed by state.
type Registry struct {
	store *state.FileStore
	cfg   *config.Config
}

// NewRegistry creates a new port Registry.
func NewRegistry(store *state.FileStore, cfg *config.Config) *Registry {
	return &Registry{store: store, cfg: cfg}
}

// AssignPort allocates a port for the given branch and service.
// If a port was previously assigned and is still valid, it is reused.
func (r *Registry) AssignPort(branch, service string) (int, error) {
	var port int
	err := r.store.WithLock(func() error {
		st, err := r.store.Load()
		if err != nil {
			return err
		}

		// Check for existing assignment. Reuse it only if it is still usable:
		// either our own service is currently running on it, or the port is
		// actually free. A stale assignment can collide with a listener that
		// appeared since (e.g. another repo's process), so blind reuse would
		// hand out a dead-on-arrival port.
		existing := state.GetPortAssignment(st, branch, service)
		if existing > 0 {
			if ownsRunningService(st, branch, service, existing) || IsFree(existing) {
				port = existing
				return nil
			}
			delete(st.PortAssignments, state.PortKey(branch, service))
		}

		// Check for fixed port override.
		fixedPort := r.cfg.FixedPortForBranch(service, branch)

		// Build used ports set.
		used := make(map[int]bool, len(st.PortAssignments))
		for _, p := range st.PortAssignments {
			used[p] = true
		}

		svc := r.cfg.Services[service]
		allocated, err := Allocate(branch, service, svc, fixedPort, used)
		if err != nil {
			return err
		}

		state.SetPortAssignment(st, branch, service, allocated)
		port = allocated
		return r.store.Save(st)
	})
	return port, err
}

// GetPort returns the currently assigned port for a branch+service, or 0.
func (r *Registry) GetPort(branch, service string) (int, error) {
	var port int
	err := r.store.WithLock(func() error {
		st, err := r.store.Load()
		if err != nil {
			return err
		}
		port = state.GetPortAssignment(st, branch, service)
		return nil
	})
	return port, err
}

// Release removes the port assignment for a branch+service.
func (r *Registry) Release(branch, service string) error {
	return r.store.WithLock(func() error {
		st, err := r.store.Load()
		if err != nil {
			return err
		}
		delete(st.PortAssignments, state.PortKey(branch, service))
		return r.store.Save(st)
	})
}

// ownsRunningService reports whether the branch's own service is currently
// running (live PID) on the given port, in which case the assignment is
// legitimately "in use by us" and must be reused as-is.
func ownsRunningService(st *state.State, branch, service string, port int) bool {
	ss := state.GetServiceState(st, branch, service)
	return ss != nil &&
		ss.Status == state.StatusRunning &&
		ss.Port == port &&
		ss.PID > 0 &&
		isProcessRunning(ss.PID)
}

// isProcessRunning checks PID liveness. Duplicated from the process package
// because importing it here would create an import cycle (process -> port).
func isProcessRunning(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}
