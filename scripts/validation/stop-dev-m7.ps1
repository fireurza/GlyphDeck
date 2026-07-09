# GlyphDeck validation harness — stop dev servers (M7)
$ErrorActionPreference = "Continue"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoRoot = Resolve-Path (Join-Path $scriptDir "..\..")
$pidDir = Join-Path $repoRoot ".glyphdeck\validation\m7\pids"
$backendPort = if ($env:GLYPHDECK_PORT) { $env:GLYPHDECK_PORT } else { "8756" }

Write-Host "=== GlyphDeck M7 — Stop Dev ==="

# Step 1: API stop
try {
    $null = Invoke-WebRequest -Uri "http://127.0.0.1:${backendPort}/api/dev/stop-all-app-owned-servers" -Method POST -TimeoutSec 10 -UseBasicParsing -ErrorAction Stop
    Write-Host "[api] stop-all-app-owned-servers OK"
} catch { Write-Host "[api] stop-all-app-owned-servers unavailable" }

# Step 2: PID cleanup
foreach ($pf in @("backend.pid", "frontend.pid")) {
    $path = Join-Path $pidDir $pf
    if (-not (Test-Path -LiteralPath $path)) { continue }
    $raw = Get-Content -LiteralPath $path -Raw -ErrorAction SilentlyContinue
    $targetPID = $raw -as [int]
    if ($targetPID) {
        try { Stop-Process -Id $targetPID -Force -ErrorAction Stop; Write-Host "[$pf] Stopped PID $targetPID" }
        catch { Write-Host "[$pf] Failed to stop PID $targetPID" }
    }
    Remove-Item -LiteralPath $path -Force -ErrorAction SilentlyContinue
}

Get-Job -Name "glyphdeck-m7-*" -ErrorAction SilentlyContinue | Remove-Job -Force -ErrorAction SilentlyContinue
Write-Host "=== Servers stopped ==="
