# GlyphDeck build script (PowerShell)

$ErrorActionPreference = "Stop"

Write-Host "=== Building Go backend ==="
go build -o bin/glyphdeck.exe ./cmd/glyphdeck
if ($LASTEXITCODE -ne 0) { throw "Go build failed" }

Write-Host "=== Building frontend ==="
Push-Location web
npm install
npm run build
Pop-Location
if ($LASTEXITCODE -ne 0) { throw "Frontend build failed" }

Write-Host "=== Build complete ==="
Write-Host "Backend: bin/glyphdeck.exe"
Write-Host "Frontend: web/dist/"
