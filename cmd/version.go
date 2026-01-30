package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Set via ldflags at build time.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version of gws",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("gws %s (commit: %s, built: %s)\n", version, commit, date)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
