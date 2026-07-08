# GlyphDeck Agent Validation Rules

## Status

These rules are mandatory for all GlyphDeck agents, subagents, testers, reviewers, and orchestrators.

Failure to follow this document invalidates the validation run.

---

## Purpose

GlyphDeck validation must be deterministic, repo-local, repeatable, and safe for the host machine.

Agents must not improvise host process cleanup, artifact locations, UI selectors, or browser validation behavior.

---

## Non-Negotiable Rules

### Forbidden Host Actions

Agents must never run or trigger any of these actions:

```powershell
Get-Process -Name opencode | Stop-Process
taskkill /IM opencode.exe
taskkill /F /IM opencode.exe
Stop-Process -Name opencode
Stop-Process -Name OpenCode
```

Agents must never globally kill these process names:

```text
opencode
OpenCode
node
npm
go
pwsh
powershell
cmd
```

Agents must never use global process cleanup by name.

Agents must never stop OpenCode Desktop.

Agents must never close the user's active OpenCode Desktop session.

Agents must never assume any OpenCode process is safe to kill unless GlyphDeck started it and tracked its exact PID.

---

## Forbidden UI/Window Actions

Agents must never open visible host applications during validation.

Forbidden:

```text
Notepad
Explorer
VS Code
visible cmd.exe windows
visible PowerShell windows
visible Windows Terminal windows
browser windows outside controlled Playwright sessions
```

Forbidden commands and patterns:

```powershell
Invoke-Item
ii
start
Start-Process without controlled hidden/background handling
cmd /c start
explorer.exe
notepad.exe
code
code-insiders
npx when it opens npx.ps1 or any script in an editor
```

If a command opens a visible app, stop validation immediately and report the violation.

---

## Artifact Location Rules

All validation artifacts must be repo-local.

Allowed path pattern:

```text
.glyphdeck/validation/<milestone>/
```

Required subdirectories:

```text
.glyphdeck/validation/<milestone>/logs/
.glyphdeck/validation/<milestone>/screenshots/
.glyphdeck/validation/<milestone>/scripts/
.glyphdeck/validation/<milestone>/pids/
```

Forbidden artifact locations:

```text
%TEMP%
AppData
Program Files
Desktop
Downloads
Documents outside the repo
repo root temporary loose files
web/node_modules generated helper scripts
system directories
```

Never commit `.glyphdeck/`.

`.glyphdeck/` must remain ignored by `.gitignore`.

---

## Process Control Rules

Validation harnesses must record every process they start.

Required PID storage:

```text
.glyphdeck/validation/<milestone>/pids/backend.pid
.glyphdeck/validation/<milestone>/pids/frontend.pid
.glyphdeck/validation/<milestone>/pids/playwright.pid
```

Only these PIDs may be stopped by validation cleanup.

Do not stop processes by name.

Do not stop processes that are not recorded by the current validation harness.

Do not reuse stale PID files without confirming the PID belongs to the expected command and current run.

---

## GlyphDeck-Owned OpenCode Process Rules

GlyphDeck may start OpenCode server processes during Milestone 2+ validation.

OpenCode cleanup rules:

1. Prefer the GlyphDeck API:
   ```http
   POST /api/projects/{projectId}/server/stop
   ```

2. If forced cleanup is required, use only the exact PID returned by:
   ```http
   GET /api/projects/{projectId}/server
   ```

3. Never kill OpenCode globally.

4. Never kill OpenCode Desktop.

5. Never assume an OpenCode PID is app-owned unless GlyphDeck returned it from server state.

---

## Host vs Sandbox Execution Rules

OpenCode Desktop stays on Local Server.

Docker Sandbox is command execution only.

Do not switch OpenCode Desktop to Docker Sandbox.

Do not use:

```powershell
sbx run glyphdeck-sbx
```

Do not publish sandbox ports unless a milestone explicitly requires it.

Use this command pattern for sandbox validation commands:

```powershell
sbx exec -w /c/Users/Fireurza/Documents/Code/GlyphDeck glyphdeck-sbx bash -lc "<command>"
```

Sandbox path:

```text
/c/Users/Fireurza/Documents/Code/GlyphDeck
```

Host path:

```text
C:\Users\Fireurza\Documents\Code\GlyphDeck
```

Never mix host paths and sandbox paths in one runtime.

If the GlyphDeck backend is running on Windows host, project paths must be Windows paths.

If the GlyphDeck backend is running inside Docker Sandbox, project paths must be sandbox paths.

---

## Required Validation Evidence

A validation pass is not accepted unless it has all required evidence.

Required evidence:

