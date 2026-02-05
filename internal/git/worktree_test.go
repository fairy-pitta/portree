package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestBranchSlug(t *testing.T) {
	tests := []struct {
		name   string
		branch string
		want   string
	}{
		{"slash to dash", "feature/auth", "feature-auth"},
		{"underscore to dash", "feature_auth", "feature-auth"},
		{"uppercase to lower", "Feature/Auth", "feature-auth"},
		{"dots to dash", "release.1.0", "release-1-0"},
		{"empty string", "", ""},
		{"already clean", "main", "main"},
		{"multiple special chars", "feature//auth__v2", "feature-auth-v2"},
		{"leading trailing special", "/feature/", "feature"},
		{"only special chars", "///", ""},
		{"mixed separators", "feat.ui/login_page", "feat-ui-login-page"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BranchSlug(tt.branch)
			if got != tt.want {
				t.Errorf("BranchSlug(%q) = %q, want %q", tt.branch, got, tt.want)
			}
		})
	}
}

func TestWorktreeSlug(t *testing.T) {
	wt := &Worktree{Branch: "feature/auth"}
	got := wt.Slug()
	want := "feature-auth"
	if got != want {
		t.Errorf("Worktree.Slug() = %q, want %q", got, want)
	}

	// Slug should match BranchSlug
	if got != BranchSlug(wt.Branch) {
		t.Errorf("Worktree.Slug() != BranchSlug(branch)")
	}
}

func TestDetectSlugCollisions(t *testing.T) {
	t.Run("no collisions", func(t *testing.T) {
		trees := []Worktree{
			{Path: "/a", Branch: "main"},
			{Path: "/b", Branch: "feature/auth"},
		}
		got := DetectSlugCollisions(trees)
		if len(got) != 0 {
			t.Errorf("DetectSlugCollisions() = %v, want empty", got)
		}
	})

	t.Run("collision", func(t *testing.T) {
		trees := []Worktree{
			{Path: "/a", Branch: "feature/auth"},
			{Path: "/b", Branch: "feature-auth"},
		}
		got := DetectSlugCollisions(trees)
		if len(got) != 1 {
			t.Fatalf("DetectSlugCollisions() returned %d collisions, want 1", len(got))
		}
		branches, ok := got["feature-auth"]
		if !ok {
			t.Fatal("expected collision for slug 'feature-auth'")
		}
		if len(branches) != 2 {
			t.Errorf("collision has %d branches, want 2", len(branches))
		}
	})

	t.Run("bare worktrees skipped", func(t *testing.T) {
		trees := []Worktree{
			{Path: "/a", Branch: "main", IsBare: true},
			{Path: "/b", Branch: "main"},
		}
		got := DetectSlugCollisions(trees)
		if len(got) != 0 {
			t.Errorf("DetectSlugCollisions() = %v, want empty (bare should be skipped)", got)
		}
	})

	t.Run("empty input", func(t *testing.T) {
		got := DetectSlugCollisions(nil)
		if len(got) != 0 {
			t.Errorf("DetectSlugCollisions(nil) = %v, want empty", got)
		}
	})
}

// initTestRepo creates a temporary git repo with an initial commit.
func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	runGit("init")
	runGit("commit", "--allow-empty", "-m", "init")
	return dir
}

func TestFindRepoRoot(t *testing.T) {
	t.Run("from repo root", func(t *testing.T) {
		dir := initTestRepo(t)
		root, err := FindRepoRoot(dir)
		if err != nil {
			t.Fatalf("FindRepoRoot() error: %v", err)
		}
		// Resolve symlinks for macOS /private/var/folders vs /var/folders
		wantAbs, _ := filepath.EvalSymlinks(dir)
		gotAbs, _ := filepath.EvalSymlinks(root)
		if gotAbs != wantAbs {
			t.Errorf("FindRepoRoot() = %q, want %q", gotAbs, wantAbs)
		}
	})

	t.Run("from subdirectory", func(t *testing.T) {
		dir := initTestRepo(t)
		sub := filepath.Join(dir, "sub", "deep")
		if err := os.MkdirAll(sub, 0755); err != nil {
			t.Fatal(err)
		}
		root, err := FindRepoRoot(sub)
		if err != nil {
			t.Fatalf("FindRepoRoot() error: %v", err)
		}
		wantAbs, _ := filepath.EvalSymlinks(dir)
		gotAbs, _ := filepath.EvalSymlinks(root)
		if gotAbs != wantAbs {
			t.Errorf("FindRepoRoot() = %q, want %q", gotAbs, wantAbs)
		}
	})

	t.Run("not a git repo", func(t *testing.T) {
		dir := t.TempDir()
		_, err := FindRepoRoot(dir)
		if err == nil {
			t.Error("FindRepoRoot() should error for non-git directory")
		}
	})
}

