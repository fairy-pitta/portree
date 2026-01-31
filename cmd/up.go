package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/fairy-pitta/portree/internal/git"
	"github.com/fairy-pitta/portree/internal/port"
	"github.com/fairy-pitta/portree/internal/process"
	"github.com/fairy-pitta/portree/internal/state"
	"github.com/spf13/cobra"
)

var (
	upAll     bool
	upService string
)

var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Start dev servers for the current worktree",
	Long:  "Starts all configured services (or a specific one) for the current worktree, or all worktrees with --all.",
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}

		// Validate service filter.
		if upService != "" {
			if _, ok := cfg.Services[upService]; !ok {
				return fmt.Errorf("unknown service %q", upService)
			}
		}

		stateDir := filepath.Join(repoRoot, ".portree")
		store, err := state.NewFileStore(stateDir)
		if err != nil {
			return err
		}

		registry := port.NewRegistry(store, cfg)
		mgr := process.NewManager(cfg, store, registry)

		var trees []git.Worktree
		if upAll {
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

		// Warn about branch slug collisions.
		if collisions := git.DetectSlugCollisions(trees); len(collisions) > 0 {
			for slug, branches := range collisions {
				fmt.Fprintf(os.Stderr, "Warning: branches %v all map to slug %q; proxy routing may be ambiguous\n", branches, slug)
			}
		}

		totalStarted := 0
		for _, tree := range trees {
			if tree.IsBare {
				continue
			}
			results := mgr.StartServices(&tree, upService)
			for _, r := range results {
				if r.Err != nil {
					fmt.Fprintf(os.Stderr, "Error starting %s/%s: %v\n", r.Branch, r.Service, r.Err)
				} else {
					fmt.Printf("Starting %s (port %d) for %s ...\n", r.Service, r.Port, r.Branch)
					totalStarted++
				}
			}
		}

		if totalStarted > 0 {
			noun := "services"
			if totalStarted == 1 {
				noun = "service"
			}
			if upAll {
				fmt.Printf("✓ %d %s started\n", totalStarted, noun)
			} else {
				fmt.Printf("✓ %d %s started for %s\n", totalStarted, noun, trees[0].Branch)
			}
		}

		return nil
	},
}

func init() {
	upCmd.Flags().BoolVar(&upAll, "all", false, "Start services for all worktrees")
	upCmd.Flags().StringVar(&upService, "service", "", "Start only a specific service")
	rootCmd.AddCommand(upCmd)
}
