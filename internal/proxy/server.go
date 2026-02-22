package proxy

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fairy-pitta/portree/internal/logging"
)

const shutdownTimeout = 5 * time.Second

// ProxyServer manages multiple HTTP reverse proxy listeners, one per proxy_port.
type ProxyServer struct {
	resolver  *Resolver
	tlsConfig *tls.Config // nil = plain HTTP
	servers   []*http.Server
	listeners []net.Listener
	mu        sync.Mutex
}

// NewProxyServer creates a new ProxyServer.
// Pass a non-nil tlsConfig to enable HTTPS.
func NewProxyServer(resolver *Resolver, tlsConfig *tls.Config) *ProxyServer {
	return &ProxyServer{resolver: resolver, tlsConfig: tlsConfig}
}

// Scheme returns "https" if TLS is configured, otherwise "http".
func (p *ProxyServer) Scheme() string {
	if p.tlsConfig != nil {
		return "https"
	}
	return "http"
}

// Start launches proxy listeners for the given proxy ports.
func (p *ProxyServer) Start(proxyPorts map[string]int) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Collect unique proxy ports.
	ports := map[int]bool{}
	for _, port := range proxyPorts {
		ports[port] = true
	}

	for port := range ports {
		srv := &http.Server{
			Addr:              "127.0.0.1:" + strconv.Itoa(port),
			Handler:           recoveryMiddleware(p.handler(port)),
			ReadTimeout:       30 * time.Second,
			ReadHeaderTimeout: 10 * time.Second,
			IdleTimeout:       120 * time.Second,
			// WriteTimeout is intentionally 0 (unlimited): dev backends often use
			// SSE or chunked streaming (e.g. Vite/webpack HMR) which would be
			// terminated by a fixed write deadline.
		}

		ln, err := net.Listen("tcp", srv.Addr)
		if err != nil {
			// Clean up already started servers.
			_ = p.stopLocked()
			return fmt.Errorf("proxy: cannot listen on %s: %w", srv.Addr, err)
		}

		if p.tlsConfig != nil {
			ln = tls.NewListener(ln, p.tlsConfig)
		}

		p.servers = append(p.servers, srv)
		p.listeners = append(p.listeners, ln)
		// Goroutine-level recovery catches panics from Serve() itself (e.g. listener errors).
		// Per-request panics are caught by recoveryMiddleware wrapping the handler.
		go func(s *http.Server, l net.Listener) {
			defer func() {
				if r := recover(); r != nil {
					logging.Error("panic in proxy server goroutine: %v\n%s", r, debug.Stack())
				}
			}()
			if err := s.Serve(l); err != nil && !errors.Is(err, http.ErrServerClosed) {
				logging.Error("proxy server error on %s: %v", s.Addr, err)
			}
		}(srv, ln)
	}

	return nil
}

// Stop gracefully shuts down all proxy listeners.
func (p *ProxyServer) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.stopLocked()
}

func (p *ProxyServer) stopLocked() error {
	var lastErr error
	for _, srv := range p.servers {
		ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		if err := srv.Shutdown(ctx); err != nil {
			lastErr = err
		}
		cancel()
	}
	// Close listeners explicitly to avoid FD leaks (idempotent after Shutdown).
	for _, ln := range p.listeners {
		_ = ln.Close()
	}
	p.servers = nil
	p.listeners = nil
	return lastErr
}

// recoveryMiddleware catches panics in HTTP handlers and returns 500.
func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				logging.Error("panic in HTTP handler: %v\n%s", rec, debug.Stack())
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// handler returns an http.Handler for a specific proxy port.
func (p *ProxyServer) handler(proxyPort int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slug := ParseSlugFromHost(r.Host)
		if slug == "" {
			http.Error(w, "portree: missing subdomain in Host header.\n"+
				"Use "+p.Scheme()+"://<branch-slug>.localhost:"+strconv.Itoa(proxyPort), http.StatusBadRequest)
			return
		}

		backendPort, err := p.resolver.Resolve(slug, proxyPort)
		if err != nil {
			msg := fmt.Sprintf("portree: no worktree found for slug %q", slug)
			if slugs, err := p.resolver.AvailableSlugs(); err == nil && len(slugs) > 0 {
				msg += fmt.Sprintf("\nAvailable: %s", strings.Join(slugs, ", "))
			}
			http.Error(w, msg, http.StatusNotFound)
			return
		}

		target, err := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", backendPort))
		if err != nil {
			http.Error(w, "portree: invalid backend URL", http.StatusInternalServerError)
			return
		}
		proxy := &httputil.ReverseProxy{
			Rewrite: func(pr *httputil.ProxyRequest) {
				pr.SetURL(target)
				pr.Out.Host = r.Host
				pr.Out.Header.Set("X-Forwarded-Host", r.Host)
			},
		}
		proxy.ServeHTTP(w, r)
	})
}
