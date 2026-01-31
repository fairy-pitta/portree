package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- Pure function tests ---

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if _, ok := cfg.Services["frontend"]; !ok {
		t.Fatal("DefaultConfig missing 'frontend' service")
	}

	fe := cfg.Services["frontend"]
	if fe.PortRange.Min != 3100 || fe.PortRange.Max != 3199 {
		t.Errorf("frontend port range = [%d, %d], want [3100, 3199]", fe.PortRange.Min, fe.PortRange.Max)
	}
	if fe.ProxyPort != 3000 {
		t.Errorf("frontend proxy_port = %d, want 3000", fe.ProxyPort)
	}
	if fe.Command != "npm run dev" {
		t.Errorf("frontend command = %q, want %q", fe.Command, "npm run dev")
	}
}

func TestValidate(t *testing.T) {
	validCfg := func() *Config {
		return &Config{
			Services: map[string]ServiceConfig{
				"web": {
					Command:   "npm start",
					PortRange: PortRange{Min: 3100, Max: 3199},
					ProxyPort: 3000,
				},
			},
			Env:       map[string]string{},
			Worktrees: map[string]WTOverride{},
		}
	}

	tests := []struct {
		name    string
		modify  func(c *Config)
		wantErr string
	}{
		{"valid config", func(c *Config) {}, ""},
		{"no services", func(c *Config) { c.Services = map[string]ServiceConfig{} }, "at least one service"},
		{"empty command", func(c *Config) {
			svc := c.Services["web"]
			svc.Command = ""
			c.Services["web"] = svc
		}, "command must not be empty"},
		{"bad port range min", func(c *Config) {
			svc := c.Services["web"]
			svc.PortRange = PortRange{Min: 0, Max: 3199}
			c.Services["web"] = svc
		}, "must be positive"},
		{"bad port range order", func(c *Config) {
			svc := c.Services["web"]
			svc.PortRange = PortRange{Min: 4000, Max: 3000}
			c.Services["web"] = svc
		}, "must be <="},
		{"zero proxy port", func(c *Config) {
			svc := c.Services["web"]
			svc.ProxyPort = 0
			c.Services["web"] = svc
		}, "proxy_port must be positive"},
		{"duplicate proxy port", func(c *Config) {
			c.Services["api"] = ServiceConfig{
				Command:   "go run .",
				PortRange: PortRange{Min: 8100, Max: 8199},
				ProxyPort: 3000, // same as web
			}
		}, "same proxy_port"},
		{"unknown worktree service", func(c *Config) {
			c.Worktrees["main"] = WTOverride{
				Services: map[string]WTServiceOverride{
					"unknown": {Port: 3100},
				},
			}
		}, "unknown service"},
		{"port outside range", func(c *Config) {
			c.Worktrees["main"] = WTOverride{
				Services: map[string]WTServiceOverride{
					"web": {Port: 9999},
				},
			}
		}, "outside range"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validCfg()
			tt.modify(cfg)
			err := cfg.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("Validate() unexpected error: %v", err)
				}
			} else {
				if err == nil {
					t.Fatalf("Validate() expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("Validate() error = %q, want substring %q", err.Error(), tt.wantErr)
				}
			}
		})
	}
}

func TestCommandForBranch(t *testing.T) {
	cfg := &Config{
		Services: map[string]ServiceConfig{
			"web": {Command: "npm start"},
		},
		Worktrees: map[string]WTOverride{
			"feature/auth": {
				Services: map[string]WTServiceOverride{
					"web": {Command: "npm run dev:auth"},
				},
			},
			"feature/empty": {
				Services: map[string]WTServiceOverride{
					"web": {Command: ""},
				},
			},
		},
	}

	t.Run("no override", func(t *testing.T) {
		got := cfg.CommandForBranch("web", "main")
		if got != "npm start" {
			t.Errorf("CommandForBranch() = %q, want %q", got, "npm start")
		}
	})

	t.Run("override exists", func(t *testing.T) {
		got := cfg.CommandForBranch("web", "feature/auth")
		if got != "npm run dev:auth" {
			t.Errorf("CommandForBranch() = %q, want %q", got, "npm run dev:auth")
		}
	})

	t.Run("override empty command falls back", func(t *testing.T) {
		got := cfg.CommandForBranch("web", "feature/empty")
		if got != "npm start" {
			t.Errorf("CommandForBranch() = %q, want %q", got, "npm start")
		}
	})
}

