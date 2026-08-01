package cmd

import (
	"os"
	"sync"
	"testing"
)

// spawnRecord captures calls the tests would otherwise make to the real
// detached-proxy spawner.
type spawnRecord struct {
	mu    sync.Mutex
	calls [][]string
}

func (s *spawnRecord) reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = nil
}

func (s *spawnRecord) record(args []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, args)
}

func (s *spawnRecord) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.calls)
}

var testSpawns = &spawnRecord{}

// TestMain replaces the detached-proxy spawner for the whole package. The real
// one re-executes os.Executable(), which under `go test` is the test binary —
// running it would re-enter the suite instead of starting a proxy.
func TestMain(m *testing.M) {
	proxySpawner = func(stateDir string, ports []int, extraArgs []string) error {
		testSpawns.record(extraArgs)
		return nil
	}
	os.Exit(m.Run())
}
