package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/fairy-pitta/portree/internal/git"
	"github.com/fairy-pitta/portree/internal/logging"
	"github.com/fairy-pitta/portree/internal/port"
	"github.com/fairy-pitta/portree/internal/process"
	"github.com/fairy-pitta/portree/internal/state"
	"github.com/spf13/cobra"
)

var (
	downAll     bool
	downService string
)

var downCmd = &cobra.Command{
	Use:   "down",
	Short: "Stop dev servers for the current worktree",
	Long:  "Stops all running services (or a specific one) for the current worktree, or all worktrees with --all.",
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("getting current directory: %w", err)
		}

		if downService != "" {
			if _, ok := cfg.Services[downService]; !ok {
				return fmt.Errorf("unknown service %q", downService)
			}
		}

		stateDir := filepath.Join(repoRoot, ".portree")
		store, err := state.NewFileStore(stateDir)
		if err != nil {
			return fmt.Errorf("creating state store: %w", err)
		}

		registry := port.NewRegistry(store, cfg)
		mgr := process.NewManager(cfg, store, registry)

		var trees []git.Worktree
		if downAll {
			trees, err = git.ListWorktrees(cwd)
			if err != nil {
				return fmt.Errorf("listing worktrees: %w", err)
			}
		} else {
			tree, err := git.CurrentWorktree(cwd)
			if err != nil {
				return fmt.Errorf("detecting worktree: %w", err)
			}
			trees = []git.Worktree{*tree}
		}

		totalStopped := 0
		for _, tree := range trees {
			if tree.IsBare {
				continue
			}
			results := mgr.StopServices(&tree, downService)
			for _, r := range results {
				if r.Err != nil {
					logging.Error("stopping %s/%s: %v", r.Branch, r.Service, r.Err)
				} else {
					logging.Info("Stopping %s for %s ...", r.Service, r.Branch)
					totalStopped++
				}
			}
		}

		if totalStopped > 0 {
			noun := "services"
			if totalStopped == 1 {
				noun = "service"
			}
			if downAll {
				logging.Info("✓ %d %s stopped", totalStopped, noun)
			} else {
				logging.Info("✓ %d %s stopped for %s", totalStopped, noun, trees[0].Branch)
			}
		}

		return nil
	},
}

func init() {
	downCmd.Flags().BoolVar(&downAll, "all", false, "Stop services for all worktrees")
	downCmd.Flags().StringVar(&downService, "service", "", "Stop only a specific service")
	rootCmd.AddCommand(downCmd)
}
