# GlyphDeck release build script (Windows)
$ErrorActionPreference = "Stop"
$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
Write-Host "=== GlyphDeck Release Build ==="

# Build frontend
Write-Host "[1/2] Building frontend..."
Set-Location (Join-Path $repoRoot "web")
& npm.cmd run build 2>&1 | Out-Null
if ($LASTEXITCODE -ne 0) { throw "Frontend build failed" }
Write-Host "[frontend] OK"

# Build Go binary
Write-Host "[2/2] Building Go binary..."
Set-Location $repoRoot
$outDir = Join-Path $repoRoot "dist"
New-Item -ItemType Directory -Path $outDir -Force | Out-Null
go build -o "$outDir\glyphdeck.exe" .\cmd\glyphdeck\
if ($LASTEXITCODE -ne 0) { throw "Go build failed" }
Write-Host "[binary] dist\glyphdeck.exe"

Write-Host "=== Build complete ==="
Write-Host "Run: .\dist\glyphdeck.exe"
Write-Host "Then open: http://127.0.0.1:8756"
