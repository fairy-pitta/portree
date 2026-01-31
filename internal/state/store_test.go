package state

import (
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

func TestSetAndGetServiceState(t *testing.T) {
	t.Run("set and get", func(t *testing.T) {
		st := emptyState()
		ss := &ServiceState{Port: 3100, PID: 1234, Status: "running"}
		SetServiceState(st, "main", "web", ss)

		got := GetServiceState(st, "main", "web")
		if got == nil {
			t.Fatal("GetServiceState returned nil")
		}
		if got.Port != 3100 || got.PID != 1234 || got.Status != "running" {
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
	if ss.Status != "running" {
		t.Errorf("Status = %q, want %q", ss.Status, "running")
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
	if ss.Status != "stopped" {
		t.Errorf("Status = %q, want %q", ss.Status, "stopped")
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
	SetServiceState(st, "main", "web", &ServiceState{Port: 3100, PID: 42, Status: "running", StartedAt: "2025-01-01T00:00:00Z"})
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
	if ss.Port != 3100 || ss.PID != 42 || ss.Status != "running" {
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
		if err != sentinel {
			t.Errorf("WithLock() error = %v, want sentinel", err)
		}
	})
}

type testError struct{}

func (e *testError) Error() string { return "test error" }
