package cmd

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fairy-pitta/portree/internal/config"
	"github.com/fairy-pitta/portree/internal/git"
	"github.com/fairy-pitta/portree/internal/state"
)

func findResult(t *testing.T, results []checkResult, substr string) checkResult {
	t.Helper()
	for _, r := range results {
		if strings.Contains(r.name, substr) {
			return r
		}
	}
	t.Fatalf("no check matching %q in %+v", substr, results)
	return checkResult{}
}

// listenerPort returns the port a listener is bound to.
func listenerPort(t *testing.T, ln net.Listener) int {
	t.Helper()
	addr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("listener address %v is not a TCP address", ln.Addr())
	}
	return addr.Port
}

// reserveLoopbackPort holds a loopback port for the duration of the test and
// returns it, so a check can be exercised against a port that is genuinely busy.
func reserveLoopbackPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserving a loopback port: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	return listenerPort(t, ln)
}

func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserving a loopback port: %v", err)
	}
	p := listenerPort(t, ln)
	if err := ln.Close(); err != nil {
		t.Fatalf("releasing reserved port: %v", err)
	}
	return p
}

// --- A: working directory checks ---

func TestCheckServiceDirs(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "frontend"), 0755); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Services: map[string]config.ServiceConfig{
			"frontend": {Dir: "frontend"},
			"backend":  {Dir: "backend"}, // never created
			"root":     {Dir: ""},        // worktree root, always present
		},
	}
	trees := []git.Worktree{{Path: root, Branch: "main"}}

	results := checkServiceDirs(cfg, trees)

	var failed []checkResult
	for _, r := range results {
		if !r.ok {
			failed = append(failed, r)
		}
	}
	if len(failed) != 1 {
		t.Fatalf("got %d failing checks, want 1 (only backend): %+v", len(failed), results)
	}

	got := failed[0].name + " " + failed[0].detail
	for _, want := range []string{"backend", "main", filepath.Join(root, "backend")} {
		if !strings.Contains(got, want) {
			t.Errorf("failing check %q does not mention %q", got, want)
		}
	}
}

func TestCheckServiceDirsAllPresent(t *testing.T) {
	root := t.TempDir()
	cfg := &config.Config{
		Services: map[string]config.ServiceConfig{"web": {Dir: ""}},
	}
	trees := []git.Worktree{{Path: root, Branch: "main"}}

	for _, r := range checkServiceDirs(cfg, trees) {
		if !r.ok {
			t.Errorf("unexpected failure: %+v", r)
		}
	}
}

func TestCheckServiceDirsSkipsBare(t *testing.T) {
	cfg := &config.Config{
		Services: map[string]config.ServiceConfig{"web": {Dir: "missing"}},
	}
	trees := []git.Worktree{{Path: t.TempDir(), Branch: "", IsBare: true}}

	for _, r := range checkServiceDirs(cfg, trees) {
		if !r.ok {
			t.Errorf("bare worktree should not be checked, got failure: %+v", r)
		}
	}
}

// --- C: proxy port checks ---

func TestCheckPortConflictsFreePort(t *testing.T) {
	p := freePort(t)
	cfg := &config.Config{Services: map[string]config.ServiceConfig{"web": {ProxyPort: p}}}

	r := findResult(t, checkPortConflicts(cfg, state.ProxyState{}), "proxy port")
	if !r.ok {
		t.Errorf("free port reported as a failure: %+v", r)
	}
	if !strings.Contains(r.name, "available") {
		t.Errorf("name = %q, want it to say the port is available", r.name)
	}
}

// TestCheckPortConflictsOurProxy is the state doctor spends most of its life in
// once the proxy is running: the port is busy, but busy is correct.
func TestCheckPortConflictsOurProxy(t *testing.T) {
	p := reserveLoopbackPort(t)
	cfg := &config.Config{Services: map[string]config.ServiceConfig{"web": {ProxyPort: p}}}
	proxy := state.ProxyState{PID: os.Getpid(), Status: state.StatusRunning}

	r := findResult(t, checkPortConflicts(cfg, proxy), "proxy port")
	if !r.ok {
		t.Errorf("our own running proxy reported as a failure: %+v", r)
	}
	if !strings.Contains(r.name+" "+r.detail, "portree proxy") {
		t.Errorf("check %q / %q does not say the port is held by portree", r.name, r.detail)
	}
}

// TestCheckPortConflictsForeignProcess is the case the old raw net.Listen probe
// missed entirely: another process holding the loopback address.
func TestCheckPortConflictsForeignProcess(t *testing.T) {
	p := reserveLoopbackPort(t)
	cfg := &config.Config{Services: map[string]config.ServiceConfig{"web": {ProxyPort: p}}}

	r := findResult(t, checkPortConflicts(cfg, state.ProxyState{Status: state.StatusStopped}), "proxy port")
	if r.ok {
		t.Errorf("port held by another process reported as OK: %+v", r)
	}
	if !strings.Contains(r.detail, "another process") {
		t.Errorf("detail = %q, want it to name the conflict", r.detail)
	}
}

// TestCheckPortConflictsStaleProxyPID guards against trusting a recorded PID
// that is no longer alive: the port is then held by something else.
func TestCheckPortConflictsStaleProxyPID(t *testing.T) {
	p := reserveLoopbackPort(t)
	cfg := &config.Config{Services: map[string]config.ServiceConfig{"web": {ProxyPort: p}}}
	// PID 0 is never a live user process; Status still claims running.
	proxy := state.ProxyState{PID: 0, Status: state.StatusRunning}

	r := findResult(t, checkPortConflicts(cfg, proxy), "proxy port")
	if r.ok {
		t.Errorf("stale proxy PID treated as a live proxy: %+v", r)
	}
}
