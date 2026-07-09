# GlyphDeck validation harness — stop dev servers (M12)
$ErrorActionPreference = "Continue"
$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoRoot = Resolve-Path (Join-Path $scriptDir "..\..")
$pidDir = Join-Path $repoRoot ".glyphdeck\validation\m12\pids"
$bp = if ($env:GLYPHDECK_PORT) { $env:GLYPHDECK_PORT } else { "8756" }
Write-Host "=== GlyphDeck M12 — Stop ==="
try { Invoke-WebRequest -Uri "http://127.0.0.1:${bp}/api/dev/stop-all-app-owned-servers" -Method POST -TimeoutSec 10 -UseBasicParsing -ErrorAction Stop | Out-Null; Write-Host "[api] Stop OK" } catch { Write-Host "[api] Unavailable" }
foreach ($pf in @("backend.pid","frontend.pid")) { $path=Join-Path $pidDir $pf; if(!(Test-Path $path)){continue}; $id=(Get-Content $path -Raw) -as [int]; if($id){try{Stop-Process -Id $id -Force -ErrorAction Stop; Write-Host "[$pf] Stopped $id"}catch{Write-Host "[$pf] Failed $id"}}; Remove-Item $path -Force -ErrorAction SilentlyContinue }
Get-Job -Name "glyphdeck-m12-*" -ErrorAction SilentlyContinue | Remove-Job -Force -ErrorAction SilentlyContinue
Write-Host "=== Stopped ==="
