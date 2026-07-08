# GlyphDeck OpenCode Workflow Rules

## Status

These rules are mandatory for all GlyphDeck work involving OpenCode, OpenCode Desktop, Docker Sandbox, OpenCode server processes, sessions, or validation.

---

## Correct Operating Model

GlyphDeck development uses two separate systems:

```text
OpenCode Desktop Local Server
Docker Sandbox command execution
```

They are not interchangeable.

---

## OpenCode Desktop Rules

OpenCode Desktop must stay on:

```text
Local Server
```

Do not switch OpenCode Desktop to:

```text
Docker Sandbox
```

Reason:

```text
Local Server preserves:
- plugins
- MCP
- LSP
- OAuth auth
- model configuration
- local OpenCode setup
```

Do not use Desktop Docker Sandbox mode for GlyphDeck validation unless a future milestone explicitly changes this document.

---

## Docker Sandbox Rules

Docker Sandbox is only a command execution target.

Allowed command pattern:

```powershell
sbx exec -w /c/Users/Fireurza/Documents/Code/GlyphDeck glyphdeck-sbx bash -lc "<command>"
```

Forbidden:

```powershell
sbx run glyphdeck-sbx
```

Forbidden unless explicitly required by a future milestone:

```powershell
sbx ports glyphdeck-sbx --publish ...
```

Do not try to make OpenCode Desktop connect to the manually created sandbox.

Do not run OpenCode Desktop inside the sandbox.

Do not install OpenCode into the sandbox unless the sandbox image already includes it or a milestone explicitly requires installation.

---

## Known Sandbox

Sandbox name:

```text
glyphdeck-sbx
```

Host workspace:

```text
C:\Users\Fireurza\Documents\Code\GlyphDeck
```

Sandbox workdir:

```text
/c/Users/Fireurza/Documents/Code/GlyphDeck
```

Validation command template:

```powershell
sbx exec -w /c/Users/Fireurza/Documents/Code/GlyphDeck glyphdeck-sbx bash -lc "<command>"
```

---

## Path Rules

When running on Windows host:

```text
C:\Users\Fireurza\Documents\Code\GlyphDeck
```

When running inside sandbox:

```text
/c/Users/Fireurza/Documents/Code/GlyphDeck
```

Do not give a Windows path to a backend running inside Linux.

Do not give a Linux path to a backend running on Windows.

Before adding a project during validation, identify where the GlyphDeck backend is running.

For normal browser validation, GlyphDeck backend usually runs on Windows host, so the project path must be:

```text
C:\Users\Fireurza\Documents\Code\GlyphDeck
```

---

## OpenCode Binary Detection

Use:

```powershell
opencode --version
```

Do not use:

```powershell
opencode version
```

`opencode version` is interpreted as a directory/subcommand path by OpenCode and can fail with directory-change errors.

---

## OpenCode Server Startup

GlyphDeck starts OpenCode servers with:

```powershell
opencode serve --port <port> --hostname 127.0.0.1
```

Rules:

```text
- Bind to 127.0.0.1 only.
- Allocate a dynamic local port.
- Run in the registered project cwd.
- Track PID.
- Track port.
- Track ownership.
- Stop only app-owned processes.
```

Do not expose OpenCode server to LAN/Tailscale/public interfaces during POC milestones.

---

## OpenCode Server Ownership

There are two classes of OpenCode processes:

```text
1. Externally-owned:
   - OpenCode Desktop
   - user-started OpenCode sessions
   - any process not started by GlyphDeck

2. GlyphDeck-owned:
   - OpenCode server process started through GlyphDeck server manager
   - PID stored in GlyphDeck server state
```

GlyphDeck may stop only GlyphDeck-owned OpenCode processes.

GlyphDeck must never stop externally-owned OpenCode processes.

Validation must never stop OpenCode by process name.

---

## OpenCode Server Stop Rules

Preferred stop path:

```http
POST /api/projects/{projectId}/server/stop
```

If forced cleanup is needed:

1. Read server state:
   ```http
   GET /api/projects/{projectId}/server
   ```

2. Confirm:
   ```text
   status is starting/ready/unhealthy/failed
   pid is present
   process is app-owned
   ```

