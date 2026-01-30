package git

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// FindRepoRoot returns the root directory of the git repository
// that contains the given directory (or the current directory).
func FindRepoRoot(dir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("not a git repository (or any parent): %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// CommonDir returns the git common directory (the .git dir of the main worktree).
// For worktrees, this points to the main repo's .git directory.
func CommonDir(dir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--git-common-dir")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get git common dir: %w", err)
	}
	result := strings.TrimSpace(string(out))
	if !filepath.IsAbs(result) {
		result = filepath.Join(dir, result)
	}
	return filepath.Clean(result), nil
}

// MainWorktreeRoot returns the root directory of the main worktree
// by resolving the common git dir.
func MainWorktreeRoot(dir string) (string, error) {
	commonDir, err := CommonDir(dir)
	if err != nil {
		return "", err
	}
	// commonDir is typically /path/to/repo/.git
	// The main worktree root is its parent
	return filepath.Dir(commonDir), nil
}
