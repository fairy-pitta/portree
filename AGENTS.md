# AI Agent Instructions for Portree

## IMPORTANT: Auto-Execution Behavior

This project uses **portree** for multi-branch development. AI agents should **automatically** manage services without waiting for explicit commands.

### On Session Start

**AUTOMATICALLY execute when beginning work:**

```bash
# Check if portree project
if [ -f .portree.toml ]; then
  # Start services
  portree up

  # Enable subdomain routing
  portree proxy start

  # Open browser
  portree open
fi
```

Then report to user:
```
Started portree services:
- frontend: http://main.localhost:3000
- backend: http://main.localhost:8000
Browser opened.
```

### On Branch Switch

**AUTOMATICALLY execute when user switches to or mentions a branch:**

```bash
# Create worktree if needed
portree add <branch> 2>/dev/null || true

# Navigate to worktree (if not already there)
cd "$(git worktree list | grep '<branch>' | head -1 | awk '{print $1}')" 2>/dev/null || true

# Start services
portree up

# Open browser
portree open
```

### On "Compare Branches" Request

**AUTOMATICALLY execute:**

```bash
portree up --all
portree proxy start
```

Report URLs:
```
Branches running:
- main: http://main.localhost:3000
- feature-x: http://feature-x.localhost:3000
```

### On Session End / Cleanup

**When user says "done", "finished", "終わり", etc.:**

```bash
portree down --all
```

---

## Quick Reference

### Commands
```bash
portree up              # Start current branch
portree up --all        # Start all branches
portree down            # Stop current
portree down --all      # Stop all
portree proxy start     # Enable subdomain routing
portree open [service]  # Open browser
portree ls              # Show status
portree dash            # Interactive dashboard
portree add <branch>    # Create worktree
portree remove <branch> # Remove worktree
```

### URL Pattern
```
http://<branch-slug>.localhost:<proxy-port>
```
Examples:
- `http://main.localhost:3000`
- `http://feature-auth.localhost:3000`

### Environment Variables (injected into services)
| Variable | Description |
|----------|-------------|
| `PORT` | Assigned port |
| `PT_BRANCH` | Branch name |
| `PT_BRANCH_SLUG` | URL-safe branch |
| `PT_{SERVICE}_PORT` | Other service's port |
| `PT_{SERVICE}_URL` | Other service's proxy URL |

### Configuration (.portree.toml)
```toml
[services.frontend]
command = "npm run dev"
port_env = "PORT"
port_range = [3100, 3199]

[services.backend]
command = "go run ."
port_range = [8100, 8199]
```

---

## Troubleshooting

| Issue | Solution |
|-------|----------|
| Service won't start | `portree doctor` |
| Port in use | `lsof -i :<port>` then kill process |
| Proxy not working | `portree proxy start` |
| Logs | `~/.portree/logs/<branch>.<service>.log` |

---

## Installation

```bash
# macOS
brew install fairy-pitta/tap/portree

# Other
# Download from https://github.com/fairy-pitta/portree/releases
```
