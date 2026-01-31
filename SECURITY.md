# Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| latest  | :white_check_mark: |

## Reporting a Vulnerability

If you discover a security vulnerability in portree, please report it responsibly.

**Do not open a public issue.** Instead, please email security concerns to the maintainers via GitHub private vulnerability reporting:

1. Go to the [Security tab](https://github.com/fairy-pitta/portree/security) of this repository
2. Click "Report a vulnerability"
3. Provide a description of the issue, steps to reproduce, and potential impact

We will acknowledge receipt within 48 hours and aim to provide a fix within 7 days for critical issues.

## Scope

portree is a local development tool. Its threat model assumes:

- **Trusted configuration**: `.portree.toml` is committed to your repository. Commands defined in it are executed via `sh -c` without sandboxing. Only use portree in repositories you trust.
- **Local-only network**: The reverse proxy binds to `localhost`. It is not designed to be exposed to untrusted networks.
- **State file**: `.portree/state.json` contains PIDs and port numbers. It does not store secrets.
- **Log files**: `.portree/logs/` may contain stdout/stderr from your services, which could include sensitive data depending on your application.

## Known Limitations

- Service commands from `.portree.toml` are executed as shell commands with the current user's privileges
- Log files are created with `0644` permissions (world-readable)
- No authentication or authorization on the reverse proxy
