package process

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestBuildEnv(t *testing.T) {
	runner := &Runner{
		config: RunnerConfig{
			ServiceName: "web",
			Branch:      "feature/auth",
			BranchSlug:  "feature-auth",
			Command:     "npm start",
			Dir:         "/tmp/project",
			Port:        3150,
			Env: map[string]string{
				"NODE_ENV": "development",
				"DEBUG":    "true",
			},
			AllServicePorts: map[string]int{
				"web": 3150,
				"api": 8150,
			},
			AllServiceProxyPorts: map[string]int{
				"web": 3000,
				"api": 8000,
			},
		},
	}

	env := runner.buildEnv()

	lookup := make(map[string]string)
	for _, e := range env {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			lookup[parts[0]] = parts[1]
		}
	}

	t.Run("PORT", func(t *testing.T) {
		if lookup["PORT"] != "3150" {
			t.Errorf("PORT = %q, want %q", lookup["PORT"], "3150")
		}
	})

	t.Run("PT_BRANCH", func(t *testing.T) {
		if lookup["PT_BRANCH"] != "feature/auth" {
			t.Errorf("PT_BRANCH = %q, want %q", lookup["PT_BRANCH"], "feature/auth")
		}
	})

	t.Run("PT_BRANCH_SLUG", func(t *testing.T) {
		if lookup["PT_BRANCH_SLUG"] != "feature-auth" {
			t.Errorf("PT_BRANCH_SLUG = %q, want %q", lookup["PT_BRANCH_SLUG"], "feature-auth")
		}
	})

	t.Run("PT_SERVICE", func(t *testing.T) {
		if lookup["PT_SERVICE"] != "web" {
			t.Errorf("PT_SERVICE = %q, want %q", lookup["PT_SERVICE"], "web")
		}
	})

	t.Run("custom env", func(t *testing.T) {
		if lookup["NODE_ENV"] != "development" {
			t.Errorf("NODE_ENV = %q, want %q", lookup["NODE_ENV"], "development")
		}
		if lookup["DEBUG"] != "true" {
			t.Errorf("DEBUG = %q, want %q", lookup["DEBUG"], "true")
		}
	})

	t.Run("cross-service ports", func(t *testing.T) {
		if lookup["PT_WEB_PORT"] != "3150" {
			t.Errorf("PT_WEB_PORT = %q, want %q", lookup["PT_WEB_PORT"], "3150")
		}
		if lookup["PT_API_PORT"] != "8150" {
			t.Errorf("PT_API_PORT = %q, want %q", lookup["PT_API_PORT"], "8150")
		}
	})

	t.Run("cross-service URLs", func(t *testing.T) {
		if lookup["PT_WEB_URL"] != "http://feature-auth.localhost:3000" {
			t.Errorf("PT_WEB_URL = %q, want %q", lookup["PT_WEB_URL"], "http://feature-auth.localhost:3000")
		}
		if lookup["PT_API_URL"] != "http://feature-auth.localhost:8000" {
			t.Errorf("PT_API_URL = %q, want %q", lookup["PT_API_URL"], "http://feature-auth.localhost:8000")
		}
	})
}

func newTestRunner(t *testing.T, command string) *Runner {
	t.Helper()
	logDir := t.TempDir()
	return NewRunner(RunnerConfig{
		ServiceName: "test-svc",
		Branch:      "main",
		BranchSlug:  "main",
		Command:     command,
		Dir:         t.TempDir(),
		Port:        9999,
		Env:         map[string]string{},
		LogDir:      logDir,
	})
}

func TestNewRunner(t *testing.T) {
	r := NewRunner(RunnerConfig{ServiceName: "web"})
	if r.cmd != nil {
		t.Error("expected cmd to be nil before Start")
	}
	if r.done != nil {
		t.Error("expected done to be nil before Start")
	}
	if r.PID() != 0 {
		t.Errorf("PID() = %d, want 0", r.PID())
	}
	if r.IsRunning() {
		t.Error("expected IsRunning() = false before Start")
	}
}

func TestRunnerStartStop(t *testing.T) {
	r := newTestRunner(t, "sleep 60")

	pid, err := r.Start()
	if err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	if pid <= 0 {
		t.Fatalf("Start() returned invalid PID: %d", pid)
	}
	if r.PID() != pid {
		t.Errorf("PID() = %d, want %d", r.PID(), pid)
	}
	if !r.IsRunning() {
		t.Error("expected IsRunning() = true after Start")
	}

	err = r.Stop()
	if err != nil {
		t.Fatalf("Stop() error: %v", err)
	}

	// After stop, process should be dead.
	// Give a moment for OS to clean up.
	time.Sleep(100 * time.Millisecond)
	if r.IsRunning() {
		t.Error("expected IsRunning() = false after Stop")
	}
}

func TestRunnerDoneChannel(t *testing.T) {
	// Use a command that exits quickly.
	r := newTestRunner(t, "echo done")

	_, err := r.Start()
	if err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	ch := r.Done()
	if ch == nil {
		t.Fatal("Done() returned nil after Start")
	}

	// The echo command should exit quickly.
	select {
	case <-ch:
		// success
	case <-time.After(5 * time.Second):
		t.Fatal("Done() channel not closed after process exited")
	}
}

func TestRunnerDoneChannelNilBeforeStart(t *testing.T) {
	r := NewRunner(RunnerConfig{ServiceName: "test"})
	ch := r.Done()
	if ch != nil {
		t.Error("Done() should return nil before Start is called")
	}
}

func TestRunnerStopBeforeStart(t *testing.T) {
	r := NewRunner(RunnerConfig{ServiceName: "test"})
	// Stop on an unstarted runner should be a no-op, not panic.
	err := r.Stop()
	if err != nil {
		t.Errorf("Stop() before Start should return nil, got: %v", err)
	}
}

func TestRunnerDoubleStart(t *testing.T) {
	r := newTestRunner(t, "sleep 60")
	defer r.Stop()

	_, err := r.Start()
	if err != nil {
		t.Fatalf("First Start() error: %v", err)
	}

	// Second Start while running should error.
	_, err = r.Start()
	if err == nil {
		t.Error("expected error from second Start() while running")
	}
}

func TestIsProcessRunning(t *testing.T) {
	t.Run("current process", func(t *testing.T) {
		if !IsProcessRunning(os.Getpid()) {
			t.Error("expected current process to be running")
		}
	})

	t.Run("zero pid", func(t *testing.T) {
		if IsProcessRunning(0) {
			t.Error("expected PID 0 to not be running")
		}
	})

	t.Run("negative pid", func(t *testing.T) {
		if IsProcessRunning(-1) {
			t.Error("expected negative PID to not be running")
		}
	})

	t.Run("nonexistent pid", func(t *testing.T) {
		// PID 99999999 is almost certainly not running.
		if IsProcessRunning(99999999) {
			t.Error("expected PID 99999999 to not be running")
		}
	})
}

func TestStopPID(t *testing.T) {
	t.Run("already dead pid", func(t *testing.T) {
		// Stopping a non-existent PID should be a no-op.
		err := StopPID(99999999)
		if err != nil {
			t.Errorf("StopPID(nonexistent) = %v, want nil", err)
		}
	})
}
