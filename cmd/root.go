package cmd

import (
	"fmt"
	"os"

	"github.com/fairy-pitta/portree/internal/config"
	"github.com/fairy-pitta/portree/internal/git"
	"github.com/fairy-pitta/portree/internal/logging"
	"github.com/spf13/cobra"
)

var (
	// Populated by PersistentPreRunE for subcommands.
	repoRoot string
	cfg      *config.Config
)

var rootCmd = &cobra.Command{
	Use:   "portree",
	Short: "Git Worktree Server Manager",
	Long:  "portree manages multiple dev servers per git worktree with automatic port allocation and reverse proxy routing.",
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

		// Skip repo/config detection for commands that don't need it.
		if cmd.Name() == "init" || cmd.Name() == "version" || cmd.Name() == "completion" || cmd.Name() == "doctor" {
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

		logging.Verbose("repo root: %s", repoRoot)

		cfg, err = config.Load(repoRoot)
		if err != nil {
			return err
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
