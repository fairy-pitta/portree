package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// StateDirName is the per-repository directory holding runtime state, service
// logs and generated development certificates.
const StateDirName = ".portree"

// EnsureStateIgnored makes sure the runtime state directory is git-ignored in
// the project at root, creating .gitignore when it does not exist. It reports
// whether an entry was added.
//
// Without this, `git add -A` in a fresh project stages the whole state
// directory, which includes the development CA private key alongside logs and
// PIDs. The configuration file itself is deliberately left tracked: it belongs
// to the project.
func EnsureStateIgnored(root string) (bool, error) {
	path := filepath.Join(root, ".gitignore")

	content, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("reading .gitignore: %w", err)
	}

	if ignoresStateDir(string(content)) {
		return false, nil
	}

	var b strings.Builder
	b.Write(content)
	if len(content) > 0 {
		// Never glue the entry onto a file that lacks a final newline, and
		// leave a blank line so the addition reads as its own block.
		if !strings.HasSuffix(string(content), "\n") {
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}
	b.WriteString("# portree runtime state, logs and development certificates\n")
	b.WriteString(StateDirName + "/\n")

	if err := os.WriteFile(path, []byte(b.String()), 0644); err != nil {
		return false, fmt.Errorf("writing .gitignore: %w", err)
	}
	return true, nil
}

// ignoresStateDir reports whether content already ignores the state directory,
// accepting the spellings someone may reasonably have written by hand.
func ignoresStateDir(content string) bool {
	for _, line := range strings.Split(content, "\n") {
		switch strings.TrimSpace(line) {
		case StateDirName, StateDirName + "/", "/" + StateDirName, "/" + StateDirName + "/":
			return true
		}
	}
	return false
}