func TestCommonDir(t *testing.T) {
	dir := initTestRepo(t)

	common, err := CommonDir(dir)
	if err != nil {
		t.Fatalf("CommonDir() error: %v", err)
	}

	// For a regular repo, CommonDir should point to .git
	wantAbs, _ := filepath.EvalSymlinks(filepath.Join(dir, ".git"))
	gotAbs, _ := filepath.EvalSymlinks(common)
	if gotAbs != wantAbs {
		t.Errorf("CommonDir() = %q, want %q", gotAbs, wantAbs)
	}
}

func TestMainWorktreeRoot(t *testing.T) {
	dir := initTestRepo(t)

	root, err := MainWorktreeRoot(dir)
	if err != nil {
		t.Fatalf("MainWorktreeRoot() error: %v", err)
	}

	wantAbs, _ := filepath.EvalSymlinks(dir)
	gotAbs, _ := filepath.EvalSymlinks(root)
	if gotAbs != wantAbs {
		t.Errorf("MainWorktreeRoot() = %q, want %q", gotAbs, wantAbs)
	}
}

func TestListWorktrees(t *testing.T) {
	dir := initTestRepo(t)

	trees, err := ListWorktrees(dir)
	if err != nil {
		t.Fatalf("ListWorktrees() error: %v", err)
	}

	if len(trees) == 0 {
		t.Fatal("ListWorktrees() returned 0 worktrees")
	}

	// At least the main worktree should be present
	found := false
	for _, tree := range trees {
		if tree.Branch == "main" || tree.Branch == "master" {
			found = true
			break
		}
	}
	if !found {
		t.Error("ListWorktrees() should contain main/master branch")
	}
}

func TestListWorktrees_WithAdditionalWorktree(t *testing.T) {
	dir := initTestRepo(t)

	// Create a branch and worktree
	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	wtDir := filepath.Join(t.TempDir(), "feature-auth")
	runGit("worktree", "add", "-b", "feature/auth", wtDir)

	trees, err := ListWorktrees(dir)
	if err != nil {
		t.Fatalf("ListWorktrees() error: %v", err)
	}

	if len(trees) < 2 {
		t.Fatalf("ListWorktrees() returned %d worktrees, want >= 2", len(trees))
	}

	// Check the additional worktree
	found := false
	for _, tree := range trees {
		if tree.Branch == "feature/auth" {
			found = true
			wantAbs, _ := filepath.EvalSymlinks(wtDir)
			gotAbs, _ := filepath.EvalSymlinks(tree.Path)
			if gotAbs != wantAbs {
				t.Errorf("worktree path = %q, want %q", gotAbs, wantAbs)
			}
			break
		}
	}
	if !found {
		t.Error("ListWorktrees() should contain feature/auth branch")
	}
}

func TestCurrentWorktree(t *testing.T) {
	dir := initTestRepo(t)
	// Resolve symlinks (macOS /var/folders → /private/var/folders)
	dir, _ = filepath.EvalSymlinks(dir)

	tree, err := CurrentWorktree(dir)
	if err != nil {
		t.Fatalf("CurrentWorktree() error: %v", err)
	}

	if tree.Branch != "main" && tree.Branch != "master" {
		t.Errorf("CurrentWorktree().Branch = %q, want main or master", tree.Branch)
	}
}

func TestCurrentWorktree_FromSubdirectory(t *testing.T) {
	dir := initTestRepo(t)
	dir, _ = filepath.EvalSymlinks(dir)
	sub := filepath.Join(dir, "subdir")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}

	tree, err := CurrentWorktree(sub)
	if err != nil {
		t.Fatalf("CurrentWorktree() from subdir error: %v", err)
	}

	if tree.Branch != "main" && tree.Branch != "master" {
		t.Errorf("CurrentWorktree().Branch = %q, want main or master", tree.Branch)
	}
}

