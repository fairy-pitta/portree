package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/fairy-pitta/portree/internal/browser"
	"github.com/fairy-pitta/portree/internal/config"
	"github.com/fairy-pitta/portree/internal/git"
	"github.com/fairy-pitta/portree/internal/logging"
	"github.com/fairy-pitta/portree/internal/process"
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
			return unknownServiceError(cfg, svcName)
		}

		proxy := describeProxy(cfg, loadProxyState(stateRoot))
		url, err := openURL(svc, tree.Slug(), proxy, serviceIsRunning(stateRoot, tree.Branch, svcName))
		if err != nil {
			return err
		}

		fmt.Printf("Opening %s ...\n", url)
		return browser.Open(url)
	},
}

// openURL returns the URL to open, or an error explaining why opening one would
// be pointless. Launching a browser at a URL nothing serves leaves the user on
// a connection error that looks like portree malfunctioning, so this refuses
// instead.
func openURL(svc config.ServiceConfig, slug string, proxy proxyStatus, serviceRunning bool) (string, error) {
	url := browser.BuildURL(proxy.Scheme, slug, svc.ProxyPort)

	if !proxy.Running {
		return "", fmt.Errorf("the proxy is not running, so %s would not respond\n"+
			"       start it with \"portree up\" or \"portree proxy start --detach\"", url)
	}
	if !serviceRunning {
		return "", fmt.Errorf("no service is running behind %s\n"+
			"       start it with \"portree up\"", url)
	}
	return url, nil
}

// serviceIsRunning reports whether the recorded process for a service is alive.
func serviceIsRunning(stateRoot, branch, svcName string) bool {
	store, err := state.NewFileStore(filepath.Join(stateRoot, ".portree"))
	if err != nil {
		return false
	}

	running := false
	if err := store.WithLock(func() error {
		st, e := store.Load()
		if e != nil {
			return e
		}
		ss := state.GetServiceState(st, branch, svcName)
		running = ss != nil && ss.PID > 0 && process.IsProcessRunning(ss.PID)
		return nil
	}); err != nil {
		logging.Warn("failed to load service state: %v", err)
		return false
	}
	return running
}

func init() {
	openCmd.Flags().StringVar(&openService, "service", "", "Service to open (default: first service)")
	rootCmd.AddCommand(openCmd)
}
