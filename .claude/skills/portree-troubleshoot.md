# portree-troubleshoot

Diagnose and fix common portree issues.

## When to Use

- User reports services not starting
- User sees port conflicts
- User has proxy routing issues
- User asks about error messages

## Diagnostic Commands

```bash
# Full system check
portree doctor

# List all services and their status
portree ls

# Check specific service logs
# Logs are in: ~/.portree/logs/<branch>.<service>.log
```

## Common Issues and Solutions

### 1. "not a known worktree" error

**Cause:** Running portree from a directory that isn't a git worktree.

**Solution:**
```bash
# Check if you're in a worktree
git worktree list

# Navigate to a worktree directory
cd /path/to/worktree

# Or create one
portree add <branch>
```

### 2. Service starts but immediately stops

**Cause:** Usually the port is already in use by an orphan process.

**Solution:**
```bash
# Check what's using the port
lsof -i :<port>

# Kill orphan process
kill <pid>

# Or let portree handle it
portree restart
```

### 3. Port conflict between services

**Cause:** Multiple services trying to use the same port range.

**Solution:** Adjust port ranges in `.portree.toml`:
```toml
[services.frontend]
port_range = [3100, 3199]

[services.backend]
port_range = [8100, 8199]  # Different range
```

### 4. Proxy not working / subdomain not resolving

**Cause:** DNS or proxy configuration issue.

**Solutions:**

a) **Check proxy is running:**
   ```bash
   portree proxy start
   portree ls  # Should show "Proxy: running"
   ```

b) **Use .localhost domain** (works without /etc/hosts):
   ```
   http://feature-branch.localhost:3000
   ```

c) **For custom domains, add to /etc/hosts:**
   ```
   127.0.0.1 feature-branch.local
   ```

### 5. Environment variables not available

**Cause:** Service command not using the provided env vars.

**Solution:** Ensure your command uses `$PORT` or the configured `port_env`:
```toml
[services.api]
command = "go run . --port=$PORT"  # Explicit
port_env = "PORT"                   # Or via env var
```

### 6. Service can't connect to another service

**Cause:** Using wrong port or URL for cross-service communication.

**Solution:** Use portree's cross-service env vars:
```toml
[services.frontend]
command = "npm run dev"
env = { API_URL = "http://localhost:$PT_BACKEND_PORT" }

[services.backend]
command = "go run ."
```

Or with proxy:
```toml
[services.frontend]
env = { API_URL = "$PT_BACKEND_URL" }  # Uses proxy URL
```

### 7. "no available port in range" error

**Cause:** All ports in the configured range are in use.

**Solutions:**
```bash
# Check what's using ports
portree ls
lsof -i -P | grep LISTEN

# Expand the port range
[services.api]
port_range = [8100, 8299]  # Wider range

# Or stop unused worktrees
portree down --all
portree remove old-branch
```

### 8. State file corruption

**Cause:** Interrupted operation or manual editing.

**Solution:**
```bash
# Reset state (services will need to be restarted)
rm -rf ~/.portree/state.json

# Restart services
portree up --all
```

## Reading Logs

```bash
# Log location
ls ~/.portree/logs/

# View specific service log
cat ~/.portree/logs/<branch>.<service>.log

# Follow logs in real-time
tail -f ~/.portree/logs/<branch>.<service>.log

# Or use dashboard
portree dash
# Press 'l' to view logs
```
