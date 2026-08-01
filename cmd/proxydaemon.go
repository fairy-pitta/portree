package cmd

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/fairy-pitta/portree/internal/config"
	"github.com/fairy-pitta/portree/internal/git"
	"github.com/fairy-pitta/portree/internal/process"
	"github.com/fairy-pitta/portree/internal/state"
)

const (
	// proxyReadyTimeout bounds how long a detached start waits for the proxy to
	// accept connections before giving up.
	proxyReadyTimeout = 5 * time.Second
	// proxyReadyPoll is the interval between readiness probes.
	proxyReadyPoll = 50 * time.Millisecond
	// proxyLogTailLines is how much of the log to show when a detached proxy
	// dies before it is ready.
	proxyLogTailLines = 20
	// proxyLogName is the log file a detached proxy writes to.
	proxyLogName = "proxy.log"
)

// proxyStatus is a snapshot of the proxy for display and for deciding whether
// anything needs starting.
type proxyStatus struct {
	Running bool
	// Stale is true when state claims the proxy runs but its PID is gone.
	Stale  bool
	PID    int
	Scheme string
	Ports  []int
}

// describeProxy reports whether the recorded proxy is actually serving. State
// alone is not enough: a proxy killed without a chance to update state leaves a
// "running" record behind, and acting on that is how a browser ends up pointed
// at a URL nothing answers.
func describeProxy(cfg *config.Config, p state.ProxyState) proxyStatus {
	st := proxyStatus{PID: p.PID, Scheme: "http"}
	if p.HTTPS {
		st.Scheme = "https"
	}

	if cfg != nil {
		seen := map[int]bool{}
		for _, svc := range cfg.Services {
			if !seen[svc.ProxyPort] {
				seen[svc.ProxyPort] = true
				st.Ports = append(st.Ports, svc.ProxyPort)
			}
		}
		sort.Ints(st.Ports)
	}

	claimsRunning := p.Status == state.StatusRunning
	alive := p.PID > 0 && process.IsProcessRunning(p.PID)

	switch {
	case claimsRunning && alive:
		st.Running = true
	case claimsRunning:
		st.Stale = true
	}
	return st
}

// portAccepting reports whether something accepts TCP connections on the port.
// Unlike a bind probe this confirms a server is actually there, which is the
// question "is the proxy usable right now" really asks.
func portAccepting(p int) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(p)), 300*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// waitForProxyReady blocks until every port accepts connections, the process
// exits, or the timeout elapses. Returning early on exit means a proxy that
// dies on startup is reported as a failure instead of a slow success.
func waitForProxyReady(ports []int, exited <-chan struct{}, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for {
		pending := make([]int, 0, len(ports))
		for _, p := range ports {
			if !portAccepting(p) {
				pending = append(pending, p)
			}
		}
		if len(pending) == 0 {
			return nil
		}

		select {
		case <-exited:
			return fmt.Errorf("proxy exited before it began serving %s", portList(pending))
		default:
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("proxy did not start serving %s within %s", portList(pending), timeout)
		}

		select {
		case <-exited:
			return fmt.Errorf("proxy exited before it began serving %s", portList(pending))
		case <-time.After(proxyReadyPoll):
		}
	}
}

func portList(ports []int) string {
	parts := make([]string, 0, len(ports))
	for _, p := range ports {
		parts = append(parts, "port "+strconv.Itoa(p))
	}
	return strings.Join(parts, ", ")
}

// tailFile returns the last maxLines lines of a file, or "" if it cannot be
// read. Used to surface why a detached proxy died.
func tailFile(path string, maxLines int) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer func() { _ = f.Close() }()

	ring := make([]string, 0, maxLines)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if len(ring) == maxLines {
			ring = ring[1:]
		}
		ring = append(ring, scanner.Text())
	}
	return strings.Join(ring, "\n")
}

// proxyLogPath returns where a detached proxy writes its output.
func proxyLogPath(stateDir string) string {
	return filepath.Join(stateDir, "logs", proxyLogName)
}

