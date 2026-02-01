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

// setupTestRepo creates a temporary git repo with .portree.toml and returns its path.
// It changes the working directory to the repo root and returns a cleanup function.
func setupTestRepo(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()

	// git init + empty commit
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

	// Write .portree.toml
	cfgPath := filepath.Join(dir, config.FileName)
	if err := os.WriteFile(cfgPath, []byte(testConfig), 0644); err != nil {
		t.Fatal(err)
	}

	// Change to repo directory.
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

// resetRootCmd resets the global state modified by PersistentPreRunE.
func resetRootCmd() {
	cfg = nil
	repoRoot = ""
}

func TestInitCommand(t *testing.T) {
	dir := t.TempDir()

	// git init in the temp dir
	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	cmd = exec.Command("git", "commit", "--allow-empty", "-m", "init")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=test",
		"GIT_COMMITTER_EMAIL=test@test.com",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, out)
	}

	origDir, _ := os.Getwd()
	_ = os.Chdir(dir)
	defer func() { _ = os.Chdir(origDir) }()

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
