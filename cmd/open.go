package cmd

import (
	"fmt"
	"os"

	"github.com/fairy-pitta/portree/internal/browser"
	"github.com/fairy-pitta/portree/internal/git"
	"github.com/spf13/cobra"
)

var openService string

var openCmd = &cobra.Command{
	Use:   "open",
	Short: "Open the current worktree's service in a browser",
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}

		tree, err := git.CurrentWorktree(cwd)
		if err != nil {
			return fmt.Errorf("detecting worktree: %w", err)
		}

		// Determine which service to open.
		svcName := openService
		if svcName == "" {
			// Use the first service alphabetically.
			for name := range cfg.Services {
				if svcName == "" || name < svcName {
					svcName = name
				}
			}
		}

		svc, ok := cfg.Services[svcName]
		if !ok {
			return fmt.Errorf("unknown service %q", svcName)
		}

		url := browser.BuildURL(tree.Slug(), svc.ProxyPort)
		fmt.Printf("Opening %s ...\n", url)
		return browser.Open(url)
	},
}

func init() {
	openCmd.Flags().StringVar(&openService, "service", "", "Service to open (default: first service)")
	rootCmd.AddCommand(openCmd)
}
