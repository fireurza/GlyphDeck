# GlyphDeck MVP v0.1.0 Plan

## Document Status

| Field | Value |
|---|---|
| Product | GlyphDeck |
| Publisher | FireGlyph Studios |
| Document type | MVP planning |
| Phase | Phase 2 — MVP / v0.1.0 |
| Created | 2026-07-09 |
| Based on | GlyphDeck Roadmap, POC Implementation Plan, Technical Architecture |
| Accepted baseline | M0 through M14; v0.1.0 release candidate accepted at `6778e4a` |

---

## 1. Purpose

Define the smallest useful v0.1.0 GlyphDeck after the accepted POC baseline.

v0.1.0 means:

```text
Local-first GlyphDeck app
Durable storage (SQLite)
Durable project and Settings state
Reliable browser-refresh selection state
Stable OpenCode integration
Usable review, usage, permissions, and terminal workflows
Single-binary install path
Clear validation and release process
```

The POC proved the core architecture. The MVP makes it durable and distributable.

---

## 2. Current Accepted Capabilities

All M0 through M14 are accepted with validated smoke evidence.

| Milestone | Capability | Evidence |
|---|---|---|
| M0 | Repo bootstrap | Go backend + React/Vite shell |
| M1 | Project registry | Add/list/delete, JSON persistence, Git detection |
| M2 | OpenCode server manager | Detect, start/stop, health check, port allocation |
| M3 | Sessions and prompt loop | Create/list sessions, send prompt, view transcript |
| M3.5 | Validation harness | data-testid, dev endpoints, controlled scripts |
| M4 | EventBridge streaming | SSE from OpenCode to browser, live transcript |
| M5 | Agent Terminal | Read-only tool/event history, category filters |
| M6 | Usage tab | Token/cost aggregation, available/unavailable states |
| M7 | Review tab | Project/Git/session/activity summary |
| M8 | Permissions | Approval popup, once/always/reject |
| M9 | User Terminal | Interactive shell via exec.Command pipes |
| M10 | POC hardening | Browser refresh, problems tab, graceful shutdown, README |
| M11 | SQLite persistence | Projects migrate from JSON to SQLite and persist across restart |
| M12 | State model cleanup | Session and intentional-stop state are reliable after refresh |
| M13 | Settings + embedded release | SQLite-backed Settings and embedded single-binary release build |
| M14 | Terminal reliability | Terminal SSE marker output is reliable |

---

## 3. M10 Validation Evidence

All evidence captured under `.glyphdeck/validation/m10/`.

| Check | Result |
|---|---|
| Smoke test | PASSED |
| Browser refresh — project persistence | Verified |
| Server start + event stream Live | Verified |
| Review/Usage/Agent Terminal regressions | Clean |
| Terminal start/run/close | Verified |
| Problems tab — clean state | Verified |
| Server stop — sane event stream state | Verified |
| Full layout — no clipping | Verified |
| Manifest | Created |
| Vision review | Batch 1 PASS (7/7), Batch 2 PASS (7/7) |
| Screenshots | 14 captured |

### 3.1 Post-M14 Validation Correction

Milestone 14's terminal SSE reliability correction was accepted at `6e46911`.
The validation record is completed with **M14 vision review PASS**: the existing
16 screenshots and manifest at `.glyphdeck/validation/m14/screenshots/` were
reviewed for the Milestone 14 label, visible terminal marker/output, terminal
open/close states, panel integrity, clipping, and unexpected error banners.

### 3.2 v0.1.0 Release-Candidate Evidence

The release candidate was accepted at `6778e4a` after validation from the embedded `dist/glyphdeck.exe`
binary with isolated app data. The `mvp` smoke passed on 2026-07-09 with 17
fresh screenshots and a manifest under `.glyphdeck/validation/mvp/screenshots/`.
It verifies the v0.1.0 label, SQLite-backed Settings persistence, project/server/
session lifecycle, Review/Usage/Agent Terminal/Terminal/Problems regressions,
terminal marker output, orderly shutdown, and a full 1280×720 layout.

Settings remains reachable from the activity rail, but it now opens as a centered
modal overlay above the dock. The dock contains only Problems, Agent Terminal,
and Terminal. Vision review PASS.

---

## 4. Known Technical Caveats

