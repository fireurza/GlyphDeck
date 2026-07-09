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

## Milestone 4 — EventBridge Streaming

Milestone 4 adds live streaming from OpenCode to the browser. The backend connects to OpenCode's `/event` SSE stream as a long-lived connection, normalizes events, and fans them out to browser clients over `GET /api/events` (also SSE). The frontend connects as soon as a project is selected (independent of session selection) and shows connection status (`Live` / `Reconnecting` / `Offline` / `Error`) in the top bar. The session transcript updates directly from streamed `message.part.updated` / `message.part.delta` events — the non-streaming fetch from Milestone 3 remains only as a reconciliation fallback, not as the primary path.

## Milestone 5 — Agent Terminal

Milestone 5 adds a read-only Agent Terminal view in the bottom panel, populated live from the same Milestone 4 event stream, plus a fix for a layout defect carried forward from Milestone 4.

**Stop Server / session list layout fix:** with many sessions in a project, a flex-layout bug could collapse the project card's controls (including Stop Server) to a 0px box while their content still rendered visually — placing the session list on top of the Stop Server button and making it unclickable without `{ force: true }`. Fixed by preventing the project list from being flex-shrunk and giving the session list its own bounded, independently-scrolling region.

**Agent Terminal behavior:**

- Read-only. There is no interactive input, no shell execution, and no command entry — it only displays activity already happening in the active OpenCode session.
- Populated entirely from the existing browser event stream (`GET /api/events`); no new backend endpoint was added. All the event types the Agent Terminal needs (`session.updated`, `message.updated`, `message.part.updated`, `message.part.delta`, `session.next.step.*`, `session.next.tool.*`, `session.next.shell.*`, `permission.*`, plus the `glyphdeck.eventstream.*` connection signals) already flow through that stream with a `sessionID` GlyphDeck can filter on. Adding a second backend log/endpoint would have duplicated state the browser already has live — so it was skipped.
- The event log is bounded in the browser (last 300 rows) and resets when the selected project/session changes, so it never shows another session's activity and never grows unbounded.
- High-volume text-streaming events (`message.part.delta`, `message.part.updated`) collapse into a single updating row per message instead of adding a row per token/chunk.
- Basic category filters (All / Tool / Shell / Message / Permission / System) and a Clear button are available.
- **User PTY terminal is not implemented yet.** The bottom panel's "Terminal" tab remains a placeholder; interactive shell access is out of scope until a later milestone.

## Milestone 6 — Usage Tab

Milestone 6 adds a functional Usage tab in the right panel. Usage data is aggregated from OpenCode assistant messages by the backend and served through a dedicated endpoint.

**Usage data source:**

- `GET /session/{id}/message` — the backend walks assistant messages in reverse to find the last one with token/cost data.
- Assistant messages carry `info.providerID`, `info.modelID`, `info.agent`, `info.mode`, `info.cost`, and `info.tokens` (total, input, output, reasoning, cache read/write).

**Usage endpoint:**

```
GET /api/projects/{projectId}/sessions/{sessionId}/usage
```

Response shape matches the OpenCode assistant message info fields:
```json
{
  "providerID": "deepseek",
  "modelID": "deepseek-v4-pro",
  "agent": "build",
  "mode": "build",
  "cost": 0.009337275,
  "tokens": {
    "total": 23329,
    "input": 21369,
    "output": 8,
    "reasoning": 32,
    "cache": { "read": 1920, "write": 0 }
  },
  "messageCount": 2
}
```

**Usage tab behavior:**

- Shows empty state when no session is selected.
- Shows loading state while fetching.
- Shows error state if usage fetch fails.
- Displays provider, model, agent, mode, token breakdown, cost, and message count.
- Missing fields render as an em-dash (`—`).
- Manual Refresh button available.
- Cost is shown as USD with up to 6 decimal places for small values.
- Reasoning tokens row is only shown when non-zero.

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
- `GET /api/projects/{projectId}/sessions/{sessionId}/usage` — get aggregated token usage and cost for a session.

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
13. Open the **Agent Terminal** tab in the bottom panel and confirm live event rows appear for the active session (session/message updates, streamed text, tool/shell events if the prompt triggered any).
14. Click Clear in the Agent Terminal and confirm the visible rows are removed.
15. Open the **Usage** tab in the right panel and confirm provider/model/token/cost data appears.
16. Click Refresh in the Usage tab and confirm the panel does not error.
17. Click Stop Server (no force-click needed).
18. Confirm server stops.

## Known Limitations

- User-controlled terminal (PTY + xterm.js) is not implemented. The bottom panel's Terminal tab is a placeholder.
- Agent Terminal shows only live activity from the moment it starts listening; it does not backfill history from before a session was selected.
- Usage tab shows data from the latest assistant message only; it does not show per-message or cumulative session totals.
- Usage data availability depends on OpenCode's assistant message shape. If OpenCode omits token/cost fields, the backend returns what it can and the UI shows em-dashes for missing values.
- Permissions approval popup and Review tab data are not implemented yet (later milestones).

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
