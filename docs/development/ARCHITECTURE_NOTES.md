# Architecture Notes

## Package Layout

```
cmd/glyphdeck      — composition root, HTTP server, adapters
internal/
  app/             — (reserved)
  auth/            — admin authentication, bcrypt, sessions, middleware
  config/          — (reserved)
  devtools/        — dev-only endpoints (GLYPHDECK_DEV_TOOLS=1)
  events/          — SSE hub, OpenCode → browser event fanout
  httpapi/         — shared HTTP helpers (JSON, errors, origin guard)
  lifecycle/       — process tree management (Job Objects on Windows)
  opencode/        — OpenCode HTTP/SSE client, types, events
  permissions/     — OpenCode permission approval
  problems/        — problem/error aggregation panel
  projects/        — project registry (SQLite-backed)
  review/          — per-project review aggregation
  sandboxes/       — server/sandbox configs, remote SSH lifecycle
  servers/         — OpenCode server lifecycle (start/stop/detect)
  sessions/        — OpenCode session management
  settings/        — application settings (SQLite-backed)
  storage/         — SQLite database (modernc.org/sqlite)
  terminal/        — interactive shell sessions (ConPTY on Windows)
  usage/           — token/cost usage aggregation
web/
  src/
    api/           — frontend API clients
    layout/        — React layout components
    styles/        — CSS
    types/         — TypeScript type definitions
```

## Dependency Graph

```
cmd/glyphdeck (root)
  ├── auth
  ├── devtools
  ├── events → opencode
  ├── httpapi
  ├── lifecycle
  ├── opencode
  ├── permissions → opencode, httpapi
  ├── problems → httpapi
  ├── projects → storage, httpapi
  ├── review → opencode, projects, httpapi
  ├── sandboxes → sql
  ├── servers → lifecycle, opencode, httpapi
  ├── sessions → opencode, httpapi
  ├── settings → httpapi
  ├── storage → modernc.org/sqlite
  ├── terminal → lifecycle, httpapi
  ├── usage → opencode, httpapi
  └── web (embedded React/Vite build)
```

The graph is a strict DAG. No circular dependencies. `httpapi` and `opencode`
are the core leaf abstractions (fan-in 10 and 6 respectively).

## Key Design Decisions

- **SQLite** via `modernc.org/sqlite` (pure Go, no CGO).
- **ConPTY** on Windows via `golang.org/x/sys/windows` (`CreatePseudoConsole`).
- **Job Objects** on Windows for process tree lifecycle.
- **HttpOnly cookies** for session management (SameSite=Lax).
- **bcrypt** for admin password hashing.
- **React + Vite + TypeScript** for the frontend.
- **Playwright** for browser-based smoke tests.
- **Embedded frontend** via `embed.FS` in release builds.
- **Remote targets** use persisted `sandboxes.ServerConfig` records for local,
  manual URL, and SSH-alias targets. The SSH runner passes aliases as arguments
  to the system `ssh` client; credentials and private keys are never persisted.
- **Remote lifecycle ownership** records PID, URL, status, and ownership metadata.
  Start captures a PID, detach only clears active-target selection, and stop
  verifies the expected remote process before acting on the exact recorded PID.
- **Container mode** (`GLYPHDECK_CONTAINER_MODE=1`) allows the server to bind
  `0.0.0.0` internally for Docker networking. Host publication remains
  loopback-only via the Compose port mapping. Outside container mode, only
  loopback hosts are accepted.
- **Password file** (`GLYPHDECK_ADMIN_PASSWORD_FILE`) reads the admin password
  from a file instead of an environment variable. Both sources cannot be set
  simultaneously. The password value is never logged.

## Docker Compose Preview

The `compose.yaml` at the repository root defines a single-service stack:

- **glyphdeck** — the Go binary serving the embedded React frontend.
- **app-data** — a named Docker volume for the SQLite database.
- **glyphdeck_admin_password** — a Docker secret read from a local file.

Security controls:

| Control | Value |
|---------|-------|
| Runtime user | `glyphdeck` (non-root) |
| Capabilities | `cap_drop: ALL` |
| Privilege escalation | `no-new-privileges:true` |
| Host publication | `127.0.0.1` loopback only |
| Docker socket | Not mounted |
| Privileged mode | Not enabled |
| Root filesystem | Read-only with `tmpfs` for `/tmp` |
| Init | `init: true` (tini) |
| Restart | `unless-stopped` |

OpenCode is **not bundled** in the container. The image includes an SSH client
for remote SSH alias targets, but no SSH keys are mounted by default.
