# GlyphDeck validation harness — stop M4 dev servers using only recorded PIDs
# Usage: .\scripts\validation\stop-dev-m4.ps1
# NEVER uses Get-Process -Name, Stop-Process -Name, or taskkill /IM.

$ErrorActionPreference = "Continue"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoRoot = Resolve-Path (Join-Path $scriptDir "..\..")
$pidDir = Join-Path $repoRoot ".glyphdeck\validation\m4_recovery\pids"

Write-Host "=== GlyphDeck M4 Validation Harness — Stop Dev ==="

# ── Dev tools: ask GlyphDeck to stop its own tracked servers ────────────────
if ($env:GLYPHDECK_DEV_TOOLS -eq "1") {
    Write-Host "[dev-tools] Calling POST /api/dev/stop-all-app-owned-servers..."
    $backendPort = if ($env:GLYPHDECK_PORT) { $env:GLYPHDECK_PORT } else { "8756" }
    try {
        $null = Invoke-WebRequest -Uri "http://127.0.0.1:${backendPort}/api/dev/stop-all-app-owned-servers" `
            -Method POST -TimeoutSec 5 -UseBasicParsing -ErrorAction SilentlyContinue
        Write-Host "[dev-tools] Stop-all request sent (response ignored if endpoint not yet registered)."
    } catch {
        Write-Host "[dev-tools] Endpoint not available or call failed: $($_.Exception.Message)"
    }
}

# ── Stop backend by recorded PID ─────────────────────────────────────────────
$backendPidFile = Join-Path $pidDir "backend.pid"
if (Test-Path -LiteralPath $backendPidFile) {
    $backendPID = (Get-Content -Raw -LiteralPath $backendPidFile).Trim()
    if ($backendPID -match '^\d+$') {
        Write-Host "[backend] Stopping process PID $backendPID..."
        Stop-Process -Id ([int]$backendPID) -Force -ErrorAction SilentlyContinue
        Write-Host "[backend] PID $backendPID stopped."
    } else {
        Write-Host "[backend] PID file contains non-numeric value: $backendPID — skipping process stop."
    }
    Remove-Item -LiteralPath $backendPidFile -Force -ErrorAction SilentlyContinue
} else {
    Write-Host "[backend] No PID file found at $backendPidFile"
}

# Stop the PowerShell job itself.
Get-Job -Name "glyphdeck-m4-backend" -ErrorAction SilentlyContinue | Stop-Job -ErrorAction SilentlyContinue
Get-Job -Name "glyphdeck-m4-backend" -ErrorAction SilentlyContinue | Remove-Job -Force -ErrorAction SilentlyContinue
Write-Host "[backend] M4 PowerShell job stopped and removed."

# ── Stop frontend by recorded PID ────────────────────────────────────────────
$frontendPidFile = Join-Path $pidDir "frontend.pid"
if (Test-Path -LiteralPath $frontendPidFile) {
    $frontendPID = (Get-Content -Raw -LiteralPath $frontendPidFile).Trim()
    if ($frontendPID -match '^\d+$') {
        Write-Host "[frontend] Stopping process PID $frontendPID..."
        Stop-Process -Id ([int]$frontendPID) -Force -ErrorAction SilentlyContinue
        Write-Host "[frontend] PID $frontendPID stopped."
    } else {
        Write-Host "[frontend] PID file contains non-numeric value: $frontendPID — skipping process stop."
    }
    Remove-Item -LiteralPath $frontendPidFile -Force -ErrorAction SilentlyContinue
} else {
    Write-Host "[frontend] No PID file found at $frontendPidFile"
}

# Stop the PowerShell job itself.
Get-Job -Name "glyphdeck-m4-frontend" -ErrorAction SilentlyContinue | Stop-Job -ErrorAction SilentlyContinue
Get-Job -Name "glyphdeck-m4-frontend" -ErrorAction SilentlyContinue | Remove-Job -Force -ErrorAction SilentlyContinue
Write-Host "[frontend] M4 PowerShell job stopped and removed."

# ── Confirm ports are clear ──────────────────────────────────────────────────
$backendPort = if ($env:GLYPHDECK_PORT) { $env:GLYPHDECK_PORT } else { "8756" }
$frontendPort = "5173"

$backendConn = Get-NetTCPConnection -LocalPort $backendPort -State Listen -ErrorAction SilentlyContinue
if ($backendConn) {
    Write-Host "[WARN] Port $backendPort still has a listener (PID $($backendConn.OwningProcess))."
} else {
    Write-Host "[backend] Port $backendPort clear."
}

$frontendConn = Get-NetTCPConnection -LocalPort $frontendPort -State Listen -ErrorAction SilentlyContinue
if ($frontendConn) {
    Write-Host "[WARN] Port $frontendPort still has a listener (PID $($frontendConn.OwningProcess))."
} else {
    Write-Host "[frontend] Port $frontendPort clear."
}

Write-Host "=== M4 Dev servers stopped ==="