```text
1. Exact commands run
2. Exit status for each command
3. Logs under .glyphdeck/validation/<milestone>/logs/
4. Screenshots under .glyphdeck/validation/<milestone>/screenshots/
5. PID files under .glyphdeck/validation/<milestone>/pids/
6. Clean process teardown using recorded PIDs or GlyphDeck stop APIs
7. git status --short output
```

Do not claim PASS from implied state.

Do not claim PASS from stale screenshots.

Do not claim PASS because a selector existed.

Do not claim PASS because a prior session already had messages.

Do not claim PASS because an assistant message exists unless it belongs to the fresh validation session.

---

## Screenshot Rules

Screenshots must be captured only after the real state is reached.

Each screenshot filename must describe the state:

```text
01-clean-state.png
02-project-added.png
03-server-ready.png
04-session-created.png
05-prompt-sent.png
06-assistant-response-visible.png
07-server-stopped.png
08-full-layout.png
```

Screenshots must be stored only under:

```text
.glyphdeck/validation/<milestone>/screenshots/
```

Before capture, the validation script must confirm the expected state by API or stable `data-testid`.

Do not reuse screenshots from previous runs.

Delete stale screenshots before a new run.

---

## Playwright Rules

Playwright must use stable `data-testid` selectors only.

Do not use fragile selectors like:

```text
text=
button:has-text()
.first()
.nth()
CSS classes meant for styling
visible text from old sessions
```

Allowed selector pattern:

```javascript
page.getByTestId("project-start-server-button")
```

Every tested UI element must have a stable `data-testid`.

If a required `data-testid` is missing, stop and fix the UI test selectors first.

Do not continue with fragile selectors.

---

## Fresh-State Rules

Validation must start from a clean validation state.

A passing M3 prompt-loop test must prove:

```text
1. One fresh validation project exists.
2. One fresh validation OpenCode server is started.
3. One fresh validation session is created.
4. The fresh session is selected by ID.
5. One fresh prompt is sent to that session.
6. A new user message appears in that session.
7. A new assistant message appears in that same session after prompt submission.
8. The assistant message is not from a previous session.
```

A result like `ASSISTANT at 0s` is not valid proof unless the script also verifies:

```text
sessionId matches fresh session
message timestamp is after prompt send
message role is assistant
message count increased after prompt send
```

---

## Dev/Test Endpoint Rules

Dev/test endpoints may exist only when explicitly enabled:

```text
GLYPHDECK_DEV_TOOLS=1
```

Required behavior:

```text
GLYPHDECK_DEV_TOOLS unset or false:
  /api/dev/* endpoints must not exist

GLYPHDECK_DEV_TOOLS=1:
  /api/dev/* endpoints may be available for validation
```

Dev/test endpoints must never delete user source files.

Dev/test endpoints must never kill global processes.

Dev/test endpoints may only affect GlyphDeck-owned state and app-owned server processes.

---

## Build and Test Rules

Backend validation:

```powershell
sbx exec -w /c/Users/Fireurza/Documents/Code/GlyphDeck glyphdeck-sbx bash -lc "go test ./... -count=1"
sbx exec -w /c/Users/Fireurza/Documents/Code/GlyphDeck glyphdeck-sbx bash -lc "go vet ./cmd/... ./internal/..."
```

Frontend validation must not corrupt host `node_modules`.

If sandbox executable-bit issues occur, use a documented safe command. Example:

```bash
cd web
chmod +x node_modules/.bin/tsc* node_modules/.bin/vite* 2>/dev/null || true
node node_modules/typescript/bin/tsc -b
node_modules/.bin/vite build
```

Do not use a command path that opens `npx.ps1` in Notepad.

---

## Failure Behavior

On any validation failure:

1. Stop.
2. Preserve logs under `.glyphdeck/validation/<milestone>/logs/`.
3. Capture a failure screenshot if browser state is relevant.
4. Stop only recorded PIDs.
5. Report the exact failed command.
6. Report the exact failing selector/API/state.
7. Do not claim milestone completion.
8. Do not commit.

---

## Commit Gate

Do not commit unless all are true:

```text
go test passes
go vet passes
frontend build passes
required browser validation passes
required screenshots exist and are fresh
vision review passes
code review passes
no visible terminal windows opened
no Notepad/Explorer/VS Code opened
OpenCode Desktop was not killed
only recorded PIDs were stopped
git status was reviewed
```

Commit command must be explicit in the milestone prompt.

---

## Validation Summary Format

Every validation report must use this format:

```text
Milestone:
Commit candidate:
Backend tests:
Go vet:
Frontend build:
Browser validation:
Screenshots:
Vision review:
Code review:
Known issues:
Process cleanup:
Git status:
Decision: PASS | FAIL
```

If any line is incomplete, the decision is FAIL.