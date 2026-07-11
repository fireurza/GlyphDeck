# GlyphDeck Release Notes

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
