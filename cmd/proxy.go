package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"syscall"

	"github.com/fairy-pitta/portree/internal/logging"
	"github.com/fairy-pitta/portree/internal/proxy"
	"github.com/fairy-pitta/portree/internal/state"
	"github.com/spf13/cobra"
)

var proxyCmd = &cobra.Command{
	Use:   "proxy",
	Short: "Manage the reverse proxy",
	Long:  "Start or stop the reverse proxy for subdomain-based routing.",
}

var proxyStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the reverse proxy",
	Long: `Start the reverse proxy in the foreground.

Launches HTTP listeners for each configured proxy_port, routing requests
based on the Host header subdomain (e.g., feature-auth.localhost:3000).
The proxy runs until interrupted with Ctrl+C (SIGINT) or SIGTERM.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		stateDir := filepath.Join(repoRoot, ".portree")
		store, err := state.NewFileStore(stateDir)
		if err != nil {
			return fmt.Errorf("creating state store: %w", err)
		}

		resolver := proxy.NewResolver(cfg, store)
		server := proxy.NewProxyServer(resolver)

		// Collect proxy ports.
		proxyPorts := map[string]int{}
		for name, svc := range cfg.Services {
			proxyPorts[name] = svc.ProxyPort
		}

		if err := server.Start(proxyPorts); err != nil {
			return err
		}

		// Update state.
		if err := store.WithLock(func() error {
			st, e := store.Load()
			if e != nil {
				return e
			}
			st.Proxy = state.ProxyState{
				PID:    os.Getpid(),
				Status: state.StatusRunning,
			}
			return store.Save(st)
		}); err != nil {
			logging.Warn("failed to save proxy state: %v", err)
		}

		fmt.Println("Proxy started:")
		// Sort for consistent output.
		names := make([]string, 0, len(proxyPorts))
		for name := range proxyPorts {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			fmt.Printf("  :%d → %s services\n", proxyPorts[name], name)
		}

		fmt.Println("\nAccess your services at:")
		fmt.Println("  http://<branch-slug>.localhost:<proxy_port>")

		// Wait for interrupt.
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig

		fmt.Println("\nStopping proxy...")
		if err := server.Stop(); err != nil {
			logging.Warn("error stopping proxy server: %v", err)
		}

		if err := store.WithLock(func() error {
			st, e := store.Load()
			if e != nil {
				return e
			}
			st.Proxy = state.ProxyState{Status: state.StatusStopped}
			return store.Save(st)
		}); err != nil {
			logging.Warn("failed to update proxy state: %v", err)
		}

		fmt.Println("Proxy stopped.")
		return nil
	},
}

var proxyStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the reverse proxy",
	Long: `Stop a running reverse proxy process.

Sends SIGTERM to the proxy process recorded in the state file
and updates the state to stopped.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		stateDir := filepath.Join(repoRoot, ".portree")
		store, err := state.NewFileStore(stateDir)
		if err != nil {
			return fmt.Errorf("creating state store: %w", err)
		}

		var st *state.State
		if err := store.WithLock(func() error {
			var e error
			st, e = store.Load()
			return e
		}); err != nil {
			return fmt.Errorf("loading proxy state: %w", err)
		}

		if st.Proxy.PID > 0 && st.Proxy.Status == state.StatusRunning {
			// Send SIGTERM to the proxy process.
			proc, err := os.FindProcess(st.Proxy.PID)
			if err == nil {
				if sigErr := proc.Signal(syscall.SIGTERM); sigErr != nil {
					logging.Warn("failed to send SIGTERM to proxy process %d: %v", st.Proxy.PID, sigErr)
				}
			}

			if err := store.WithLock(func() error {
				st, e := store.Load()
				if e != nil {
					return e
				}
				st.Proxy = state.ProxyState{Status: state.StatusStopped}
				return store.Save(st)
			}); err != nil {
				logging.Warn("failed to update proxy state: %v", err)
			}

			fmt.Println("Proxy stopped.")
		} else {
			fmt.Println("Proxy is not running.")
		}

		return nil
	},
}

func init() {
	proxyCmd.AddCommand(proxyStartCmd)
	proxyCmd.AddCommand(proxyStopCmd)
	rootCmd.AddCommand(proxyCmd)
}
