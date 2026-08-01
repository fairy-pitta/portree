package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"

	"github.com/fairy-pitta/portree/internal/config"
	"github.com/fairy-pitta/portree/internal/git"
	"github.com/fairy-pitta/portree/internal/port"
	"github.com/fairy-pitta/portree/internal/process"
	"github.com/fairy-pitta/portree/internal/state"
	"github.com/spf13/cobra"
)

type checkResult struct {
	name   string
	ok     bool
	detail string
}

var doctorCmd = &cobra.Command{
	Use:         "doctor",
	Short:       "Check environment and diagnose common issues",
	Long:        "Runs a series of checks to verify that portree's dependencies and configuration are healthy.",
	Annotations: map[string]string{"skipRepoDetection": "true"},
	RunE: func(cmd *cobra.Command, args []string) error {
		var results []checkResult

		results = append(results, checkGit())

		cwd, err := os.Getwd()
		if err != nil {
			results = append(results, checkResult{
				name: "inside git repository", ok: false, detail: err.Error(),
			})
			printResults(results)
			return nil
		}

		results = append(results, checkRepo(cwd))

		// Config checks use the current worktree root; state checks use the
		// main worktree root, where the shared .portree state lives.
		root, rootErr := git.FindRepoRoot(cwd)
		if rootErr == nil {
			results = append(results, checkConfig(root))

			cfgObj, cfgErr := config.Load(root)
			if cfgErr == nil {
				stateRoot, stateErr := git.MainWorktreeRoot(cwd)
				if stateErr != nil {
					stateRoot = root
				}

				results = append(results, checkPortConflicts(cfgObj, loadProxyState(stateRoot))...)

				if trees, err := git.ListWorktrees(cwd); err == nil {
					results = append(results, checkServiceDirs(cfgObj, trees)...)
				}

				results = append(results, checkStaleState(stateRoot))
				results = append(results, checkStaleWorktrees(stateRoot, cwd))
			}
		}

		printResults(results)
		return nil
	},
}

func printResults(results []checkResult) {
	allOK := true
	for _, r := range results {
		mark := "✓"
		if !r.ok {
			mark = "✗"
			allOK = false
		}
		fmt.Printf("  %s  %s\n", mark, r.name)
		if r.detail != "" {
			fmt.Printf("     %s\n", r.detail)
		}
	}

	if allOK {
		fmt.Println("\nAll checks passed.")
	} else {
		fmt.Println("\nSome checks failed. See details above.")
	}
}

func checkGit() checkResult {
	path, err := exec.LookPath("git")
	if err != nil {
		return checkResult{name: "git installed", ok: false, detail: "git not found in PATH"}
	}
	out, err := exec.Command("git", "--version").Output()
	if err != nil {
		return checkResult{name: "git installed", ok: false, detail: "git found but failed to run"}
	}
	return checkResult{
		name:   "git installed",
		ok:     true,
		detail: fmt.Sprintf("%s (%s)", trimNewline(string(out)), path),
	}
}

func checkRepo(cwd string) checkResult {
	root, err := git.FindRepoRoot(cwd)
	if err != nil {
		return checkResult{name: "inside git repository", ok: false, detail: "not inside a git repository"}
	}
	return checkResult{name: "inside git repository", ok: true, detail: root}
}

func checkConfig(root string) checkResult {
	cfgPath := filepath.Join(root, config.FileName)
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		return checkResult{
			name:   "config file",
			ok:     false,
			detail: fmt.Sprintf("%s not found (run 'portree init' to create)", config.FileName),
		}
	}

	cfg, err := config.Load(root)
	if err != nil {
		return checkResult{name: "config file", ok: false, detail: err.Error()}
	}

	return checkResult{
		name:   "config file",
		ok:     true,
		detail: fmt.Sprintf("%d service(s) defined", len(cfg.Services)),
	}
}

// loadProxyState reads the recorded proxy state, returning the zero value when
// it cannot be read. Diagnostics should still run when state is unavailable.
func loadProxyState(stateRoot string) state.ProxyState {
	store, err := state.NewFileStore(filepath.Join(stateRoot, ".portree"))
	if err != nil {
		return state.ProxyState{}
	}
	var proxy state.ProxyState
	if err := store.WithLock(func() error {
		st, e := store.Load()
		if e != nil {
			return e
		}
		proxy = st.Proxy
		return nil
	}); err != nil {
		return state.ProxyState{}
	}
	return proxy
}

