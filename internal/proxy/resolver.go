package proxy

import (
	"fmt"
	"sort"
	"strings"

	"github.com/fairy-pitta/portree/internal/config"
	"github.com/fairy-pitta/portree/internal/git"
	"github.com/fairy-pitta/portree/internal/state"
)

// AmbiguousSlugError reports that a slug maps to more than one branch, so the
// request cannot be routed. Resolving to either candidate would silently serve
// a different worktree than the one the URL names, which is worse than failing.
type AmbiguousSlugError struct {
	Slug     string
	Branches []string
}

func (e *AmbiguousSlugError) Error() string {
	return fmt.Sprintf("slug %q is ambiguous between branches %s", e.Slug, strings.Join(e.Branches, ", "))
}

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

	// Resolve branch and port in a single lock to avoid inconsistency.
	var branch string
	var port int
	if err := r.store.WithLock(func() error {
		st, e := r.store.Load()
		if e != nil {
			return e
		}
		// Collect every branch matching this slug. Map iteration order is
		// random, so stopping at the first match would route a colliding slug
		// to a different worktree from one request to the next.
		var matches []string
		seen := map[string]bool{}
		for key := range st.PortAssignments {
			b, _ := state.ParsePortKey(key)
			if b == "" || seen[b] || git.BranchSlug(b) != slug {
				continue
			}
			seen[b] = true
			matches = append(matches, b)
		}
		if len(matches) == 0 {
			return fmt.Errorf("no worktree found for slug %q", slug)
		}
		if len(matches) > 1 {
			sort.Strings(matches)
			return &AmbiguousSlugError{Slug: slug, Branches: matches}
		}
		branch = matches[0]
		port = state.GetPortAssignment(st, branch, serviceName)
		return nil
	}); err != nil {
		return 0, err
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
