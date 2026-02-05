# portree-dev

Development workflow with portree for multi-branch development.

## When to Use

- User wants to work on a feature branch while keeping main running
- User asks how to start/stop services
- User wants to compare branches side-by-side
- User needs to manage multiple worktrees

## Commands Reference

### Worktree Management

```bash
# Create worktree for a branch
portree add <branch>

# Create worktree for new branch (from current HEAD)
portree add -b <new-branch>

# List all worktrees and their services
portree ls

# Remove a worktree
portree remove <branch>
```

### Service Management

```bash
# Start services for current worktree
portree up

# Start all services across all worktrees
portree up --all

# Stop services for current worktree
portree down

# Stop all services
portree down --all

# Restart services
portree restart
```

### Proxy & Dashboard

```bash
# Start reverse proxy (enables subdomain routing)
portree proxy start

# Stop proxy
portree proxy stop

# Open interactive dashboard
portree dash

# Dashboard keybindings:
#   j/k or ↑/↓  Navigate services
#   s           Start selected service
#   x           Stop selected service
#   r           Restart selected service
#   o           Open in browser
#   a           Start all services
#   X           Stop all services
#   l           View logs
#   p           Toggle proxy
#   q           Quit dashboard
```

### Browser Access

```bash
# Open service in browser
portree open [service]

# With proxy running, access via subdomain:
# http://<branch-slug>.localhost:<proxy-port>
# Example: http://feature-auth.localhost:3000
```

## Common Workflows

### 1. Start working on a feature branch

```bash
# Create worktree and start services
portree add feature/new-feature
cd ../feature-new-feature  # or use the path shown
portree up

# Or stay in main and start everything
portree add feature/new-feature
portree up --all
```

### 2. Compare two branches side-by-side

```bash
# Start proxy for subdomain routing
portree proxy start

# Start both branches
portree up --all

# Access:
# - main: http://main.localhost:3000
# - feature: http://feature-new-feature.localhost:3000
```

### 3. Quick branch switching

```bash
# Use dashboard for visual management
portree dash

# Or use CLI
portree ls                    # See all branches/services
portree down                  # Stop current
cd ../other-branch
portree up                    # Start other
```

### 4. Clean up after PR merge

```bash
portree down --all            # Stop everything
portree remove feature/done   # Remove worktree
```

## Troubleshooting

### Service won't start
```bash
portree doctor                # Check configuration
portree ls                    # Check port conflicts
```

### Port already in use
```bash
# Check what's using the port
lsof -i :<port>

# portree tracks orphan processes - restart should work
portree restart
```

### Proxy not routing correctly
```bash
# Ensure proxy is running
portree proxy start

# Check /etc/hosts has localhost entries (macOS)
# Or use *.localhost which resolves automatically
```
