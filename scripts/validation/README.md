# GlyphDeck Validation Harness

Purpose: Controlled, deterministic, repo-local validation harness for GlyphDeck. Runs start/stop/smoke flows without visible windows, global process kills, or fragile selectors.

## Scripts

| Script | Purpose |
|---|---|
| `start-dev.ps1` | Start Go backend and Vite frontend as hidden PS background jobs |
| `stop-dev.ps1` | Stop dev servers using only recorded PIDs; never kills by name |
| `run-m3-smoke.ps1` | Full Milestone 3 smoke test: start, reset state, Playwright run, stop |

## Usage

```powershell
# Start dev servers
.\scripts\validation\start-dev.ps1

# Run M3 smoke test (starts servers, runs Playwright, stops servers)
.\scripts\validation\run-m3-smoke.ps1

# Stop dev servers (PID-based cleanup only)
.\scripts\validation\stop-dev.ps1
```

## Artifact Layout

All artifacts live under `.glyphdeck/validation/m3_5/` (gitignored):

```
.glyphdeck/validation/m3_5/
├── logs/
│   ├── backend.log
│   └── frontend.log
├── screenshots/
│   ├── 01-clean-state.png
│   ├── 02-project-added.png
│   ├── 03-server-ready.png
│   ├── 04-session-created.png
│   ├── 05-prompt-sent.png
│   ├── 06-assistant-response-visible.png
│   ├── 07-server-stopped.png
│   └── 08-full-layout.png
├── scripts/
│   └── smoke-test.cjs
└── pids/
    ├── backend.pid
    └── frontend.pid
```

## Safety Rules (Non-Negotiable)

- **PID-based cleanup only** — `stop-dev.ps1` reads `.pid` files and stops only those exact PIDs.
- **Never** `Get-Process -Name` or `Stop-Process -Name`.
- **Never** `taskkill /IM`.
- **Never** open visible windows (Notepad, Explorer, VS Code, cmd.exe).
- **Never** kill OpenCode Desktop or global processes.
- **Never** write artifacts to `%TEMP%` or outside `.glyphdeck/validation/`.
- **Playwright selectors** — `data-testid` only; no `text=`, `has-text`, `.first()`, `.nth()`, or CSS-only selectors.

## Dev/Test Endpoints

When `GLYPHDECK_DEV_TOOLS=1` is set:

- `POST /api/dev/reset-validation-state` — resets validation state for a clean smoke test.
- `POST /api/dev/stop-all-app-owned-servers` — stops only GlyphDeck-tracked server processes.

These endpoints are guarded; they do not exist when `GLYPHDECK_DEV_TOOLS` is unset.

## Prerequisites

- Go 1.23+
- Node.js 20+ with npm
- Playwright (`npm install playwright` in the repo or globally)
- PowerShell 7
- OpenCode CLI on PATH (for server management)
