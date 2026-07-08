# GlyphDeck

GlyphDeck is a local web workspace for managing projects, detecting OpenCode, and running per-project OpenCode servers.

## Milestone 1 — Project Registry

Milestone 1 adds a local project registry backed by a JSON file. The left Projects panel can add a local directory, show whether it is a Git repository, show the current branch when available, persist the entry across backend restarts, and remove the entry.

Registry data is stored at:

```text
.glyphdeck/projects.json
```

`.glyphdeck/` is local development data and is ignored by Git.

## Milestone 2 — OpenCode Server Manager

Milestone 2 adds OpenCode CLI detection and per-project server lifecycle management. Each registered project can start and stop an `opencode serve` instance bound to a dynamic loopback port.

Server state is tracked in memory (not persisted across GlyphDeck restarts).

## Milestone 3 — Sessions and Prompt Loop

Milestone 3 adds OpenCode session management and a non-streaming prompt loop. Registered projects with a ready OpenCode server can create sessions, send prompts, and view assistant responses in the center panel.

Streaming (SSE/EventBridge) starts in Milestone 4. This milestone uses request/response only.

## Prerequisites

- [Go](https://go.dev/dl/) (1.23+)
- [Node.js](https://nodejs.org/) (20+) with npm
- [OpenCode](https://opencode.ai) CLI (`opencode` on PATH) for server management

OpenCode server communication uses HTTP Basic Auth credentials from environment variables:

- `OPENCODE_SERVER_USERNAME` (default: `opencode`)
- `OPENCODE_SERVER_PASSWORD` (set by OpenCode Desktop)

These must be available in the environment where GlyphDeck's backend runs.

## Run Locally

Shell: PowerShell 7
Working directory: project root

**Backend:**

```powershell
go run ./cmd/glyphdeck
```

Starts on `http://127.0.0.1:8756`. Health check at `/healthz`.

**Frontend:**

```powershell
cd web && npm install
cd web && npm run dev
```

Starts Vite dev server (default: `http://localhost:5173`).

## Project Registry API

- `GET /api/projects` — list registered projects.
- `POST /api/projects` — add a local project path.
- `GET /api/projects/{projectId}` — get one registered project.
- `DELETE /api/projects/{projectId}` — remove one registered project.

## OpenCode Server API

- `GET /api/opencode` — detect OpenCode CLI and version.
- `GET /api/projects/{projectId}/server` — get server status for a project.
- `POST /api/projects/{projectId}/server/start` — start an OpenCode server.
- `POST /api/projects/{projectId}/server/stop` — stop an OpenCode server.

OpenCode servers bind to `127.0.0.1` only. Ports are allocated dynamically. Health checks use OpenCode's `/global/health` endpoint.

## Session API

- `GET /api/projects/{projectId}/sessions` — list OpenCode sessions for a project.
- `POST /api/projects/{projectId}/sessions` — create a new OpenCode session.
- `GET /api/projects/{projectId}/sessions/{sessionId}` — get session details.
- `GET /api/projects/{projectId}/sessions/{sessionId}/messages` — list messages in a session.
- `POST /api/projects/{projectId}/sessions/{sessionId}/prompt` — send a non-streaming prompt and receive the response.

The project's OpenCode server must be in `ready` state. Session/message data is sourced from the OpenCode server, not persisted in GlyphDeck.

## Manual Smoke Test

Shell: PowerShell 7
Working directory: project root

1. Start backend:

   ```powershell
   go run ./cmd/glyphdeck
   ```

2. Start frontend in a second terminal:

   ```powershell
   cd web && npm run dev
   ```

3. Add the current GlyphDeck repo path in the left Projects panel.
4. Confirm the project appears with Git repo status and branch.
5. Confirm OpenCode detection banner shows ready with version.
6. Click Start Server.
7. Confirm server reaches ready with port and version displayed.
8. Click the project to select it (sessions list appears).
9. Click Create Session.
10. Click the new session to open it in the center panel.
11. Type a prompt (e.g., "List the validation commands from README.") and click Send.
12. Confirm the assistant response appears in the transcript.
13. Click Stop Server.
14. Confirm server stops.

## Validation Commands

Shell: PowerShell 7
Working directory: project root

```powershell
go test ./...
cd web && npm run build
go run ./cmd/glyphdeck
cd web && npm run dev
```

## Stopping Dev Servers

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

# Kill stuck OpenCode processes (if left running by GlyphDeck)
Get-Process opencode -ErrorAction SilentlyContinue | Stop-Process -Force
```

**Linux/macOS:**

```bash
# Backend
lsof -ti:8756 | xargs kill -9

# Frontend
lsof -ti:5173 | xargs kill -9
```
