# GlyphDeck Milestone 12 smoke test runner
$ErrorActionPreference = "Stop"
$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoRoot = Resolve-Path (Join-Path $scriptDir "..\..")
$valDir = Join-Path $repoRoot ".glyphdeck\validation\m12"
$logDir = Join-Path $valDir "logs"
$screenshotDir = Join-Path $valDir "screenshots"
$pidDir = Join-Path $valDir "pids"
$scriptsRuntimeDir = Join-Path $valDir "scripts"
Write-Host "=== GlyphDeck M12 State Model Cleanup Smoke ==="
$env:GLYPHDECK_DEV_TOOLS = "1"
New-Item -ItemType Directory -Path $logDir,$screenshotDir,$pidDir,$scriptsRuntimeDir -Force | Out-Null
Get-ChildItem -LiteralPath $screenshotDir -Filter "*.png" -ErrorAction SilentlyContinue | Remove-Item -Force -ErrorAction SilentlyContinue
Write-Host "[harness] Starting M12 dev servers..."
& (Join-Path $scriptDir "start-dev-m12.ps1")
$cs = Join-Path $scriptDir "m12-smoke.cjs"
$rs = Join-Path $scriptsRuntimeDir "m12-smoke.cjs"
Copy-Item -LiteralPath $cs -Destination $rs -Force
$result = & node $rs 2>&1; $exit = $LASTEXITCODE
$result | ForEach-Object { Write-Host $_ }
if ($exit -eq 0) { Write-Host "[playwright] PASS" } else { Write-Host "[playwright] FAIL ($exit)" }
Write-Host "[harness] Stopping..."
& (Join-Path $scriptDir "stop-dev-m12.ps1")
if ($exit -eq 0) { Write-Host "Result: PASS" } else { Write-Host "Result: FAIL" }
exit $exit
