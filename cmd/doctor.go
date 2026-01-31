package cmd

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"

	"github.com/fairy-pitta/portree/internal/config"
	"github.com/fairy-pitta/portree/internal/git"
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

		// Config and state checks require a repo.
		root, rootErr := git.FindRepoRoot(cwd)
		if rootErr == nil {
			results = append(results, checkConfig(root))

			cfgObj, cfgErr := config.Load(root)
			if cfgErr == nil {
				results = append(results, checkPortConflicts(cfgObj)...)
				results = append(results, checkStaleState(root))
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

func checkPortConflicts(cfg *config.Config) []checkResult {
	// Sort for deterministic output order.
	names := make([]string, 0, len(cfg.Services))
	for name := range cfg.Services {
		names = append(names, name)
	}
	sort.Strings(names)

	var results []checkResult
	for _, name := range names {
		svc := cfg.Services[name]
		ln, err := net.Listen("tcp", ":"+strconv.Itoa(svc.ProxyPort))
		if err != nil {
			results = append(results, checkResult{
				name:   fmt.Sprintf("proxy port %d (%s) available", svc.ProxyPort, name),
				ok:     false,
				detail: fmt.Sprintf("port %d already in use", svc.ProxyPort),
			})
		} else {
			_ = ln.Close()
			results = append(results, checkResult{
				name: fmt.Sprintf("proxy port %d (%s) available", svc.ProxyPort, name),
				ok:   true,
			})
		}
	}
	return results
}

func checkStaleState(root string) checkResult {
	stateDir := filepath.Join(root, ".portree")
	store, err := state.NewFileStore(stateDir)
	if err != nil {
		return checkResult{name: "state file healthy", ok: true, detail: "no state directory"}
	}

	st, err := store.Load()
	if err != nil {
		return checkResult{name: "state file healthy", ok: false, detail: err.Error()}
	}

	var staleDetails []string
	for branch, services := range st.Services {
		for svcName, ss := range services {
			if ss.Status == "running" && ss.PID > 0 && !process.IsProcessRunning(ss.PID) {
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

func trimNewline(s string) string {
	if len(s) > 0 && s[len(s)-1] == '\n' {
		return s[:len(s)-1]
	}
	return s
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}
