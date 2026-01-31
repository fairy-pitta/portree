package proxy

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ProxyServer manages multiple HTTP reverse proxy listeners, one per proxy_port.
type ProxyServer struct {
	resolver *Resolver
	servers  []*http.Server
	mu       sync.Mutex
}

// NewProxyServer creates a new ProxyServer.
func NewProxyServer(resolver *Resolver) *ProxyServer {
	return &ProxyServer{resolver: resolver}
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
			Addr:    ":" + strconv.Itoa(port),
			Handler: p.handler(port),
		}

		ln, err := net.Listen("tcp", srv.Addr)
		if err != nil {
			// Clean up already started servers.
			_ = p.stopLocked()
			return fmt.Errorf("proxy: cannot listen on %s: %w", srv.Addr, err)
		}

		p.servers = append(p.servers, srv)
		go func() { _ = srv.Serve(ln) }()
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
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := srv.Shutdown(ctx); err != nil {
			lastErr = err
		}
		cancel()
	}
	p.servers = nil
	return lastErr
}

// handler returns an http.Handler for a specific proxy port.
func (p *ProxyServer) handler(proxyPort int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slug := ParseSlugFromHost(r.Host)
		if slug == "" {
			http.Error(w, "portree: missing subdomain in Host header.\n"+
				"Use http://<branch-slug>.localhost:"+strconv.Itoa(proxyPort), http.StatusBadRequest)
			return
		}

		backendPort, err := p.resolver.Resolve(slug, proxyPort)
		if err != nil {
			slugs, _ := p.resolver.AvailableSlugs()
			msg := fmt.Sprintf("portree: no worktree found for slug %q\nAvailable: %s",
				slug, strings.Join(slugs, ", "))
			http.Error(w, msg, http.StatusNotFound)
			return
		}

		target, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", backendPort))
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