These limitations remain after the release-candidate work:

| Issue | Severity | Plan |
|---|---|---|
| Terminal uses exec.Command pipes, not true PTY | Medium | ConPTY/alternative terminal work remains a later enhancement |
| Server processes and terminals do not persist across backend restart | Medium | App-owned processes intentionally stop at shutdown; restart them explicitly |
| Session data is supplied by the running OpenCode server, not cached in SQLite | Medium | Reload sessions after the server is available |
| No auth — localhost only | High | Remains a v0.1.0 non-goal |
| No installer — `go build` + manual setup | Medium | v0.2.x per roadmap |
| OpenCode API may drift | Medium | Monitor; OpenCode adapter isolation helps |
| Usage is unavailable until OpenCode supplies usage fields | Low | Expected provider-dependent state, shown explicitly in the UI |

---

## 5. MVP Target

### v0.1.0 Definition

```text
GlyphDeck v0.1.0 is a single-binary, locally hosted web desktop for OpenCode.
It persists registered projects and Settings in SQLite.
It restores selected project/session IDs across a browser refresh while the
associated OpenCode server remains available.
It intentionally stops app-owned servers and terminals at backend shutdown.
It binds to localhost by default.
It can be installed by downloading a single Go binary.
It passes a complete release validation harness.
```

### Exit criteria

```text
1. SQLite stores registered projects and Settings, with legacy projects JSON migration.
2. Backend starts from a single Go binary serving embedded React assets.
3. Browser refresh restores the selected project/session when the OpenCode server is available.
4. App-owned OpenCode servers and terminals stop cleanly at backend shutdown.
5. All POC panels (Review, Usage, Agent Terminal, Terminal, Problems, Permissions) remain functional.
6. Release validation harness passes end-to-end.
7. README documents the single-binary install and run path.
```

---

## 6. MVP Non-Goals

These remain out of scope for v0.1.0 unless a later decision moves them in:

```text
Auth beyond localhost guard
Tailscale/LAN binding
Team mode
Worktrees
MCP/plugin/skills editors
Built-in editor
Installer/packaging beyond single binary
Docker hosting
Desktop wrapper
Public network exposure
```

---

## 7. Completed MVP Milestone Sequence

Based on the GlyphDeck Roadmap (v0.1.0 — MVP Local Product) and POC Implementation Plan:

| Milestone | Name | Roadmap source |
|---|---|---|
| M11 | SQLite persistence | Roadmap: "SQLite app storage" |
| M12 | State model cleanup | Roadmap: "Session list/create/resume/reopen", "Transcript persistence/reload" |
| M13 | Settings + embed + release | Roadmap: "Basic settings page", "Go binary serving embedded React assets" |
| M14 | Terminal reliability | Release-candidate reliability correction |
| v0.1.0 | MVP release candidate | Roadmap exit criteria |

The POC Implementation Plan task breakdown ends at M10. The Roadmap v0.1.0 scope
defined the next steps; M11-M14 completed the gap from POC to MVP.

### Why this order

- SQLite must come first because it underpins durable state for everything else.
- State model cleanup follows because it depends on SQLite for persistence.
- Settings + embed + release comes last because it packages the stable app.

---

## 8. Historical MVP Planning Details

The remaining detailed milestone, storage, API, and risk sections preserve the
original roadmap plan. They are not claims that every planned persistence table
or restart behavior is in the current release candidate; the current capability
and caveat sections above are authoritative.

### M11 — SQLite Persistence

**Goal:** Replace JSON/localStorage with SQLite for all app state.

**In scope:**
- SQLite database for projects, server state, session cache, problems, UI layout.
- Migration from existing `.glyphdeck/projects.json` to SQLite on first startup.
- All existing API endpoints read from SQLite.
- `internal/storage/` package for SQLite access.
- Graceful DB open/close on server start/shutdown.
- No data loss on migration — backup JSON before migration.

**Out of scope:**
- Auth/per-user databases.
- Remote/networked database.
- Complex ORM — use `database/sql` with `modernc.org/sqlite` or `github.com/mattn/go-sqlite3`.
- Schema versioning beyond basic migration flag.

**Files likely touched:**
- `internal/storage/` (new or expanded)
- `internal/projects/` (read from SQLite)
- `internal/servers/` (persist server state)
- `cmd/glyphdeck/main.go` (DB init, migration)
- `go.mod` (add SQLite driver)

