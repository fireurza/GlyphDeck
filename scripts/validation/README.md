# GlyphDeck Validation Harness

Controlled, deterministic, repo-local validation for GlyphDeck. It runs hidden
start/stop/smoke flows without global process kills, visible editor windows, or
fragile browser selectors.

## Scripts

| Script | Purpose |
|---|---|
| `start-dev.ps1` / `stop-dev.ps1` | Legacy M3.5 development harness, tracked PID cleanup only |
| `run-m3-smoke.ps1` | Historical M3 smoke |
| `start-dev-mvp.ps1` / `stop-dev-mvp.ps1` | Isolated v0.1.0 release-binary lifecycle |
| `run-mvp-smoke.ps1` | v0.1.0 release-candidate smoke: start, browser checks, screenshots, teardown |

## v0.1.0 Usage

The release binary embeds `web/dist`, so build the frontend before every Go
compile or test that imports the `web` package.

```powershell
npm.cmd --prefix web run build
go test ./... -count=1
go vet ./cmd/... ./internal/... ./web
go build -o .\dist\glyphdeck.exe .\cmd\glyphdeck\
pwsh -NoProfile -ExecutionPolicy Bypass -File .\scripts\validation\run-mvp-smoke.ps1
```

The MVP runner launches the binary from `.glyphdeck/validation/mvp/launch/`,
not the repository root, to prove the frontend is embedded. Generated artifacts
remain under `.glyphdeck/validation/mvp/` (git-ignored):

```text
.glyphdeck/validation/mvp/
├── data/
├── launch/
├── logs/
├── pids/
├── screenshots/
├── scripts/
└── workspace/
```

## Safety Rules (Non-Negotiable)

- PID-based cleanup only. Teardown verifies the recorded binary path and dynamic
  port ownership before stopping a process.
- No process-name/global kills, `taskkill /IM`, or OpenCode Desktop changes.
- Recursive validation-data cleanup verifies lexical containment and rejects any
  reparse point/junction in the path ancestry before deletion.
- No visible editor, Explorer, cmd, or other interactive windows. The MVP runner
  snapshots existing Notepad PIDs solely to detect and close a Notepad process
  created during that run; it never touches pre-existing Notepad processes.
- Use `npm.cmd` in Windows scripts. Do not use `npx`, `npm exec`, npm.ps1, or
  npx.ps1.
- Browser automation uses stable `data-testid` selectors only; no text, CSS,
  positional, `.first()`, or `.nth()` selectors.
- Logs, screenshots, PID records, workspaces, and validation data stay under
  `.glyphdeck/validation/` and are never committed.

## Dev/Test Endpoints

When `GLYPHDECK_DEV_TOOLS=1` is set:

- `POST /api/dev/reset-validation-state` — reset validation state for a clean smoke.
- `POST /api/dev/stop-all-app-owned-servers` — stop only GlyphDeck-tracked servers.

These endpoints do not exist when `GLYPHDECK_DEV_TOOLS` is unset.

## Prerequisites

- Go 1.23+
- Node.js 20+ with npm and Playwright available to the repository
- PowerShell 7
- OpenCode CLI on PATH (for server-management checks)
