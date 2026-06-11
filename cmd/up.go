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
	upAll     bool
	upService string
	upNoProxy bool
	upSkip    []string
)

var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Start dev servers for the current worktree",
	Long:  "Starts all configured services (or a specific one) for the current worktree, or all worktrees with --all.",
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("getting current directory: %w", err)
		}

		// Validate service filter.
		if upService != "" {
			if _, ok := cfg.Services[upService]; !ok {
				return unknownServiceError(cfg, upService)
			}
		}
		for _, skip := range upSkip {
			if _, ok := cfg.Services[skip]; !ok {
				return fmt.Errorf("unknown service %q in --skip", skip)
			}
		}

		stateDir := filepath.Join(stateRoot, ".portree")
		store, err := state.NewFileStore(stateDir)
		if err != nil {
			return fmt.Errorf("creating state store: %w", err)
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
				logging.Warn("branches %v all map to slug %q; proxy routing may be ambiguous", branches, slug)
			}
		}

		totalStarted := 0
		alreadyRunning := 0
		startFailures := 0
		for _, tree := range trees {
			if tree.IsBare {
				continue
			}
			logging.Verbose("starting services for worktree %s (%s)", tree.Branch, tree.Path)
			results := mgr.StartServices(&tree, upService, upSkip...)
			for _, r := range results {
				switch {
				case r.Err != nil:
					logging.Error("starting %s/%s: %v", r.Branch, r.Service, r.Err)
					startFailures++
				case r.AlreadyRunning:
					logging.Info("%s already running (port %d, pid %d) for %s", r.Service, r.Port, r.PID, r.Branch)
					alreadyRunning++
				default:
					logging.Info("Starting %s (port %d) for %s ...", r.Service, r.Port, r.Branch)
					totalStarted++
				}
			}
		}

		svcNoun := func(n int) string {
			if n == 1 {
				return "service"
			}
			return "services"
		}

		if totalStarted > 0 {
			if upAll {
				logging.Info("✓ %d %s started", totalStarted, svcNoun(totalStarted))
			} else {
				logging.Info("✓ %d %s started for %s", totalStarted, svcNoun(totalStarted), trees[0].Branch)
			}
		}

		if alreadyRunning > 0 {
			if upAll {
				logging.Info("%d %s already running", alreadyRunning, svcNoun(alreadyRunning))
			} else {
				logging.Info("%d %s already running for %s", alreadyRunning, svcNoun(alreadyRunning), trees[0].Branch)
			}
		}

		if startFailures > 0 {
			return fmt.Errorf("%d %s failed to start", startFailures, svcNoun(startFailures))
		}

		// Bring the proxy up so the URLs printed below actually answer. A
		// failure here does not fail the command: the services did start and
		// remain reachable on their direct ports.
		if !upNoProxy {
			status, perr := ensureProxyRunning(stateRoot, cfg, nil)
			if perr != nil {
				logging.Warn("could not start the proxy: %v", perr)
				logging.Warn("services are reachable on their direct ports; see 'portree ls'")
				return nil
			}
			if status.Running {
				logging.Info("✓ Proxy running (%s, pid %d)", status.Scheme, status.PID)
				for _, line := range serviceURLs(trees, cfg, status.Scheme, upService) {
					logging.Info("  %s", line)
				}
			}
		}

		return nil
	},
}

func init() {
	upCmd.Flags().BoolVar(&upAll, "all", false, "Start services for all worktrees")
	upCmd.Flags().StringVar(&upService, "service", "", "Start only a specific service")
	upCmd.Flags().BoolVar(&upNoProxy, "no-proxy", false, "Do not start the reverse proxy")
	upCmd.Flags().StringSliceVar(&upSkip, "skip", nil, "Allocate ports but do not start these services")
	rootCmd.AddCommand(upCmd)
}
