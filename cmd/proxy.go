package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"syscall"

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
	RunE: func(cmd *cobra.Command, args []string) error {
		stateDir := filepath.Join(repoRoot, ".portree")
		store, err := state.NewFileStore(stateDir)
		if err != nil {
			return err
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
		_ = store.WithLock(func() error {
			st, e := store.Load()
			if e != nil {
				return e
			}
			st.Proxy = state.ProxyState{
				PID:    os.Getpid(),
				Status: "running",
			}
			return store.Save(st)
		})

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
		_ = server.Stop()

		_ = store.WithLock(func() error {
			st, e := store.Load()
			if e != nil {
				return e
			}
			st.Proxy = state.ProxyState{Status: "stopped"}
			return store.Save(st)
		})

		fmt.Println("Proxy stopped.")
		return nil
	},
}

var proxyStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the reverse proxy",
	RunE: func(cmd *cobra.Command, args []string) error {
		stateDir := filepath.Join(repoRoot, ".portree")
		store, err := state.NewFileStore(stateDir)
		if err != nil {
			return err
		}

		var st *state.State
		_ = store.WithLock(func() error {
			st, err = store.Load()
			return err
		})

		if st.Proxy.PID > 0 && st.Proxy.Status == "running" {
			// Send SIGTERM to the proxy process.
			proc, err := os.FindProcess(st.Proxy.PID)
			if err == nil {
				_ = proc.Signal(syscall.SIGTERM)
			}

			_ = store.WithLock(func() error {
				st, e := store.Load()
				if e != nil {
					return e
				}
				st.Proxy = state.ProxyState{Status: "stopped"}
				return store.Save(st)
			})

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
