package proxy

import (
	"sort"
	"testing"

	"github.com/fairy-pitta/portree/internal/config"
	"github.com/fairy-pitta/portree/internal/state"
)

// --- Pure function tests ---

func TestParseSlugFromHost(t *testing.T) {
	tests := []struct {
		name string
		host string
		want string
	}{
		{"subdomain with port", "feature-auth.localhost:3000", "feature-auth"},
		{"subdomain without port", "feature-auth.localhost", "feature-auth"},
		{"no subdomain with port", "localhost:3000", ""},
		{"no subdomain without port", "localhost", ""},
		{"empty", "", ""},
		{"IP address", "127.0.0.1:3000", ""},
		{"non-localhost", "feature-auth.example.com:3000", ""},
		{"just .localhost", ".localhost:3000", ""},
		{"deep subdomain", "my-feature.localhost:8080", "my-feature"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseSlugFromHost(tt.host)
			if got != tt.want {
				t.Errorf("ParseSlugFromHost(%q) = %q, want %q", tt.host, got, tt.want)
			}
		})
	}
}

// --- State-backed tests ---

func setupResolver(t *testing.T) (*Resolver, *state.FileStore) {
	t.Helper()
	dir := t.TempDir()
	store, err := state.NewFileStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Services: map[string]config.ServiceConfig{
			"web": {
				Command:   "npm start",
				PortRange: config.PortRange{Min: 3100, Max: 3199},
				ProxyPort: 3000,
			},
		},
		Env:       map[string]string{},
		Worktrees: map[string]config.WTOverride{},
	}

	// Set up state with a known port assignment
	st := &state.State{
		Services:        map[string]map[string]*state.ServiceState{},
		PortAssignments: map[string]int{},
	}
	state.SetPortAssignment(st, "feature/auth", "web", 3150)
	if err := store.Save(st); err != nil {
		t.Fatal(err)
	}

	return NewResolver(cfg, store), store
}

func TestResolverResolve(t *testing.T) {
	resolver, _ := setupResolver(t)

	port, err := resolver.Resolve("feature-auth", 3000)
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	if port != 3150 {
		t.Errorf("Resolve() = %d, want 3150", port)
	}
}

func TestResolverResolveUnknownSlug(t *testing.T) {
	resolver, _ := setupResolver(t)

	_, err := resolver.Resolve("unknown-slug", 3000)
	if err == nil {
		t.Fatal("Resolve() expected error for unknown slug")
	}
}

func TestResolverAvailableSlugs(t *testing.T) {
	resolver, store := setupResolver(t)

	// Add another branch
	_ = store.WithLock(func() error {
		st, _ := store.Load()
		state.SetPortAssignment(st, "main", "web", 3100)
		return store.Save(st)
	})

	slugs, err := resolver.AvailableSlugs()
	if err != nil {
		t.Fatalf("AvailableSlugs() error: %v", err)
	}

	sort.Strings(slugs)
	if len(slugs) != 2 {
		t.Fatalf("AvailableSlugs() returned %d slugs, want 2", len(slugs))
	}
	if slugs[0] != "feature-auth" || slugs[1] != "main" {
		t.Errorf("AvailableSlugs() = %v, want [feature-auth, main]", slugs)
	}
}
