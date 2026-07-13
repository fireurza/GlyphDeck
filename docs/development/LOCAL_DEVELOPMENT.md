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
| `GLYPHDECK_ADMIN_PASSWORD_FILE` | Path to file containing admin password | (unset) |
| `GLYPHDECK_CONTAINER_MODE` | Allow `0.0.0.0` bind (`1` to enable) | (unset) |
| `OPENCODE_SERVER_USERNAME` | OpenCode Basic Auth username | `opencode` |
| `OPENCODE_SERVER_PASSWORD` | OpenCode Basic Auth password | (required) |

### GLYPHDECK_ADMIN_PASSWORD_FILE

Instead of setting the admin password directly in an environment variable,
you can store it in a file and point `GLYPHDECK_ADMIN_PASSWORD_FILE` to that
path. The file is read at startup, trimmed of whitespace, and used to bootstrap
the admin account. The password value is never logged.

Setting both `GLYPHDECK_ADMIN_PASSWORD` and `GLYPHDECK_ADMIN_PASSWORD_FILE` is
an error. The application will refuse to start.

### GLYPHDECK_CONTAINER_MODE

When running inside a Docker container, set `GLYPHDECK_CONTAINER_MODE=1`. This
allows the server to bind `0.0.0.0` internally so the container port is
reachable from the Docker network. The host publication is still restricted
to loopback by the Compose port mapping.

Container mode requires `GLYPHDECK_HOST=0.0.0.0`. Any other host value is
rejected. Outside container mode, only loopback hosts (`127.0.0.1`,
`localhost`) are accepted.

## Docker Compose Preview

A Docker Compose preview stack is available for evaluation. Docker is **not**
the primary stable install path yet.

### Prerequisites

- Docker Engine 24+ with Compose plugin
- Git (to clone the repository)

### Quick start

```powershell
# Clone the repository
git clone https://github.com/fireurza/GlyphDeck.git
cd GlyphDeck

# Create an admin password secret.
New-Item -ItemType Directory -Force -Path secrets
Set-Content -Path secrets\glyphdeck_admin_password.txt -Value "your-admin-password" -NoNewline

# Validate and start.
docker compose -f compose.yaml config
docker compose -f compose.yaml up -d --wait
```

Open `http://127.0.0.1:8756`.

### Stopping

```powershell
docker compose -f compose.yaml down
```

### Persistence

SQLite app data is stored in the `glyphdeck-app-data` named volume. The
database survives container recreation and image rebuilds.

Back up the volume:

```powershell
docker run --rm -v glyphdeck-app-data:/data -v ${PWD}:/backup alpine tar czf /backup/glyphdeck-data-backup.tar.gz -C /data .
```

### OpenCode targets

The container does **not** bundle OpenCode. Use a Manual URL or SSH Alias
target to connect to a running OpenCode server on another machine.

SSH targets from Docker require an explicit read-only mount of your SSH
configuration and key material:

```powershell
docker compose -f compose.yaml run --rm `
  -v "$env:USERPROFILE\.ssh:/home/glyphdeck/.ssh:ro" `
  glyphdeck
```

This mount is **not enabled by default**.

### Healthcheck

The container runs a healthcheck against `http://localhost:8756/healthz`.
Healthy status is required before the `--wait` flag returns.

### Security controls

- Non-root container user (`glyphdeck`).
- All Linux capabilities dropped (`cap_drop: ALL`).
- `no-new-privileges:true`.
- Loopback-only host publication (`127.0.0.1:8756`).
- No Docker socket mount.
- No privileged mode.
- Read-only root filesystem (with `tmpfs` for `/tmp`).

### Preview limitations

- Not an isolated multi-user deployment.
- No built-in TLS — add a reverse proxy if remote access is needed.
- SSH targets require explicit, user-controlled key mounts.
- OpenCode is always external.