// startProxyDetached re-executes portree as a background proxy and waits until
// it is serving. The child records its own PID through the normal foreground
// path, so "portree proxy stop" needs no special handling.
func startProxyDetached(stateDir string, ports []int, extraArgs []string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locating the portree binary: %w", err)
	}

	logPath := proxyLogPath(stateDir)
	if err := os.MkdirAll(filepath.Dir(logPath), 0700); err != nil {
		return fmt.Errorf("creating log dir: %w", err)
	}
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return fmt.Errorf("opening %s: %w", logPath, err)
	}
	defer func() { _ = logFile.Close() }()

	args := append([]string{"proxy", "start"}, extraArgs...)
	child := exec.Command(exe, args...)
	child.Stdout = logFile
	child.Stderr = logFile
	child.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := child.Start(); err != nil {
		return fmt.Errorf("starting the proxy: %w", err)
	}

	exited := make(chan struct{})
	go func() {
		_ = child.Wait()
		close(exited)
	}()

	if err := waitForProxyReady(ports, exited, proxyReadyTimeout); err != nil {
		if tail := tailFile(logPath, proxyLogTailLines); tail != "" {
			return fmt.Errorf("%w\n--- %s ---\n%s", err, logPath, tail)
		}
		return fmt.Errorf("%w (see %s)", err, logPath)
	}
	return nil
}

// proxySpawner is the seam used to launch a detached proxy. Tests replace it,
// because the real implementation re-executes os.Executable(), which under
// `go test` is the test binary.
var proxySpawner = startProxyDetached

// serviceURLs lists the proxy URL for every service of every worktree, sorted
// by service name so the output is stable. svcFilter, when set, narrows it to
// one service.
func serviceURLs(trees []git.Worktree, cfg *config.Config, scheme, svcFilter string) []string {
	if cfg == nil {
		return nil
	}

	names := sortedServiceNames(cfg)
	var lines []string
	for _, tree := range trees {
		if tree.IsBare {
			continue
		}
		for _, name := range names {
			if svcFilter != "" && name != svcFilter {
				continue
			}
			svc := cfg.Services[name]
			lines = append(lines, fmt.Sprintf("%s://%s.localhost:%d  → %s",
				scheme, tree.Slug(), svc.ProxyPort, name))
		}
	}
	return lines
}

// ensureProxyRunning starts a detached proxy unless one is already serving.
// An existing proxy is left untouched so its scheme is not silently changed.
func ensureProxyRunning(stateRoot string, cfg *config.Config, extraArgs []string) (proxyStatus, error) {
	status := describeProxy(cfg, loadProxyState(stateRoot))
	if status.Running {
		return status, nil
	}
	if status.Stale {
		clearProxyState(stateRoot)
	}

	if len(status.Ports) == 0 {
		return status, fmt.Errorf("no proxy_port configured for any service")
	}

	stateDir := filepath.Join(stateRoot, ".portree")
	if err := proxySpawner(stateDir, status.Ports, extraArgs); err != nil {
		return status, err
	}

	return describeProxy(cfg, loadProxyState(stateRoot)), nil
}

// clearProxyState resets a proxy record left behind by a process that died
// without updating it.
func clearProxyState(stateRoot string) {
	store, err := state.NewFileStore(filepath.Join(stateRoot, ".portree"))
	if err != nil {
		return
	}
	_ = store.WithLock(func() error {
		st, e := store.Load()
		if e != nil {
			return e
		}
		st.Proxy = state.ProxyState{Status: state.StatusStopped}
		return store.Save(st)
	})
}

// printProxyReady reports a serving proxy and the ports it answers on.
func printProxyReady(status proxyStatus) {
	fmt.Printf("Proxy: running (%s, pid %d)\n", status.Scheme, status.PID)
	for _, p := range status.Ports {
		fmt.Printf("  %s://<branch-slug>.localhost:%d\n", status.Scheme, p)
	}
}

// proxyFlagArgs rebuilds the TLS-related flags so a detached child starts with
// the same scheme the user asked for.
func proxyFlagArgs(https bool, certFile, keyFile string) []string {
	var args []string
	if https {
		args = append(args, "--https")
	}
	if certFile != "" {
		args = append(args, "--cert", certFile)
	}
	if keyFile != "" {
		args = append(args, "--key", keyFile)
	}
	return args
}
