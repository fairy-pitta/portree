package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/fairy-pitta/portree/internal/browser"
	"github.com/fairy-pitta/portree/internal/git"
	"github.com/fairy-pitta/portree/internal/logging"
	"github.com/fairy-pitta/portree/internal/state"
	"github.com/spf13/cobra"
)

var openService string

var openCmd = &cobra.Command{
	Use:   "open",
	Short: "Open the current worktree's service in a browser",
	Long: `Open the current worktree's service URL in the default browser.

The URL is constructed as http://<branch-slug>.localhost:<proxy_port>.
By default, the first service (alphabetically) is used.
Use --service to specify a different service.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("getting current directory: %w", err)
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

		// Determine scheme from proxy state.
		scheme := "http"
		stateDir := filepath.Join(repoRoot, ".portree")
		if store, err := state.NewFileStore(stateDir); err == nil {
			if err := store.WithLock(func() error {
				st, e := store.Load()
				if e != nil {
					return e
				}
				if st.Proxy.HTTPS {
					scheme = "https"
				}
				return nil
			}); err != nil {
				logging.Warn("failed to load proxy state: %v", err)
			}
		}

		url := browser.BuildURL(scheme, tree.Slug(), svc.ProxyPort)
		fmt.Printf("Opening %s ...\n", url)
		return browser.Open(url)
	},
}

func init() {
	openCmd.Flags().StringVar(&openService, "service", "", "Service to open (default: first service)")
	rootCmd.AddCommand(openCmd)
}
