package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fairy-pitta/portree/internal/config"
	"github.com/fairy-pitta/portree/internal/git"
	"github.com/fairy-pitta/portree/internal/logging"
	"github.com/fairy-pitta/portree/internal/state"
	"github.com/spf13/pflag"
)

// testCfg is a config for use in unit tests of buildLsEntries.
var testCfg = &config.Config{
	Services: map[string]config.ServiceConfig{
		"api": {ProxyPort: 8000},
		"web": {ProxyPort: 3000},
	},
}

const testConfig = `[services.web]
command = "sleep 30"
port_range = { min = 19100, max = 19199 }
proxy_port = 19000
`

// setupGitRepo creates a temporary git repo and changes to it.
// Returns the repo directory. Cleanup is handled via t.Cleanup.
//
// NOTE: Uses os.Chdir which mutates process-wide state.
// Tests using this helper cannot use t.Parallel() and should run with -count=1.
func setupGitRepo(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		// Use a sanitized env so the temp repo is created at dir, not at an
		// inherited GIT_DIR (e.g. when the suite runs inside a git hook).
		cmd.Env = append(git.SanitizedEnv(),
			"GIT_AUTHOR_NAME=test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("command %v failed: %v\n%s", args, err, out)
		}
	}

	run("git", "init")
	run("git", "commit", "--allow-empty", "-m", "init")

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(origDir)
	})

	return dir
}

// setupTestRepo creates a temporary git repo with .portree.toml and changes to it.
func setupTestRepo(t *testing.T) string {
	t.Helper()

	dir := setupGitRepo(t)

	cfgPath := filepath.Join(dir, config.FileName)
	if err := os.WriteFile(cfgPath, []byte(testConfig), 0644); err != nil {
		t.Fatal(err)
	}

	return dir
}

// resetRootCmd resets the global state modified by PersistentPreRunE and cobra flags.
func resetRootCmd() {
	cfg = nil
	repoRoot = ""

	// Reset cobra flag variables to defaults.
	downAll = false
	downService = ""
	downPrune = false
	upAll = false
	upService = ""
	upNoProxy = false
	openService = ""
	logsFollow = false
	logsTail = 50
	logsService = ""

	// Reset proxy start flags.
	proxyStartCmd.Flags().VisitAll(func(f *pflag.Flag) {
		f.Changed = false
	})

	// Reset logging level and persistent flag "changed" state.
	logging.SetLevel(logging.LevelNormal)
	rootCmd.PersistentFlags().VisitAll(func(f *pflag.Flag) {
		f.Changed = false
	})
}

func TestInitCommand(t *testing.T) {
	dir := setupGitRepo(t)

	resetRootCmd()
	rootCmd.SetArgs([]string{"init"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("init command: %v", err)
	}

	// Verify config file was created.
	cfgPath := filepath.Join(dir, config.FileName)
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		t.Errorf("%s was not created", config.FileName)
	}
}

