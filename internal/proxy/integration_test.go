package proxy

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/fairy-pitta/portree/internal/cert"
	"github.com/fairy-pitta/portree/internal/config"
	"github.com/fairy-pitta/portree/internal/git"
	"github.com/fairy-pitta/portree/internal/state"
)

// worktreeBackend is a stand-in for one worktree's running dev server.
type worktreeBackend struct {
	branch string
	slug   string
}

// startWorktreeBackends launches one live backend per branch, each serving a
// body naming the branch it belongs to, and records its port in a fresh store.
// Several worktrees are always running at once, which is what makes it possible
// to catch the proxy routing a request to the wrong one. proxyPort must match
// the port the handler is built for, since the resolver keys off it to find
// the service.
func startWorktreeBackends(t *testing.T, proxyPort int, branches ...string) ([]worktreeBackend, *Resolver) {
	t.Helper()

	store, err := state.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	st := &state.State{
		Services:        map[string]map[string]*state.ServiceState{},
		PortAssignments: map[string]int{},
	}

	backends := make([]worktreeBackend, 0, len(branches))
	for _, branch := range branches {
		body := backendBody(branch)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = fmt.Fprint(w, body)
		}))
		t.Cleanup(srv.Close)

		state.SetPortAssignment(st, branch, "web", addrPort(t, srv.Listener.Addr().String()))
		backends = append(backends, worktreeBackend{branch: branch, slug: git.BranchSlug(branch)})
	}

	if err := store.Save(st); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Services: map[string]config.ServiceConfig{
			"web": {
				Command:   "npm start",
				PortRange: config.PortRange{Min: 3100, Max: 3199},
				ProxyPort: proxyPort,
			},
		},
		Env:       map[string]string{},
		Worktrees: map[string]config.WTOverride{},
	}

	return backends, NewResolver(cfg, store)
}

func backendBody(branch string) string { return "served by " + branch }

func addrPort(t *testing.T, addr string) int {
	t.Helper()
	_, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("splitting addr %q: %v", addr, err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("parsing port %q: %v", portStr, err)
	}
	return port
}

// freeLoopbackPort reserves and releases a loopback port so the proxy can bind
// it, avoiding the cross-test collisions a hardcoded port would risk.
func freeLoopbackPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserving a loopback port: %v", err)
	}
	port := addrPort(t, ln.Addr().String())
	if err := ln.Close(); err != nil {
		t.Fatalf("releasing reserved port: %v", err)
	}
	return port
}

// loopbackTransport keeps the "<slug>.localhost" host (and SNI) intact while
// always dialing the loopback proxy, so these tests do not depend on the
// machine resolving *.localhost through DNS. Pass roots to speak TLS.
func loopbackTransport(proxyPort int, roots *x509.CertPool) *http.Transport {
	dialProxy := func(ctx context.Context, network string) (net.Conn, error) {
		var d net.Dialer
		return d.DialContext(ctx, network, net.JoinHostPort("127.0.0.1", strconv.Itoa(proxyPort)))
	}

	if roots == nil {
		return &http.Transport{
			DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
				return dialProxy(ctx, network)
			},
		}
	}

	return &http.Transport{
		DialTLSContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			serverName, _, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			raw, err := dialProxy(ctx, network)
			if err != nil {
				return nil, err
			}
			conn := tls.Client(raw, &tls.Config{ServerName: serverName, RootCAs: roots})
			if err := conn.HandshakeContext(ctx); err != nil {
				_ = raw.Close()
				return nil, err
			}
			return conn, nil
		},
	}
}

// getBody issues a GET and returns the status and body.
func getBody(t *testing.T, client *http.Client, url string) (int, string) {
	t.Helper()
	resp, err := client.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body of %s: %v", url, err)
	}
	return resp.StatusCode, string(body)
}

