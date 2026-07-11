# GlyphDeck Validation

Controlled, deterministic, repo-local validation for GlyphDeck. Runs hidden
start/stop/smoke flows without global process kills, visible editor windows, or
fragile browser selectors.

## v0.1.0 Smoke

| Script | Purpose |
|---|---|
| `start-dev-mvp.ps1` / `stop-dev-mvp.ps1` | Isolated v0.1.0 release-binary lifecycle |
| `run-mvp-smoke.ps1` | v0.1.0 release-candidate smoke: start, browser checks, screenshots, teardown |

The release binary embeds `web/dist`, so build the frontend before every Go
compile or test that imports the `web` package.

```powershell
.\scripts\build.ps1
.\scripts\validation\run-mvp-smoke.ps1
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
- No visible editor, Explorer, cmd, or other interactive windows. The runner
  monitors for forbidden visible windows (cmd, pwsh, notepad, explorer, etc.)
  during smoke and fails if any appear.
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
