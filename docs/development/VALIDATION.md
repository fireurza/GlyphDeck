# Validation

## MVP Smoke Test

The MVP smoke test validates 17 checks across the full application stack.

Run from the project root:

```powershell
.\scripts\build.ps1
.\scripts\validation\run-mvp-smoke.ps1
```

The smoke test:

- Uses an isolated data directory and dynamic port.
- Bootstraps admin via `GLYPHDECK_ADMIN_PASSWORD`.
- Logs in through the UI.
- Creates a project, starts an OpenCode server, creates a session, sends a prompt.
- Verifies Review, Usage, Agent Terminal, User Terminal, Problems panels.
- Verifies terminal child-process startup and cleanup.
- Verifies Settings persistence and modal behavior.
- Verifies server stop and event stream offline state.
- Captures 17 screenshots under `.glyphdeck/validation/mvp/screenshots/`.
- Runs a visible-window monitor to detect forbidden host windows.

### Expected output

```
=== v0.1.1 Release Candidate Smoke PASSED ===
[mvp] Browser smoke PASS (17 fresh screenshots).
Result: PASS
```

## Quick Validation

```powershell
go test ./... -count=1
go vet ./cmd/... ./internal/... ./web/...
npm.cmd --prefix web run test
npm.cmd --prefix web run build
```

## Validation Artifacts

All validation artifacts live under `.glyphdeck/validation/<milestone>/`:

- `logs/` — smoke, backend, and monitor logs
- `screenshots/` — Playwright screenshots
- `pids/` — PID files for controlled processes
- `scripts/` — copied smoke scripts

Artifacts are excluded from Git via `.gitignore`.

## Artifact Verification

Release binaries are signed with GitHub Artifact Attestations and include
SHA-256 checksums and a CycloneDX SBOM.

Verify a downloaded binary:

```powershell
# Verify the checksum
shasum -a 256 -c checksums.txt --ignore-missing

# Verify the attestation (requires gh CLI)
gh attestation verify dist/glyphdeck-windows-amd64.exe --repo fireurza/GlyphDeck
```