func TestLsCommand(t *testing.T) {
	setupTestRepo(t)
	resetRootCmd()

	rootCmd.SetArgs([]string{"ls"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("ls command: %v", err)
	}
}

func TestLsJSONCommand(t *testing.T) {
	setupTestRepo(t)
	resetRootCmd()

	rootCmd.SetArgs([]string{"ls", "--json"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("ls --json command: %v", err)
	}
}

func TestDoctorCommand(t *testing.T) {
	setupTestRepo(t)
	resetRootCmd()

	rootCmd.SetArgs([]string{"doctor"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("doctor command: %v", err)
	}
}

func TestUpDownCommand(t *testing.T) {
	setupTestRepo(t)
	resetRootCmd()

	t.Setenv("PORTREE_STARTUP_GRACE", "200ms")
	rootCmd.SetArgs([]string{"up"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("up command: %v", err)
	}

	// Stop services.
	resetRootCmd()
	rootCmd.SetArgs([]string{"down"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("down command: %v", err)
	}
}

// runDown stops whatever a test started, so leftover processes do not leak
// into later tests.
func runDown(t *testing.T) {
	t.Helper()
	resetRootCmd()
	rootCmd.SetArgs([]string{"down"})
	_ = rootCmd.Execute()
}

// TestUpStartsProxy covers the point of the change: "up" alone leaves the user
// with URLs that answer, instead of requiring a second command in another
// terminal.
func TestUpStartsProxy(t *testing.T) {
	setupTestRepo(t)
	resetRootCmd()
	testSpawns.reset()
	t.Cleanup(func() { runDown(t) })

	rootCmd.SetArgs([]string{"up"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("up command: %v", err)
	}

	if got := testSpawns.count(); got != 1 {
		t.Errorf("proxy spawned %d times, want 1", got)
	}
}

func TestUpNoProxy(t *testing.T) {
	setupTestRepo(t)
	resetRootCmd()
	testSpawns.reset()
	t.Cleanup(func() { runDown(t) })

	rootCmd.SetArgs([]string{"up", "--no-proxy"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("up --no-proxy command: %v", err)
	}

	if got := testSpawns.count(); got != 0 {
		t.Errorf("proxy spawned %d times with --no-proxy, want 0", got)
	}
}

// TestUnknownServiceError checks the message names the alternatives. Echoing
// only the rejected name leaves the user to go and read the config to find out
// what they should have typed.
func TestUnknownServiceError(t *testing.T) {
	c := &config.Config{Services: map[string]config.ServiceConfig{
		"web": {}, "api": {}, "admin": {},
	}}

	err := unknownServiceError(c, "wbe")
	if err == nil {
		t.Fatal("unknownServiceError() = nil, want an error")
	}

	msg := err.Error()
	if !strings.Contains(msg, `"wbe"`) {
		t.Errorf("error %q does not quote the rejected name", msg)
	}
	if !strings.Contains(msg, "admin, api, web") {
		t.Errorf("error %q does not list the configured services in sorted order", msg)
	}
}

func TestVersionCommand(t *testing.T) {
	resetRootCmd()
	rootCmd.SetArgs([]string{"version"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("version command: %v", err)
	}
}

func TestVersionJSONCommand(t *testing.T) {
	resetRootCmd()
	rootCmd.SetArgs([]string{"version", "--json"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("version --json command: %v", err)
	}
}

func TestInitAlreadyExists(t *testing.T) {
	setupTestRepo(t) // already has .portree.toml
	resetRootCmd()

	rootCmd.SetArgs([]string{"init"})
	err := rootCmd.Execute()
	if err == nil {
		t.Error("init on existing config should error")
	}
}

func TestUpServiceFilter(t *testing.T) {
	setupTestRepo(t)
	resetRootCmd()

	t.Setenv("PORTREE_STARTUP_GRACE", "200ms")
	rootCmd.SetArgs([]string{"up", "--service", "web"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("up --service web: %v", err)
	}
}

// A service that exits within the startup grace window must make `up` return a
// non-zero error at the CLI layer, not silently report success.
func TestUpReportsStartFailure(t *testing.T) {
	dir := setupGitRepo(t)
	const crashConfig = `[services.web]
command = "exit 1"
port_range = { min = 19100, max = 19199 }
proxy_port = 19000
`
	if err := os.WriteFile(filepath.Join(dir, config.FileName), []byte(crashConfig), 0644); err != nil {
		t.Fatal(err)
	}
	resetRootCmd()

	t.Setenv("PORTREE_STARTUP_GRACE", "500ms")
	rootCmd.SetArgs([]string{"up"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("up should return an error when a service fails to start")
	}
	// Also asserts the singular noun for a single failure.
	if !strings.Contains(err.Error(), "1 service failed to start") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "1 service failed to start")
	}
}

func TestUpUnknownService(t *testing.T) {
	setupTestRepo(t)
	resetRootCmd()

	rootCmd.SetArgs([]string{"up", "--service", "nonexistent"})
	err := rootCmd.Execute()
	if err == nil {
		t.Error("up --service nonexistent should error")
	}
}

func TestDownUnknownService(t *testing.T) {
	setupTestRepo(t)
	resetRootCmd()

	rootCmd.SetArgs([]string{"down", "--service", "nonexistent"})
	err := rootCmd.Execute()
	if err == nil {
		t.Error("down --service nonexistent should error")
	}
}

func TestDownPrune(t *testing.T) {
	setupTestRepo(t)
	resetRootCmd()

	rootCmd.SetArgs([]string{"down", "--prune"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("down --prune: %v", err)
	}
}

func TestDownServiceFilter(t *testing.T) {
	setupTestRepo(t)
	resetRootCmd()

	rootCmd.SetArgs([]string{"down", "--service", "web"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("down --service web: %v", err)
	}
}

func TestRootNoGitRepo(t *testing.T) {
	// chdir to a non-git temp dir
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	_ = os.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	resetRootCmd()
	rootCmd.SetArgs([]string{"ls"})
	err := rootCmd.Execute()
	if err == nil {
		t.Error("ls outside git repo should error")
	}
}

func TestRootNoConfig(t *testing.T) {
	// Create git repo but no .portree.toml
	dir := setupGitRepo(t)
	_ = dir

	resetRootCmd()
	rootCmd.SetArgs([]string{"ls"})
	err := rootCmd.Execute()
	if err == nil {
		t.Error("ls without config should error")
	}
}

func TestVerboseFlag(t *testing.T) {
	setupTestRepo(t)
	resetRootCmd()

	rootCmd.SetArgs([]string{"ls", "-v"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("ls -v: %v", err)
	}
}

func TestQuietFlag(t *testing.T) {
	setupTestRepo(t)
	resetRootCmd()

	rootCmd.SetArgs([]string{"ls", "-q"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("ls -q: %v", err)
	}
}

func TestBuildLsEntries(t *testing.T) {
	trees := []git.Worktree{
		{Path: "/a", Branch: "main"},
		{Path: "/b", Branch: "feature/auth"},
		{Path: "/c", Branch: "", IsBare: true}, // bare should be skipped
	}
	serviceNames := []string{"api", "web"}
	st := &state.State{
		Services: map[string]map[string]*state.ServiceState{
			"main": {
				"web": {Port: 3100, PID: 123, Status: state.StatusRunning},
			},
		},
		PortAssignments: map[string]int{},
	}

	entries := buildLsEntries(trees, serviceNames, st, testCfg, nil)

	// bare worktree should be skipped: 2 trees × 2 services = 4
	if len(entries) != 4 {
		t.Fatalf("buildLsEntries returned %d entries, want 4", len(entries))
	}

	// Check running service
	found := false
	for _, e := range entries {
		if e.Worktree == "main" && e.Service == "web" {
			found = true
			if e.Port != 3100 {
				t.Errorf("main/web port = %d, want 3100", e.Port)
			}
		}
	}
	if !found {
		t.Error("main/web entry not found")
	}
}

// TestBuildLsEntriesURLIsStable keeps the agent-facing discovery field
// predictable. The URL is a property of the config, not of whether the proxy
// happens to be up, so it is always present and proxy_running says whether
// anything will answer it.
func TestBuildLsEntriesURLIsStable(t *testing.T) {
	trees := []git.Worktree{{Path: "/a", Branch: "feature/auth"}}
	serviceNames := []string{"web"}
	newState := func() *state.State {
		return &state.State{
			Services:        map[string]map[string]*state.ServiceState{},
			PortAssignments: map[string]int{},
		}
	}
	const wantURL = "http://feature-auth.localhost:3000"

	stopped := buildLsEntries(trees, serviceNames, newState(), testCfg,
		&state.ProxyState{Status: state.StatusStopped})
	if len(stopped) != 1 {
		t.Fatalf("got %d entries, want 1", len(stopped))
	}
	if stopped[0].URL != wantURL {
		t.Errorf("URL with the proxy stopped = %q, want %q", stopped[0].URL, wantURL)
	}
	if stopped[0].ProxyRunning {
		t.Error("ProxyRunning = true while the proxy is stopped")
	}

	running := buildLsEntries(trees, serviceNames, newState(), testCfg,
		&state.ProxyState{PID: os.Getpid(), Status: state.StatusRunning})
	if running[0].URL != wantURL {
		t.Errorf("URL with the proxy running = %q, want %q", running[0].URL, wantURL)
	}
	if !running[0].ProxyRunning {
		t.Error("ProxyRunning = false while the proxy is running")
	}
}

// TestBuildLsEntriesStaleProxyPIDIsNotRunning stops a dead PID left in state
// from advertising the proxy as live.
func TestBuildLsEntriesStaleProxyPIDIsNotRunning(t *testing.T) {
	trees := []git.Worktree{{Path: "/a", Branch: "main"}}
	st := &state.State{
		Services:        map[string]map[string]*state.ServiceState{},
		PortAssignments: map[string]int{},
	}

	entries := buildLsEntries(trees, []string{"web"}, st, testCfg,
		&state.ProxyState{PID: 0, Status: state.StatusRunning})

	if entries[0].ProxyRunning {
		t.Error("ProxyRunning = true for a proxy PID that is not alive")
	}
}

func TestBuildLsEntries_DetachedHead(t *testing.T) {
	trees := []git.Worktree{
		{Path: "/a", Branch: ""},
	}
	serviceNames := []string{"web"}
	st := &state.State{
		Services:        map[string]map[string]*state.ServiceState{},
		PortAssignments: map[string]int{},
	}

	entries := buildLsEntries(trees, serviceNames, st, testCfg, nil)
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	if entries[0].Worktree != "(detached)" {
		t.Errorf("worktree = %q, want (detached)", entries[0].Worktree)
	}
}

func TestBuildLsEntries_StaleProcess(t *testing.T) {
	trees := []git.Worktree{
		{Path: "/a", Branch: "main"},
	}
	serviceNames := []string{"web"}
	st := &state.State{
		Services: map[string]map[string]*state.ServiceState{
			"main": {
				"web": {Port: 3100, PID: 99999999, Status: state.StatusRunning},
			},
		},
		PortAssignments: map[string]int{},
	}

	entries := buildLsEntries(trees, serviceNames, st, testCfg, nil)
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	// PID 99999999 is almost certainly not running, so status should be stopped
	if entries[0].Status != state.StatusStopped {
		t.Errorf("stale process should show as stopped, got %q", entries[0].Status)
	}
}

func TestPrintLsTable(t *testing.T) {
	entries := []lsEntry{
		{Worktree: "main", Service: "web", Port: 3100, Status: state.StatusRunning, PID: 123},
		{Worktree: "main", Service: "api", Port: 0, Status: state.StatusStopped, PID: 0},
	}

	// printLsTable writes to stdout; just verify it doesn't error
	err := printLsTable(entries)
	if err != nil {
		t.Fatalf("printLsTable error: %v", err)
	}
}

func TestDownAll(t *testing.T) {
	setupTestRepo(t)
	resetRootCmd()

	rootCmd.SetArgs([]string{"down", "--all"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("down --all: %v", err)
	}
}
