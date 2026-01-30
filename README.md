# gws - Git Worktree Server Manager

**gws** automatically manages multiple dev servers per [git worktree](https://git-scm.com/docs/git-worktree) — with automatic port allocation, environment variable injection, and `*.localhost` subdomain routing via reverse proxy.

> Japanese version: [README.ja.md](./README.ja.md)

---

## Features

- **Multi-service** — Define frontend, backend, and any number of services per worktree
- **Automatic port allocation** — Hash-based port assignment (FNV32) with per-service ranges; no port conflicts across worktrees
- **Subdomain reverse proxy** — Access any worktree via `branch-name.localhost:<port>` (no `/etc/hosts` editing required)
- **Environment variable injection** — `$PORT`, `$GWS_BRANCH`, `$GWS_BACKEND_URL`, etc. are injected automatically
- **TUI dashboard** — Interactive terminal UI to start, stop, restart, and monitor all services
- **Process lifecycle** — Graceful shutdown (SIGTERM → SIGKILL), log files, stale PID cleanup
- **Per-worktree overrides** — Customize commands, ports, and env vars per branch

---

## Quick Start

### 1. Install

```bash
# From source
go install github.com/shuna/gws@latest

# Or build locally
git clone https://github.com/shuna/gws.git
cd gws
make build
```

### 2. Initialize

```bash
cd your-project
gws init
# Creates .gws.toml in the repo root
```

### 3. Configure

Edit `.gws.toml` to match your project:

```toml
[services.frontend]
command = "pnpm run dev"
dir = "frontend"
port_range = { min = 3100, max = 3199 }
proxy_port = 3000

[services.backend]
command = "source .venv/bin/activate && python manage.py runserver 0.0.0.0:$PORT"
dir = "backend"
port_range = { min = 8100, max = 8199 }
proxy_port = 8000

[env]
NODE_ENV = "development"
```

### 4. Start services

```bash
gws up            # Start all services for the current worktree
gws up --all      # Start all services for ALL worktrees
```

### 5. Start the proxy

```bash
gws proxy start
# :3000 → frontend services
# :8000 → backend services
```

### 6. Open in browser

```bash
gws open                    # Opens http://main.localhost:3000
gws open --service backend  # Opens http://main.localhost:8000
```

---

## Commands

| Command            | Description                                           |
| ------------------ | ----------------------------------------------------- |
| `gws init`         | Create a `.gws.toml` configuration file               |
| `gws up`           | Start services for the current worktree               |
| `gws up --all`     | Start services for all worktrees                      |
| `gws up --service` | Start a specific service only                         |
| `gws down`         | Stop services for the current worktree                |
| `gws down --all`   | Stop services for all worktrees                       |
| `gws ls`           | List all worktrees, services, ports, status, and PIDs |
| `gws dash`         | Open the interactive TUI dashboard                    |
| `gws proxy start`  | Start the reverse proxy (foreground)                  |
| `gws proxy stop`   | Stop the reverse proxy                                |
| `gws open`         | Open the current worktree in a browser                |
| `gws version`      | Print version information                             |

---

## Configuration Reference

The `.gws.toml` file lives at the root of your git repository.

### `[services.<name>]`

Define one or more services. Each worktree will run all defined services.

| Field        | Type         | Required | Description                                                 |
| ------------ | ------------ | -------- | ----------------------------------------------------------- |
| `command`    | string       | yes      | Shell command to start the service                          |
| `dir`        | string       | no       | Working directory relative to worktree root (default: root) |
| `port_range` | `{min, max}` | yes      | Port allocation range for this service                      |
| `proxy_port` | int          | yes      | Port the reverse proxy listens on for this service          |

```toml
[services.frontend]
command = "pnpm run dev"
dir = "frontend"
port_range = { min = 3100, max = 3199 }
proxy_port = 3000
```

### `[env]`

Global environment variables injected into all services.

```toml
[env]
NODE_ENV = "development"
DATABASE_URL = "postgres://localhost/mydb"
```

### `[worktrees."<branch>"]`

Per-worktree overrides. You can customize the command, fix a specific port, or add extra environment variables.

```toml
[worktrees.main]
services.frontend.port = 3100       # Fixed port for main branch

[worktrees."feature/auth"]
services.backend.command = "python manage.py runserver --settings=myapp.auth 0.0.0.0:$PORT"
services.backend.env = { DEBUG = "1" }
```

---

## Environment Variables

gws automatically injects the following environment variables into every service process:

| Variable             | Example                                              | Description                       |
| -------------------- | ---------------------------------------------------- | --------------------------------- |
| `PORT`               | `3117`                                               | Allocated port for this service   |
| `GWS_BRANCH`         | `feature/auth`                                       | Current branch name               |
| `GWS_BRANCH_SLUG`    | `feature-auth`                                       | URL-safe slug of the branch name  |
| `GWS_SERVICE`        | `frontend`                                           | Name of the current service       |
| `GWS_<SERVICE>_PORT` | `GWS_FRONTEND_PORT=3117`                             | Port of each sibling service      |
| `GWS_<SERVICE>_URL`  | `GWS_BACKEND_URL=http://feature-auth.localhost:8000` | Proxy URL of each sibling service |

This allows services to discover each other automatically:

```js
// next.config.js
module.exports = {
  async rewrites() {
    return [
      {
        source: "/api/:path*",
        destination: `${process.env.GWS_BACKEND_URL}/api/:path*`,
      },
    ];
  },
};
```

---

## How It Works

```
┌─────────────────────────────────────────────────────────────┐
│  git repository                                             │
│                                                             │
│  main worktree          feature/auth worktree               │
│  ┌───────────────┐      ┌───────────────┐                   │
│  │ frontend :3100│      │ frontend :3117│                   │
│  │ backend  :8100│      │ backend  :8104│                   │
│  └───────────────┘      └───────────────┘                   │
│         │                      │                            │
└─────────┼──────────────────────┼────────────────────────────┘
          │                      │
    ┌─────▼──────────────────────▼─────┐
    │       gws reverse proxy          │
    │                                  │
    │  :3000  ←  *.localhost:3000      │
    │  :8000  ←  *.localhost:8000      │
    └──────────────────────────────────┘
          │                      │
          ▼                      ▼
  main.localhost:3000    feature-auth.localhost:3000
  main.localhost:8000    feature-auth.localhost:8000
```

1. **Port allocation** — Each service gets a port via `FNV32(branch:service) % range`. Stable across restarts.
2. **Process management** — Services run as child processes with process groups. Logs go to `.gws/logs/`.
3. **Reverse proxy** — One HTTP listener per `proxy_port`. Routes based on `Host` header subdomain.
4. **`*.localhost`** — Per [RFC 6761](https://tools.ietf.org/html/rfc6761), modern browsers resolve `*.localhost` to `127.0.0.1` automatically.

---

## TUI Dashboard

Launch with `gws dash`:

```
╭─ gws dashboard ──────────────────────────────────────────────╮
│                                                               │
│  WORKTREE        SERVICE    PORT   STATUS      PID            │
│  ──────────────────────────────────────────────────────────── │
│ ▸ main           frontend   3100   ● running   12345          │
│   main           backend    8100   ● running   12346          │
│   feature/auth   frontend   3117   ○ stopped   —              │
│   feature/auth   backend    8104   ○ stopped   —              │
│                                                               │
│  Proxy: ● running (:3000, :8000)                              │
│                                                               │
│  [s] start  [x] stop  [r] restart  [o] open in browser       │
│  [a] start all  [X] stop all  [p] toggle proxy                │
│  [l] view logs  [q] quit                                      │
╰───────────────────────────────────────────────────────────────╯
```

**Key bindings:**

| Key     | Action                   |
| ------- | ------------------------ |
| `j`/`k` | Move cursor down/up      |
| `s`     | Start selected service   |
| `x`     | Stop selected service    |
| `r`     | Restart selected service |
| `o`     | Open in browser          |
| `a`     | Start all services       |
| `X`     | Stop all services        |
| `p`     | Toggle proxy             |
| `l`     | View log file path       |
| `q`     | Quit                     |

---

## Example Workflow

```bash
# You're working on a monorepo with frontend + backend
cd my-project

# Initialize gws
gws init
# Edit .gws.toml to define your services...

# Create a feature branch worktree
git worktree add ../my-project-feature-auth feature/auth

# Start services on your current branch
gws up
# Starting frontend (port 3100) for main ...
# Starting backend (port 8100) for main ...
# ✓ 2 services started for main

# Start services on ALL worktrees at once
gws up --all
# ✓ 4 services started

# Check status
gws ls
# WORKTREE        SERVICE    PORT   STATUS    PID
# main            frontend   3100   running   12345
# main            backend    8100   running   12346
# feature/auth    frontend   3117   running   12347
# feature/auth    backend    8104   running   12348

# Start the proxy
gws proxy start
# Access:
#   http://main.localhost:3000          → frontend (main)
#   http://main.localhost:8000          → backend (main)
#   http://feature-auth.localhost:3000  → frontend (feature/auth)
#   http://feature-auth.localhost:8000  → backend (feature/auth)

# Open in browser
gws open
# Opening http://main.localhost:3000 ...

# Or use the TUI
gws dash

# When done
gws down --all
# ✓ 4 services stopped
```

---

## FAQ

### Does `*.localhost` work in all browsers?

Modern browsers (Chrome, Firefox, Edge, Safari) resolve `*.localhost` to `127.0.0.1` per [RFC 6761](https://tools.ietf.org/html/rfc6761). No `/etc/hosts` editing or DNS configuration is needed.

### What happens if two worktrees hash to the same port?

gws uses linear probing — if the hash-derived port is already taken, it tries the next port in the range until it finds a free one.

### Can I use gws without the proxy?

Yes. `gws up` starts your services with allocated ports. You can access them directly at `localhost:<port>`. The proxy is optional.

### Where are logs stored?

Service logs are written to `.gws/logs/<branch-slug>.<service>.log` in the main worktree's root.

### Where is state stored?

Runtime state (PIDs, port assignments) is stored in `.gws/state.json` with file-level locking for concurrent access safety.

### Can I run different commands per branch?

Yes, use `[worktrees."branch-name"]` overrides in `.gws.toml`:

```toml
[worktrees."feature/auth"]
services.backend.command = "python manage.py runserver --settings=auth 0.0.0.0:$PORT"
services.backend.env = { DEBUG = "1" }
```

---

## Project Structure

```
gws/
├── main.go                      # Entry point
├── cmd/                         # CLI commands (cobra)
│   ├── root.go                  # Root command + repo/config detection
│   ├── init.go                  # gws init
│   ├── up.go                    # gws up
│   ├── down.go                  # gws down
│   ├── ls.go                    # gws ls
│   ├── dash.go                  # gws dash
│   ├── proxy.go                 # gws proxy start|stop
│   ├── open.go                  # gws open
│   └── version.go               # gws version
├── internal/
│   ├── config/config.go         # .gws.toml loading & validation
│   ├── git/
│   │   ├── repo.go              # Repo root / common dir detection
│   │   └── worktree.go          # Worktree listing & branch slugs
│   ├── state/store.go           # JSON state persistence with flock
│   ├── port/
│   │   ├── allocator.go         # FNV32 hash-based port allocation
│   │   └── registry.go          # Port assignment management
│   ├── process/
│   │   ├── runner.go            # Single process lifecycle
│   │   └── manager.go           # Multi-service orchestration
│   ├── proxy/
│   │   ├── resolver.go          # Slug + port → backend resolution
│   │   └── server.go            # HTTP reverse proxy
│   ├── browser/open.go          # OS-aware browser opening
│   └── tui/                     # Bubble Tea TUI dashboard
│       ├── app.go               # Top-level model
│       ├── dashboard.go         # Table rendering
│       ├── keys.go              # Key bindings
│       ├── messages.go          # Custom messages
│       └── styles.go            # Lip Gloss styles
├── Makefile
├── .goreleaser.yaml
└── .github/workflows/
    ├── ci.yaml
    └── release.yaml
```

---

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing`)
3. Commit your changes (`git commit -m 'feat: add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing`)
5. Open a Pull Request

```bash
# Development
make build      # Build binary
make test       # Run tests with race detector
make lint       # Run golangci-lint
make all        # fmt + vet + lint + test + build
```

---

## License

MIT License. See [LICENSE](./LICENSE) for details.
