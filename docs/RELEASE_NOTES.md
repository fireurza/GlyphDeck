# GlyphDeck Release Notes

## v0.1.0 — MVP Release

GlyphDeck v0.1.0 is the first usable, local-first web workspace for managing
OpenCode projects and workflows from a browser UI.

### Highlights

- Single Windows release binary serves the embedded React frontend and Go API;
  Vite is not required at runtime.
- SQLite stores registered projects and Settings, with migration from the legacy
  project JSON registry.
- Project registration, Git status detection, OpenCode discovery, per-project
  server lifecycle, sessions, prompts, and live SSE transcript streaming.
- Review, Usage, Agent Terminal, Permissions, User Terminal, and Problems
  workflows are available from the workspace UI.
- Browser refresh restores selected project and session state while its OpenCode
  server remains available.
- Settings opens from the activity rail as a modal overlay. The bottom dock
  contains Problems, Agent Terminal, and Terminal.
- The User Terminal streams output reliably, including the v0.1.0 terminal
  marker path validated by the release smoke suite.

### Build and Run

On Windows, build the release binary from the repository root:

```powershell
pwsh -NoProfile -ExecutionPolicy Bypass -File .\scripts\build.ps1
.\dist\glyphdeck.exe
```

GlyphDeck listens on `http://127.0.0.1:8756`. See the
[README](../README.md) for prerequisites, local development, and validation
commands.

### Validation

The accepted release candidate was validated from the embedded
`dist\glyphdeck.exe` binary with isolated app data. The release smoke suite
checks the homepage, project/server/session lifecycle, Settings modal,
terminal open/close flow and marker output, Problems happy path, shutdown, and
embedded frontend loading outside the repository root.

### Known Limitations

- GlyphDeck is localhost-only and has no authentication. Do not expose it to a
  LAN or public network.
- On Windows, the User Terminal uses `exec.Command` pipes rather than a true
  PTY. TTY resize and terminal signals are unavailable.
- App-owned OpenCode servers and terminals intentionally stop when GlyphDeck
  shuts down; restart them after backend restart.
- Sessions are supplied by the running OpenCode server and are not cached in
  SQLite. Reload sessions after the server is available.
- There is no installer. Build the binary and perform manual local setup.
- Usage remains unavailable until OpenCode supplies usage fields.