3. Stop only that exact PID.

Forbidden cleanup:

```powershell
Get-Process -Name opencode | Stop-Process
taskkill /IM opencode.exe
Stop-Process -Name opencode
```

---

## OpenCode Health Check Rules

OpenCode server may return `401 Unauthorized` for HTTP API endpoints.

A `401` from an OpenCode endpoint can mean:

```text
server is reachable but requires auth
```

It must not automatically be treated as:

```text
server process failed
```

Health strategy:

```text
1. Prefer authenticated health check when password is configured.
2. Support OPENCODE_SERVER_PASSWORD when available.
3. If auth is unavailable, use TCP reachability as readiness fallback.
4. Do not store provider/model secrets.
5. Do not leak credentials to logs.
```

---

## OpenCode Auth Rules

OpenCode server auth may use Basic Auth.

Username:

```text
opencode
```

Password source:

```text
OPENCODE_SERVER_PASSWORD
```

Rules:

```text
- Use password only if present.
- Do not set fake passwords.
- Do not log password values.
- Do not persist password values.
- Do not commit password values.
```

---

## OpenCode API Boundary Rules

All OpenCode HTTP calls must live under:

```text
internal/opencode
```

Do not call OpenCode endpoints directly from random backend modules.

Other backend modules must use internal service/client abstractions.

Allowed module dependency pattern:

```text
internal/sessions -> internal/opencode client
internal/review -> internal/opencode client
internal/usage -> internal/opencode client
internal/permissions -> internal/opencode client
```

Forbidden:

```text
frontend -> OpenCode server directly
random handler -> raw OpenCode URL
project registry -> raw OpenCode API
```

GlyphDeck backend hides OpenCode ports from browser where possible.

---

## Current Confirmed OpenCode API Direction

For OpenCode 1.17.x, session operations should use direct session endpoints, not stale project-prefixed endpoints.

Use:

```text
GET  /session
POST /session
GET  /session/:id
GET  /session/:id/message
POST /session/:id/message
```

Do not assume these exist without testing against the installed OpenCode version:

```text
POST /project/init
GET  /project/:projectID/session
POST /project/:projectID/session
```

If an endpoint returns HTML SPA content instead of JSON, the endpoint path is likely wrong.

Do not treat HTML as JSON.

Do not claim API success unless JSON response shape is validated.

---

## Session Validation Rules

A valid session prompt-loop validation must prove all of the following:

```text
1. OpenCode server is ready.
2. A fresh session is created through GlyphDeck.
3. The returned session ID is captured.
4. That exact session ID appears in the UI.
5. The session is selected by ID, not by first visible item.
6. Prompt is sent to that exact session.
7. User message count increases after prompt send.
8. Assistant message count increases after prompt send.
9. Assistant response belongs to the same session ID.
```

Do not validate against old sessions.

Do not click the first `.session-item`.

Do not use previous transcript data as proof.

---

## Prompt Submission Rules

For Milestone 3, prompt loop is non-streaming.

Valid behavior:

```text
send prompt
wait for completion/response
refresh messages
render user + assistant messages
```

Invalid proof:

```text
assistant message existed before send
assistant message appeared in another session
assistant message count did not change
script only found any .assistant node
```

Milestone 4 introduces streaming. Do not implement streaming in Milestone 3 or 3.5.

---

## OpenCode Desktop Protection

OpenCode Desktop is user-owned.

Agents must not:

```text
close it
restart it
kill it
switch its server mode
alter its plugin/MCP/LSP configuration
alter OAuth authentication
alter model routing
```

If OpenCode Desktop becomes unstable, stop validation and report.

Do not attempt automated repair unless explicitly instructed.

---

## Problems That Must Stop Validation

Stop immediately if any occurs:

```text
OpenCode Desktop closes
visible Notepad opens
visible cmd.exe opens
visible PowerShell opens
files open in an editor
validation writes to %TEMP% or AppData
global process kill is attempted
stale screenshots are detected
session validation uses existing old sessions
server stop button is blocked by layout overlap
```

Do not continue after these violations.

Report the violation and wait for direction.