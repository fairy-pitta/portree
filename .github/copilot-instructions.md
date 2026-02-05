# GitHub Copilot Instructions for Portree

## AUTO-EXECUTION REQUIRED

This project uses **portree** for multi-branch development. Copilot should **automatically** start services when working on this project.

### When Starting Work

**Execute immediately:**
```bash
portree up && portree proxy start && portree open
```

Report the running services and URLs to the user.

### When Switching Branches

**Execute automatically:**
```bash
portree add <branch> 2>/dev/null || true
portree up
portree open
```

### When Comparing Branches

**Execute:**
```bash
portree up --all
portree proxy start
```

Report: `http://main.localhost:3000` vs `http://feature-x.localhost:3000`

### When Done

**Execute:**
```bash
portree down --all
```

---

## Quick Reference

### Essential Commands
| Command | Action |
|---------|--------|
| `portree up` | Start current branch |
| `portree up --all` | Start all branches |
| `portree down` | Stop current |
| `portree proxy start` | Enable subdomain routing |
| `portree open` | Open browser |
| `portree ls` | Show status |

### URL Format
```
http://<branch-slug>.localhost:<port>
```

### Environment Variables
| Variable | Description |
|----------|-------------|
| `PORT` | Assigned port |
| `PT_BRANCH` | Branch name |
| `PT_BRANCH_SLUG` | URL-safe branch |
| `PT_{SERVICE}_PORT` | Other service's port |

### Configuration (.portree.toml)
```toml
[services.frontend]
command = "npm run dev"
port_env = "PORT"

[services.backend]
command = "go run ."
port_range = [8100, 8199]
```

### Troubleshooting
- Won't start: `portree doctor`
- Port conflict: `lsof -i :<port>`
- Logs: `~/.portree/logs/`
