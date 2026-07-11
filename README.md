# GlyphDeck

A local-first web workspace for managing [OpenCode](https://opencode.ai) projects,
servers, sessions, and terminals — all from a browser UI.

[![Verify](https://github.com/fireurza/GlyphDeck/actions/workflows/verify.yml/badge.svg)](https://github.com/fireurza/GlyphDeck/actions/workflows/verify.yml)
[![CodeQL](https://github.com/fireurza/GlyphDeck/actions/workflows/codeql.yml/badge.svg)](https://github.com/fireurza/GlyphDeck/actions/workflows/codeql.yml)
[![OpenSSF Scorecard](https://api.securityscorecards.dev/projects/github.com/fireurza/GlyphDeck/badge)](https://securityscorecards.dev/viewer/?uri=github.com/fireurza/GlyphDeck)

## What It Does

- Manage multiple OpenCode project directories.
- Start, stop, and attach to local or remote OpenCode servers.
- Create sessions, send prompts, and view transcripts in real time.
- Review per-project Git status, token usage, and activity summaries.
- Handle OpenCode permissions (approve/reject/always).
- Run an interactive terminal in the project working directory.
- Secured by admin authentication and localhost-only binding.

## Screenshots

<!-- TODO: add screenshots of primary views -->

## Features

| Feature | Status |
|---------|--------|
| Project registry (SQLite-backed) | ✅ |
| OpenCode server start/stop/detect (per-project, PID-scoped) | ✅ |
| Session list, create, prompt, transcript (SSE live) | ✅ |
| Review panel (Git, project, session, activity summary) | ✅ |
| Usage panel (token/cost aggregation) | ✅ |
| Agent Terminal (tool call history) | ✅ |
| User Terminal (interactive shell in project cwd) | ✅ |
| Permissions (once/always/reject popup) | ✅ |
| Admin authentication (bcrypt, HttpOnly sessions) | ✅ |
| Servers/Sandboxes (local, manual URL, SSH alias targets) | ✅ |
| Remote SSH lifecycle (detect, start, stop — PID-scoped) | ✅ |
| Windows ConPTY terminal support | ✅ |
| Release build (single binary with embedded React frontend) | ✅ |
| CI/CD (verify, CodeQL, Dependabot) | ✅ |

## Requirements

- **Go** 1.23+ (to build from source)
- **Node.js** 22+ with npm (to build the frontend)
- **OpenCode** CLI installed and on PATH
- **Git** (optional; enables per-project Git status)
- **PowerShell 7** (Windows; required for validation scripts)

OpenCode server communication uses HTTP Basic Auth credentials from environment:

- `OPENCODE_SERVER_USERNAME` (default: `opencode`)
- `OPENCODE_SERVER_PASSWORD` (set by OpenCode Desktop)

## Quick Start

### From source

```powershell
# Clone and build
git clone https://github.com/fireurza/GlyphDeck.git
cd GlyphDeck

# Build frontend + Go binary
pwsh -NoProfile -ExecutionPolicy Bypass -File .\scripts\build.ps1

# Run
.\dist\glyphdeck.exe
```

Open `http://127.0.0.1:8756` in your browser.

### Release binary

Download `glyphdeck.exe` from [releases](https://github.com/fireurza/GlyphDeck/releases).
Run it directly — the frontend is embedded. No separate `dist/` or `node_modules/` needed.

## First-Run Setup

On first launch, GlyphDeck shows a setup screen to create an admin password.
The password is bcrypt-hashed and stored in a local SQLite database.

### Headless bootstrap

Set `GLYPHDECK_ADMIN_PASSWORD` before starting to skip interactive setup:

```powershell
$env:GLYPHDECK_ADMIN_PASSWORD = "your-password"
.\dist\glyphdeck.exe
```

The password is never logged. Bootstrap runs only when no admin exists.

## Servers & Sandboxes

GlyphDeck supports three server target types:

| Type | Description |
|------|------------|
| **Local** | Start an OpenCode server on your machine (PID-scoped, stops only that instance) |
| **Manual URL** | Attach to any OpenCode server by URL |
| **SSH Alias** | Connect to a remote OpenCode server via SSH config alias |

Remote SSH operations are PID-scoped — start captures the remote PID, stop
verifies PID ownership before killing. Blanket `pkill` is never used.

## Security Model

GlyphDeck v0.1.0 is designed for **local use**.

- Binds to `127.0.0.1` (loopback) by default.
- Mutating API requests require same-origin `Origin` and loopback host.
- Admin auth required for all API access (bcrypt + HttpOnly cookies).
- **Not designed for public internet exposure.** Add a reverse proxy with TLS
  and additional auth layers if remote access is needed.

See [SECURITY.md](SECURITY.md) for details and vulnerability reporting.

## Validation

Run the full validation suite:

```powershell
go test ./... -count=1
go vet ./cmd/... ./internal/... ./web/...
cd web && npm test && npm run build
.\scripts\build.ps1
.\scripts\validation\run-mvp-smoke.ps1
npm.cmd --prefix web audit --audit-level=high
```

The smoke test verifies 17 checks including auth, project/server/session
lifecycle, terminal child-process cleanup, and visual regression screenshots.

## Known Limitations

- Designed for single-user, local-machine use.
- No multi-user or role-based access control.
- WebSocket/SSE connections share the same session cookie as HTTP.
- Remote agent/skill/MCP sync not yet implemented.
- No built-in TLS — add a reverse proxy if needed.

## Roadmap

See [docs/ROADMAP.md](docs/ROADMAP.md) for planned features and milestones.

## Contributing

Contributions are welcome! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

[MIT](LICENSE)
