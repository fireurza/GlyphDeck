# GlyphDeck validation harness — stop m14 release server
$ErrorActionPreference = "Continue"
$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoRoot = Resolve-Path (Join-Path $scriptDir "..\..")
$pidDir = Join-Path $repoRoot ".glyphdeck\validation\m14\pids"
$bp = if ($env:GLYPHDECK_PORT) { $env:GLYPHDECK_PORT } else { "8756" }
Write-Host "=== GlyphDeck m14 — Stop ==="
try { Invoke-WebRequest -Uri "http://127.0.0.1:${bp}/api/dev/stop-all-app-owned-servers" -Method POST -TimeoutSec 10 -UseBasicParsing -ErrorAction Stop | Out-Null; Write-Host "[api] Stop OK" } catch { Write-Host "[api] Unavailable" }
$path=Join-Path $pidDir "backend.pid"
if (Test-Path $path) {
  $id=(Get-Content $path -Raw -ErrorAction SilentlyContinue) -as [int]
  if ($id) { try { Stop-Process -Id $id -Force -ErrorAction Stop; Write-Host "[backend.pid] Stopped $id" } catch { Write-Host "[backend.pid] Failed $id" } }
  Remove-Item $path -Force -ErrorAction SilentlyContinue
}
Get-Job -Name "glyphdeck-m14-*" -ErrorAction SilentlyContinue | Remove-Job -Force -ErrorAction SilentlyContinue
Write-Host "=== Stopped ==="