**Validation:**
- go test with in-memory SQLite.
- Smoke: add project, restart backend, verify project persists.
- Existing M10 smoke regressions pass.
- JSON migration smoke: existing projects.json is migrated without data loss.

**Acceptance criteria:**
- Backend restart preserves all projects.
- Server status survives backend restart.
- Problems persist across sessions.
- No JSON file read/write in production code paths (only migration).

**Risk:** Medium — data migration must be safe and reversible.
**Dependencies:** None (first MVP milestone).

---

### M12 — State Model Cleanup

**Goal:** Make session/project/UI state reliable across restarts, refreshes, and reconnects.

**In scope:**
- Session cache in SQLite: session list, last message timestamp, status.
- Transcript persistence: reload messages from OpenCode after restart.
- Project/server state reconciliation after backend restart.
- LeftPanel polling coordination fix (carried forward from M10).
- Browser refresh fully restores selected project, loaded sessions, and panel state.
- Problems tab is aware of OpenCode server state transitions.
- Event stream reconnect uses accurate status (no stale Error/Reconnecting).

**Out of scope:**
- Full transcript storage in SQLite (cache metadata only — messages come from OpenCode).
- Session resume across OpenCode server restarts (reattachment).
- Cross-device state sync.

**Files likely touched:**
- `internal/sessions/`
- `internal/servers/`
- `internal/events/`
- `web/src/layout/LeftPanel.tsx`
- `web/src/App.tsx`
- `web/src/api/events.ts`

**Validation:**
- Smoke: add project, start server, create session, send prompt, refresh browser.
- Verify: project selected, session list loaded, transcript reloaded.
- Verify: after Stop Server, event stream shows Offline/Idle, not Error/Reconnecting.
- Problems tab shows server-stopped notification during transitions.

**Acceptance criteria:**
- Browser refresh fully restores working state.
- Event stream never shows permanent Error/Reconnecting after intentional stop.
- Session list loads after project selection within 5 seconds.

**Risk:** Medium — complex state interactions across frontend/backend.
**Dependencies:** M11 (SQLite).

---

### M13 — Settings, Embedded Frontend, Release Validation

**Goal:** Package GlyphDeck as a single distributable binary with basic settings and a release harness.

**In scope:**
- Embed React build artifacts into Go binary (`embed` package).
- Single `glyphdeck.exe` (or platform equivalent) serves both backend API and frontend.
- Basic settings page: project defaults, OpenCode binary path override, log level.
- Settings stored in SQLite, editable from UI.
- Release validation harness: end-to-end smoke covering all M0-M13 capabilities.
- Cross-platform build script (Windows, Linux, macOS).
- Updated README with single-binary install instructions.
- Settings opens from the activity rail as an overlay and does not consume a dock tab.

**Out of scope:**
- Auth settings UI.
- LAN/Tailscale bind settings.
- Plugin/skills/agents settings.
- npm/npx launcher (v0.2.x per roadmap).
- Auto-update.

**Files likely touched:**
- `cmd/glyphdeck/main.go` (embed frontend)
- `web/` (build output embedded)
- `internal/config/` or `internal/settings/` (new)
- `scripts/build.ps1`, `scripts/build.sh` (new)
- `scripts/validation/run-mvp-smoke.ps1` (new)

**Validation:**
- `go build` produces single binary.
- Binary starts and serves frontend without `npm run dev`.
- Settings page accessible, changes persist.
- Full release smoke passes: project, server, session, prompt, review, usage, agent terminal, permissions, terminal, problems, shutdown.

**Acceptance criteria:**
- Single binary serves complete GlyphDeck UI.
- Settings changes survive restart.
- Settings is reachable from the activity rail and leaves the dock with only Problems, Agent Terminal, and Terminal.
- Release smoke passes on all target platforms.

**Risk:** Medium — embed requires build pipeline changes and path resolution updates.
**Dependencies:** M12 (state model cleanup).

---

### v0.1.0 — Release Candidate

**Goal:** Validate the candidate after M11-M14. Tagging and changelog publication
are separate release actions.

