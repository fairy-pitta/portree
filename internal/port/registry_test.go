package port

import (
	"fmt"
	"net"
	"os"
	"testing"

	"github.com/fairy-pitta/portree/internal/config"
	"github.com/fairy-pitta/portree/internal/state"
)

func newTestRegistry(t *testing.T) *Registry {
	t.Helper()
	dir := t.TempDir()
	store, err := state.NewFileStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Services: map[string]config.ServiceConfig{
			"web": {
				Command:   "npm start",
				PortRange: config.PortRange{Min: 3100, Max: 3199},
				ProxyPort: 3000,
			},
		},
		Env:       map[string]string{},
		Worktrees: map[string]config.WTOverride{},
	}

	return NewRegistry(store, cfg)
}

func TestRegistryAssignPort(t *testing.T) {
	t.Run("port in range", func(t *testing.T) {
		reg := newTestRegistry(t)
		port, err := reg.AssignPort("main", "web")
		if err != nil {
			t.Fatalf("AssignPort() error: %v", err)
		}
		if port < 3100 || port > 3199 {
			t.Errorf("AssignPort() = %d, not in [3100, 3199]", port)
		}
	})

	t.Run("idempotent", func(t *testing.T) {
		reg := newTestRegistry(t)
		first, err := reg.AssignPort("main", "web")
		if err != nil {
			t.Fatal(err)
		}
		second, err := reg.AssignPort("main", "web")
		if err != nil {
			t.Fatal(err)
		}
		if first != second {
			t.Errorf("AssignPort not idempotent: %d != %d", first, second)
		}
	})

	t.Run("different pairs differ", func(t *testing.T) {
		reg := newTestRegistry(t)
		a, err := reg.AssignPort("main", "web")
		if err != nil {
			t.Fatal(err)
		}
		b, err := reg.AssignPort("feature/auth", "web")
		if err != nil {
			t.Fatal(err)
		}
		if a == b {
			t.Errorf("different branches got same port %d", a)
		}
	})
}

func TestRegistryGetPort(t *testing.T) {
	reg := newTestRegistry(t)

	t.Run("before assignment", func(t *testing.T) {
		port, err := reg.GetPort("main", "web")
		if err != nil {
			t.Fatal(err)
		}
		if port != 0 {
			t.Errorf("GetPort before assign = %d, want 0", port)
		}
	})

	t.Run("after assignment", func(t *testing.T) {
		assigned, err := reg.AssignPort("main", "web")
		if err != nil {
			t.Fatal(err)
		}
		got, err := reg.GetPort("main", "web")
		if err != nil {
			t.Fatal(err)
		}
		if got != assigned {
			t.Errorf("GetPort = %d, want %d", got, assigned)
		}
	})
}

func TestRegistryRelease(t *testing.T) {
	reg := newTestRegistry(t)

	assigned, err := reg.AssignPort("main", "web")
	if err != nil {
		t.Fatal(err)
	}
	if assigned == 0 {
		t.Fatal("AssignPort returned 0")
	}

	t.Run("release clears", func(t *testing.T) {
		if err := reg.Release("main", "web"); err != nil {
			t.Fatalf("Release() error: %v", err)
		}
		port, err := reg.GetPort("main", "web")
		if err != nil {
			t.Fatal(err)
		}
		if port != 0 {
			t.Errorf("GetPort after Release = %d, want 0", port)
		}
	})

	t.Run("re-assign works", func(t *testing.T) {
		port, err := reg.AssignPort("main", "web")
		if err != nil {
			t.Fatalf("AssignPort after Release error: %v", err)
		}
		if port < 3100 || port > 3199 {
			t.Errorf("re-assigned port = %d, not in range", port)
		}
	})
}

func TestRegistryStaleAssignmentReallocated(t *testing.T) {
	reg := newTestRegistry(t)

	first, err := reg.AssignPort("feature", "web")
	if err != nil {
		t.Fatal(err)
	}

	// Simulate a foreign process (e.g. another worktree's stack) grabbing the
	// assigned port while our service is down. Note no service state is
	// recorded for "feature", so the assignment is not "ours and running".
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", first))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()

	second, err := reg.AssignPort("feature", "web")
	if err != nil {
		t.Fatal(err)
	}
	if second == first {
		t.Errorf("AssignPort reused port %d despite a foreign listener on it", first)
	}
}

func TestRegistryOwnRunningServiceKeepsPort(t *testing.T) {
	dir := t.TempDir()
	store, err := state.NewFileStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		Services: map[string]config.ServiceConfig{
			"web": {PortRange: config.PortRange{Min: 3100, Max: 3199}},
		},
	}
	reg := NewRegistry(store, cfg)

	first, err := reg.AssignPort("feature", "web")
	if err != nil {
		t.Fatal(err)
	}

	// Our own service is running on the port (this test process stands in for
	// it: live PID, port held by a listener).
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", first))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	if err := store.WithLock(func() error {
		st, e := store.Load()
		if e != nil {
			return e
		}
		state.SetServiceState(st, "feature", "web", state.RunningServiceState(first, os.Getpid()))
		return store.Save(st)
	}); err != nil {
		t.Fatal(err)
	}

	second, err := reg.AssignPort("feature", "web")
	if err != nil {
		t.Fatal(err)
	}
	if second != first {
		t.Errorf("AssignPort moved a running service from port %d to %d", first, second)
	}
}
