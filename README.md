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

**Note:** OpenCode integration begins in later milestones. Milestone 0 is purely a scaffold with placeholder panels.
