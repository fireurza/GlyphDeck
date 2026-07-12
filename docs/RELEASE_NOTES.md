# GlyphDeck Release Notes

## v0.1.2 — Security Hardening Release

GlyphDeck v0.1.2 is a security and governance hardening release.

### Security hardening

- **Path validation:** Containment checks rewritten to use `filepath.Rel` with volume awareness, traversal rejection, and symlink resolution. Rejects Windows drive-relative and UNC paths. Terminal `cwd` requires a registered project — no arbitrary-path fallback.
- **Cookie policy:** Centralized `newSessionCookie` with `HttpOnly`, `SameSite: Lax`, and conditional `Secure` flag. `Secure: true` for TLS and non-loopback connections; `Secure: false` only for direct loopback HTTP. No `X-Forwarded-Proto` trust.
- **Vulnerability scanning:** `govulncheck` enforced in CI (previously `|| true`). GO-2026-5932 (OpenPGP) confirmed unreachable — narrow exception in `osv-scanner.toml` with review expiry.
- **Fuzzing:** Six Go fuzz targets across path containment, network validation, and cookie policy. Weekly fuzz workflow with crash artifact upload.
- **Dependency installation:** `npm ci` in build scripts (deterministic lockfile installation).

### Governance

- **Code of Conduct:** Contributor Covenant v2.1 with enforcement by The ANB Collective LLC d/b/a FireGlyph Studios.
- **Contributor Terms:** Repository-hosted contributor license grant (not copyright assignment). Covers reproduction, modification, distribution, sublicensing, commercial licensing, relicensing, assignability, and patent grant.
- **Contributor Terms check:** Automated PR workflow verifying explicit acceptance before merge.
- **Dependabot:** Grouped npm dependency updates limited to minor and patch versions. Major upgrades reviewed individually.

### Release artifacts

- **SBOM:** CycloneDX JSON (`glyphdeck-v0.1.2.cdx.json`) — Go and embedded frontend dependencies.
- **Provenance:** Sigstore-signed attestation for all four release binaries (`glyphdeck-v0.1.2.provenance.sigstore.json`).
- **SBOM attestation:** Sigstore-signed SBOM attestation associating binaries with the CycloneDX SBOM (`glyphdeck-v0.1.2.sbom.sigstore.json`).

### Verification

Verify attestations for all release binaries:

```bash
gh attestation verify glyphdeck-linux-amd64 --repo fireurza/GlyphDeck
gh attestation verify glyphdeck-darwin-amd64 --repo fireurza/GlyphDeck
gh attestation verify glyphdeck-darwin-arm64 --repo fireurza/GlyphDeck
gh attestation verify glyphdeck-windows-amd64.exe --repo fireurza/GlyphDeck
```

Verify checksums:

```bash
sha256sum -c checksums.txt
```

### License

GlyphDeck is source-available under the PolyForm Noncommercial License 1.0.0.
Commercial use requires a separate written commercial license from FireGlyph Studios.
See [LICENSE](LICENSE) and [COMMERCIAL-LICENSING.md](COMMERCIAL-LICENSING.md).

---

## v0.1.1 — First Supported MVP Release

GlyphDeck v0.1.1 is the first supported public MVP release of a local-first
web workspace for managing OpenCode projects and workflows from a browser UI.

### Highlights

- Admin authentication (bcrypt, HttpOnly sessions).
- Project registry (SQLite-backed) with Git status detection.
- OpenCode server start/stop/detect (per-project, PID-scoped).
- Session management, prompt, live SSE transcript streaming.
- Review, Usage, Agent Terminal, Permissions, User Terminal, Problems panels.
- Servers/Sandboxes: local, manual URL, and SSH alias targets.
- Remote SSH lifecycle: test, detect, start, stop (PID-scoped, no blanket kill).
- Windows ConPTY terminal support.
- Single release binary with embedded React/Vite frontend.
- CI/CD: Verify (Go + Node), CodeQL, Dependency Review, OpenSSF Scorecard.
- Source-available under PolyForm Noncommercial License 1.0.0.
- Commercial use requires a separate written license from FireGlyph Studios.

### Supported Platforms

| Platform | Binary |
|----------|--------|
| Linux amd64 | `glyphdeck-linux-amd64` |
| macOS amd64 | `glyphdeck-darwin-amd64` |
| macOS arm64 (Apple Silicon) | `glyphdeck-darwin-arm64` |
| Windows amd64 | `glyphdeck-windows-amd64.exe` |

### Security Model

- Admin authentication required (bcrypt password, HttpOnly session cookie).
- Binds to `127.0.0.1` (loopback) by default.
- Mutating API requests require same-origin `Origin` and loopback host.
- Not designed for public internet exposure without additional auth and TLS.

### Build and Run

```powershell
.\scripts\build.ps1
.\dist\glyphdeck.exe
```

GlyphDeck listens on `http://127.0.0.1:8756`. See the [README](../README.md).

### Artifact Verification

```bash
# Verify checksum
sha256sum -c checksums.txt --ignore-missing

# Verify attestation (requires gh CLI)
gh attestation verify dist/glyphdeck-windows-amd64.exe --repo fireurza/GlyphDeck
```

### Known Limitations

- Designed for single-user, local-machine use.
- No multi-user or role-based access control.
- Remote agent/skill/MCP sync not yet implemented.
- No built-in TLS — add a reverse proxy if needed.
- Single admin account only.
