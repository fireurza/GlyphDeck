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
- Windows cleanup targets the tracked PID with `taskkill /T /F`; it never uses
  a process-name or global kill.
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

## Pending Gates

- Full test, vet, frontend build, release build, and isolated smoke.
- Manual review of fresh smoke screenshots.
- SonarQube scan: blocked until its MCP integration, container runtime, and
  credentials are available.
- Brooks-Lint review-only health report.
