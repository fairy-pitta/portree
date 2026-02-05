package state

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// --- Pure function tests ---

func TestPortKey(t *testing.T) {
	tests := []struct {
		name    string
		branch  string
		service string
		want    string
	}{
		{"simple", "main", "web", "main:web"},
		{"slash in branch", "feature/auth", "api", "feature/auth:api"},
		{"empty", "", "", ":"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PortKey(tt.branch, tt.service)
			if got != tt.want {
				t.Errorf("PortKey(%q, %q) = %q, want %q", tt.branch, tt.service, got, tt.want)
			}
		})
	}
}

func TestParsePortKey(t *testing.T) {
	tests := []struct {
		key     string
		branch  string
		service string
	}{
		{"main:web", "main", "web"},
		{"feature/auth:api", "feature/auth", "api"},
		{"no-separator", "no-separator", ""},
		{":", "", ""},
		{"a:b:c", "a", "b:c"},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			b, s := ParsePortKey(tt.key)
			if b != tt.branch || s != tt.service {
				t.Errorf("ParsePortKey(%q) = (%q, %q), want (%q, %q)",
					tt.key, b, s, tt.branch, tt.service)
			}
		})
	}
}

func TestOrphanedBranches(t *testing.T) {
	t.Run("detects orphans", func(t *testing.T) {
		st := emptyState()
		SetServiceState(st, "main", "web", &ServiceState{Port: 3100})
		SetServiceState(st, "stale-branch", "web", &ServiceState{Port: 3200})
		SetServiceState(st, "another-stale", "api", &ServiceState{Port: 3300})

		active := map[string]bool{"main": true}
		orphaned := OrphanedBranches(st, active)

		if len(orphaned) != 2 {
			t.Fatalf("OrphanedBranches() len = %d, want 2", len(orphaned))
		}
	})

	t.Run("no orphans", func(t *testing.T) {
		st := emptyState()
		SetServiceState(st, "main", "web", &ServiceState{Port: 3100})

		active := map[string]bool{"main": true}
		orphaned := OrphanedBranches(st, active)

		if len(orphaned) != 0 {
			t.Fatalf("OrphanedBranches() len = %d, want 0", len(orphaned))
		}
	})

	t.Run("empty state", func(t *testing.T) {
		st := emptyState()
		active := map[string]bool{"main": true}
		orphaned := OrphanedBranches(st, active)

		if len(orphaned) != 0 {
			t.Fatalf("OrphanedBranches() len = %d, want 0", len(orphaned))
		}
	})
}

func TestSetAndGetServiceState(t *testing.T) {
	t.Run("set and get", func(t *testing.T) {
		st := emptyState()
		ss := &ServiceState{Port: 3100, PID: 1234, Status: StatusRunning}
		SetServiceState(st, "main", "web", ss)

		got := GetServiceState(st, "main", "web")
		if got == nil {
			t.Fatal("GetServiceState returned nil")
		}
		if got.Port != 3100 || got.PID != 1234 || got.Status != StatusRunning {
			t.Errorf("GetServiceState = %+v, want port=3100 pid=1234 status=running", got)
		}
	})

	t.Run("nonexistent", func(t *testing.T) {
		st := emptyState()
		got := GetServiceState(st, "main", "web")
		if got != nil {
			t.Errorf("GetServiceState for nonexistent = %+v, want nil", got)
		}
	})

	t.Run("overwrite", func(t *testing.T) {
		st := emptyState()
		SetServiceState(st, "main", "web", &ServiceState{Port: 3100})
		SetServiceState(st, "main", "web", &ServiceState{Port: 3200})
		got := GetServiceState(st, "main", "web")
		if got.Port != 3200 {
			t.Errorf("after overwrite, port = %d, want 3200", got.Port)
		}
	})

	t.Run("nil map init", func(t *testing.T) {
		st := &State{
			Services:        map[string]map[string]*ServiceState{},
			PortAssignments: map[string]int{},
		}
		SetServiceState(st, "new-branch", "web", &ServiceState{Port: 3100})
		got := GetServiceState(st, "new-branch", "web")
		if got == nil || got.Port != 3100 {
			t.Error("SetServiceState should initialize nil inner map")
		}
	})
}

func TestSetAndGetPortAssignment(t *testing.T) {
	t.Run("set and get", func(t *testing.T) {
		st := emptyState()
		SetPortAssignment(st, "main", "web", 3150)
		got := GetPortAssignment(st, "main", "web")
		if got != 3150 {
			t.Errorf("GetPortAssignment = %d, want 3150", got)
		}
	})

	t.Run("unassigned", func(t *testing.T) {
		st := emptyState()
		got := GetPortAssignment(st, "main", "web")
		if got != 0 {
			t.Errorf("GetPortAssignment for unassigned = %d, want 0", got)
		}
	})
}

func TestRunningServiceState(t *testing.T) {
	before := time.Now().Add(-time.Second)
	ss := RunningServiceState(3100, 1234)
	after := time.Now().Add(time.Second)

	if ss.Port != 3100 {
		t.Errorf("Port = %d, want 3100", ss.Port)
	}
	if ss.PID != 1234 {
		t.Errorf("PID = %d, want 1234", ss.PID)
	}
	if ss.Status != StatusRunning {
		t.Errorf("Status = %q, want %q", ss.Status, StatusRunning)
	}

	ts, err := time.Parse(time.RFC3339, ss.StartedAt)
	if err != nil {
		t.Fatalf("StartedAt %q is not valid RFC3339: %v", ss.StartedAt, err)
	}
	if ts.Before(before) || ts.After(after) {
		t.Errorf("StartedAt %v not between %v and %v", ts, before, after)
	}
}

