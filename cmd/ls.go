package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"text/tabwriter"

	"github.com/fairy-pitta/portree/internal/git"
	"github.com/fairy-pitta/portree/internal/process"
	"github.com/fairy-pitta/portree/internal/state"
	"github.com/spf13/cobra"
)

var lsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List all worktrees and their services",
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}

		trees, err := git.ListWorktrees(cwd)
		if err != nil {
			return fmt.Errorf("listing worktrees: %w", err)
		}

		// Load state for runtime info.
		stateDir := filepath.Join(repoRoot, ".portree")
		store, err := state.NewFileStore(stateDir)
		if err != nil {
			return err
		}

		var st *state.State
		_ = store.WithLock(func() error {
			st, err = store.Load()
			return err
		})
		if st == nil {
			st = &state.State{
				Services:        map[string]map[string]*state.ServiceState{},
				PortAssignments: map[string]int{},
			}
		}

		// Sort service names for consistent output.
		serviceNames := make([]string, 0, len(cfg.Services))
		for name := range cfg.Services {
			serviceNames = append(serviceNames, name)
		}
		sort.Strings(serviceNames)

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "WORKTREE\tSERVICE\tPORT\tSTATUS\tPID")

		for _, tree := range trees {
			if tree.IsBare {
				continue
			}
			branch := tree.Branch
			if branch == "" {
				branch = "(detached)"
			}

			for _, svcName := range serviceNames {
				ss := state.GetServiceState(st, tree.Branch, svcName)
				portStr := "—"
				statusStr := "stopped"
				pidStr := "—"

				if ss != nil {
					if ss.Port > 0 {
						portStr = fmt.Sprintf("%d", ss.Port)
					}
					if ss.PID > 0 && process.IsProcessRunning(ss.PID) {
						statusStr = "running"
						pidStr = fmt.Sprintf("%d", ss.PID)
					} else if ss.Status == "running" && ss.PID > 0 {
						statusStr = "stopped" // stale
					} else {
						statusStr = ss.Status
					}
				}

				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", branch, svcName, portStr, statusStr, pidStr)
			}
		}

		return w.Flush()
	},
}

func init() {
	rootCmd.AddCommand(lsCmd)
}
