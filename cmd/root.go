package cmd

import (
	"fmt"
	"os"

	"github.com/fairy-pitta/portree/internal/config"
	"github.com/fairy-pitta/portree/internal/git"
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
		// Skip repo/config detection for commands that don't need it.
		if cmd.Name() == "init" || cmd.Name() == "version" {
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

		cfg, err = config.Load(repoRoot)
		if err != nil {
			return err
		}

		return nil
	},
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}
