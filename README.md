# GlyphDeck

GlyphDeck is a local web workspace for registering project directories and showing their basic repository status.

## Milestone 1 — Project Registry

Milestone 1 adds a local project registry backed by a JSON file. The left Projects panel can add a local directory, show whether it is a Git repository, show the current branch when available, persist the entry across backend restarts, and remove the entry.

Registry data is stored at:

```text
.glyphdeck/projects.json
```

`.glyphdeck/` is local development data and is ignored by Git.

## Prerequisites

- [Go](https://go.dev/dl/) (1.23+)
- [Node.js](https://nodejs.org/) (20+) with npm

## Run Locally

Shell: PowerShell 7
Working directory: project root

**Backend:**

```powershell
go run ./cmd/glyphdeck
```

Starts on `http://127.0.0.1:8756`. Health check at `/healthz`.

**Frontend:**

```powershell
cd web && npm install
cd web && npm run dev
```

Starts Vite dev server (default: `http://localhost:5173`).

## Project Registry API

- `GET /api/projects` — list registered projects.
- `POST /api/projects` — add a local project path.
- `GET /api/projects/{projectId}` — get one registered project.
- `DELETE /api/projects/{projectId}` — remove one registered project.

## Manual Smoke Test

Shell: PowerShell 7
Working directory: project root

1. Start backend:

   ```powershell
   go run ./cmd/glyphdeck
   ```

2. Start frontend in a second terminal:

   ```powershell
   cd web && npm run dev
   ```

3. Add the current GlyphDeck repo path in the left Projects panel.
4. Confirm the project appears with Git repo status and branch.
5. Restart the backend.
6. Confirm the project persists.
7. Remove the project.
8. Confirm it disappears.

## Validation Commands

Shell: PowerShell 7
Working directory: project root

```powershell
go test ./...
cd web && npm run build
go run ./cmd/glyphdeck
cd web && npm run dev
```

## Stopping Dev Servers

**Normal stop:**

- Press `Ctrl+C` in the terminal running `go run ./cmd/glyphdeck` to stop the backend.
- Press `Ctrl+C` in the terminal running `npm run dev` to stop the frontend.

**Kill stuck port (Windows):**

If a server process is left running and the port is stuck:

```powershell
# Kill stuck backend on port 8756
$glyphdeckPortProcess = Get-NetTCPConnection -LocalPort 8756 -State Listen | Select-Object -First 1 -ExpandProperty OwningProcess
Stop-Process -Id $glyphdeckPortProcess -Force

# Kill stuck frontend on port 5173 (adjust port if Vite uses another)
$vitePortProcess = Get-NetTCPConnection -LocalPort 5173 -State Listen | Select-Object -First 1 -ExpandProperty OwningProcess
Stop-Process -Id $vitePortProcess -Force
```

**Linux/macOS:**

```bash
# Backend
lsof -ti:8756 | xargs kill -9

# Frontend
lsof -ti:5173 | xargs kill -9
```
