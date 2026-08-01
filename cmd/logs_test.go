package cmd

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fairy-pitta/portree/internal/git"
	"github.com/fairy-pitta/portree/internal/process"
)

// captureStdout redirects os.Stdout (including any child process output wired to
// it) for the duration of f and returns what was written.
func captureStdout(t *testing.T, f func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()

	f()

	_ = w.Close()
	os.Stdout = orig
	out := <-done
	_ = r.Close()
	return out
}

// writeLog writes content to the log file for the current worktree's service.
func writeLog(t *testing.T, repoDir, service, content string) {
	t.Helper()
	tree, err := git.CurrentWorktree(repoDir)
	if err != nil {
		t.Fatal(err)
	}
	stateDir := filepath.Join(repoDir, ".portree")
	if err := os.MkdirAll(filepath.Join(stateDir, "logs"), 0700); err != nil {
		t.Fatal(err)
	}
	path := process.LogPath(stateDir, tree.Slug(), service)
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

const twoServiceConfig = `[services.web]
command = "echo web"
port_range = { min = 19100, max = 19199 }
proxy_port = 19000

[services.api]
command = "echo api"
port_range = { min = 19200, max = 19299 }
proxy_port = 19001
`

// TestAllLogsEmpty distinguishes "started but silent" from "command did
// nothing". Tailing an empty file prints nothing, which reads as a failure.
func TestAllLogsEmpty(t *testing.T) {
	dir := t.TempDir()
	empty := filepath.Join(dir, "empty.log")
	written := filepath.Join(dir, "written.log")
	if err := os.WriteFile(empty, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(written, []byte("listening\n"), 0600); err != nil {
		t.Fatal(err)
	}

	if !allLogsEmpty([]logTarget{{service: "a", path: empty}}) {
		t.Error("allLogsEmpty() = false for a single empty log")
	}
	if allLogsEmpty([]logTarget{{service: "a", path: empty}, {service: "b", path: written}}) {
		t.Error("allLogsEmpty() = true although one log has content")
	}
	if !allLogsEmpty([]logTarget{{service: "a", path: filepath.Join(dir, "absent.log")}}) {
		t.Error("allLogsEmpty() = false for a log file that does not exist")
	}
}

func TestLogsSingleServiceNoPrefix(t *testing.T) {
	dir := setupTestRepo(t) // single service: "web"
	writeLog(t, dir, "web", "hello from web\n")
	resetRootCmd()

	out := captureStdout(t, func() {
		rootCmd.SetArgs([]string{"logs"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("logs: %v", err)
		}
	})

	if !strings.Contains(out, "hello from web") {
		t.Errorf("output missing log content: %q", out)
	}
	if strings.Contains(out, "web |") {
		t.Errorf("single service should not be prefixed: %q", out)
	}
}

func TestLogsMultiServicePrefixed(t *testing.T) {
	dir := setupGitRepo(t)
	if err := os.WriteFile(filepath.Join(dir, ".portree.toml"), []byte(twoServiceConfig), 0644); err != nil {
		t.Fatal(err)
	}
	writeLog(t, dir, "web", "web line\n")
	writeLog(t, dir, "api", "api line\n")
	resetRootCmd()

	out := captureStdout(t, func() {
		rootCmd.SetArgs([]string{"logs"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("logs: %v", err)
		}
	})

	if !strings.Contains(out, "web | web line") {
		t.Errorf("missing prefixed web line: %q", out)
	}
	if !strings.Contains(out, "api | api line") {
		t.Errorf("missing prefixed api line: %q", out)
	}
}

func TestLogsTailLimit(t *testing.T) {
	dir := setupTestRepo(t)
	writeLog(t, dir, "web", "line1\nline2\nline3\nline4\n")
	resetRootCmd()

	out := captureStdout(t, func() {
		rootCmd.SetArgs([]string{"logs", "--tail", "2"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("logs --tail 2: %v", err)
		}
	})

	if strings.Contains(out, "line2") {
		t.Errorf("--tail 2 should not include line2: %q", out)
	}
	if !strings.Contains(out, "line3") || !strings.Contains(out, "line4") {
		t.Errorf("--tail 2 should include the last two lines: %q", out)
	}
}

func TestLogsNoLogsYet(t *testing.T) {
	setupTestRepo(t) // no log files written
	resetRootCmd()

	rootCmd.SetArgs([]string{"logs"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("logs with no logs should exit cleanly, got: %v", err)
	}
}

func TestLogsUnknownWorktree(t *testing.T) {
	setupTestRepo(t)
	resetRootCmd()

	rootCmd.SetArgs([]string{"logs", "does-not-exist"})
	if err := rootCmd.Execute(); err == nil {
		t.Error("logs for unknown worktree should error")
	}
}

func TestLogsUnknownService(t *testing.T) {
	setupTestRepo(t)
	resetRootCmd()

	rootCmd.SetArgs([]string{"logs", "--service", "nonexistent"})
	if err := rootCmd.Execute(); err == nil {
		t.Error("logs --service nonexistent should error")
	}
}