**Checklist:**
- M11-M14 regressions and the release smoke pass.
- README documents the embedded single-binary build/run path.
- Known limitations and intentional shutdown behavior are documented.
- A v0.1.0 tag and changelog are created only when separately authorized.

---

## 9. Data/Storage Plan

### Transition: JSON/localStorage → SQLite

| Data | POC storage | MVP storage | Migration |
|---|---|---|---|
| Projects | `.glyphdeck/projects.json` | SQLite `projects` table | Auto-migrate on first M11 startup; backup JSON file |
| Server state | In-memory (lost on restart) | SQLite `servers` table | Fresh on M11 (no migration needed) |
| Session cache | In-memory | SQLite `sessions` table | Fresh on M11 |
| Problems | Bounded in-memory ring buffer | SQLite `problems` table (bounded) | None (problems are ephemeral) |
| UI state | localStorage in browser | SQLite `ui_state` table | localStorage remains for fast client restore; SQLite as backend source |
| Settings | None | SQLite `settings` table | New in M13 |
| Config | Environment variables | Environment variables + settings page | Settings stored in SQLite, env vars as overrides |

### Proposed high-level SQLite tables (not implemented yet)

```sql
projects (id, name, path, trusted, tags_json, git_is_repo, git_branch, created_at, updated_at)
servers (project_id, port, pid, status, version, base_url, started_at, stopped_at)
sessions (id, project_id, title, status, message_count, last_message_at, updated_at)
problems (id, level, source, message, created_at)
ui_state (key, value_json, updated_at)
settings (key, value, updated_at)
```

### Migration strategy

1. On first M11 startup, check if `projects` table is empty.
2. If empty and `.glyphdeck/projects.json` exists, read and insert all projects.
3. Backup `.glyphdeck/projects.json` to `.glyphdeck/projects.json.bak-<timestamp>`.
4. All subsequent reads/writes go through SQLite.
5. Schema version tracked in a `meta` table; migrations run on version mismatch.

### Backup/export

- SQLite database file is self-contained and can be backed up by copying.
- Export endpoint (dev-only or settings page) to dump Projects/Settings as JSON.

---

## 10. API/Backend Plan

### Error model

- All API errors use consistent JSON envelope: `{"error": {"code": "...", "message": "..."}}`.
- Problems tab receives real-time error events via the existing Problems manager.
- API errors that affect the user are recorded in Problems (bounded, deduplicated).

### Logging

- Structured logs using `log/slog` or consistent `log.Printf` patterns.
- Log levels: startup/shutdown, server lifecycle, session create, errors, migrations.
- No secrets in logs.
- Validation logs remain under `.glyphdeck/validation/<milestone>/`.

### Graceful shutdown

- Already hardened in M10:
  - Stop all app-owned OpenCode servers.
  - Stop all app-owned terminals.
  - Stop event hub bridges.
  - Close SQLite database.
- Add: flush Problems to SQLite before close.

### Server/session/terminal lifecycle

- One OpenCode server per project (current model, unchanged).
- Terminals bound to project lifetime; closed on project/server stop.
- Session cache invalidated on OpenCode server restart; re-fetched on next request.

### OpenCode adapter boundaries

- All OpenCode HTTP calls remain in `internal/opencode/`.
- Other packages use `opencode.SessionClient` interface or equivalent per-package abstractions.
- No direct OpenCode URL access from frontend or non-opencode backend packages.

---

## 11. Frontend Plan

### State ownership

- `App.tsx` owns: selectedProjectId, selectedSessionId (persisted to localStorage for fast restore).
- `LeftPanel` owns: project list, server statuses, session list (fetched from API).
- `CenterPanel` owns: transcript messages (fetched from API, updated via events).
- `ProblemsPanel` owns: problem list (polled from API).
- Right panels (Review, Usage) own their data (fetched on demand).
- Bottom panels (Agent Terminal, User Terminal) own their state.

### Reload behavior

- Browser refresh: localStorage restores selection; API calls reload data.
- Backend restart: frontend detects via health check or event stream disconnect; shows reconnecting state; reloads data on reconnect.
- OpenCode server stop: event stream goes Offline; session/usage/review panels show appropriate unavailable states.

### Error presentation

- API errors show inline in the relevant panel (not global popups).
- Problems tab aggregates app-level errors.
- No silent failures — every caught error is either displayed or logged to Problems.

### Layout constraints

