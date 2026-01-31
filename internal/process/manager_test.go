package process

import (
	"testing"

	"github.com/fairy-pitta/portree/internal/config"
	"github.com/fairy-pitta/portree/internal/state"
)

func TestTargetServices(t *testing.T) {
	cfg := &config.Config{
		Services: map[string]config.ServiceConfig{
			"web":    {Command: "npm start"},
			"api":    {Command: "go run ."},
			"worker": {Command: "python worker.py"},
		},
	}
	store, _ := state.NewFileStore(t.TempDir())
	m := NewManager(cfg, store, nil)

	t.Run("no filter returns sorted", func(t *testing.T) {
		got := m.targetServices("")
		if len(got) != 3 {
			t.Fatalf("targetServices() returned %d, want 3", len(got))
		}
		if got[0] != "api" || got[1] != "web" || got[2] != "worker" {
			t.Errorf("targetServices() = %v, want [api, web, worker]", got)
		}
	})

	t.Run("filter exists", func(t *testing.T) {
		got := m.targetServices("web")
		if len(got) != 1 || got[0] != "web" {
			t.Errorf("targetServices(web) = %v, want [web]", got)
		}
	})

	t.Run("filter not found", func(t *testing.T) {
		got := m.targetServices("nonexistent")
		if got != nil {
			t.Errorf("targetServices(nonexistent) = %v, want nil", got)
		}
	})
}
