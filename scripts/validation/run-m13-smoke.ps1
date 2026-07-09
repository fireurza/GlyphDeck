# GlyphDeck Milestone 13 smoke test runner (release mode: binary serves frontend)
$ErrorActionPreference = "Stop"
$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoRoot = Resolve-Path (Join-Path $scriptDir "..\..")
$valDir = Join-Path $repoRoot ".glyphdeck\validation\m13"
$logDir = Join-Path $valDir "logs"
$screenshotDir = Join-Path $valDir "screenshots"
$pidDir = Join-Path $valDir "pids"
$scriptsRuntimeDir = Join-Path $valDir "scripts"
Write-Host "=== GlyphDeck M13 Settings + Release Smoke ==="
$env:GLYPHDECK_DEV_TOOLS = "1"
New-Item -ItemType Directory -Path $logDir,$screenshotDir,$pidDir,$scriptsRuntimeDir -Force | Out-Null
Get-ChildItem -LiteralPath $screenshotDir -Filter "*.png" -ErrorAction SilentlyContinue | Remove-Item -Force -ErrorAction SilentlyContinue

$notepadBefore = (Get-Process -Name "notepad" -ErrorAction SilentlyContinue | Select-Object -ExpandProperty Id)

Write-Host "[harness] Starting M13 backend (release mode)..."
& (Join-Path $scriptDir "start-dev-m13.ps1")
$cs = Join-Path $scriptDir "m13-smoke.cjs"
$rs = Join-Path $scriptsRuntimeDir "m13-smoke.cjs"
Copy-Item -LiteralPath $cs -Destination $rs -Force
$result = & node $rs 2>&1; $exit = $LASTEXITCODE
$result | ForEach-Object { Write-Host $_ }
if ($exit -eq 0) { Write-Host "[playwright] PASS" } else { Write-Host "[playwright] FAIL ($exit)" }
Write-Host "[harness] Stopping..."
& (Join-Path $scriptDir "stop-dev-m13.ps1")

$notepadAfter = (Get-Process -Name "notepad" -ErrorAction SilentlyContinue | Select-Object -ExpandProperty Id)
$newNotepad = $notepadAfter | Where-Object { $_ -notin $notepadBefore }
if ($newNotepad) {
  Write-Host "[guard] WARNING: New notepad.exe PID(s): $($newNotepad -join ', ')"
  $newNotepad | ForEach-Object { try { Stop-Process -Id $_ -Force -ErrorAction Stop } catch {} }
  Write-Host "[guard] Forbidden host action: FAIL"
  $exit = 1
}

if ($exit -eq 0) { Write-Host "Result: PASS" } else { Write-Host "Result: FAIL" }
exit $exit
