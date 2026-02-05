package git

import (
	"bufio"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// Worktree represents a single git worktree.
type Worktree struct {
	Path   string // absolute path
	Branch string // branch name (e.g., "main", "feature/auth")
	Head   string // HEAD commit hash
	IsBare bool
}

// Slug returns a URL-safe slug for the branch name.
// e.g., "feature/auth" -> "feature-auth"
func (w *Worktree) Slug() string {
	return BranchSlug(w.Branch)
}

var nonAlphaNum = regexp.MustCompile(`[^a-zA-Z0-9]+`)

// BranchSlug converts a branch name to a URL-safe slug.
func BranchSlug(branch string) string {
	slug := nonAlphaNum.ReplaceAllString(branch, "-")
	slug = strings.Trim(slug, "-")
	return strings.ToLower(slug)
}

// DetectSlugCollisions returns a map of slug -> branch names for any slugs that
// map to more than one branch. An empty map means no collisions.
func DetectSlugCollisions(trees []Worktree) map[string][]string {
	slugBranches := map[string][]string{}
	for _, t := range trees {
		if t.IsBare {
			continue
		}
		slug := t.Slug()
		slugBranches[slug] = append(slugBranches[slug], t.Branch)
	}
	collisions := map[string][]string{}
	for slug, branches := range slugBranches {
		if len(branches) > 1 {
			collisions[slug] = branches
		}
	}
	return collisions
}

// ListWorktrees returns all worktrees for the repo containing dir.
func ListWorktrees(dir string) ([]Worktree, error) {
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git worktree list: %w", err)
	}
	return parsePorcelain(string(out))
}

// CurrentWorktree returns the worktree for the given directory.
func CurrentWorktree(dir string) (*Worktree, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	// Resolve symlinks for proper path comparison (e.g., /tmp -> /private/tmp on macOS)
	absDir, err = filepath.EvalSymlinks(absDir)
	if err != nil {
		return nil, err
	}
	trees, err := ListWorktrees(dir)
	if err != nil {
		return nil, err
	}
	for _, t := range trees {
		if t.Path == absDir {
			return &t, nil
		}
	}
	// Try to match by checking if absDir is under a worktree path
	for _, t := range trees {
		rel, err := filepath.Rel(t.Path, absDir)
		if err == nil && !strings.HasPrefix(rel, "..") {
			return &t, nil
		}
	}
	return nil, fmt.Errorf("current directory %s is not a known worktree", absDir)
}

// parsePorcelain parses the porcelain output of `git worktree list --porcelain`.
// Format:
//
//	worktree /path/to/worktree
//	HEAD <sha>
//	branch refs/heads/<name>
//	<blank line>
func parsePorcelain(output string) ([]Worktree, error) {
	var trees []Worktree
	var current *Worktree
	scanner := bufio.NewScanner(strings.NewReader(output))

	for scanner.Scan() {
		line := scanner.Text()

		switch {
		case strings.HasPrefix(line, "worktree "):
			if current != nil {
				trees = append(trees, *current)
			}
			current = &Worktree{Path: strings.TrimPrefix(line, "worktree ")}

		case strings.HasPrefix(line, "HEAD "):
			if current != nil {
				current.Head = strings.TrimPrefix(line, "HEAD ")
			}

		case strings.HasPrefix(line, "branch "):
			if current != nil {
				ref := strings.TrimPrefix(line, "branch ")
				current.Branch = strings.TrimPrefix(ref, "refs/heads/")
			}

		case line == "bare":
			if current != nil {
				current.IsBare = true
			}

		case line == "detached":
			if current != nil && current.Branch == "" {
				if len(current.Head) >= 8 {
					current.Branch = current.Head[:8]
				} else {
					current.Branch = current.Head
				}
			}

		case line == "":
			// block separator
		}
	}
	if current != nil {
		trees = append(trees, *current)
	}
	return trees, scanner.Err()
}
