# Local Development

## Prerequisites

- [Go](https://go.dev/dl/) 1.23+
- [Node.js](https://nodejs.org/) 22+ with npm
- [OpenCode](https://opencode.ai) CLI (`opencode` on PATH)
- [Git](https://git-scm.com/)
- [PowerShell 7](https://github.com/PowerShell/PowerShell) (for validation scripts on Windows)

OpenCode server communication uses HTTP Basic Auth credentials from environment variables:

- `OPENCODE_SERVER_USERNAME` (default: `opencode`)
- `OPENCODE_SERVER_PASSWORD` (set by OpenCode Desktop)

## Run Locally

Shell: PowerShell 7
Working directory: project root

**Backend:**

```powershell
npm.cmd --prefix web run build
go run ./cmd/glyphdeck
```

Starts on `http://127.0.0.1:8756`. Health check at `/healthz`.

**Frontend (dev mode):**

```powershell
npm.cmd --prefix web install
npm.cmd --prefix web run dev
```

Starts Vite dev server (default: `http://localhost:5173`).

### First-run auth

On first launch, the UI shows a setup screen. For headless development:

```powershell
$env:GLYPHDECK_ADMIN_PASSWORD = "dev-password"
go run ./cmd/glyphdeck
```

## Remote SSH target development

GlyphDeck remote targets use an SSH configuration alias already available to
the local `ssh` client. Configure the alias outside GlyphDeck, then open the
**Servers** activity-rail view and add an **SSH Alias** target.

The target form stores a display name, SSH alias, and optional remote working
directory and lifecycle commands. It does not store private keys, passwords,
or key-file paths.

Use **Test SSH** to validate the alias, **Detect** to refresh remote OpenCode
status, and **Start** to launch the configured remote command. **Attach**
selects an online target for the application. **Detach** clears that selection
only; it does not stop the remote process. **Stop** remains restricted to an
eligible recorded PID and verifies the remote process before stopping that
exact PID. The delete confirmation warns about attached targets and recorded
processes; it never stops a process automatically.

When troubleshooting, report the action and sanitized error text. Do not put
passwords, private keys, or complete SSH command output in logs or issues.

### Release binary

```powershell
pwsh -NoProfile -ExecutionPolicy Bypass -File .\scripts\build.ps1
.\dist\glyphdeck.exe
```

The binary listens on `http://127.0.0.1:8756` and does not require a `web/dist`
directory beside its working directory after it has been built.

## Architecture

See [ARCHITECTURE_NOTES.md](ARCHITECTURE_NOTES.md) for package layout, dependency graph,
and design decisions.

## `GLYPHDECK_DATA_DIR`

Set `GLYPHDECK_DATA_DIR` to use an isolated data directory for validation runs.
The application uses a repo-local `.glyphdeck/` directory by default.

## Environment Variables

| Variable | Purpose | Default |
|----------|---------|---------|
| `GLYPHDECK_HOST` | Listen address (must be loopback) | `127.0.0.1` |
| `GLYPHDECK_PORT` | Listen port | `8756` |
| `GLYPHDECK_DATA_DIR` | Data directory for SQLite | `.glyphdeck/` |
| `GLYPHDECK_DEV_TOOLS` | Enable dev endpoints (`1` to enable) | (unset) |
| `GLYPHDECK_ADMIN_PASSWORD` | Bootstrap admin on first start | (unset) |
| `OPENCODE_SERVER_USERNAME` | OpenCode Basic Auth username | `opencode` |
| `OPENCODE_SERVER_PASSWORD` | OpenCode Basic Auth password | (required) |
