# GlyphDeck validation harness — stop dev servers (M6 recovery)
$ErrorActionPreference = "Continue"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoRoot = Resolve-Path (Join-Path $scriptDir "..\..")
$pidDir = Join-Path $repoRoot ".glyphdeck\validation\m6_recovery\pids"
$backendPort = if ($env:GLYPHDECK_PORT) { $env:GLYPHDECK_PORT } else { "8756" }

Write-Host "=== GlyphDeck M6 Recovery — Stop Dev ==="

# ---- Step 1: Stop app-owned OpenCode servers through GlyphDeck API ----
try {
    $null = Invoke-WebRequest -Uri "http://127.0.0.1:${backendPort}/api/dev/stop-all-app-owned-servers" `
        -Method POST -TimeoutSec 10 -UseBasicParsing -ErrorAction Stop
    Write-Host "[api] stop-all-app-owned-servers OK"
} catch { Write-Host "[api] stop-all-app-owned-servers unavailable or failed: $($_.Exception.Message)" }

# ---- Step 2: PID-based cleanup for backend/frontend ----
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

# ---- Step 3: Remove PowerShell background jobs ----
Get-Job -Name "glyphdeck-m6r-*" -ErrorAction SilentlyContinue | Remove-Job -Force -ErrorAction SilentlyContinue
Write-Host "=== Servers stopped ==="
