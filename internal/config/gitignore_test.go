package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readGitignore(t *testing.T, root string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatalf("reading .gitignore: %v", err)
	}
	return string(b)
}

// countLines reports how many non-empty lines equal want.
func countLines(content, want string) int {
	n := 0
	for _, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) == want {
			n++
		}
	}
	return n
}

// TestEnsureStateIgnoredCreatesFile covers a project with no .gitignore at all.
// Without this, `git add -A` stages .portree/, which holds the dev CA private
// key along with logs and runtime state.
func TestEnsureStateIgnoredCreatesFile(t *testing.T) {
	root := t.TempDir()

	added, err := EnsureStateIgnored(root)
	if err != nil {
		t.Fatalf("EnsureStateIgnored() error: %v", err)
	}
	if !added {
		t.Error("added = false when there was no .gitignore")
	}
	if got := readGitignore(t, root); countLines(got, StateDirName+"/") != 1 {
		t.Errorf(".gitignore = %q, want it to ignore %q", got, StateDirName+"/")
	}
}

func TestEnsureStateIgnoredAppendsAndKeepsExisting(t *testing.T) {
	root := t.TempDir()
	existing := "node_modules/\ndist/\n"
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(existing), 0644); err != nil {
		t.Fatal(err)
	}

	added, err := EnsureStateIgnored(root)
	if err != nil {
		t.Fatalf("EnsureStateIgnored() error: %v", err)
	}
	if !added {
		t.Error("added = false although the entry was missing")
	}

	got := readGitignore(t, root)
	if !strings.HasPrefix(got, existing) {
		t.Errorf(".gitignore = %q, want it to keep the existing entries first", got)
	}
	if countLines(got, StateDirName+"/") != 1 {
		t.Errorf(".gitignore = %q, want exactly one %q entry", got, StateDirName+"/")
	}
}

// TestEnsureStateIgnoredNoTrailingNewline guards against gluing the new entry
// onto the last existing one.
func TestEnsureStateIgnoredNoTrailingNewline(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("dist/"), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := EnsureStateIgnored(root); err != nil {
		t.Fatalf("EnsureStateIgnored() error: %v", err)
	}

	got := readGitignore(t, root)
	if countLines(got, "dist/") != 1 {
		t.Errorf(".gitignore = %q, want the existing dist/ entry intact on its own line", got)
	}
	if countLines(got, StateDirName+"/") != 1 {
		t.Errorf(".gitignore = %q, want exactly one %q entry", got, StateDirName+"/")
	}
}

func TestEnsureStateIgnoredIsIdempotent(t *testing.T) {
	root := t.TempDir()

	if _, err := EnsureStateIgnored(root); err != nil {
		t.Fatal(err)
	}
	first := readGitignore(t, root)

	added, err := EnsureStateIgnored(root)
	if err != nil {
		t.Fatalf("second EnsureStateIgnored() error: %v", err)
	}
	if added {
		t.Error("added = true on the second call, want false")
	}
	if got := readGitignore(t, root); got != first {
		t.Errorf(".gitignore changed on the second call:\n%q\nwant\n%q", got, first)
	}
}

// TestEnsureStateIgnoredRecognisesExistingForms accepts the spellings a user
// may already have written by hand.
func TestEnsureStateIgnoredRecognisesExistingForms(t *testing.T) {
	for _, entry := range []string{".portree/", ".portree", "/.portree/", "  .portree/  "} {
		t.Run(entry, func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(entry+"\n"), 0644); err != nil {
				t.Fatal(err)
			}

			added, err := EnsureStateIgnored(root)
			if err != nil {
				t.Fatalf("EnsureStateIgnored() error: %v", err)
			}
			if added {
				t.Errorf("added = true although %q already ignores the state dir", entry)
			}
		})
	}
}
