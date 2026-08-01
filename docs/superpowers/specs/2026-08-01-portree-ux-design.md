# portree UX improvements — design

Date: 2026-08-01

## Problem

Walking the first-run path (`portree init` → `up` → `open`) surfaced five defects.
Each was reproduced against a scratch repository before being written down.

| ID | Defect | Evidence |
| -- | ------ | -------- |
| A | `init` then `up` always fails, and the error hides the cause | `error: starting main/web: fork/exec /bin/sh: no such file or directory`. `/bin/sh` exists; the real cause is that the generated template points `dir` at `frontend`/`backend`, which do not exist. Creating the directory makes the same command start. `doctor` reports "All checks passed" throughout. |
| B | `open` launches a browser at a URL that cannot answer | After `up` with no proxy running, `open` prints `Opening http://main.localhost:3000` and shells out to the browser. `curl` against that URL fails to connect. |
| C | `doctor`'s port check bypasses `port.IsFree` | `cmd/doctor.go:148` calls `net.Listen("tcp", ":3000")`. Go enables `SO_REUSEADDR` and binds the wildcard, so a listener on `127.0.0.1:3000` is missed. Measured: `doctor` printed `✓ proxy port 3000 (web) available` while portree's own proxy held that port. |
| D | The proxy is foreground-only | No `--detach`, no `proxy status`. Reviewing several worktrees means dedicating a terminal, and `up` does not start the proxy, so the documented workflow needs two steps plus a resident shell. |
| E | `ls --json` drops `url` when the proxy is down | `url` is `omitempty` and only set when the proxy runs. The README advertises it as the agent-facing discovery field, so consumers cannot tell a missing URL from a stopped proxy. |

## Scope

Two independently shippable changes.

- **PR 1 — corrections (A, C, E).** No new surface; existing behaviour is wrong.
- **PR 2 — proxy experience (B, D).** New flags and commands.

## PR 1

### A. Detect a missing working directory before starting

- `internal/process/runner.go` `Start` stats `r.config.Dir` before `exec` and returns
  `service %q: working dir %q does not exist`. Placing the check in `Runner` keeps the
  message good for every caller.
- `cmd/doctor.go` gains a "service working dirs" check covering every worktree × service,
  listing each missing path.
- `cmd/init.go` comments out the `dir` lines in the generated template so a freshly
  initialised config runs at the repository root without edits.

### C. Three-way port check

Replace `net.Listen` with `port.IsFree`. A straight swap would flag a healthy proxy as a
failure, so the check distinguishes three states, using the recorded proxy PID
(`process.IsProcessRunning`) to tell our own listener from a stranger's.

| State | Result |
| ----- | ------ |
| Nobody holds the port | ✓ `proxy port 3000 (web) available` |
| Our proxy holds it | ✓ `proxy port 3000 (web) — portree proxy running` |
| Another process holds it | ✗ `proxy port 3000 (web) — already in use by another process` |

### E. Stable JSON shape

`url` is always emitted, and each entry gains `proxy_running` (bool) so a consumer can
distinguish "no URL" from "URL exists but nothing is serving it". README updated to match.

## PR 2

### `proxy start --detach`

Re-executes `os.Executable()` as `proxy start` (dropping `--detach`, carrying `--https`,
`--cert`, `--key`) with `Setpgid: true` and stdio redirected to `.portree/logs/proxy.log`.
The child records its own PID through the existing state write, so `proxy stop` is unchanged.

The parent does not return until every `proxy_port` accepts a TCP connection, capped at
five seconds. If the child exits during that window, the parent prints the tail of
`proxy.log` and fails. Without this the command would report success while `open` still
fails — the defect this work exists to remove.

Foreground remains the default for `proxy start`.

### `proxy status`

Reports scheme, ports, and PID, judged by liveness of the recorded PID plus a real
connection to each port. Clears a stale PID when it finds one. Supports `--json`.

### `up` starts the proxy

After services start, `up` ensures the proxy is up, starting it detached when it is not.
An already-running proxy is left alone so its HTTPS mode is preserved. `--no-proxy` opts
out. `up` finishes by listing the reachable URLs.

### `open` refuses to open a dead URL

When the proxy is not running, or the target service is stopped, `open` fails with the
command to run instead and does not launch a browser.

## Testing

Test-driven throughout. Characterisation tests covering existing behaviour are proven by
mutation: break the production path, confirm the test fails, restore.

- The `--detach` readiness wait is tested with a proxy that exits immediately, asserting
  the failure is reported rather than swallowed.
- The `open` guard is tested with a fake `open` binary on `PATH`, asserting no browser
  launch is recorded.
- Each of the three `doctor` port states is set up for real by holding the port.
