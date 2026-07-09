# GlyphDeck

GlyphDeck is a local-first web workspace for managing projects, detecting OpenCode, running per-project OpenCode servers, streaming transcripts, reviewing changes, tracking usage, handling permissions, and using an interactive terminal — all from a browser UI.

## POC Capabilities (M0–M10)

| Milestone | Feature | Status |
|---|---|---|
| M0 | Repo bootstrap (Go backend + React/Vite shell) | Accepted |
| M1 | Project registry (add/list/delete, JSON persistence, Git detection) | Accepted |
| M2 | OpenCode server manager (detect/start/stop, health check, port allocation) | Accepted |
| M3 | Sessions and prompt loop (create/list, send prompt, view transcript) | Accepted |
| M3.5 | Validation harness hardening (data-testid, dev endpoints, controlled scripts) | Accepted |
| M4 | EventBridge streaming (SSE from OpenCode to browser, live transcript) | Accepted |
| M5 | Agent Terminal (read-only tool/event history, category filters) | Accepted |
| M6 | Usage tab (token/cost aggregation, available/unavailable states) | Accepted |
| M7 | Review tab (project/Git/session/activity summary) | Accepted |
| M8 | Permissions (approval popup with once/always/reject) | Accepted |
| M9 | User Terminal (interactive shell in project cwd) | Accepted |
| M10 | POC hardening (browser refresh, problems tab, graceful shutdown, docs) | Accepted |

## Prerequisites

