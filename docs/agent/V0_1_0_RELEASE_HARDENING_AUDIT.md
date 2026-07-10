# v0.1.0 Release-Hardening Audit

## Status

In progress. The local `v0.1.0` tag was removed before this audit. Do not create
a new tag until every gate in this document passes.

## Release Boundary

- GlyphDeck defaults to `127.0.0.1` and rejects non-loopback bind values.
- Mutating HTTP requests require a loopback host and, when supplied, a loopback
  `Origin`. This covers terminal, Settings, permission, server, project, and
  session mutations.
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

`.github/workflows/ci.yml` runs frontend dependency install/build, Go test, Go
vet, and the release build script with read-only repository permissions.

## Completed Gates

- `go test ./... -count=1`, `go vet ./cmd/... ./internal/... ./web`,
  `npm.cmd --prefix web run build`, and `scripts/build.ps1` passed.
- The isolated release smoke passed from outside the repository root with
  embedded assets, 17 fresh screenshots, no Vite process, tracked OpenCode PID
  exit, terminal-child exit, Settings modal, session refresh, and clean
  Problems state.
- All 17 source screenshots and their manifest were manually inspected. The
  standalone Playwright image decoder intermittently showed partially painted
  black regions; the in-app browser rendered the same release shell, Settings
  modal, and running-terminal DOM geometry correctly. This is recorded as a
  validation-capture limitation, not an app layout defect.
- Brooks review-only audit found no dependency cycles or boundary violations.
  Deferred non-blocking debt: duplicate local HTTP-origin/JSON helper logic in
  several API packages; no broad refactor was made.

## Remaining Gate

- SonarQube scan is blocked: no runnable scanner or MCP integration, no
  Docker/Podman runtime, and no configured project credentials. Do not accept
  or tag v0.1.0 until this scan is completed or the user formally resolves the
  gate.
