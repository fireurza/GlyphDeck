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
go test -race ./... -count=1
npm.cmd --prefix web audit --audit-level=high
.\scripts\build.ps1
.\scripts\validation\run-mvp-smoke.ps1
.\scripts\validation\run-docker-preview-smoke.ps1
docker compose -f compose.yaml config
docker build -t glyphdeck:preview .
```

## Docker Preview Smoke Test

The Docker preview smoke test validates the Compose stack in an isolated
environment.

Run from the project root:

```powershell
.\scripts\validation\run-docker-preview-smoke.ps1
```

The test:

- Uses an isolated, unique Compose project name.
- Generates a random admin password — never exposed.
- Builds the image from the repository Dockerfile.
- Validates `docker compose config`.
- Starts the service and waits for healthy status.
- Verifies `/healthz` and the embedded UI respond.
- Bootstraps admin, logs in, creates a persisted API record.
- Recreates the container without deleting the data volume.
- Confirms the persisted record survives recreation.
- Confirms the container runs as a non-root user.
- Confirms the published host address is loopback-only.
- Confirms no Docker socket is mounted.
- Confirms OpenCode is not required for startup.
- Cleans up the isolated stack and volume in `finally`/error handling.
- Preserves sanitized logs under `.glyphdeck/validation/docker-preview/`.

## Remote lifecycle UI validation

After building `dist\glyphdeck.exe`, run the headless remote UI harness:

```powershell
node .\scripts\validation\server-panel-screenshots.cjs
```

The harness creates isolated data under
`.glyphdeck\validation\remote-lifecycle\`, bootstraps an admin with a random
`GLYPHDECK_ADMIN_PASSWORD`, logs in headlessly, and rejects setup or login
screens before every retained screenshot. It captures empty, add-form,
validation, offline, successful SSH-test, online, attached, lifecycle-error,
protected-delete, and narrow-layout states. SSH lifecycle success and failure
responses are mocked in the browser so no SSH host, private key, or credential
is required.

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
