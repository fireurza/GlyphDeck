# GlyphDeck validation harness — stop dev servers (M6)
# Usage: .\scripts\validation\stop-dev-m6.ps1
# Stops only recorded PIDs. Never kills by name.

$ErrorActionPreference = "Continue"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoRoot = Resolve-Path (Join-Path $scriptDir "..\..")
$valDir = Join-Path $repoRoot ".glyphdeck\validation\m6"
$pidDir = Join-Path $valDir "pids"

Write-Host "=== GlyphDeck M6 Validation Harness — Stop Dev ==="

# Stop by recorded PID files.
foreach ($pidFile in @("backend.pid", "frontend.pid")) {
    $path = Join-Path $pidDir $pidFile
    if (-not (Test-Path -LiteralPath $path)) { continue }
    $raw = Get-Content -LiteralPath $path -Raw -ErrorAction SilentlyContinue
    $targetPID = $raw -as [int]
    if ($targetPID) {
        try {
            Stop-Process -Id $targetPID -Force -ErrorAction Stop
            Write-Host "[$pidFile] Stopped PID $targetPID"
        } catch {
            Write-Host "[$pidFile] Failed to stop PID $targetPID (may already be stopped)"
        }
    }
    Remove-Item -LiteralPath $path -Force -ErrorAction SilentlyContinue
}

# Clean up job objects.
Get-Job -Name "glyphdeck-m6-*" -ErrorAction SilentlyContinue | Remove-Job -Force -ErrorAction SilentlyContinue

Write-Host "=== M6 Dev servers stopped ==="