func TestStoppedServiceState(t *testing.T) {
	ss := StoppedServiceState(3100)

	if ss.Port != 3100 {
		t.Errorf("Port = %d, want 3100", ss.Port)
	}
	if ss.Status != StatusStopped {
		t.Errorf("Status = %q, want %q", ss.Status, StatusStopped)
	}
	if ss.PID != 0 {
		t.Errorf("PID = %d, want 0", ss.PID)
	}
}

// --- Filesystem tests ---

func TestNewFileStore(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "portree-state")
	store, err := NewFileStore(dir)
	if err != nil {
		t.Fatalf("NewFileStore() error: %v", err)
	}

	if store.Dir() != dir {
		t.Errorf("Dir() = %q, want %q", store.Dir(), dir)
	}
}

func TestFileStoreLoadEmpty(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	st, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if st.Services == nil {
		t.Error("Load() returned nil Services map")
	}
	if st.PortAssignments == nil {
		t.Error("Load() returned nil PortAssignments map")
	}
	if len(st.Services) != 0 {
		t.Errorf("Load() Services len = %d, want 0", len(st.Services))
	}
}

func TestFileStoreSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	st := emptyState()
	SetServiceState(st, "main", "web", &ServiceState{Port: 3100, PID: 42, Status: StatusRunning, StartedAt: "2025-01-01T00:00:00Z"})
	SetPortAssignment(st, "main", "web", 3100)
	SetPortAssignment(st, "feature/auth", "api", 8150)

	if err := store.Save(st); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	ss := GetServiceState(loaded, "main", "web")
	if ss == nil {
		t.Fatal("loaded state missing main/web service")
	}
	if ss.Port != 3100 || ss.PID != 42 || ss.Status != StatusRunning {
		t.Errorf("loaded service state = %+v", ss)
	}

	if GetPortAssignment(loaded, "main", "web") != 3100 {
		t.Error("loaded port assignment for main:web != 3100")
	}
	if GetPortAssignment(loaded, "feature/auth", "api") != 8150 {
		t.Error("loaded port assignment for feature/auth:api != 8150")
	}
}

func TestFileStoreWithLock(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("fn called", func(t *testing.T) {
		called := false
		err := store.WithLock(func() error {
			called = true
			return nil
		})
		if err != nil {
			t.Fatalf("WithLock() error: %v", err)
		}
		if !called {
			t.Error("WithLock fn was not called")
		}
	})

	t.Run("error propagated", func(t *testing.T) {
		sentinel := &testError{}
		err := store.WithLock(func() error {
			return sentinel
		})
		if !errors.Is(err, sentinel) {
			t.Errorf("WithLock() error = %v, want sentinel", err)
		}
	})
}

type testError struct{}

func (e *testError) Error() string { return "test error" }

func TestFileStoreLoadCorruptFile(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Write corrupt JSON
	if err := os.WriteFile(filepath.Join(dir, "state.json"), []byte("{invalid json"), 0600); err != nil {
		t.Fatal(err)
	}

	// Load should return empty state (not error)
	st, err := store.Load()
	if err != nil {
		t.Fatalf("Load() with corrupt file should not error, got: %v", err)
	}
	if st == nil {
		t.Fatal("Load() returned nil state")
	}
	if len(st.Services) != 0 {
		t.Errorf("corrupt file should give empty services, got %d", len(st.Services))
	}
}

func TestFileStoreLoadEmptyFile(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Write empty file
	if err := os.WriteFile(filepath.Join(dir, "state.json"), []byte(""), 0600); err != nil {
		t.Fatal(err)
	}

	st, err := store.Load()
	if err != nil {
		t.Fatalf("Load() with empty file should not error, got: %v", err)
	}
	if st == nil {
		t.Fatal("Load() returned nil state")
	}
}

func TestFileStoreLoadNullMaps(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Write valid JSON but with null maps
	if err := os.WriteFile(filepath.Join(dir, "state.json"), []byte(`{"services":null,"proxy":{},"port_assignments":null}`), 0600); err != nil {
		t.Fatal(err)
	}

	st, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if st.Services == nil {
		t.Error("Services should be initialized even if null in JSON")
	}
	if st.PortAssignments == nil {
		t.Error("PortAssignments should be initialized even if null in JSON")
	}
}

func TestFileStoreSavePermissions(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	st := emptyState()
	if err := store.Save(st); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(filepath.Join(dir, "state.json"))
	if err != nil {
		t.Fatal(err)
	}

	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Errorf("state.json permissions = %o, want 0600", perm)
	}
}

func TestWithLockConcurrent(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Run multiple goroutines that all increment a counter via WithLock
	const n = 10
	done := make(chan bool, n)

	for i := 0; i < n; i++ {
		go func() {
			err := store.WithLock(func() error {
				st, e := store.Load()
				if e != nil {
					return e
				}
				port := GetPortAssignment(st, "main", "web")
				SetPortAssignment(st, "main", "web", port+1)
				return store.Save(st)
			})
			done <- (err == nil)
		}()
	}

	for i := 0; i < n; i++ {
		if !<-done {
			t.Error("a WithLock goroutine failed")
		}
	}

	// Verify final value
	st, _ := store.Load()
	port := GetPortAssignment(st, "main", "web")
	if port != n {
		t.Errorf("after %d increments, port = %d", n, port)
	}
}
