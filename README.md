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
| Docker Compose preview | ✅ |
| npm launcher preview | ✅ |

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

### npm launcher preview

**The npm package is not yet published.** The launcher downloads and runs the
verified release binary from GitHub. Requires Node.js 22+.

```bash
# Run GlyphDeck (downloads on first use):
npx @fireglyph/glyphdeck

# Pass arguments through:
npx @fireglyph/glyphdeck --help
```

The launcher caches binaries per version and verifies every download against
`checksums.txt` before execution. See `packages/launcher/README.md` for details.

### Docker Compose preview

**Docker is a preview install path, not the primary stable install path yet.**

Prerequisites: Docker Engine 24+ with Compose plugin.

```powershell
# Create a local admin password secret.
New-Item -ItemType Directory -Force -Path secrets
Set-Content -Path secrets\glyphdeck_admin_password.txt -Value "your-admin-password" -NoNewline

# Validate and start.
docker compose -f compose.yaml config
docker compose -f compose.yaml up -d --wait
```

Open `http://127.0.0.1:8756` in your browser. The published host port is
configured via `GLYPHDECK_HOST_PORT` (default: `8756`).

Stop:

```powershell
docker compose -f compose.yaml down
```

**Persistence:** SQLite app data is stored in the `glyphdeck-app-data` named
volume. The database survives container recreation and image rebuilds. To back
up:

```powershell
docker run --rm -v glyphdeck-app-data:/data -v ${PWD}:/backup alpine tar czf /backup/glyphdeck-data-backup.tar.gz -C /data .
```

**Security controls:**

- Non-root container user.
- All Linux capabilities dropped (`cap_drop: ALL`).
- `no-new-privileges:true`.
- Loopback-only host publication (`127.0.0.1:8756`).
- No Docker socket mounted.
- No privileged mode.
- Read-only root filesystem.

**OpenCode is external.** The container does not bundle OpenCode. Use a manual
URL or SSH alias target to connect to an OpenCode server running elsewhere.

**SSH targets from Docker** require an explicit, user-controlled read-only mount
of SSH configuration and key material:

```powershell
docker compose -f compose.yaml run --rm `
  -v "$env:USERPROFILE\.ssh:/home/glyphdeck/.ssh:ro" `
  glyphdeck
```

This mount is **not enabled by default**. See [docs/development/LOCAL_DEVELOPMENT.md](docs/development/LOCAL_DEVELOPMENT.md)
for details.

## First-Run Setup

On first launch, GlyphDeck shows a setup screen to create an admin password.
The password is bcrypt-hashed and stored in a local SQLite database.

### Headless bootstrap

Set `GLYPHDECK_ADMIN_PASSWORD` before starting to skip interactive setup:

```powershell
$env:GLYPHDECK_ADMIN_PASSWORD = "your-password"
.\dist\glyphdeck.exe
```

Or use a file-based secret with `GLYPHDECK_ADMIN_PASSWORD_FILE`:

```powershell
$env:GLYPHDECK_ADMIN_PASSWORD_FILE = "C:\path\to\secret.txt"
.\dist\glyphdeck.exe
```

Setting both `GLYPHDECK_ADMIN_PASSWORD` and `GLYPHDECK_ADMIN_PASSWORD_FILE`
is an error. The password is never logged. Bootstrap runs only when no admin
exists.

## Servers & Sandboxes

GlyphDeck supports three server target types:

| Type | Description |
|------|------------|
| **Local** | Start an OpenCode server on your machine (PID-scoped, stops only that instance) |
| **Manual URL** | Attach to any OpenCode server by URL |
| **SSH Alias** | Connect to a remote OpenCode server via SSH config alias |

Remote SSH operations are PID-scoped — start captures the remote PID, stop
verifies PID ownership before killing. Blanket `pkill` is never used.

### Remote SSH targets

Configure an SSH host alias in your local SSH configuration before adding a
target. GlyphDeck passes that alias to the system `ssh` client; it does not
store private keys, passwords, or key-file paths.

Open **Servers** from the activity rail, then add or edit an **SSH Alias**
target. Supply a display name and alias. Optional working-directory, start,
stop, and status commands are used only for that target's lifecycle actions.

Use **Test SSH** to check the configured alias, **Detect** to refresh remote
OpenCode status, and **Start** to launch a remote server. A successful start
records the returned PID and URL. **Attach** selects a known online target for
GlyphDeck; **Detach** only clears that selection and never stops the remote
process. **Stop** is available only for an eligible recorded PID and verifies
the expected remote OpenCode process before stopping that exact PID.

Deleting a target is confirmed in the UI. GlyphDeck warns when the target is
attached or has a recorded process; deletion never silently stops a remote
process. For failed SSH actions, verify the alias and remote OpenCode command
without placing credentials in target fields or issue reports.

## Security Model

GlyphDeck v0.1.2 is designed for **local use**.

- Binds to `127.0.0.1` (loopback) by default. Container-mode (`GLYPHDECK_CONTAINER_MODE=1`) allows internal `0.0.0.0` bind; host publication remains loopback-only.
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

### Docker preview smoke

```powershell
.\scripts\validation\run-docker-preview-smoke.ps1
```

Validates the Docker Compose preview stack in an isolated environment:
build, startup, healthcheck, admin auth, data persistence across container
recreation, non-root user, loopback-only publication, and no Docker socket.

## Known Limitations

- Designed for single-user, local-machine use.
- No multi-user or role-based access control.
- WebSocket/SSE connections share the same session cookie as HTTP.
- Remote agent/skill/MCP sync not yet implemented.
- No built-in TLS — add a reverse proxy if needed.
- Docker Compose is a preview; the primary install path is the native binary.
- SSH targets from Docker require explicit read-only key mounts.

## Roadmap

See [docs/ROADMAP.md](docs/ROADMAP.md) for planned features and milestones.

## Contributing

Contributions are welcome! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.
See [CONTRIBUTOR_TERMS.md](CONTRIBUTOR_TERMS.md) for the contributor license grant
and [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md) for community standards.

## License

GlyphDeck is source-available. Noncommercial use is governed by the
[PolyForm Noncommercial License 1.0.0](LICENSE).

**Commercial use requires a separate written commercial license from
FireGlyph Studios.** See [COMMERCIAL-LICENSING.md](COMMERCIAL-LICENSING.md)
for details.