func TestEnvForBranch(t *testing.T) {
	cfg := &Config{
		Services: map[string]ServiceConfig{
			"web": {Command: "npm start"},
		},
		Env: map[string]string{
			"NODE_ENV": "development",
			"DEBUG":    "false",
		},
		Worktrees: map[string]WTOverride{
			"feature/auth": {
				Services: map[string]WTServiceOverride{
					"web": {Env: map[string]string{"DEBUG": "true", "AUTH": "1"}},
				},
			},
		},
	}

	t.Run("global only", func(t *testing.T) {
		env := cfg.EnvForBranch("web", "main")
		if env["NODE_ENV"] != "development" {
			t.Errorf("expected NODE_ENV=development, got %q", env["NODE_ENV"])
		}
		if env["DEBUG"] != "false" {
			t.Errorf("expected DEBUG=false, got %q", env["DEBUG"])
		}
	})

	t.Run("merge with override", func(t *testing.T) {
		env := cfg.EnvForBranch("web", "feature/auth")
		if env["NODE_ENV"] != "development" {
			t.Errorf("expected NODE_ENV=development, got %q", env["NODE_ENV"])
		}
		if env["DEBUG"] != "true" {
			t.Errorf("expected DEBUG=true (overridden), got %q", env["DEBUG"])
		}
		if env["AUTH"] != "1" {
			t.Errorf("expected AUTH=1, got %q", env["AUTH"])
		}
	})

	t.Run("returns copy", func(t *testing.T) {
		env := cfg.EnvForBranch("web", "main")
		env["NODE_ENV"] = "production"
		if cfg.Env["NODE_ENV"] != "development" {
			t.Error("EnvForBranch did not return a copy; original was mutated")
		}
	})

	t.Run("empty env", func(t *testing.T) {
		emptyCfg := &Config{
			Services:  map[string]ServiceConfig{"web": {Command: "npm start"}},
			Env:       map[string]string{},
			Worktrees: map[string]WTOverride{},
		}
		env := emptyCfg.EnvForBranch("web", "main")
		if len(env) != 0 {
			t.Errorf("expected empty env, got %v", env)
		}
	})
}

func TestFixedPortForBranch(t *testing.T) {
	cfg := &Config{
		Services: map[string]ServiceConfig{
			"web": {Command: "npm start"},
			"api": {Command: "go run ."},
		},
		Worktrees: map[string]WTOverride{
			"main": {
				Services: map[string]WTServiceOverride{
					"web": {Port: 3100},
				},
			},
		},
	}

	t.Run("no entry", func(t *testing.T) {
		got := cfg.FixedPortForBranch("web", "develop")
		if got != 0 {
			t.Errorf("FixedPortForBranch() = %d, want 0", got)
		}
	})

	t.Run("no service in worktree", func(t *testing.T) {
		got := cfg.FixedPortForBranch("api", "main")
		if got != 0 {
			t.Errorf("FixedPortForBranch() = %d, want 0", got)
		}
	})

	t.Run("fixed port set", func(t *testing.T) {
		got := cfg.FixedPortForBranch("web", "main")
		if got != 3100 {
			t.Errorf("FixedPortForBranch() = %d, want 3100", got)
		}
	})

	t.Run("zero port", func(t *testing.T) {
		cfg.Worktrees["develop"] = WTOverride{
			Services: map[string]WTServiceOverride{
				"web": {Port: 0},
			},
		}
		got := cfg.FixedPortForBranch("web", "develop")
		if got != 0 {
			t.Errorf("FixedPortForBranch() = %d, want 0", got)
		}
	})
}

// --- Filesystem tests ---

func TestInit(t *testing.T) {
	dir := t.TempDir()

	path, err := Init(dir)
	if err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	if filepath.Base(path) != FileName {
		t.Errorf("Init() returned path %q, want filename %q", path, FileName)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading init file: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "[services.frontend]") {
		t.Error("Init file missing [services.frontend]")
	}
	if !strings.Contains(content, "port_range") {
		t.Error("Init file missing port_range")
	}

	// Double init should error
	_, err = Init(dir)
	if err == nil {
		t.Fatal("Init() second call expected error, got nil")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("Init() second call error = %q, want 'already exists'", err.Error())
	}
}

func TestLoad(t *testing.T) {
	t.Run("valid TOML", func(t *testing.T) {
		dir := t.TempDir()
		tomlContent := `
[services.web]
command = "npm start"
port_range = { min = 3100, max = 3199 }
proxy_port = 3000
`
		if err := os.WriteFile(filepath.Join(dir, FileName), []byte(tomlContent), 0644); err != nil {
			t.Fatal(err)
		}
		cfg, err := Load(dir)
		if err != nil {
			t.Fatalf("Load() error: %v", err)
		}
		if cfg.Services["web"].Command != "npm start" {
			t.Errorf("loaded command = %q, want %q", cfg.Services["web"].Command, "npm start")
		}
	})

	t.Run("file not found", func(t *testing.T) {
		dir := t.TempDir()
		_, err := Load(dir)
		if err == nil {
			t.Fatal("Load() expected error for missing file")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("Load() error = %q, want 'not found'", err.Error())
		}
	})

	t.Run("invalid TOML", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, FileName), []byte("{{invalid"), 0644); err != nil {
			t.Fatal(err)
		}
		_, err := Load(dir)
		if err == nil {
			t.Fatal("Load() expected error for invalid TOML")
		}
		if !strings.Contains(err.Error(), "parsing") {
			t.Errorf("Load() error = %q, want 'parsing'", err.Error())
		}
	})
}

func TestInitThenLoad(t *testing.T) {
	dir := t.TempDir()

	_, err := Init(dir)
	if err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() after Init() error: %v", err)
	}

	if len(cfg.Services) == 0 {
		t.Error("loaded config has no services")
	}
}
