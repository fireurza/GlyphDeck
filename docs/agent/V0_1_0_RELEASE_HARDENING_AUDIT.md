# v0.1.0 Release-Hardening Audit

## Status

In progress. The local `v0.1.0` tag was removed before this audit. The
hardening branch gates passed locally, but do not create a new tag until the
user authorizes final v0.1.0 release acceptance.

## Release Boundary

- GlyphDeck defaults to `127.0.0.1` and rejects non-loopback bind values.
- Mutating HTTP requests require a loopback host and an exact same-origin
  `Origin` host+port. Different loopback ports are rejected in release mode.
  Vite/dev cross-origin is allowed only with `GLYPHDECK_DEV_TOOLS=1`.
- GlyphDeck has no user authentication. The loopback-only boundary is not a
  substitute for authentication and must not be exposed to another network.

## Process Lifecycle

- OpenCode servers and User Terminal shells are app-owned only when GlyphDeck
  starts and tracks their process IDs.
- On Windows, each app-owned shell and OpenCode process is assigned to a Job
  Object with `KILL_ON_JOB_CLOSE`; closing the Job terminates its descendants.
  The existing tracked-PID `taskkill /T /F` path remains the non-Job fallback;
  neither path uses a process-name or global kill.
- Release smoke verifies tracked OpenCode PID exit and terminal-child exit after
  close.

## Session Model

OpenCode is the source of truth for session lists, messages, and session state.
GlyphDeck restores browser selection only. If the OpenCode server is unavailable,
session operations return an unavailable/error state rather than cached data.

## Architecture Review

- `cmd/glyphdeck` is the composition root.
- `internal/projects`, `servers`, `sessions`, `review`, `usage`, `permissions`,
  `terminal`, and `settings` own their bounded workflows.
- `internal/opencode` isolates OpenCode HTTP/SSE access from UI and workflow
  packages.
- `internal/lifecycle` owns shared app-owned process-tree cleanup.

## Repository Hygiene

`git ls-files` and `.gitignore` were audited. Generated validation state, local
SQLite data, logs, frontend dependencies/build output, release binaries, and
quality-tool caches are ignored and not tracked.

## CI Baseline

`.github/workflows/ci.yml` runs frontend dependency install, frontend tests,
frontend build, Go test, Go vet, and the release build script with read-only
repository permissions.

## Completed Gates

- `go test ./... -count=1`, `go vet ./cmd/... ./internal/... ./web`,
  `npm.cmd --prefix web run test`, `npm.cmd --prefix web run build`, and
  `scripts/build.ps1` passed.
- The isolated release smoke passed from outside the repository root with
  embedded assets, 17 fresh screenshots, no Vite process, tracked OpenCode PID
  exit, terminal-child exit, Settings modal, session refresh, and clean
  Problems state.
- All 17 source screenshots plus `contact-sheet.png` were manually inspected.
  No layout, modal, panel, terminal, or Problems visual blocker was found.
- Repo hygiene passed: `git ls-files` showed no tracked DBs, logs,
  screenshots, `dist`, `node_modules`, `.glyphdeck`, or secrets.
- `npm.cmd --prefix web audit --audit-level=high` passed with 0
  vulnerabilities. `govulncheck`, `staticcheck`, and `actionlint` were not
  installed locally, so they were not run.
- Brooks Review / Brooks Health branch review found the prior critical
  loopback-origin finding fixed and no remaining critical or warning findings
  in the branch diff. Health result: release-clean for this correction branch.

## Fixed Brooks Findings

- Critical — loopback origin becomes global trust rule: fixed by requiring exact
  same-origin host+port for mutations in release mode and gating Vite/dev
  cross-origin behind `GLYPHDECK_DEV_TOOLS=1`.
- Warning — frontend has no unit/component tests: fixed with a Vitest baseline
  covering Settings modal lifecycle, Settings save success, API client errors,
  and Review null-file-list resilience.
- Warning — duplicated HTTP security/response helpers: fixed with
  `internal/httpapi` for JSON responses, errors, content type checks, mutation
  method checks, and local-origin guard logic.
- Warning — SSE tests use fixed sleeps: fixed by replacing sleeps with
  condition/event waits and bounded deadlines.
- Suggestion — OpenCode command built twice: fixed by building the command once
  after creating the child context.

## Remaining Release Limitations

1. **Windows ConPTY disabled — pipe-based terminal active.** The ConPTY implementation
   exists in `internal/terminal/session_windows.go` but is disabled. Child processes
   started inside a ConPTY session are not visible to `Get-CimInstance Win32_Process`
   queries (by command line, process name, parent PID, or PID-diff) from outside the
   pseudo console. The smoke test uses WMI PID-diff detection which works with
   pipe-based terminals but fails under ConPTY. Three detection approaches were tested
   and failed: (1) command-line search by marker, (2) `Get-Process -Name node`,
   (3) parent-PID filter via `Get-CimInstance`. ConPTY enablement requires either
   in-terminal process tracking or Windows API changes.

## First-Run Admin Auth

- First-run setup: if no admin exists, the UI shows a setup screen.
- Login: admin password is bcrypt-hashed, sessions use HttpOnly cookies.
- Bootstrap: `GLYPHDECK_ADMIN_PASSWORD` env var creates admin on startup if none exists.
- Same-origin/loopback guards remain as defense-in-depth.
