package process

import (
	"strings"
	"testing"
)

func TestBuildEnv(t *testing.T) {
	runner := &Runner{
		config: RunnerConfig{
			ServiceName: "web",
			Branch:      "feature/auth",
			BranchSlug:  "feature-auth",
			Command:     "npm start",
			Dir:         "/tmp/project",
			Port:        3150,
			Env: map[string]string{
				"NODE_ENV": "development",
				"DEBUG":    "true",
			},
			AllServicePorts: map[string]int{
				"web": 3150,
				"api": 8150,
			},
			AllServiceProxyPorts: map[string]int{
				"web": 3000,
				"api": 8000,
			},
		},
	}

	env := runner.buildEnv()

	lookup := make(map[string]string)
	for _, e := range env {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			lookup[parts[0]] = parts[1]
		}
	}

	t.Run("PORT", func(t *testing.T) {
		if lookup["PORT"] != "3150" {
			t.Errorf("PORT = %q, want %q", lookup["PORT"], "3150")
		}
	})

	t.Run("PT_BRANCH", func(t *testing.T) {
		if lookup["PT_BRANCH"] != "feature/auth" {
			t.Errorf("PT_BRANCH = %q, want %q", lookup["PT_BRANCH"], "feature/auth")
		}
	})

	t.Run("PT_BRANCH_SLUG", func(t *testing.T) {
		if lookup["PT_BRANCH_SLUG"] != "feature-auth" {
			t.Errorf("PT_BRANCH_SLUG = %q, want %q", lookup["PT_BRANCH_SLUG"], "feature-auth")
		}
	})

	t.Run("PT_SERVICE", func(t *testing.T) {
		if lookup["PT_SERVICE"] != "web" {
			t.Errorf("PT_SERVICE = %q, want %q", lookup["PT_SERVICE"], "web")
		}
	})

	t.Run("custom env", func(t *testing.T) {
		if lookup["NODE_ENV"] != "development" {
			t.Errorf("NODE_ENV = %q, want %q", lookup["NODE_ENV"], "development")
		}
		if lookup["DEBUG"] != "true" {
			t.Errorf("DEBUG = %q, want %q", lookup["DEBUG"], "true")
		}
	})

	t.Run("cross-service ports", func(t *testing.T) {
		if lookup["PT_WEB_PORT"] != "3150" {
			t.Errorf("PT_WEB_PORT = %q, want %q", lookup["PT_WEB_PORT"], "3150")
		}
		if lookup["PT_API_PORT"] != "8150" {
			t.Errorf("PT_API_PORT = %q, want %q", lookup["PT_API_PORT"], "8150")
		}
	})

	t.Run("cross-service URLs", func(t *testing.T) {
		if lookup["PT_WEB_URL"] != "http://feature-auth.localhost:3000" {
			t.Errorf("PT_WEB_URL = %q, want %q", lookup["PT_WEB_URL"], "http://feature-auth.localhost:3000")
		}
		if lookup["PT_API_URL"] != "http://feature-auth.localhost:8000" {
			t.Errorf("PT_API_URL = %q, want %q", lookup["PT_API_URL"], "http://feature-auth.localhost:8000")
		}
	})
}