func TestCurrentWorktree_NotGitRepo(t *testing.T) {
	dir := t.TempDir()
	_, err := CurrentWorktree(dir)
	if err == nil {
		t.Error("CurrentWorktree() should error for non-git directory")
	}
}

func TestCommonDir_FromWorktree(t *testing.T) {
	dir := initTestRepo(t)

	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	wtDir := filepath.Join(t.TempDir(), "wt-test")
	runGit("worktree", "add", "-b", "test-branch", wtDir)

	// CommonDir from the additional worktree should point to main's .git
	common, err := CommonDir(wtDir)
	if err != nil {
		t.Fatalf("CommonDir() from worktree error: %v", err)
	}

	mainGit, _ := filepath.EvalSymlinks(filepath.Join(dir, ".git"))
	gotAbs, _ := filepath.EvalSymlinks(common)
	if gotAbs != mainGit {
		t.Errorf("CommonDir() from worktree = %q, want %q", gotAbs, mainGit)
	}
}

func TestMainWorktreeRoot_FromWorktree(t *testing.T) {
	dir := initTestRepo(t)

	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	wtDir := filepath.Join(t.TempDir(), "wt-test2")
	runGit("worktree", "add", "-b", "test-branch2", wtDir)

	root, err := MainWorktreeRoot(wtDir)
	if err != nil {
		t.Fatalf("MainWorktreeRoot() from worktree error: %v", err)
	}

	wantAbs, _ := filepath.EvalSymlinks(dir)
	gotAbs, _ := filepath.EvalSymlinks(root)
	if gotAbs != wantAbs {
		t.Errorf("MainWorktreeRoot() from worktree = %q, want %q", gotAbs, wantAbs)
	}
}

func TestParsePorcelain(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []Worktree
		wantErr bool
	}{
		{
			name: "single worktree",
			input: `worktree /home/user/project
HEAD abc1234567890123456789012345678901234567
branch refs/heads/main

`,
			want: []Worktree{
				{Path: "/home/user/project", Head: "abc1234567890123456789012345678901234567", Branch: "main"},
			},
		},
		{
			name: "two worktrees",
			input: `worktree /home/user/project
HEAD abc1234567890123456789012345678901234567
branch refs/heads/main

worktree /home/user/project-feature
HEAD def1234567890123456789012345678901234567
branch refs/heads/feature/auth

`,
			want: []Worktree{
				{Path: "/home/user/project", Head: "abc1234567890123456789012345678901234567", Branch: "main"},
				{Path: "/home/user/project-feature", Head: "def1234567890123456789012345678901234567", Branch: "feature/auth"},
			},
		},
		{
			name: "bare worktree",
			input: `worktree /home/user/project.git
HEAD abc1234567890123456789012345678901234567
branch refs/heads/main
bare

`,
			want: []Worktree{
				{Path: "/home/user/project.git", Head: "abc1234567890123456789012345678901234567", Branch: "main", IsBare: true},
			},
		},
		{
			name: "detached head",
			input: `worktree /home/user/project
HEAD abc12345abcdef01234567890123456789012345
detached

`,
			want: []Worktree{
				{Path: "/home/user/project", Head: "abc12345abcdef01234567890123456789012345", Branch: "abc12345"},
			},
		},
		{
			name:  "empty input",
			input: "",
			want:  nil,
		},
		{
			name: "branch stripping refs/heads/",
			input: `worktree /tmp/wt
HEAD aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
branch refs/heads/release/v2.0

`,
			want: []Worktree{
				{Path: "/tmp/wt", Head: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Branch: "release/v2.0"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parsePorcelain(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parsePorcelain() error = %v, wantErr %v", err, tt.wantErr)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("parsePorcelain() returned %d worktrees, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i].Path != tt.want[i].Path {
					t.Errorf("worktree[%d].Path = %q, want %q", i, got[i].Path, tt.want[i].Path)
				}
				if got[i].Branch != tt.want[i].Branch {
					t.Errorf("worktree[%d].Branch = %q, want %q", i, got[i].Branch, tt.want[i].Branch)
				}
				if got[i].Head != tt.want[i].Head {
					t.Errorf("worktree[%d].Head = %q, want %q", i, got[i].Head, tt.want[i].Head)
				}
				if got[i].IsBare != tt.want[i].IsBare {
					t.Errorf("worktree[%d].IsBare = %v, want %v", i, got[i].IsBare, tt.want[i].IsBare)
				}
			}
		})
	}
}
