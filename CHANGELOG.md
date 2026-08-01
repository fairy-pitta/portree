# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.4.0](https://github.com/fairy-pitta/portree/compare/v0.3.0...v0.4.0) (2026-08-01)


### Features

* **config:** let a project say which service "portree open" opens ([#28](https://github.com/fairy-pitta/portree/issues/28)) ([10a1797](https://github.com/fairy-pitta/portree/commit/10a1797fa85158280ccb312c1480cf8e934b9424))
* **proxy:** start the proxy from up, and refuse to open dead URLs ([#26](https://github.com/fairy-pitta/portree/issues/26)) ([e99b77a](https://github.com/fairy-pitta/portree/commit/e99b77a27e318a970111155a2ec6df13915a135d))


### Bug Fixes

* **browser:** stop the test suite from opening real browser tabs ([#23](https://github.com/fairy-pitta/portree/issues/23)) ([12a4447](https://github.com/fairy-pitta/portree/commit/12a4447a0337f1598b946118203e72065effc0c8))
* **cmd:** keep the dev CA key out of git, and stop reporting phantom stops ([#27](https://github.com/fairy-pitta/portree/issues/27)) ([9484680](https://github.com/fairy-pitta/portree/commit/94846806e760d0bcdd94bea30414770281cee2b3))
* **cmd:** make the first run work and let doctor see what is wrong ([#25](https://github.com/fairy-pitta/portree/issues/25)) ([044e8dc](https://github.com/fairy-pitta/portree/commit/044e8dc1812c2eadc3f6da3912941553faf86709))
* **cmd:** show real URLs in proxy status, and say when logs are empty ([#29](https://github.com/fairy-pitta/portree/issues/29)) ([8d64e52](https://github.com/fairy-pitta/portree/commit/8d64e52259a7002b106361fa7ec88ce1e1e996d8))
* **proxy:** reject ambiguous branch slugs instead of routing at random ([#24](https://github.com/fairy-pitta/portree/issues/24)) ([c2dea99](https://github.com/fairy-pitta/portree/commit/c2dea9937d7bce7ee582627f8051be300cc7f88f))

## [Unreleased]

### Added

- `portree completion` command for bash, zsh, fish, and powershell
- `--json` flag on `portree ls` and `portree version` for machine-readable output
- `--verbose` / `--quiet` global flags with leveled logging (`internal/logging` package)
- `portree doctor` command for environment diagnostics
- Homebrew tap distribution (`brew install fairy-pitta/tap/portree`)
- Pre-commit hook (`.githooks/pre-commit`) running vet, lint, and short tests
- `make setup-hooks` target to configure git hooks
- Branch slug collision detection with warnings on `portree up`
- Comprehensive tests for Runner lifecycle, Manager integration, and ProxyServer
- CHANGELOG.md

### Fixed

- Process lifecycle race: `done` channel now initialized before `cmd.Start()`
- Double `cmd.Wait()` crash replaced with single-goroutine done channel
- File descriptor leak in proxy server (listeners now tracked and closed)
- `sync.Mutex` upgraded to `sync.RWMutex` in Manager for concurrent reads
- `WithLock` errors in resolver, manager, and TUI no longer silently swallowed
- Windows browser open (`cmd /c start` instead of `rundll32`)
- `os.Getwd()` error handling in TUI start/stop all actions
- Service status check before opening browser in TUI
- All `errcheck` lint violations resolved across 8 files
- golangci-lint CI configuration for Go 1.25 compatibility (action v9, lint v2.8)
- TOCTOU race condition in port allocator documented

### Changed

- Renamed project from `gws` to `portree`
- Go test matrix reduced to Go 1.25 only (matches go.mod requirement)

## [0.1.0] - Initial Release

### Added

- Core process manager: start, stop, restart services per worktree
- Automatic port allocation via FNV32 hash with configurable ranges
- Reverse proxy with subdomain-based routing (`<slug>.localhost:<port>`)
- Interactive TUI dashboard (`portree dash`) with Bubble Tea
- TOML configuration (`.portree.toml`) with validation
- Per-worktree and per-branch service overrides
- Cross-service environment variables (`PT_<SVC>_PORT`, `PT_<SVC>_URL`)
- Service log files in `.portree/logs/`
- File-based state persistence with file locking
- Commands: `init`, `up`, `down`, `ls`, `dash`, `proxy start/stop`, `open`, `version`
- Cross-platform support: Linux, macOS, Windows (amd64, arm64)
- GoReleaser-based release pipeline with GitHub Actions CI
