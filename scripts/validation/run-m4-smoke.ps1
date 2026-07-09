# GlyphDeck validation harness -- Milestone 4 smoke test (EventBridge streaming)
# Usage: .\scripts\validation\run-m4-smoke.ps1
# Starts M4 servers, runs Playwright EventBridge streaming smoke test, stops servers.

$ErrorActionPreference = "Stop"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoRoot = Resolve-Path (Join-Path $scriptDir "..\..")
$valDir = Join-Path $repoRoot ".glyphdeck\validation\m4_recovery"
$logDir = Join-Path $valDir "logs"
$screenshotDir = Join-Path $valDir "screenshots"
$pidDir = Join-Path $valDir "pids"
$scriptsDir = Join-Path $valDir "scripts"

Write-Host "=== GlyphDeck M4 EventBridge Streaming Smoke Test ==="
Write-Host "Artifact root: $valDir"
Write-Host ""

# --- Setup -------------------------------------------------------------------
$env:GLYPHDECK_DEV_TOOLS = "1"
$env:GLYPHDECK_PROJECT_PATH = (Get-Location).Path
Write-Host "[setup] GLYPHDECK_DEV_TOOLS=1"
Write-Host "[setup] GLYPHDECK_PROJECT_PATH=$env:GLYPHDECK_PROJECT_PATH"

New-Item -ItemType Directory -Path $logDir -Force | Out-Null
New-Item -ItemType Directory -Path $screenshotDir -Force | Out-Null
New-Item -ItemType Directory -Path $pidDir -Force | Out-Null
New-Item -ItemType Directory -Path $scriptsDir -Force | Out-Null
Write-Host "[setup] Artifact directories ready."

# Delete stale screenshots.
Get-ChildItem -LiteralPath $screenshotDir -Filter "*.png" -ErrorAction SilentlyContinue |
    Remove-Item -Force -ErrorAction SilentlyContinue
Write-Host "[setup] Stale screenshots removed."

# --- Start servers -----------------------------------------------------------
Write-Host ""
Write-Host "[harness] Starting M4 dev servers..."
$startScript = Join-Path $scriptDir "start-dev-m4.ps1"
& $startScript
Write-Host ""

# --- Call reset endpoint -----------------------------------------------------
$backendPort = if ($env:GLYPHDECK_PORT) { $env:GLYPHDECK_PORT } else { "8756" }
Write-Host "[reset] Calling POST /api/dev/reset-validation-state..."
try {
    $null = Invoke-WebRequest -Uri "http://127.0.0.1:${backendPort}/api/dev/reset-validation-state" `
        -Method POST -TimeoutSec 5 -UseBasicParsing -ErrorAction Stop
    Write-Host "[reset] Validation state reset."
} catch {
    Write-Host "[reset] Endpoint not available: $($_.Exception.Message)"
    Write-Host "[reset] Continuing - state may not be clean."
}

# --- Run Playwright ----------------------------------------------------------
$playwrightScript = Join-Path $scriptsDir "m4-recovery-smoke.cjs"
Write-Host ""
Write-Host "[playwright] Running M4 smoke test..."
Write-Host "[playwright] Script: $playwrightScript"
$playwrightResult = & node $playwrightScript 2>&1
$playwrightExit = $LASTEXITCODE

Write-Host "--- Playwright output ---"
$playwrightResult | ForEach-Object { Write-Host $_ }
Write-Host "--- End Playwright output ---"
Write-Host ""

if ($playwrightExit -eq 0) {
    Write-Host "[playwright] M4 smoke test PASSED (exit 0)"
} else {
    Write-Host "[playwright] M4 smoke test FAILED (exit $playwrightExit)"
}

# --- Stop servers ------------------------------------------------------------
Write-Host ""
Write-Host "[harness] Stopping M4 dev servers..."
$stopScript = Join-Path $scriptDir "stop-dev-m4.ps1"
& $stopScript

# --- Report ------------------------------------------------------------------
Write-Host ""
Write-Host "=== GlyphDeck M4 EventBridge Streaming Smoke Test Complete ==="
if ($playwrightExit -eq 0) {
    Write-Host "Result: PASS"
} else {
    Write-Host "Result: FAIL"
}
Write-Host "Screenshots: $screenshotDir"
Write-Host "Logs: $logDir"
exit $playwrightExit