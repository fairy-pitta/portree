# AI Agent Instructions for Portree

This document provides context for AI coding assistants working with portree or projects that use portree.

## Overview

**Portree** is a Git Worktree Server Manager that solves the problem of running the same service on multiple branches simultaneously. It provides:

- **Automatic port allocation**: Each branch/service gets a unique port
- **Subdomain routing**: Access branches via `<branch>.localhost:<port>`
- **Process management**: Start, stop, restart services across worktrees
- **Interactive dashboard**: TUI for managing all services

## Quick Reference

### Installation
```bash
brew install fairy-pitta/tap/portree  # macOS
# Or download from https://github.com/fairy-pitta/portree/releases
```

### Configuration (.portree.toml)
```toml
[services.SERVICE_NAME]
command = "COMMAND"           # Required: command to run
port_env = "PORT"             # Env var for port (default: PORT)
port_range = [3100, 3199]     # Port range (optional)
env = { KEY = "value" }       # Additional env vars
```

### Essential Commands
```bash
portree add <branch>     # Create worktree
portree up [--all]       # Start services
portree down [--all]     # Stop services
portree ls               # List status
portree proxy start      # Enable subdomain routing
portree dash             # Interactive dashboard
portree doctor           # Diagnose issues
```

### Environment Variables (injected by portree)
| Variable | Example | Description |
|----------|---------|-------------|
| `PORT` | `3150` | Assigned port |
| `PT_BRANCH` | `feature/auth` | Branch name |
| `PT_BRANCH_SLUG` | `feature-auth` | URL-safe branch |
| `PT_SERVICE` | `frontend` | Service name |
| `PT_{SVC}_PORT` | `8150` | Other service's port |
| `PT_{SVC}_URL` | `http://feature-auth.localhost:8000` | Other service's URL |

---

## Detailed Guide

### Setting Up Portree in a Project

1. **Create `.portree.toml`** in the project root
2. **Define services** with their start commands
3. **Run `portree doctor`** to verify configuration

Example configurations:

**Single service (Node.js):**
```toml
[services.web]
command = "npm run dev"
port_env = "PORT"
```

**Multiple services:**
```toml
[services.frontend]
command = "npm run dev"
port_range = [3100, 3199]

[services.api]
command = "go run ./cmd/server"
port_range = [8100, 8199]
```

**With cross-service communication:**
```toml
[services.frontend]
command = "npm run dev"
env = { API_URL = "http://localhost:$PT_API_PORT" }

[services.api]
command = "python -m uvicorn main:app --port $PORT"
```

### Workflow: Feature Branch Development

```bash
# 1. Create worktree for feature branch
portree add feature/new-feature

# 2. Start all services (main + feature)
portree up --all

# 3. Enable subdomain routing
portree proxy start

# 4. Access both versions:
#    - main: http://main.localhost:3000
#    - feature: http://feature-new-feature.localhost:3000

# 5. When done, clean up
portree down --all
portree remove feature/new-feature
```

### Workflow: Interactive Management

```bash
portree dash
```

Dashboard keybindings:
- `j/k` or arrows: Navigate
- `s`: Start selected
- `x`: Stop selected
- `r`: Restart
- `o`: Open in browser
- `a`: Start all
- `X`: Stop all
- `l`: View logs
- `p`: Toggle proxy
- `q`: Quit

### Troubleshooting

| Issue | Cause | Solution |
|-------|-------|----------|
| "not a known worktree" | Not in a git worktree | `cd` to a worktree or `portree add <branch>` |
| Service stops immediately | Port in use | `lsof -i :<port>` then kill orphan process |
| Proxy not routing | Proxy not started | `portree proxy start` |
| Can't connect to other service | Wrong URL/port | Use `$PT_{SERVICE}_PORT` or `$PT_{SERVICE}_URL` |
| "no available port" | Port range exhausted | Expand `port_range` or stop unused worktrees |

### Log Files

Logs are stored in `~/.portree/logs/<branch>.<service>.log`

```bash
# View logs
cat ~/.portree/logs/main.frontend.log
tail -f ~/.portree/logs/feature-auth.api.log
```

---

## For AI Assistants

When helping users with portree:

1. **Check if portree is installed**: `portree version`
2. **Check configuration**: `portree doctor`
3. **Check current state**: `portree ls`
4. **Suggest `.portree.toml`** appropriate for their stack
5. **Use environment variables** for cross-service communication
6. **Recommend `portree dash`** for interactive management

Common user intents:
- "Run multiple branches" → `portree add` + `portree up --all`
- "Compare branches" → `portree proxy start` + subdomain access
- "Service won't start" → `portree doctor` + check logs
- "Set up portree" → Create appropriate `.portree.toml`
