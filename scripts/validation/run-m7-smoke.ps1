# GlyphDeck Milestone 7 smoke test runner
$ErrorActionPreference = "Stop"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoRoot = Resolve-Path (Join-Path $scriptDir "..\..")
$valDir = Join-Path $repoRoot ".glyphdeck\validation\m7"
$logDir = Join-Path $valDir "logs"
$screenshotDir = Join-Path $valDir "screenshots"
$pidDir = Join-Path $valDir "pids"
$scriptsRuntimeDir = Join-Path $valDir "scripts"
$workspaceDir = Join-Path $valDir "workspace"

Write-Host "=== GlyphDeck M7 Review Tab Smoke Test ==="
Write-Host "Artifact root: $valDir"

$env:GLYPHDECK_DEV_TOOLS = "1"
$env:GLYPHDECK_PROJECT_PATH = (Get-Location).Path

New-Item -ItemType Directory -Path $logDir -Force | Out-Null
New-Item -ItemType Directory -Path $screenshotDir -Force | Out-Null
New-Item -ItemType Directory -Path $pidDir -Force | Out-Null
New-Item -ItemType Directory -Path $scriptsRuntimeDir -Force | Out-Null
New-Item -ItemType Directory -Path $workspaceDir -Force | Out-Null

Get-ChildItem -LiteralPath $screenshotDir -Filter "*.png" -ErrorAction SilentlyContinue |
    Remove-Item -Force -ErrorAction SilentlyContinue
Write-Host "[setup] Stale screenshots removed."

# Start servers
Write-Host "[harness] Starting M7 dev servers..."
& (Join-Path $scriptDir "start-dev-m7.ps1")

# Sync smoke script
$committedSmoke = Join-Path $scriptDir "m7-recovery-smoke.cjs"
$runtimeSmoke = Join-Path $scriptsRuntimeDir "m7-recovery-smoke.cjs"
Copy-Item -LiteralPath $committedSmoke -Destination $runtimeSmoke -Force

# Run Playwright
Write-Host "[playwright] Running M7 smoke..."
$result = & node $runtimeSmoke 2>&1
$exit = $LASTEXITCODE
$result | ForEach-Object { Write-Host $_ }
if ($exit -eq 0) { Write-Host "[playwright] PASS" } else { Write-Host "[playwright] FAIL (exit $exit)" }

# Stop
Write-Host "[harness] Stopping..."
& (Join-Path $scriptDir "stop-dev-m7.ps1")

Write-Host ""
if ($exit -eq 0) { Write-Host "Result: PASS" } else { Write-Host "Result: FAIL" }
Write-Host "Screenshots: $screenshotDir"
exit $exit
