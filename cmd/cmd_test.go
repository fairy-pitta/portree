package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/fairy-pitta/portree/internal/config"
)

const testConfig = `[services.web]
command = "echo hello"
port_range = { min = 19100, max = 19199 }
proxy_port = 19000
`

// setupGitRepo creates a temporary git repo and changes to it.
// Returns the repo directory. Cleanup is handled via t.Cleanup.
func setupGitRepo(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
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
	openService = ""
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

	// Start services (echo hello exits immediately).
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
