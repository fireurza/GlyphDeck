# GlyphDeck Milestone Status

## Status

This file is the compact milestone source of truth for agents.

Agents must read this file before starting or validating any milestone.

Agents must update this file after completing any accepted milestone or validation-hardening milestone.

---

## Current Project

```text
Product: GlyphDeck
Publisher: FireGlyph Studios
Repo: C:\Users\Fireurza\Documents\Code\GlyphDeck
Sandbox: glyphdeck-sbx
Sandbox workdir: /c/Users/Fireurza/Documents/Code/GlyphDeck
```

---

## Current Required Operating Model

```text
OpenCode Desktop: Local Server only
Docker Sandbox: sbx exec command execution only
Desktop Docker Sandbox mode: not used
```

---

## Current Accepted Milestones

| Milestone | Status | Commit | Notes |
|---|---:|---|---|
| Milestone 0 — Repo bootstrap | Accepted | c4a4853 | Go backend, React/Vite shell, baseline layout |
| Milestone 1 — Project registry | Accepted | e7c6ddc + validation cleanup | Project add/list/delete, JSON persistence, Git detection |
| Milestone 2 — OpenCode server manager | Accepted | f64f0a8 | OpenCode detection, start/stop server, ready/version UI |
| Milestone 3 — Sessions and prompt loop | Accepted | c8ba09d | M3 smoke re-validated in M3.5 harness |
| Milestone 3.5 — Validation Harness Hardening | Accepted | c8ba09d | data-testid selectors, dev/test endpoints, validation scripts, M3 smoke re-validated |
| Milestone 4 — EventBridge streaming | Accepted | 6fe2911 | Real OpenCode /event SSE parser fix; live streaming proven end-to-end (backend probe + browser smoke, exact streamed text); connected status real; fresh-session selection by id |
| Milestone 5 — Agent Terminal | Accepted | 732e22b | Fixed Stop Server/session-list flex-collapse overlap (root-caused via DOM measurement); read-only Agent Terminal fed live from existing M4 event stream, bounded 300-row client log, category filters, Clear; no new backend endpoint needed (documented in README) |
| Milestone 6 — Usage tab | Accepted | 9f1c259 | Backend usage aggregation with `available`/`reason` fields; unavailable state in frontend; recovery smoke scripts hardened with durability, isolated workspaces, fresh-session-by-ID selection |
| Milestone 7 — Review tab | Accepted | 8d3639b | Review tab with project/Git/session/activity summary; M7 smoke captured 14 fresh screenshots; Usage and Agent Terminal regressions verified |
| Milestone 8 — Permissions | Accepted | a004682 | Permission approval popup with once/always/reject; forced-bash rule triggers popup; dismiss and resume verified; M5/M6/M7 regressions clean |
| Milestone 9 — User Terminal | Accepted | 4473bab | Interactive shell via exec.Command pipes (PTY unsupported on Windows with current stack); SSE output streaming; HTTP POST input; start/run/close verified; layout preserved |

---

## Current Unaccepted / Incomplete Milestones

| Milestone | Status | Reason |
|---|---:|---|
---

## Current Next Step

The next milestone is:

```text
Milestone 10 — POC hardening pass
```

Milestone 9 accepted (User Terminal with interactive shell via exec.Command pipes).

The Stop Server/session-list overlap carried forward from Milestone 4 is
fixed and verified (root-caused via DOM measurement, re-verified in the M5
smoke with a normal, non-force click).

---

## Top-Right Version Label Rule

The top-right UI label must be updated every milestone.

Current expected label (Milestone 9 accepted; bump to Milestone 10 when M10 work begins):

```text
Milestone 9
```

Rules:

```text
- Do not leave stale labels.
- Do not show Milestone 1 after Milestone 2+.
- Do not show Milestone 2 after Milestone 3+.
- Prefer centralizing the label in one frontend constant.
- Validation must screenshot the label.
```

---

## Current Validation Problems To Fix

Milestone 3 validation exposed process problems that must be corrected before feature work continues.

