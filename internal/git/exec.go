package git

import (
	"os"
	"os/exec"
	"strings"
)

// repoPinningGitVars are environment variables that bind git to a specific
// repository, work tree, or index. portree always selects the target repo via
// the command's working directory (cmd.Dir), so any of these inherited from the
// environment must be stripped. They are exported by git when it runs hooks
// (e.g. a pre-commit hook sets GIT_DIR/GIT_INDEX_FILE), and an inherited value
// would otherwise hijack the command and make it operate on the wrong repo.
// See "Environment Variables" in https://git-scm.com/docs/git.
var repoPinningGitVars = map[string]struct{}{
	"GIT_DIR":                          {},
	"GIT_WORK_TREE":                    {},
	"GIT_INDEX_FILE":                   {},
	"GIT_COMMON_DIR":                   {},
	"GIT_PREFIX":                       {},
	"GIT_OBJECT_DIRECTORY":             {},
	"GIT_ALTERNATE_OBJECT_DIRECTORIES": {},
	"GIT_NAMESPACE":                    {},
}

// SanitizedEnv returns the current process environment with the repo-pinning
// git variables removed (see repoPinningGitVars). When none are set — the
// common case — the environment is returned unchanged. It is exported so tests
// in other packages can spawn git for fixture setup in a chosen directory
// without an inherited GIT_DIR hijacking the command.
func SanitizedEnv() []string {
	environ := os.Environ()
	env := make([]string, 0, len(environ))
	for _, e := range environ {
		key, _, _ := strings.Cut(e, "=")
		if _, pinned := repoPinningGitVars[key]; pinned {
			continue
		}
		env = append(env, e)
	}
	return env
}

// gitCmd builds a git command that operates on the repository at dir,
// independent of any inherited GIT_DIR/GIT_INDEX_FILE/etc.
func gitCmd(dir string, args ...string) *exec.Cmd {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = SanitizedEnv()
	return cmd
}
