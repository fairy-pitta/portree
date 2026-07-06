package process

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/fairy-pitta/portree/internal/logging"
	"github.com/fairy-pitta/portree/internal/port"
)

const stopTimeout = 10 * time.Second

// RunnerConfig contains all parameters needed to start a process.
type RunnerConfig struct {
	ServiceName string
	Branch      string
	BranchSlug  string
	Command     string
	Dir         string // absolute working directory
	Port        int
	Env         map[string]string // merged environment variables
	LogDir      string            // directory for log files
	// AllServicePorts maps service name -> assigned port for cross-service env vars.
	AllServicePorts map[string]int
	// AllServiceProxyPorts maps service name -> proxy port for URL env vars.
	AllServiceProxyPorts map[string]int
	// ProxyScheme is "http" or "https" for PT_*_URL env vars.
	ProxyScheme string
}

// Runner manages a single child process.
type Runner struct {
	config  RunnerConfig
	cmd     *exec.Cmd
	logFile *os.File
	done    chan struct{} // closed when the process exits
}

// NewRunner creates a new Runner.
func NewRunner(cfg RunnerConfig) *Runner {
	return &Runner{config: cfg}
}

// Start launches the process.
// Child processes are intentionally detached and survive CLI exit so that
// development servers keep running after the portree command returns.
// Use `portree down` to stop them.
func (r *Runner) Start() (int, error) {
	if r.cmd != nil && r.cmd.Process != nil {
		if r.IsRunning() {
			return 0, fmt.Errorf("service %s is already running (pid %d)", r.config.ServiceName, r.cmd.Process.Pid)
		}
	}

	// Ensure log directory exists.
	if err := os.MkdirAll(r.config.LogDir, 0700); err != nil {
		return 0, fmt.Errorf("creating log dir: %w", err)
	}

	logPath := filepath.Join(r.config.LogDir, LogFileName(r.config.BranchSlug, r.config.ServiceName))
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
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

	// Initialize the done channel before Start so that Stop can never
	// encounter a nil channel, even if a panic occurs between Start and
	// the goroutine launch.
	r.done = make(chan struct{})

	if err := r.cmd.Start(); err != nil {
		close(r.done)
		_ = f.Close()
		return 0, fmt.Errorf("starting %s: %w", r.config.ServiceName, err)
	}

	// Track process exit via a single Wait call to avoid the race of calling
	// Wait() twice on the same exec.Cmd.
	go func() {
		_ = r.cmd.Wait()
		close(r.done)
	}()

	return r.cmd.Process.Pid, nil
}

