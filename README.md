# GlyphDeck

GlyphDeck is a local web-based workspace for managing OpenCode AI coding sessions. It provides a multi-panel UI to review agent activity, monitor token usage, manage task boards, and orchestrate terminal sessions — all running on your machine.

## Milestone 0 — Project Scaffold

This milestone establishes the project skeleton with a minimal Go backend and a React + TypeScript + Vite frontend shell layout. No OpenCode integration, no sessions, no database, no auth.

### Non-goals for Milestone 0

- OpenCode integration / detection / server manager
- Sessions, prompt sending, SSE/EventBridge
- WebSocket events, PTY terminal
- SQLite persistence, auth, Tailscale/LAN binding
- Team mode, worktrees
- MCP/plugin/skills editors, built-in editor
- Installer, Docker hosting, desktop wrapper

### Prerequisites

- [Go](https://go.dev/dl/) (1.23+)
- [Node.js](https://nodejs.org/) (20+) with npm

### Quick Start

**Backend:**

```bash
go run ./cmd/glyphdeck
```

Starts on `http://127.0.0.1:8756`. Health check at `/healthz`.

**Frontend:**

```bash
cd web
npm install
npm run dev
```

Starts Vite dev server (default: `http://localhost:5173`).

### Validation Commands

```bash
# Go
go test ./...
go run ./cmd/glyphdeck

# Frontend
cd web && npm install
cd web && npm run build
cd web && npm run dev
```

### Stopping Dev Servers

**Normal stop:**

- Press `Ctrl+C` in the terminal running `go run ./cmd/glyphdeck` to stop the backend.
- Press `Ctrl+C` in the terminal running `npm run dev` to stop the frontend.

**Kill stuck port (Windows):**

If a server process is left running and the port is stuck:

```powershell
# Kill stuck backend on port 8756
$glyphdeckPortProcess = Get-NetTCPConnection -LocalPort 8756 -State Listen | Select-Object -First 1 -ExpandProperty OwningProcess
Stop-Process -Id $glyphdeckPortProcess -Force

# Kill stuck frontend on port 5173 (adjust port if Vite uses another)
$vitePortProcess = Get-NetTCPConnection -LocalPort 5173 -State Listen | Select-Object -First 1 -ExpandProperty OwningProcess
Stop-Process -Id $vitePortProcess -Force
```

**Linux/macOS:**

```bash
# Backend
lsof -ti:8756 | xargs kill -9

# Frontend
lsof -ti:5173 | xargs kill -9
```

**Note:** OpenCode integration begins in later milestones. Milestone 0 is purely a scaffold with placeholder panels.
