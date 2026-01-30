package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

const FileName = ".gws.toml"

// Config represents the .gws.toml configuration file.
type Config struct {
	Services  map[string]ServiceConfig `toml:"services"`
	Env       map[string]string        `toml:"env"`
	Worktrees map[string]WTOverride    `toml:"worktrees"`
}

// ServiceConfig defines a single service within a worktree.
type ServiceConfig struct {
	Command   string    `toml:"command"`
	Dir       string    `toml:"dir"`
	PortRange PortRange `toml:"port_range"`
	ProxyPort int       `toml:"proxy_port"`
}

// PortRange defines the range of ports available for allocation.
type PortRange struct {
	Min int `toml:"min"`
	Max int `toml:"max"`
}

// WTOverride defines per-worktree overrides.
type WTOverride struct {
	Services map[string]WTServiceOverride `toml:"services"`
}

// WTServiceOverride defines per-worktree per-service overrides.
type WTServiceOverride struct {
	Command string            `toml:"command,omitempty"`
	Port    int               `toml:"port,omitempty"`
	Env     map[string]string `toml:"env,omitempty"`
}

// DefaultConfig returns a default configuration with a single frontend service.
func DefaultConfig() *Config {
	return &Config{
		Services: map[string]ServiceConfig{
			"frontend": {
				Command: "npm run dev",
				Dir:     "",
				PortRange: PortRange{
					Min: 3100,
					Max: 3199,
				},
				ProxyPort: 3000,
			},
		},
		Env:       map[string]string{},
		Worktrees: map[string]WTOverride{},
	}
}

// Load reads and parses the config file from the given repo root.
func Load(repoRoot string) (*Config, error) {
	path := filepath.Join(repoRoot, FileName)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%s not found in %s; run 'gws init' first", FileName, repoRoot)
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", FileName, err)
	}

	if cfg.Services == nil {
		cfg.Services = map[string]ServiceConfig{}
	}
	if cfg.Env == nil {
		cfg.Env = map[string]string{}
	}
	if cfg.Worktrees == nil {
		cfg.Worktrees = map[string]WTOverride{}
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	return &cfg, nil
}

// Validate checks the configuration for errors.
func (c *Config) Validate() error {
	if len(c.Services) == 0 {
		return fmt.Errorf("at least one service must be defined in [services]")
	}

	proxyPorts := make(map[int]string)
	for name, svc := range c.Services {
		if svc.Command == "" {
			return fmt.Errorf("service %q: command must not be empty", name)
		}
		if svc.PortRange.Min <= 0 || svc.PortRange.Max <= 0 {
			return fmt.Errorf("service %q: port_range.min and port_range.max must be positive", name)
		}
		if svc.PortRange.Min > svc.PortRange.Max {
			return fmt.Errorf("service %q: port_range.min (%d) must be <= port_range.max (%d)",
				name, svc.PortRange.Min, svc.PortRange.Max)
		}
		if svc.ProxyPort <= 0 {
			return fmt.Errorf("service %q: proxy_port must be positive", name)
		}
		if existing, ok := proxyPorts[svc.ProxyPort]; ok {
			return fmt.Errorf("services %q and %q have the same proxy_port %d", existing, name, svc.ProxyPort)
		}
		proxyPorts[svc.ProxyPort] = name
	}

	// Validate per-worktree port overrides are within range
	for wtName, wt := range c.Worktrees {
		for svcName, svcOverride := range wt.Services {
			svc, ok := c.Services[svcName]
			if !ok {
				return fmt.Errorf("worktree %q references unknown service %q", wtName, svcName)
			}
			if svcOverride.Port != 0 && (svcOverride.Port < svc.PortRange.Min || svcOverride.Port > svc.PortRange.Max) {
				return fmt.Errorf("worktree %q service %q port %d is outside range [%d, %d]",
					wtName, svcName, svcOverride.Port, svc.PortRange.Min, svc.PortRange.Max)
			}
		}
	}
	return nil
}

// Init creates a default .gws.toml file in the given directory.
func Init(dir string) (string, error) {
	path := filepath.Join(dir, FileName)
	if _, err := os.Stat(path); err == nil {
		return path, fmt.Errorf("%s already exists", FileName)
	}

	content := `# gws - Git Worktree Server Manager configuration
# See: https://github.com/shuna/gws

# --- Service definitions ---
# Define services to run per worktree.
# Each service has its own command, directory, port range, and proxy port.

[services.frontend]
command = "pnpm run dev"
dir = "frontend"                        # relative to worktree root (empty = root)
port_range = { min = 3100, max = 3199 } # port allocation range for this service
proxy_port = 3000                        # proxy listens on this port

[services.backend]
command = "source .venv/bin/activate && python manage.py runserver 0.0.0.0:$PORT"
dir = "backend"
port_range = { min = 8100, max = 8199 }
proxy_port = 8000

# --- Global environment variables ---
[env]
# NODE_ENV = "development"

# --- Per-worktree overrides (optional) ---
# [worktrees.main]
# services.frontend.port = 3100       # fixed port
#
# [worktrees."feature/auth"]
# services.backend.command = "source .venv/bin/activate && python manage.py runserver --settings=myapp.settings_auth 0.0.0.0:$PORT"
# services.backend.env = { DEBUG = "1" }
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("writing %s: %w", FileName, err)
	}
	return path, nil
}

// CommandForBranch returns the command for a given service and branch,
// checking for per-worktree overrides.
func (c *Config) CommandForBranch(service, branch string) string {
	if wt, ok := c.Worktrees[branch]; ok {
		if svc, ok := wt.Services[service]; ok && svc.Command != "" {
			return svc.Command
		}
	}
	return c.Services[service].Command
}

// EnvForBranch returns merged environment variables for a given service and branch.
// Priority: worktree service env > global env
func (c *Config) EnvForBranch(service, branch string) map[string]string {
	merged := make(map[string]string, len(c.Env))
	for k, v := range c.Env {
		merged[k] = v
	}
	if wt, ok := c.Worktrees[branch]; ok {
		if svc, ok := wt.Services[service]; ok {
			for k, v := range svc.Env {
				merged[k] = v
			}
		}
	}
	return merged
}

// FixedPortForBranch returns the fixed port for a branch+service, or 0 if none.
func (c *Config) FixedPortForBranch(service, branch string) int {
	if wt, ok := c.Worktrees[branch]; ok {
		if svc, ok := wt.Services[service]; ok {
			return svc.Port
		}
	}
	return 0
}
