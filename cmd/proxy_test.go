package cmd

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fairy-pitta/portree/internal/config"
	"github.com/fairy-pitta/portree/internal/state"
)

// listenLoopback holds a port for the test and returns it.
func listenLoopback(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listening: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	return listenerPort(t, ln)
}

// --- readiness ---

func TestWaitForProxyReady(t *testing.T) {
	ports := []int{listenLoopback(t), listenLoopback(t)}

	if err := waitForProxyReady(ports, make(chan struct{}), 2*time.Second); err != nil {
		t.Errorf("waitForProxyReady() = %v, want nil", err)
	}
}

// TestWaitForProxyReadyTimeout covers a proxy that never binds: reporting
// success here is what would let "up" claim victory while "open" still fails.
func TestWaitForProxyReadyTimeout(t *testing.T) {
	dead := freePort(t) // reserved then released, so nothing is listening

	err := waitForProxyReady([]int{dead}, make(chan struct{}), 200*time.Millisecond)
	if err == nil {
		t.Fatal("waitForProxyReady() = nil for a port nobody listens on, want an error")
	}
	if !strings.Contains(err.Error(), fmt.Sprint(dead)) {
		t.Errorf("error %q does not name the port %d", err, dead)
	}
}

// TestWaitForProxyReadyChildExited asserts a crashed proxy is reported at once
// rather than after the full timeout.
func TestWaitForProxyReadyChildExited(t *testing.T) {
	dead := freePort(t)
	exited := make(chan struct{})
	close(exited)

	start := time.Now()
	err := waitForProxyReady([]int{dead}, exited, 10*time.Second)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("waitForProxyReady() = nil after the process exited, want an error")
	}
	if !strings.Contains(err.Error(), "exited") {
		t.Errorf("error %q does not say the process exited", err)
	}
	if elapsed > 2*time.Second {
		t.Errorf("took %v to notice the exit; it should not wait out the timeout", elapsed)
	}
}

func TestWaitForProxyReadyPartial(t *testing.T) {
	ports := []int{listenLoopback(t), freePort(t)}

	if err := waitForProxyReady(ports, make(chan struct{}), 200*time.Millisecond); err == nil {
		t.Error("waitForProxyReady() = nil when only one of two ports is bound, want an error")
	}
}

// --- log tail ---

func TestTailFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "proxy.log")
	var b strings.Builder
	for i := 1; i <= 10; i++ {
		fmt.Fprintf(&b, "line %d\n", i)
	}
	if err := os.WriteFile(path, []byte(b.String()), 0600); err != nil {
		t.Fatal(err)
	}

	got := tailFile(path, 3)
	for _, want := range []string{"line 8", "line 9", "line 10"} {
		if !strings.Contains(got, want) {
			t.Errorf("tail %q missing %q", got, want)
		}
	}
	if strings.Contains(got, "line 7") {
		t.Errorf("tail %q includes more than the requested 3 lines", got)
	}
}

func TestTailFileMissing(t *testing.T) {
	if got := tailFile(filepath.Join(t.TempDir(), "absent.log"), 5); got != "" {
		t.Errorf("tailFile on a missing file = %q, want empty", got)
	}
}

// --- status ---

func TestDescribeProxyRunning(t *testing.T) {
	p := listenLoopback(t)
	cfg := &config.Config{Services: map[string]config.ServiceConfig{"web": {ProxyPort: p}}}

	got := describeProxy(cfg, state.ProxyState{PID: os.Getpid(), Status: state.StatusRunning})

	if !got.Running {
		t.Error("Running = false for a live PID with a bound port")
	}
	if got.PID != os.Getpid() {
		t.Errorf("PID = %d, want %d", got.PID, os.Getpid())
	}
	if got.Scheme != "http" {
		t.Errorf("Scheme = %q, want http", got.Scheme)
	}
	if len(got.Ports) != 1 || got.Ports[0] != p {
		t.Errorf("Ports = %v, want [%d]", got.Ports, p)
	}
}

func TestDescribeProxyHTTPS(t *testing.T) {
	p := listenLoopback(t)
	cfg := &config.Config{Services: map[string]config.ServiceConfig{"web": {ProxyPort: p}}}

	got := describeProxy(cfg, state.ProxyState{PID: os.Getpid(), Status: state.StatusRunning, HTTPS: true})
	if got.Scheme != "https" {
		t.Errorf("Scheme = %q, want https", got.Scheme)
	}
}

func TestDescribeProxyStopped(t *testing.T) {
	cfg := &config.Config{Services: map[string]config.ServiceConfig{"web": {ProxyPort: freePort(t)}}}

	if got := describeProxy(cfg, state.ProxyState{Status: state.StatusStopped}); got.Running {
		t.Error("Running = true for a stopped proxy")
	}
}

// TestDescribeProxyStalePID covers a proxy that died without updating state.
func TestDescribeProxyStalePID(t *testing.T) {
	cfg := &config.Config{Services: map[string]config.ServiceConfig{"web": {ProxyPort: freePort(t)}}}

	got := describeProxy(cfg, state.ProxyState{PID: 0, Status: state.StatusRunning})
	if got.Running {
		t.Error("Running = true for a PID that is not alive")
	}
	if !got.Stale {
		t.Error("Stale = false although state claims running with a dead PID")
	}
}

// TestDescribeProxyPortsSorted keeps status output stable across runs.
func TestDescribeProxyPortsSorted(t *testing.T) {
	cfg := &config.Config{Services: map[string]config.ServiceConfig{
		"web": {ProxyPort: 8000},
		"api": {ProxyPort: 3000},
		"adm": {ProxyPort: 5000},
	}}

	got := describeProxy(cfg, state.ProxyState{Status: state.StatusStopped})
	want := []int{3000, 5000, 8000}
	if len(got.Ports) != len(want) {
		t.Fatalf("Ports = %v, want %v", got.Ports, want)
	}
	for i := range want {
		if got.Ports[i] != want[i] {
			t.Fatalf("Ports = %v, want %v", got.Ports, want)
		}
	}
}
