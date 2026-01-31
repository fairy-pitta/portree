package proxy

import (
	"fmt"
	"strings"

	"github.com/fairy-pitta/portree/internal/config"
	"github.com/fairy-pitta/portree/internal/git"
	"github.com/fairy-pitta/portree/internal/state"
)

// Resolver maps slug + proxy_port to real backend port.
type Resolver struct {
	cfg   *config.Config
	store *state.FileStore
}

// NewResolver creates a new Resolver.
func NewResolver(cfg *config.Config, store *state.FileStore) *Resolver {
	return &Resolver{cfg: cfg, store: store}
}

// Resolve returns the real backend port for a slug and proxy port.
func (r *Resolver) Resolve(slug string, proxyPort int) (int, error) {
	// Find which service uses this proxy port.
	serviceName := ""
	for name, svc := range r.cfg.Services {
		if svc.ProxyPort == proxyPort {
			serviceName = name
			break
		}
	}
	if serviceName == "" {
		return 0, fmt.Errorf("no service configured for proxy_port %d", proxyPort)
	}

	// Find the branch that matches this slug.
	branch, err := r.slugToBranch(slug)
	if err != nil {
		return 0, err
	}

	// Look up the real port from state.
	var port int
	if err := r.store.WithLock(func() error {
		st, e := r.store.Load()
		if e != nil {
			return e
		}
		port = state.GetPortAssignment(st, branch, serviceName)
		return nil
	}); err != nil {
		return 0, fmt.Errorf("loading state: %w", err)
	}

	if port == 0 {
		return 0, fmt.Errorf("no port assigned for %s/%s (slug: %s)", branch, serviceName, slug)
	}
	return port, nil
}

// AvailableSlugs returns all known branch slugs.
func (r *Resolver) AvailableSlugs() ([]string, error) {
	var slugs []string

	if err := r.store.WithLock(func() error {
		st, e := r.store.Load()
		if e != nil {
			return e
		}
		seen := map[string]bool{}
		for key := range st.PortAssignments {
			parts := strings.SplitN(key, ":", 2)
			if len(parts) == 2 {
				slug := git.BranchSlug(parts[0])
				if !seen[slug] {
					seen[slug] = true
					slugs = append(slugs, slug)
				}
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return slugs, nil
}

// slugToBranch converts a URL slug back to the original branch name
// by checking state for known branches.
func (r *Resolver) slugToBranch(slug string) (string, error) {
	var branch string

	if err := r.store.WithLock(func() error {
		st, e := r.store.Load()
		if e != nil {
			return e
		}
		for key := range st.PortAssignments {
			parts := strings.SplitN(key, ":", 2)
			if len(parts) == 2 {
				candidate := parts[0]
				if git.BranchSlug(candidate) == slug {
					branch = candidate
					return nil
				}
			}
		}
		return nil
	}); err != nil {
		return "", err
	}

	if branch == "" {
		return "", fmt.Errorf("no worktree found for slug %q", slug)
	}
	return branch, nil
}

// ParseSlugFromHost extracts the slug from a Host header value.
// "feature-auth.localhost:3000" -> "feature-auth"
// "localhost:3000" -> ""
func ParseSlugFromHost(host string) string {
	// Remove port.
	h := host
	if idx := strings.LastIndex(h, ":"); idx != -1 {
		h = h[:idx]
	}

	// Check for .localhost suffix.
	if !strings.HasSuffix(h, ".localhost") {
		return ""
	}

	slug := strings.TrimSuffix(h, ".localhost")
	if slug == "" {
		return ""
	}
	return slug
}
