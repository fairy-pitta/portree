package git

import (
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
