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