// TestHandlerRoutesEachSlugToItsOwnBackend is the core promise of portree:
// with several worktrees up at once, each "<slug>.localhost" must reach that
// worktree's own dev server and no other.
func TestHandlerRoutesEachSlugToItsOwnBackend(t *testing.T) {
	backends, resolver := startWorktreeBackends(t, 3000, "feature/auth", "main", "bugfix/login-500")
	handler := NewProxyServer(resolver, nil).handler(3000)

	for _, b := range backends {
		t.Run(b.slug, func(t *testing.T) {
			host := b.slug + ".localhost:3000"
			req := httptest.NewRequest("GET", "http://"+host+"/", nil)
			req.Host = host
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
			}
			if got, want := rec.Body.String(), backendBody(b.branch); got != want {
				t.Errorf("%s served %q, want %q", host, got, want)
			}
		})
	}
}

// TestProxyServerRoutesOverRealListener exercises the same routing through a
// bound listener and a real HTTP client, covering Start() and the server
// plumbing that a direct handler call skips.
func TestProxyServerRoutesOverRealListener(t *testing.T) {
	proxyPort := freeLoopbackPort(t)
	backends, resolver := startWorktreeBackends(t, proxyPort, "feature/auth", "main")

	proxy := NewProxyServer(resolver, nil)
	if err := proxy.Start(map[string]int{"web": proxyPort}); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer func() { _ = proxy.Stop() }()

	if got := proxy.Scheme(); got != "http" {
		t.Errorf("Scheme() = %q, want %q", got, "http")
	}

	client := &http.Client{Transport: loopbackTransport(proxyPort, nil), Timeout: 10 * time.Second}

	for _, b := range backends {
		t.Run(b.slug, func(t *testing.T) {
			url := fmt.Sprintf("http://%s.localhost:%d/", b.slug, proxyPort)
			status, body := getBody(t, client, url)

			if status != http.StatusOK {
				t.Fatalf("status = %d, want %d", status, http.StatusOK)
			}
			if want := backendBody(b.branch); body != want {
				t.Errorf("%s served %q, want %q", url, body, want)
			}
		})
	}
}

// TestProxyServerRoutesOverTLSWithSNI covers the HTTPS path end to end: the
// per-SNI leaf certificate is minted for "<slug>.localhost", verified by a
// strict client against the dev CA, and the request still lands on that
// worktree's backend.
func TestProxyServerRoutesOverTLSWithSNI(t *testing.T) {
	proxyPort := freeLoopbackPort(t)
	backends, resolver := startWorktreeBackends(t, proxyPort, "feature/auth", "main")

	paths, err := cert.EnsureCerts(t.TempDir())
	if err != nil {
		t.Fatalf("EnsureCerts() error: %v", err)
	}
	getCertificate, err := cert.NewSNIGetCertificate(paths)
	if err != nil {
		t.Fatalf("NewSNIGetCertificate() error: %v", err)
	}

	caPEM, err := os.ReadFile(paths.CACert)
	if err != nil {
		t.Fatalf("reading dev CA: %v", err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(caPEM) {
		t.Fatal("dev CA was not accepted into the trust pool")
	}

	proxy := NewProxyServer(resolver, &tls.Config{GetCertificate: getCertificate})
	if err := proxy.Start(map[string]int{"web": proxyPort}); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer func() { _ = proxy.Stop() }()

	if got := proxy.Scheme(); got != "https" {
		t.Errorf("Scheme() = %q, want %q", got, "https")
	}

	client := &http.Client{Transport: loopbackTransport(proxyPort, roots), Timeout: 10 * time.Second}

	for _, b := range backends {
		t.Run(b.slug, func(t *testing.T) {
			url := fmt.Sprintf("https://%s.localhost:%d/", b.slug, proxyPort)
			status, body := getBody(t, client, url)

			if status != http.StatusOK {
				t.Fatalf("status = %d, want %d", status, http.StatusOK)
			}
			if want := backendBody(b.branch); body != want {
				t.Errorf("%s served %q, want %q", url, body, want)
			}
		})
	}
}
