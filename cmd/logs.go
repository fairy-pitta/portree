package cmd

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/fairy-pitta/portree/internal/git"
	"github.com/fairy-pitta/portree/internal/logging"
	"github.com/fairy-pitta/portree/internal/process"
	"github.com/fairy-pitta/portree/internal/state"
	"github.com/spf13/cobra"
)

var (
	logsFollow  bool
	logsTail    int
	logsService string
)

var logsCmd = &cobra.Command{
	Use:   "logs [worktree]",
	Short: "Tail logs for a worktree's services",
	Long: `Show the captured logs for a worktree's services.

With no argument, logs for the current worktree are shown. Pass a branch name to
target a different worktree. By default every service's log is shown, each line
prefixed with the service name; use --service to show just one (unprefixed).

Use --follow/-f to keep streaming new output until Ctrl-C.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := exec.LookPath("tail"); err != nil {
			return fmt.Errorf("the 'tail' command is required but was not found in PATH")
		}

		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("getting current directory: %w", err)
		}

		tree, err := resolveLogsWorktree(cwd, args)
		if err != nil {
			return err
		}
		if tree.IsBare {
			return fmt.Errorf("cannot tail logs for a bare worktree")
		}

		if logsService != "" {
			if _, ok := cfg.Services[logsService]; !ok {
				return fmt.Errorf("unknown service %q", logsService)
			}
		}

		// Logs live under the shared state dir (main worktree), not the current
		// worktree, so a single 'portree logs <branch>' can reach every worktree.
		stateDir := filepath.Join(stateRoot, ".portree")
		store, err := state.NewFileStore(stateDir)
		if err != nil {
			return fmt.Errorf("creating state store: %w", err)
		}

		targets := logsTargets(tree, store, stateDir)
		if len(targets) == 0 {
			logging.Info("no logs yet for %s", tree.Branch)
			return nil
		}

		// A service that has not written anything yet leaves an empty log file,
		// and tailing it prints nothing at all — indistinguishable from the
		// command having failed.
		if !logsFollow && allLogsEmpty(targets) {
			logging.Info("no output yet for %s (log files exist but are empty)", tree.Branch)
			return nil
		}

		return streamLogs(targets)
	},
}

// allLogsEmpty reports whether every target log file is still empty.
func allLogsEmpty(targets []logTarget) bool {
	for _, t := range targets {
		if info, err := os.Stat(t.path); err == nil && info.Size() > 0 {
			return false
		}
	}
	return true
}

// logTarget is a single service's log file to stream.
type logTarget struct {
	service string
	path    string
	running bool
}

// resolveLogsWorktree returns the target worktree: the current one when no
// argument is given, otherwise the worktree whose branch matches args[0].
func resolveLogsWorktree(cwd string, args []string) (*git.Worktree, error) {
	if len(args) == 0 {
		tree, err := git.CurrentWorktree(cwd)
		if err != nil {
			return nil, fmt.Errorf("detecting worktree: %w", err)
		}
		return tree, nil
	}

	name := args[0]
	trees, err := git.ListWorktrees(cwd)
	if err != nil {
		return nil, fmt.Errorf("listing worktrees: %w", err)
	}

	var branches []string
	for i := range trees {
		t := trees[i]
		if t.IsBare {
			continue
		}
		if t.Branch == name {
			return &t, nil
		}
		branches = append(branches, t.Branch)
	}
	sort.Strings(branches)
	return nil, fmt.Errorf("worktree %q not found; available: %s", name, strings.Join(branches, ", "))
}

// logsTargets resolves which service log files to stream. Files that do not
// exist yet are included only in follow mode (tail -F waits for them).
func logsTargets(tree *git.Worktree, store *state.FileStore, stateDir string) []logTarget {
	var services []string
	if logsService != "" {
		services = []string{logsService}
	} else {
		for name := range cfg.Services {
			services = append(services, name)
		}
		sort.Strings(services)
	}

	var st *state.State
	if err := store.WithLock(func() error {
		var e error
		st, e = store.Load()
		return e
	}); err != nil {
		logging.Warn("failed to load state: %v", err)
	}

	slug := tree.Slug()
	var targets []logTarget
	for _, svc := range services {
		path := process.LogPath(stateDir, slug, svc)
		_, statErr := os.Stat(path)
		if statErr != nil && !logsFollow {
			continue // no log yet and not following: nothing to show
		}

		running := false
		if st != nil {
			if ss := state.GetServiceState(st, tree.Branch, svc); ss != nil {
				running = ss.PID > 0 && process.IsProcessRunning(ss.PID)
			}
		}
		targets = append(targets, logTarget{service: svc, path: path, running: running})
	}
	return targets
}

// streamLogs runs tail over the target log files, prefixing each line with the
// service name when more than one service is shown.
func streamLogs(targets []logTarget) error {
	for _, t := range targets {
		if !t.running {
			logging.Warn("%s is not running — showing existing logs", t.service)
		}
	}

	multi := len(targets) > 1
	width := 0
	if multi {
		for _, t := range targets {
			if len(t.service) > width {
				width = len(t.service)
			}
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Single service: stream raw to stdout, no prefix (clean for piping).
	if !multi {
		c := exec.CommandContext(ctx, "tail", tailArgs(targets[0].path)...)
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		return runTail(ctx, c)
	}

	// One-shot, multiple services: run sequentially for deterministic grouping.
	if !logsFollow {
		for _, t := range targets {
			if ctx.Err() != nil {
				break
			}
			c := exec.CommandContext(ctx, "tail", tailArgs(t.path)...)
			c.Stderr = os.Stderr
			pipe, err := c.StdoutPipe()
			if err != nil {
				return err
			}
			if err := c.Start(); err != nil {
				return err
			}
			scanPrefix(pipe, linePrefix(t.service, width), nil)
			_ = c.Wait()
		}
		return nil
	}

	// Follow, multiple services: concurrent tails, whole lines serialized
	// under a mutex so output from different services never interleaves mid-line.
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, t := range targets {
		c := exec.CommandContext(ctx, "tail", tailArgs(t.path)...)
		c.Stderr = nil // discard tail's "cannot open ... will retry" notice
		pipe, err := c.StdoutPipe()
		if err != nil {
			return err
		}
		if err := c.Start(); err != nil {
			return err
		}
		wg.Add(1)
		go func(p io.ReadCloser, prefix string, cmd *exec.Cmd) {
			defer wg.Done()
			scanPrefix(p, prefix, &mu)
			_ = cmd.Wait()
		}(pipe, linePrefix(t.service, width), c)
	}
	wg.Wait()
	return nil
}

// tailArgs builds the argument list for a single tail invocation.
func tailArgs(path string) []string {
	args := []string{"-n", strconv.Itoa(logsTail)}
	if logsFollow {
		args = append(args, "-F") // follow by name: survives rotation, waits for missing files
	}
	return append(args, path)
}

// runTail runs a tail command, treating termination caused by Ctrl-C as a clean exit.
func runTail(ctx context.Context, c *exec.Cmd) error {
	err := c.Run()
	if ctx.Err() != nil {
		return nil // interrupted by the user; tail was killed on purpose
	}
	return err
}

// linePrefix returns the left-aligned "service | " prefix for a log line.
func linePrefix(service string, width int) string {
	return fmt.Sprintf("%-*s | ", width, service)
}

// scanPrefix reads r line by line and writes each prefixed line to stdout.
// When mu is non-nil, each whole-line write is serialized to avoid interleaving.
func scanPrefix(r io.Reader, prefix string, mu *sync.Mutex) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := prefix + sc.Text() + "\n"
		if mu != nil {
			mu.Lock()
		}
		_, _ = io.WriteString(os.Stdout, line)
		if mu != nil {
			mu.Unlock()
		}
	}
}

func init() {
	logsCmd.Flags().BoolVarP(&logsFollow, "follow", "f", false, "Stream new log output until Ctrl-C")
	logsCmd.Flags().IntVarP(&logsTail, "tail", "n", 50, "Number of trailing lines to show")
	logsCmd.Flags().StringVarP(&logsService, "service", "s", "", "Show only a specific service (default: all)")
	rootCmd.AddCommand(logsCmd)
}
