# GlyphDeck Milestone 6 recovery smoke test runner
# Sources committed scripts under scripts/validation/.
# Runtime artifacts go under .glyphdeck/validation/m6_recovery/ (git-ignored).
$ErrorActionPreference = "Stop"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoRoot = Resolve-Path (Join-Path $scriptDir "..\..")
$valDir = Join-Path $repoRoot ".glyphdeck\validation\m6_recovery"
$logDir = Join-Path $valDir "logs"
$screenshotDir = Join-Path $valDir "screenshots"
$pidDir = Join-Path $valDir "pids"
$scriptsRuntimeDir = Join-Path $valDir "scripts"

Write-Host "=== GlyphDeck M6 Recovery Smoke Test ==="
Write-Host "Artifact root: $valDir"

$env:GLYPHDECK_DEV_TOOLS = "1"
$env:GLYPHDECK_PROJECT_PATH = (Get-Location).Path

# Create artifact directories.
New-Item -ItemType Directory -Path $logDir -Force | Out-Null
New-Item -ItemType Directory -Path $screenshotDir -Force | Out-Null
New-Item -ItemType Directory -Path $pidDir -Force | Out-Null
New-Item -ItemType Directory -Path $scriptsRuntimeDir -Force | Out-Null

# Delete stale screenshots only (never delete the whole .glyphdeck/ tree).
Get-ChildItem -LiteralPath $screenshotDir -Filter "*.png" -ErrorAction SilentlyContinue |
    Remove-Item -Force -ErrorAction SilentlyContinue
Write-Host "[setup] Stale screenshots removed."

# Start servers.
Write-Host "[harness] Starting M6 recovery dev servers..."
& (Join-Path $scriptDir "start-dev-m6-recovery.ps1")

# Sync committed Playwright script to runtime directory so __dirname resolves
# relative to the .glyphdeck/ tree (screenshots/logs expectations).
$committedSmoke = Join-Path $scriptDir "m6-recovery-smoke.cjs"
$runtimeSmoke = Join-Path $scriptsRuntimeDir "m6-recovery-smoke.cjs"
Copy-Item -LiteralPath $committedSmoke -Destination $runtimeSmoke -Force
Write-Host "[setup] Synced committed smoke script to runtime dir."

# Run Playwright (use runtime copy so __dirname is under .glyphdeck/).
$backendPort = if ($env:GLYPHDECK_PORT) { $env:GLYPHDECK_PORT } else { "8756" }
Write-Host "[playwright] Running M6 recovery smoke from committed source..."
$result = & node $runtimeSmoke 2>&1
$exit = $LASTEXITCODE

Write-Host "--- Playwright ---"
$result | ForEach-Object { Write-Host $_ }
Write-Host "--- End Playwright ---"

if ($exit -eq 0) { Write-Host "[playwright] PASS" } else { Write-Host "[playwright] FAIL (exit $exit)" }

# Stop servers (call stop-all-app-owned-servers first, then PID cleanup).
Write-Host "[harness] Stopping servers..."
& (Join-Path $scriptDir "stop-dev-m6-recovery.ps1")

Write-Host ""
if ($exit -eq 0) { Write-Host "Result: PASS" } else { Write-Host "Result: FAIL" }
Write-Host "Screenshots: $screenshotDir"
Write-Host "Logs: $logDir"
exit $exit
