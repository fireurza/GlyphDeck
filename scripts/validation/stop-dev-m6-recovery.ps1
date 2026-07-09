# GlyphDeck validation harness — stop dev servers (M6 recovery)
$ErrorActionPreference = "Continue"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoRoot = Resolve-Path (Join-Path $scriptDir "..\..")
$pidDir = Join-Path $repoRoot ".glyphdeck\validation\m6_recovery\pids"

Write-Host "=== GlyphDeck M6 Recovery — Stop Dev ==="

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

Get-Job -Name "glyphdeck-m6r-*" -ErrorAction SilentlyContinue | Remove-Job -Force -ErrorAction SilentlyContinue
Write-Host "=== Servers stopped ==="
