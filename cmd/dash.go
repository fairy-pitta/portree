package cmd

import (
	"github.com/fairy-pitta/portree/internal/tui"
	"github.com/spf13/cobra"
)

var dashCmd = &cobra.Command{
	Use:   "dash",
	Short: "Open the TUI dashboard",
	Long:  "Launches an interactive terminal dashboard to manage all worktree services.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return tui.Run(cfg, repoRoot)
	},
}

func init() {
	rootCmd.AddCommand(dashCmd)
}
