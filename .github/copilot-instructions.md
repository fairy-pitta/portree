# GitHub Copilot Instructions for Portree

## About Portree

Portree is a Git Worktree Server Manager that enables running the same service on multiple branches simultaneously with automatic port allocation and subdomain-based reverse proxy routing.

**Key Features:**
- Automatic port allocation per branch/service
- Subdomain routing (e.g., `feature-auth.localhost:3000`)
- Process lifecycle management across worktrees
- Interactive TUI dashboard

## Configuration File

The configuration file is `.portree.toml` in the project root:

```toml
[services.frontend]
command = "npm run dev"
port_env = "PORT"
port_range = [3100, 3199]

[services.backend]
command = "go run ./cmd/server"
port_env = "PORT"
port_range = [8100, 8199]
env = { DATABASE_URL = "postgres://localhost/mydb" }
```

### Configuration Options

| Option | Description | Default |
|--------|-------------|---------|
| `command` | Command to start the service | Required |
| `port_env` | Environment variable name for port | `PORT` |
| `port_range` | `[min, max]` port range | `[3100, 3199]` |
| `env` | Additional environment variables | `{}` |

## Environment Variables

Portree injects these environment variables into each service:

| Variable | Description | Example |
|----------|-------------|---------|
| `PORT` | Assigned port number | `3150` |
| `PT_BRANCH` | Current branch name | `feature/auth` |
| `PT_BRANCH_SLUG` | URL-safe branch name | `feature-auth` |
| `PT_SERVICE` | Service name | `frontend` |
| `PT_{SERVICE}_PORT` | Another service's port | `8150` |
| `PT_{SERVICE}_URL` | Another service's proxy URL | `http://feature-auth.localhost:8000` |

## CLI Commands

### Worktree Management
```bash
portree add <branch>        # Create worktree for existing branch
portree add -b <branch>     # Create new branch and worktree
portree remove <branch>     # Remove worktree
portree ls                  # List all worktrees and services
```

### Service Control
```bash
portree up                  # Start services for current worktree
portree up --all            # Start all services across all worktrees
portree down                # Stop services for current worktree
portree down --all          # Stop all services
portree restart             # Restart services
```

### Proxy & Browser
```bash
portree proxy start         # Start reverse proxy
portree proxy stop          # Stop reverse proxy
portree open [service]      # Open service in browser
portree dash                # Launch interactive dashboard
```

### Diagnostics
```bash
portree doctor              # Check configuration and system status
portree version             # Show version information
```

## Common Patterns

### Setting up a new project
```toml
# .portree.toml
[services.web]
command = "npm run dev"
port_env = "PORT"
```

### Multi-service with cross-service communication
```toml
[services.frontend]
command = "npm run dev"
port_range = [3100, 3199]
env = { API_URL = "http://localhost:$PT_BACKEND_PORT" }

[services.backend]
command = "go run ."
port_range = [8100, 8199]
```

### Using proxy URLs for production-like routing
```toml
[services.frontend]
command = "npm run dev"
env = { API_BASE = "$PT_BACKEND_URL" }

[services.backend]
command = "python manage.py runserver 0.0.0.0:$PORT"
```

## Troubleshooting Tips

1. **Service won't start**: Check `portree doctor` and ensure command is correct
2. **Port conflict**: Another process may be using the port; check with `lsof -i :<port>`
3. **Proxy not routing**: Ensure `portree proxy start` is running; use `.localhost` domain
4. **Cross-service connection fails**: Use `$PT_{SERVICE}_PORT` or `$PT_{SERVICE}_URL` env vars

## Installation

```bash
# macOS (Homebrew)
brew install fairy-pitta/tap/portree

# Manual (download from GitHub releases)
# https://github.com/fairy-pitta/portree/releases
```
