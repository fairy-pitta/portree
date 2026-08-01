package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/fairy-pitta/portree/internal/config"
	"github.com/fairy-pitta/portree/internal/git"
	"github.com/fairy-pitta/portree/internal/logging"
	"github.com/spf13/cobra"
)

var (
	// Populated by PersistentPreRunE for subcommands.
	//
	// repoRoot is the current worktree root (from `git rev-parse --show-toplevel`).
	// It is used for loading .portree.toml and resolving service working dirs,
	// both of which are per-worktree.
	repoRoot string
	// stateRoot is the main worktree root. Runtime state (.portree/state.json)
	// is shared across all worktrees, so it must live under the main worktree
	// rather than each linked worktree's own root — otherwise a single proxy
	// can only resolve the worktree it was started from.
	stateRoot string
	cfg       *config.Config
)

var rootCmd = &cobra.Command{
	Use:           "portree",
	Short:         "Git Worktree Server Manager",
	Long:          "portree manages multiple dev servers per git worktree with automatic port allocation and reverse proxy routing.",
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Configure log level from flags.
		verbose, _ := cmd.Flags().GetBool("verbose")
		quiet, _ := cmd.Flags().GetBool("quiet")
		if verbose {
			logging.SetLevel(logging.LevelVerbose)
		}
		if quiet {
			logging.SetLevel(logging.LevelQuiet)
		}

		// Skip repo/config detection for commands that opt out.
		if cmd.Annotations["skipRepoDetection"] == "true" {
			return nil
		}

		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("getting current directory: %w", err)
		}

		repoRoot, err = git.FindRepoRoot(cwd)
		if err != nil {
			return fmt.Errorf("not inside a git repository")
		}

		stateRoot, err = git.MainWorktreeRoot(cwd)
		if err != nil {
			return fmt.Errorf("resolving main worktree root: %w", err)
		}

		logging.Verbose("repo root: %s", repoRoot)
		logging.Verbose("state root: %s", stateRoot)

		cfg, err = config.Load(repoRoot)
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		logging.Verbose("loaded config with %d service(s)", len(cfg.Services))

		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Enable verbose output")
	rootCmd.PersistentFlags().BoolP("quiet", "q", false, "Suppress all non-error output")
	rootCmd.MarkFlagsMutuallyExclusive("verbose", "quiet")
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

// unknownServiceError reports a --service value that is not configured, listing
// what is, so the user does not have to open the config to find the right name.
func unknownServiceError(c *config.Config, name string) error {
	if c == nil || len(c.Services) == 0 {
		return fmt.Errorf("unknown service %q: no services are configured in %s", name, config.FileName)
	}
	return fmt.Errorf("unknown service %q; configured services: %s",
		name, strings.Join(sortedServiceNames(c), ", "))
}