// Stop sends SIGTERM then SIGKILL to the process group.
func (r *Runner) Stop() error {
	if r.logFile != nil {
		defer func() { _ = r.logFile.Close() }()
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
	if err := syscall.Kill(-pgid, syscall.SIGTERM); err != nil {
		logging.Warn("failed to send SIGTERM to process group %d: %v", pgid, err)
	}

	// Reuse the done channel from Start instead of calling Wait again.
	select {
	case <-r.done:
		return nil
	case <-time.After(stopTimeout):
		// Force kill the process group.
		if err := syscall.Kill(-pgid, syscall.SIGKILL); err != nil {
			logging.Warn("failed to send SIGKILL to process group %d: %v", pgid, err)
		}
		return nil
	}
}

// Done returns a channel that is closed when the process exits.
// Returns nil if Start has not been called yet.
func (r *Runner) Done() <-chan struct{} {
	return r.done
}

// StopPID stops a process by PID (used for stale processes from state).
func StopPID(pid int) error {
	pgid, err := syscall.Getpgid(pid)
	if err != nil {
		return nil // already dead
	}

	if err := syscall.Kill(-pgid, syscall.SIGTERM); err != nil {
		logging.Warn("failed to send SIGTERM to process group %d: %v", pgid, err)
	}

	// Poll briefly for process exit, then force kill.
	for i := 0; i < 30; i++ {
		time.Sleep(100 * time.Millisecond)
		if !IsProcessRunning(pid) {
			return nil
		}
	}
	if err := syscall.Kill(-pgid, syscall.SIGKILL); err != nil {
		logging.Warn("failed to send SIGKILL to process group %d: %v", pgid, err)
	}
	for i := 0; i < 5; i++ {
		time.Sleep(50 * time.Millisecond)
		if !IsProcessRunning(pid) {
			return nil
		}
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

// IsPortAvailable checks if a TCP port is available for binding.
// See port.IsFree for the probe semantics.
func IsPortAvailable(p int) bool {
	return port.IsFree(p)
}

// buildEnv constructs the full environment for the child process.
func (r *Runner) buildEnv() []string {
	env := os.Environ()

	// Build a lookup of the auto-injected portree vars first so config env
	// values can interpolate against them (e.g. "${PT_API_URL}").
	scheme := r.config.ProxyScheme
	if scheme == "" {
		scheme = "http"
	}
	injected := map[string]string{
		"PORT":           fmt.Sprintf("%d", r.config.Port),
		"PT_BRANCH":      r.config.Branch,
		"PT_BRANCH_SLUG": r.config.BranchSlug,
		"PT_SERVICE":     r.config.ServiceName,
	}
	for svcName, svcPort := range r.config.AllServicePorts {
		injected["PT_"+strings.ToUpper(svcName)+"_PORT"] = fmt.Sprintf("%d", svcPort)
	}
	for svcName, proxyPort := range r.config.AllServiceProxyPorts {
		injected["PT_"+strings.ToUpper(svcName)+"_URL"] = fmt.Sprintf("%s://%s.localhost:%d", scheme, r.config.BranchSlug, proxyPort)
	}

	// Expand ${VAR} references in config env values against the injected vars,
	// falling back to the process environment so ${HOME} and similar still
	// resolve. Only the explicit ${...} form is interpolated; a bare "$" is
	// left literal so existing values such as passwords ("p$ssw0rd") survive
	// byte-for-byte.
	expand := func(name string) string {
		if v, ok := injected[name]; ok {
			return v
		}
		return os.Getenv(name)
	}

	// Add global and worktree-override env vars (interpolated).
	for k, v := range r.config.Env {
		if strings.ContainsRune(k, 0) || strings.ContainsRune(v, 0) {
			logging.Warn("skipping env var %q: contains null byte", k)
			continue
		}
		env = append(env, k+"="+expandBraces(v, expand))
	}

	// Add portree auto-injected vars last so built-ins remain authoritative.
	env = append(env,
		fmt.Sprintf("PORT=%d", r.config.Port),
		fmt.Sprintf("PT_BRANCH=%s", r.config.Branch),
		fmt.Sprintf("PT_BRANCH_SLUG=%s", r.config.BranchSlug),
		fmt.Sprintf("PT_SERVICE=%s", r.config.ServiceName),
	)
	for svcName, svcPort := range r.config.AllServicePorts {
		env = append(env, fmt.Sprintf("PT_%s_PORT=%d", strings.ToUpper(svcName), svcPort))
	}
	for svcName, proxyPort := range r.config.AllServiceProxyPorts {
		env = append(env, fmt.Sprintf("PT_%s_URL=%s://%s.localhost:%d", strings.ToUpper(svcName), scheme, r.config.BranchSlug, proxyPort))
	}

	return env
}

// expandBraces expands ${VAR} references in s using mapping, leaving bare "$"
// characters untouched. Unlike os.Expand, the bare "$VAR" form is NOT
// interpreted, so existing config values containing a literal "$" (e.g.
// passwords like "p$ssw0rd") are preserved exactly. A malformed reference with
// no closing brace is left as-is.
func expandBraces(s string, mapping func(string) string) string {
	if !strings.Contains(s, "${") {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		if s[i] == '$' && i+1 < len(s) && s[i+1] == '{' {
			if end := strings.IndexByte(s[i+2:], '}'); end >= 0 {
				b.WriteString(mapping(s[i+2 : i+2+end]))
				i += 2 + end + 1
				continue
			}
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}