- Left panel: 270px (fixed in M7).
- Right panel: 280px.
- Bottom panel: 200px.
- Center panel: flex-fill.
- All panels collapsible (future).

### Panel responsibilities

| Panel | Responsibility |
|---|---|
| TopBar | Event stream status, version label |
| LeftPanel | Projects, server controls, sessions |
| CenterPanel | Transcript, prompt composer, permission popup |
| RightPanel | Review, Usage tabs |
| BottomPanel | Problems, Agent Terminal, User Terminal tabs |

---

## 12. Validation Plan

### Unit tests

- Build `web/dist` first; it is embedded by the Go release package.
- Go packages: use `go test ./... -count=1`.
- Mock OpenCode client for session/usage/review/permission tests.
- In-memory SQLite for storage tests.

### Static analysis

- `npm.cmd --prefix web run build` (TypeScript + Vite; before Go compilation on Windows).
- `go vet ./cmd/... ./internal/... ./web`.

### Browser smoke

- PowerShell 7 runner scripts under `scripts/validation/`.
- Isolated workspace under `.glyphdeck/validation/<milestone>/workspace/`.
- Fresh session by exact ID.
- data-testid selectors only.
- Machine assertions before every screenshot.
- Validate app behavior, not model obedience.

### Screenshot manifest

- Created per milestone under `.glyphdeck/validation/<milestone>/screenshots/manifest.md`.
- Lists expected state, machine assertion, error-banner check, freshness.

### Vision review

- fireglyph-vision-reviewer for screenshot batches (max 8 per batch).
- Verifies labels, layout, error banners, functional states.

### Hard rules (carried forward)

- No global process kills by name.
- No OpenCode Desktop changes.
- No stale screenshots.
- No fragile selectors.
- No `.glyphdeck/` or `.cortexkit/` in commits.
- No LLM/model obedience as hard validation gate.

---

## 13. Release Criteria

Before tagging v0.1.0, all must be true:

```text
1. M11 (SQLite) accepted.
2. M12 (State model cleanup) accepted.
3. M13 (Embed + settings + release) accepted.
4. Single binary serves complete app.
5. Release validation smoke passes on Windows.
6. README documents single-binary install path.
7. Known limitations documented.
8. No known data-loss paths.
9. POC JSON migration tested and reversible.
10. Git tag v0.1.0 created.
11. Changelog written.
```

---

## 14. Risk Register

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| OpenCode API changes break integration | Medium | High | OpenCode adapter isolation; version compatibility tests |
| SQLite migration corrupts existing projects.json | Low | High | Backup before migration; dry-run mode |
| Windows ConPTY still blocked for true PTY | Medium | Medium | Document exec.Command pipes limitation; accept for v0.1.0 |
| Terminal SSE buffering persists | Medium | Low | Timer flush in M10; additional treatment in M14 if needed |
| Embedded frontend breaks path resolution | Medium | Medium | Test with `embed` package early; document path assumptions |
| Browser refresh state reconciliation is complex | Medium | Medium | Incremental M12 changes; test each state transition |
| Validation harness flakes on Windows | Medium | Medium | Pre-built binary; consistent port cleanup; generous timeouts |
| DeepSeek/model obedience varies between runs | High | Low | Never use model obedience as validation gate; validate app behavior only |

---

## 15. Decision Log

| Decision | When | Rationale |
|---|---|---|
| Go backend | POC M0 | Performance, single binary, cross-platform |
| React/TypeScript/Vite frontend | POC M0 | Ecosystem, type safety, fast dev |
| Internal Go HTTP/SSE adapter | POC M0 | No stale SDK dependency; full control |
| SQLite for MVP | Planning | Durable, self-contained, no server process |
| JSON for POC only | POC M1 | Fast spike; replaced in M11 |
| OpenCode Desktop Local Server only | POC M0 | Preserves plugins, MCP, LSP, auth |
| Docker Sandbox for command execution only | POC M0 | Isolated test environment |
| One OpenCode server per project | POC M2 | Simpler lifecycle; avoids cross-project contamination |
| exec.Command pipes for terminal (not PTY) | M9 | PTY library unsupported on Windows; honest fallback |
| localStorage for browser refresh persistence | M10 | Fast restore; SQLite as backend source |
```

(End of plan)
