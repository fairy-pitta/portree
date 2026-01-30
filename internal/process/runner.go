package process

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const stopTimeout = 10 * time.Second

// RunnerConfig contains all parameters needed to start a process.
type RunnerConfig struct {
	ServiceName string
	Branch      string
	BranchSlug  string
	Command     string
	Dir         string            // absolute working directory
	Port        int
	Env         map[string]string // merged environment variables
	LogDir      string            // directory for log files
	// AllServicePorts maps service name -> assigned port for cross-service env vars.
	AllServicePorts map[string]int
	// AllServiceProxyPorts maps service name -> proxy port for URL env vars.
	AllServiceProxyPorts map[string]int
}

// Runner manages a single child process.
type Runner struct {
	config  RunnerConfig
	cmd     *exec.Cmd
	logFile *os.File
}

// NewRunner creates a new Runner.
func NewRunner(cfg RunnerConfig) *Runner {
	return &Runner{config: cfg}
}

// Start launches the process.
func (r *Runner) Start() (int, error) {
	if r.cmd != nil && r.cmd.Process != nil {
		if r.IsRunning() {
			return 0, fmt.Errorf("service %s is already running (pid %d)", r.config.ServiceName, r.cmd.Process.Pid)
		}
	}

	// Ensure log directory exists.
	if err := os.MkdirAll(r.config.LogDir, 0755); err != nil {
		return 0, fmt.Errorf("creating log dir: %w", err)
	}

	logPath := filepath.Join(r.config.LogDir, fmt.Sprintf("%s.%s.log", r.config.BranchSlug, r.config.ServiceName))
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return 0, fmt.Errorf("opening log file: %w", err)
	}
	r.logFile = f

	r.cmd = exec.Command("sh", "-c", r.config.Command)
	r.cmd.Dir = r.config.Dir
	r.cmd.Stdout = f
	r.cmd.Stderr = f
	r.cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	r.cmd.Env = r.buildEnv()

	if err := r.cmd.Start(); err != nil {
		f.Close()
		return 0, fmt.Errorf("starting %s: %w", r.config.ServiceName, err)
	}

	// Detach: don't wait in this goroutine. The process runs independently.
	go r.cmd.Wait()

	return r.cmd.Process.Pid, nil
}

// Stop sends SIGTERM then SIGKILL to the process group.
func (r *Runner) Stop() error {
	if r.logFile != nil {
		defer r.logFile.Close()
	}

	if r.cmd == nil || r.cmd.Process == nil {
		return nil
	}

	pid := r.cmd.Process.Pid
	pgid, err := syscall.Getpgid(pid)
	if err != nil {
		// Process may already be dead.
		return nil
	}

	// Send SIGTERM to the process group.
	syscall.Kill(-pgid, syscall.SIGTERM)

	// Wait for process to exit.
	done := make(chan struct{})
	go func() {
		r.cmd.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-time.After(stopTimeout):
		// Force kill the process group.
		syscall.Kill(-pgid, syscall.SIGKILL)
		return nil
	}
}

// StopPID stops a process by PID (used for stale processes from state).
func StopPID(pid int) error {
	pgid, err := syscall.Getpgid(pid)
	if err != nil {
		return nil // already dead
	}

	syscall.Kill(-pgid, syscall.SIGTERM)

	// Wait briefly then force kill.
	time.Sleep(3 * time.Second)
	if IsProcessRunning(pid) {
		syscall.Kill(-pgid, syscall.SIGKILL)
	}
	return nil
}

// IsRunning checks if the process is still alive.
func (r *Runner) IsRunning() bool {
	if r.cmd == nil || r.cmd.Process == nil {
		return false
	}
	return IsProcessRunning(r.cmd.Process.Pid)
}

// PID returns the process ID, or 0 if not started.
func (r *Runner) PID() int {
	if r.cmd != nil && r.cmd.Process != nil {
		return r.cmd.Process.Pid
	}
	return 0
}

// IsProcessRunning checks if a process with the given PID is alive.
func IsProcessRunning(pid int) bool {
	if pid <= 0 {
		return false
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// buildEnv constructs the full environment for the child process.
func (r *Runner) buildEnv() []string {
	env := os.Environ()

	// Add global and worktree-override env vars.
	for k, v := range r.config.Env {
		env = append(env, k+"="+v)
	}

	// Add GWS auto-injected vars.
	env = append(env,
		fmt.Sprintf("PORT=%d", r.config.Port),
		fmt.Sprintf("GWS_BRANCH=%s", r.config.Branch),
		fmt.Sprintf("GWS_BRANCH_SLUG=%s", r.config.BranchSlug),
		fmt.Sprintf("GWS_SERVICE=%s", r.config.ServiceName),
	)

	// Add cross-service port and URL vars.
	for svcName, svcPort := range r.config.AllServicePorts {
		upper := strings.ToUpper(svcName)
		env = append(env, fmt.Sprintf("GWS_%s_PORT=%d", upper, svcPort))
	}
	for svcName, proxyPort := range r.config.AllServiceProxyPorts {
		upper := strings.ToUpper(svcName)
		env = append(env, fmt.Sprintf("GWS_%s_URL=http://%s.localhost:%d", upper, r.config.BranchSlug, proxyPort))
	}

	return env
}