// sortedServiceNames returns service names in a stable order so doctor output
// does not reshuffle between runs.
func sortedServiceNames(cfg *config.Config) []string {
	names := make([]string, 0, len(cfg.Services))
	for name := range cfg.Services {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// checkServiceDirs reports service working directories that do not exist.
// A missing dir otherwise surfaces at start time as ENOENT on /bin/sh, which
// points nowhere near the actual problem.
func checkServiceDirs(cfg *config.Config, trees []git.Worktree) []checkResult {
	var results []checkResult

	for _, tree := range trees {
		if tree.IsBare {
			continue
		}
		branch := tree.Branch
		if branch == "" {
			branch = "(detached)"
		}

		for _, name := range sortedServiceNames(cfg) {
			svc := cfg.Services[name]
			// An empty dir means the worktree root, which always exists.
			if svc.Dir == "" {
				continue
			}

			path := filepath.Join(tree.Path, svc.Dir)
			label := fmt.Sprintf("working dir for %s in %s", name, branch)

			info, err := os.Stat(path)
			switch {
			case os.IsNotExist(err):
				results = append(results, checkResult{
					name:   label,
					ok:     false,
					detail: fmt.Sprintf("%s does not exist", path),
				})
			case err != nil:
				results = append(results, checkResult{
					name: label, ok: false, detail: err.Error(),
				})
			case !info.IsDir():
				results = append(results, checkResult{
					name:   label,
					ok:     false,
					detail: fmt.Sprintf("%s is not a directory", path),
				})
			}
		}
	}

	if len(results) == 0 {
		return []checkResult{{name: "service working dirs", ok: true}}
	}
	return results
}

// checkPortConflicts reports whether each proxy port can be bound. A busy port
// is only a problem when someone other than portree holds it, so the recorded
// proxy PID decides between "our proxy is serving" and a real conflict.
func checkPortConflicts(cfg *config.Config, proxy state.ProxyState) []checkResult {
	ourProxyRunning := proxy.Status == state.StatusRunning &&
		proxy.PID > 0 && process.IsProcessRunning(proxy.PID)

	var results []checkResult
	for _, name := range sortedServiceNames(cfg) {
		svc := cfg.Services[name]

		// port.IsFree probes both stacks with SO_REUSEADDR disabled. A raw
		// net.Listen on the wildcard address misses a listener bound to
		// 127.0.0.1, which is exactly how the proxy binds.
		if port.IsFree(svc.ProxyPort) {
			results = append(results, checkResult{
				name: fmt.Sprintf("proxy port %d (%s) available", svc.ProxyPort, name),
				ok:   true,
			})
			continue
		}

		if ourProxyRunning {
			results = append(results, checkResult{
				name:   fmt.Sprintf("proxy port %d (%s) — portree proxy running", svc.ProxyPort, name),
				ok:     true,
				detail: fmt.Sprintf("pid %d", proxy.PID),
			})
			continue
		}

		results = append(results, checkResult{
			name:   fmt.Sprintf("proxy port %d (%s) unavailable", svc.ProxyPort, name),
			ok:     false,
			detail: fmt.Sprintf("port %d is held by another process", svc.ProxyPort),
		})
	}
	return results
}

func checkStaleState(root string) checkResult {
	stateDir := filepath.Join(root, ".portree")
	store, err := state.NewFileStore(stateDir)
	if err != nil {
		return checkResult{name: "state file healthy", ok: true, detail: "no state directory"}
	}

	var st *state.State
	if err := store.WithLock(func() error {
		var e error
		st, e = store.Load()
		return e
	}); err != nil {
		return checkResult{name: "state file healthy", ok: false, detail: err.Error()}
	}

	var staleDetails []string
	for branch, services := range st.Services {
		for svcName, ss := range services {
			if ss.Status == state.StatusRunning && ss.PID > 0 && !process.IsProcessRunning(ss.PID) {
				staleDetails = append(staleDetails, fmt.Sprintf("%s/%s (PID %d)", branch, svcName, ss.PID))
			}
		}
	}

	if len(staleDetails) > 0 {
		return checkResult{
			name:   "state file healthy",
			ok:     false,
			detail: fmt.Sprintf("%d stale: %v", len(staleDetails), staleDetails),
		}
	}

	return checkResult{name: "state file healthy", ok: true}
}

func checkStaleWorktrees(root, cwd string) checkResult {
	stateDir := filepath.Join(root, ".portree")
	store, err := state.NewFileStore(stateDir)
	if err != nil {
		return checkResult{name: "worktree state consistent", ok: true}
	}

	var st *state.State
	if err := store.WithLock(func() error {
		var e error
		st, e = store.Load()
		return e
	}); err != nil {
		return checkResult{name: "worktree state consistent", ok: false, detail: err.Error()}
	}

	trees, err := git.ListWorktrees(cwd)
	if err != nil {
		return checkResult{name: "worktree state consistent", ok: false, detail: err.Error()}
	}

	// Build set of branches that have worktrees on disk.
	activeBranches := make(map[string]bool, len(trees))
	for _, t := range trees {
		if !t.IsBare {
			activeBranches[t.Branch] = true
		}
	}

	// Find branches in state that have no worktree on disk.
	orphaned := state.OrphanedBranches(st, activeBranches)
	sort.Strings(orphaned)

	if len(orphaned) > 0 {
		return checkResult{
			name:   "worktree state consistent",
			ok:     false,
			detail: fmt.Sprintf("%d orphaned: %v (run 'portree down --prune' to clean)", len(orphaned), orphaned),
		}
	}

	return checkResult{name: "worktree state consistent", ok: true}
}

func trimNewline(s string) string {
	if len(s) > 0 && s[len(s)-1] == '\n' {
		return s[:len(s)-1]
	}
	return s
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}
