package proxy

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fairy-pitta/portree/internal/config"
	"github.com/fairy-pitta/portree/internal/state"
)

func setupProxyTest(t *testing.T) (*ProxyServer, *state.FileStore) {
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

	// Set up state with known assignments
	st := &state.State{
		Services:        map[string]map[string]*state.ServiceState{},
		PortAssignments: map[string]int{},
	}
	state.SetPortAssignment(st, "feature/auth", "web", 3150)
	if err := store.Save(st); err != nil {
		t.Fatal(err)
	}

	resolver := NewResolver(cfg, store)
	return NewProxyServer(resolver), store
}

func TestHandlerMissingSubdomain(t *testing.T) {
	proxy, _ := setupProxyTest(t)
	handler := proxy.handler(3000)

	req := httptest.NewRequest("GET", "http://localhost:3000/", nil)
	req.Host = "localhost:3000"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandlerUnknownSlug(t *testing.T) {
	proxy, _ := setupProxyTest(t)
	handler := proxy.handler(3000)

	req := httptest.NewRequest("GET", "http://unknown-branch.localhost:3000/", nil)
	req.Host = "unknown-branch.localhost:3000"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHandlerResolvesToBackend(t *testing.T) {
	dir := t.TempDir()
	store, err := state.NewFileStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Start a dummy backend server
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "hello from backend")
	}))
	defer backend.Close()

	// Parse the backend port
	var backendPort int
	fmt.Sscanf(backend.Listener.Addr().String(), "127.0.0.1:%d", &backendPort)

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

	// Set up state with the backend's actual port
	st := &state.State{
		Services:        map[string]map[string]*state.ServiceState{},
		PortAssignments: map[string]int{},
	}
	state.SetPortAssignment(st, "feature/auth", "web", backendPort)
	if err := store.Save(st); err != nil {
		t.Fatal(err)
	}

	resolver := NewResolver(cfg, store)
	proxy := NewProxyServer(resolver)
	handler := proxy.handler(3000)

	req := httptest.NewRequest("GET", "http://feature-auth.localhost:3000/", nil)
	req.Host = "feature-auth.localhost:3000"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if body := rec.Body.String(); body != "hello from backend" {
		t.Errorf("body = %q, want %q", body, "hello from backend")
	}
}
