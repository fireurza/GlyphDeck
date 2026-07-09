# GlyphDeck Milestone 6 recovery smoke test runner
$ErrorActionPreference = "Stop"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoRoot = Resolve-Path (Join-Path $scriptDir "..\..")
$valDir = Join-Path $repoRoot ".glyphdeck\validation\m6_recovery"
$logDir = Join-Path $valDir "logs"
$screenshotDir = Join-Path $valDir "screenshots"
$pidDir = Join-Path $valDir "pids"
$scriptsDir = Join-Path $valDir "scripts"

Write-Host "=== GlyphDeck M6 Recovery Smoke Test ==="
Write-Host "Artifact root: $valDir"

$env:GLYPHDECK_DEV_TOOLS = "1"
$env:GLYPHDECK_PROJECT_PATH = (Get-Location).Path

New-Item -ItemType Directory -Path $logDir -Force | Out-Null
New-Item -ItemType Directory -Path $screenshotDir -Force | Out-Null
New-Item -ItemType Directory -Path $pidDir -Force | Out-Null
New-Item -ItemType Directory -Path $scriptsDir -Force | Out-Null

Get-ChildItem -LiteralPath $screenshotDir -Filter "*.png" -ErrorAction SilentlyContinue | Remove-Item -Force -ErrorAction SilentlyContinue
Write-Host "[setup] Stale screenshots removed."

# Start servers
Write-Host "[harness] Starting M6 recovery dev servers..."
& (Join-Path $scriptDir "start-dev-m6-recovery.ps1")

# Reset state
$backendPort = if ($env:GLYPHDECK_PORT) { $env:GLYPHDECK_PORT } else { "8756" }
try {
    $null = Invoke-WebRequest -Uri "http://127.0.0.1:${backendPort}/api/dev/reset-validation-state" -Method POST -TimeoutSec 5 -UseBasicParsing -ErrorAction Stop
    Write-Host "[reset] Validation state reset."
} catch { Write-Host "[reset] Endpoint unavailable (continuing)" }

# Run Playwright
$ps = Join-Path $scriptsDir "m6-recovery-smoke.cjs"
Write-Host "[playwright] Running M6 recovery smoke..."
$result = & node $ps 2>&1
$exit = $LASTEXITCODE

Write-Host "--- Playwright ---"
$result | ForEach-Object { Write-Host $_ }
Write-Host "--- End Playwright ---"

if ($exit -eq 0) { Write-Host "[playwright] PASS" } else { Write-Host "[playwright] FAIL (exit $exit)" }

# Stop servers
Write-Host "[harness] Stopping servers..."
& (Join-Path $scriptDir "stop-dev-m6-recovery.ps1")

Write-Host ""
if ($exit -eq 0) { Write-Host "Result: PASS" } else { Write-Host "Result: FAIL" }
Write-Host "Screenshots: $screenshotDir"
Write-Host "Logs: $logDir"
exit $exit