- [Go](https://go.dev/dl/) 1.23+
- [Node.js](https://nodejs.org/) 20+ with npm
- [OpenCode](https://opencode.ai) CLI (`opencode` on PATH)
- [Git](https://git-scm.com/)
- [PowerShell 7](https://github.com/PowerShell/PowerShell) (for validation scripts on Windows)

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

## Validation

### Quick validation

```powershell
go test ./...
cd web && npm run build
```

### Full POC smoke test (M10)

```powershell
pwsh -NoProfile -ExecutionPolicy Bypass -File .\scripts\validation\run-m10-smoke.ps1
```

All validation artifacts are stored under `.glyphdeck/validation/<milestone>/` (git-ignored):

```
.glyphdeck/validation/m10/
├── logs/
├── screenshots/
├── scripts/
├── pids/
└── workspace/
```

## API Reference

### Project Registry

- `GET /api/projects` — list registered projects.
- `POST /api/projects` — add a local project path.
- `GET /api/projects/{projectId}` — get one registered project.
- `DELETE /api/projects/{projectId}` — remove one registered project.

### OpenCode Server

- `GET /api/opencode` — detect OpenCode CLI and version.
- `GET /api/projects/{projectId}/server` — get server status.
- `POST /api/projects/{projectId}/server/start` — start an OpenCode server.
- `POST /api/projects/{projectId}/server/stop` — stop an OpenCode server.

### Sessions

- `GET /api/projects/{projectId}/sessions` — list OpenCode sessions.
- `POST /api/projects/{projectId}/sessions` — create a new session.
- `GET /api/projects/{projectId}/sessions/{sessionId}` — get session details.
- `GET /api/projects/{projectId}/sessions/{sessionId}/messages` — list messages.
- `POST /api/projects/{projectId}/sessions/{sessionId}/prompt` — send a prompt.

### Usage

- `GET /api/projects/{projectId}/sessions/{sessionId}/usage` — aggregated token usage and cost.

### Review

- `GET /api/projects/{projectId}/sessions/{sessionId}/review` — project/Git/session/activity summary.

### Permissions

- `GET /api/permissions?projectId={id}` — list pending permission requests.
- `POST /api/permissions/{requestId}/reply?projectId={id}` — reply once/always/reject.

### User Terminal

- `POST /api/projects/{projectId}/terminals` — start a terminal.
- `GET /api/terminals/{terminalId}/stream` — SSE output stream.
- `POST /api/terminals/{terminalId}/input` — send input.
- `POST /api/terminals/{terminalId}/resize` — resize (no-op on Windows pipes).
- `POST /api/terminals/{terminalId}/close` — close terminal.
- `GET /api/terminals/{terminalId}/status` — get terminal status.

### Problems

- `GET /api/problems` — list app-level problems.
- `POST /api/problems/clear` — clear all problems.

### Events

- `GET /api/events` — SSE event stream from OpenCode to browser.

### Dev/Test (requires `GLYPHDECK_DEV_TOOLS=1`)

- `POST /api/dev/reset-validation-state` — reset validation state.
- `POST /api/dev/stop-all-app-owned-servers` — stop app-owned servers.

## Manual Smoke Test

Shell: PowerShell 7
Working directory: project root

1. Start backend: `go run ./cmd/glyphdeck`
2. Start frontend in second terminal: `cd web && npm run dev`
3. Add the current GlyphDeck repo path in the left Projects panel.
4. Confirm the project appears with Git repo status and branch.
5. Confirm OpenCode detection banner shows ready with version.
6. Click Start Server.
7. Confirm server reaches ready with port and version displayed.
8. Click the project to select it (sessions list appears, event stream connects).
9. Click Create Session.
10. Click the new session to open it in the center panel.
11. Type a prompt (e.g., "List the validation commands from README.") and click Send.
12. Confirm the assistant response appears in the transcript.
13. Open the Agent Terminal tab and confirm live event rows appear.
14. Open the Usage tab and confirm provider/model/token/cost data appears.
15. Open the Review tab and confirm project/Git/session/activity data appears.
16. Force a bash permission rule or use your project config to trigger permission popup; confirm popup appears and can be approved.
17. Open the Terminal tab, click Start Terminal, send commands, confirm output.
18. Click Stop Server (no force-click needed).
19. Confirm server stops.
20. Open the Problems tab and confirm "No problems detected." is shown.
21. Refresh the browser and confirm the selected project is restored.

## Known Limitations

- **No auth.** GlyphDeck binds to 127.0.0.1 only. Do not expose to public networks.
- **No SQLite.** Project registry is stored as JSON under `.glyphdeck/projects.json`.
- **No LAN/Tailscale binding.** Only localhost access.
- **No installer.** Run via `go run` and `npm run dev`.
- **Terminal is pipe-based on Windows.** True PTY (ConPTY) is blocked with current Go libraries. The terminal uses `os/exec` with pipes — interactive shell works but no TTY resize, no signals.
- **Agent Terminal shows only live activity.** Does not backfill history from before session selection.
- **Usage tab shows latest assistant message only.** Not per-message or cumulative totals.
- **Review tab uses local `git` commands for file status.** No OpenCode VCS API integration yet.
- **Permissions polling is interval-based (2s).** SSE events for live permission updates are available but not yet wired to dismiss popups automatically.
- **Problems tab tracks up to 100 app-level issues.** Older problems are evicted.

## Troubleshooting

### OpenCode not detected

```powershell
opencode --version
```

If not found, ensure OpenCode is installed and on PATH.

### Server stuck

```powershell
# Kill stuck backend on port 8756
$p = Get-NetTCPConnection -LocalPort 8756 -State Listen | Select-Object -First 1 -ExpandProperty OwningProcess
Stop-Process -Id $p -Force

# Kill stuck frontend on port 5173
$p = Get-NetTCPConnection -LocalPort 5173 -State Listen | Select-Object -First 1 -ExpandProperty OwningProcess
Stop-Process -Id $p -Force
```

### Event stream offline

1. Ensure the OpenCode server is running (check server status in left panel).
2. Ensure `OPENCODE_SERVER_PASSWORD` is set in the environment.
3. Check the Problems tab for any app-level errors.

### Terminal closed/detached after refresh

Terminal sessions are not reattachable after browser refresh. Start a new terminal.

### Validation artifacts

All validation logs, screenshots, PIDs, and workspaces are stored under `.glyphdeck/validation/<milestone>/`. This directory is git-ignored.

## Logs

Backend logs are written to stderr (visible in the terminal running `go run ./cmd/glyphdeck`). Key operations logged:

- Server startup/shutdown
- OpenCode server start/stop
- Terminal start/close
- Permission replies
- App-level problems

Validation harness logs are stored under `.glyphdeck/validation/<milestone>/logs/`.