Known violations:

```text
- Visible cmd.exe windows spawned.
- npx.ps1 opened in Notepad.
- Artifacts were written to %TEMP%.
- Stale screenshots were reused.
- OpenCode Desktop/global OpenCode processes were killed.
- Agent claimed success from unreliable signals.
- Playwright used fragile selectors.
- Existing sessions were clicked instead of fresh validation sessions.
- Stop Server button was blocked/intercepted by session items.
```

Milestone 3.5 must prevent these from recurring.

---

## Required Agent Docs

Agents must read these before implementation or validation:

```text
docs/agent/VALIDATION_RULES.md
docs/agent/OPENCODE_WORKFLOW.md
docs/agent/MILESTONE_STATUS.md
```

If any are missing, stop and ask for them.

---

## Source-of-Truth Project Docs

For every milestone, agents must also read the attached/project planning docs provided by the user:

```text
GlyphDeck Planning Notes
GlyphDeck POC Implementation Plan
GlyphDeck Roadmap
GlyphDeck Stack
GlyphDeck Technical Architecture
Baseline Layout
```

Agents must not rely on memory alone.

---

## Accepted Stack

```text
Backend: Go
Frontend: React + TypeScript + Vite
OpenCode integration: internal Go HTTP/SSE adapter
OpenCode event stream: later via Go SSE client to /event
Browser realtime: later via GlyphDeck WebSocket/SSE
Terminal: later Go PTY backend + xterm.js frontend
Storage: JSON for early spike config, SQLite for MVP
POC: go run + npm dev
MVP: Go binary serving embedded React assets
```

---

## Current Explicit Non-Goals

Do not implement these until their milestone:

```text
SQLite
Auth
Tailscale/LAN binding
Team mode
Worktrees
MCP/plugin/skills editors
Built-in editor
Installer
Docker hosting
Desktop wrapper
```

---

## Completion Criteria For Any Milestone

A milestone is accepted only when all are true:

```text
1. Scope matches milestone only.
2. go test passes.
3. go vet passes.
4. frontend build passes.
5. browser validation passes with fresh state.
6. screenshots are fresh and repo-local.
7. vision review passes.
8. code review passes.
9. no forbidden host actions occurred.
10. no OpenCode Desktop/global process was killed.
11. validation artifacts are under .glyphdeck/validation/<milestone>/.
12. git status is reviewed.
13. commit is created with approved message.
```

If any item fails, milestone status is:

```text
Not accepted
```

---

## Required Validation Artifact Layout

Each milestone must use:

```text
.glyphdeck/validation/<milestone>/
├─ logs/
├─ screenshots/
├─ scripts/
└─ pids/
```

`.glyphdeck/` is ignored and must not be committed.

---

## Current Milestone 3.5 Acceptance Requirements

Milestone 3.5 is accepted only when it delivers:

```text
1. Stable data-testid selectors for validation-critical UI.
2. Dev/test-only endpoints guarded by GLYPHDECK_DEV_TOOLS=1.
3. Controlled start/stop validation scripts.
4. Repo-local logs/screenshots/scripts/PIDs.
5. No visible terminal/editor windows.
6. Fresh M3 smoke validation using data-testid selectors.
7. Verified fresh session and fresh assistant response.
8. Vision review against fresh screenshots.
9. Code review of validation safety.
10. Commit created.
```

---

## Milestone 3.5 Must Not

Milestone 3.5 must not implement:

```text
Milestone 4 streaming
SSE/EventBridge
WebSockets
Agent Terminal
Usage aggregation
Review data
Permissions popup
PTY terminal
SQLite
auth
LAN/Tailscale
team mode
worktrees
```

---

## Update Rules For This File

After an accepted milestone, update:

```text
Current Accepted Milestones
Current Unaccepted / Incomplete Milestones
Current Next Step
Top-Right Version Label Rule
Current Explicit Non-Goals
```

Do not delete historical accepted milestone rows.

Do not mark a milestone accepted without evidence.

Do not mark validation accepted if any forbidden host action occurred.