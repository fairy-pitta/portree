package cmd

import (
	"strings"
	"testing"

	"github.com/fairy-pitta/portree/internal/config"
	"github.com/fairy-pitta/portree/internal/git"
)

const testProxyPort = 3000

func testService() config.ServiceConfig {
	return config.ServiceConfig{ProxyPort: testProxyPort}
}

func TestOpenURLOK(t *testing.T) {
	proxy := proxyStatus{Running: true, Scheme: "http", PID: 1, Ports: []int{testProxyPort}}

	got, err := openURL(testService(), "feature-auth", proxy, true)
	if err != nil {
		t.Fatalf("openURL() error: %v", err)
	}
	if want := "http://feature-auth.localhost:3000"; got != want {
		t.Errorf("openURL() = %q, want %q", got, want)
	}
}

func TestOpenURLHTTPS(t *testing.T) {
	proxy := proxyStatus{Running: true, Scheme: "https", PID: 1, Ports: []int{testProxyPort}}

	got, err := openURL(testService(), "main", proxy, true)
	if err != nil {
		t.Fatalf("openURL() error: %v", err)
	}
	if want := "https://main.localhost:3000"; got != want {
		t.Errorf("openURL() = %q, want %q", got, want)
	}
}

// TestOpenURLProxyStopped is the guard that matters: opening a browser at a URL
// the proxy is not serving lands the user on a connection error and looks like
// portree misbehaving.
func TestOpenURLProxyStopped(t *testing.T) {
	proxy := proxyStatus{Running: false, Scheme: "http", Ports: []int{testProxyPort}}

	got, err := openURL(testService(), "main", proxy, true)
	if err == nil {
		t.Fatal("openURL() succeeded with a stopped proxy, want an error")
	}
	if got != "" {
		t.Errorf("openURL() returned %q alongside an error; nothing should be opened", got)
	}
	if !strings.Contains(err.Error(), "portree up") && !strings.Contains(err.Error(), "proxy start") {
		t.Errorf("error %q does not tell the user how to fix it", err)
	}
}

func TestOpenURLServiceStopped(t *testing.T) {
	proxy := proxyStatus{Running: true, Scheme: "http", PID: 1, Ports: []int{testProxyPort}}

	got, err := openURL(testService(), "main", proxy, false)
	if err == nil {
		t.Fatal("openURL() succeeded with a stopped service, want an error")
	}
	if got != "" {
		t.Errorf("openURL() returned %q alongside an error", got)
	}
	if !strings.Contains(err.Error(), "portree up") {
		t.Errorf("error %q does not tell the user how to fix it", err)
	}
}

// --- URL listing ---

func TestServiceURLs(t *testing.T) {
	trees := []git.Worktree{
		{Path: "/a", Branch: "main"},
		{Path: "/b", Branch: "feature/auth"},
		{Path: "/c", IsBare: true},
	}
	cfg := &config.Config{Services: map[string]config.ServiceConfig{
		"web": {ProxyPort: 3000},
		"api": {ProxyPort: 8000},
	}}

	got := serviceURLs(trees, cfg, "http", "")

	want := []string{
		"http://main.localhost:8000  → api",
		"http://main.localhost:3000  → web",
		"http://feature-auth.localhost:8000  → api",
		"http://feature-auth.localhost:3000  → web",
	}
	if len(got) != len(want) {
		t.Fatalf("serviceURLs() returned %d lines, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestServiceURLsRespectsFilter(t *testing.T) {
	trees := []git.Worktree{{Path: "/a", Branch: "main"}}
	cfg := &config.Config{Services: map[string]config.ServiceConfig{
		"web": {ProxyPort: 3000},
		"api": {ProxyPort: 8000},
	}}

	got := serviceURLs(trees, cfg, "http", "web")
	if len(got) != 1 || !strings.Contains(got[0], "3000") {
		t.Errorf("serviceURLs() with filter web = %v, want only the web URL", got)
	}
}
