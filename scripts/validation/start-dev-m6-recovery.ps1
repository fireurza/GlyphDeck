# GlyphDeck validation harness — start dev servers (M6 recovery)
$ErrorActionPreference = "Stop"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoRoot = Resolve-Path (Join-Path $scriptDir "..\..\..\..")
$valDir = Join-Path $repoRoot ".glyphdeck\validation\m6_recovery"
$logDir = Join-Path $valDir "logs"
$pidDir = Join-Path $valDir "pids"

New-Item -ItemType Directory -Path $logDir -Force | Out-Null
New-Item -ItemType Directory -Path $pidDir -Force | Out-Null

$backendLog = Join-Path $logDir "backend.log"
$frontendLog = Join-Path $logDir "frontend.log"
$backendPort = if ($env:GLYPHDECK_PORT) { $env:GLYPHDECK_PORT } else { "8756" }
$frontendPort = "5173"

Write-Host "=== GlyphDeck M6 Recovery — Start Dev ==="

# Start backend
Write-Host "[backend] Starting..."
$backendJob = Start-Job -Name "glyphdeck-m6r-backend" -ScriptBlock {
    param($Root, $Log, $Port)
    $env:GLYPHDECK_PORT = $Port
    if ($env:GLYPHDECK_DEV_TOOLS -ne "1") { Remove-Item Env:\GLYPHDECK_DEV_TOOLS -ErrorAction SilentlyContinue }
    Set-Location -LiteralPath $Root
    & go run ./cmd/glyphdeck *>> $Log 2>&1
} -ArgumentList $repoRoot, $backendLog, $backendPort

$backendPID = $null
for ($i = 1; $i -le 20; $i++) {
    try {
        $resp = Invoke-WebRequest -Uri "http://127.0.0.1:${backendPort}/healthz" -Method GET -TimeoutSec 2 -UseBasicParsing -ErrorAction Stop
        if ($resp.StatusCode -eq 200) {
            Write-Host "[backend] Health OK (attempt $i)"
            $conn = Get-NetTCPConnection -LocalPort $backendPort -State Listen -ErrorAction SilentlyContinue | Select-Object -First 1
            if ($conn) { $backendPID = $conn.OwningProcess }
            break
        }
    } catch { }
    Start-Sleep -Milliseconds 500
}

if ($backendPID) {
    $backendPID | Out-File -FilePath (Join-Path $pidDir "backend.pid") -NoNewline -Encoding ASCII
    Write-Host "[backend] PID $backendPID"
}

# Start frontend
Write-Host "[frontend] Starting..."
$frontendJob = Start-Job -Name "glyphdeck-m6r-frontend" -ScriptBlock {
    param($WebDir, $Log)
    Set-Location -LiteralPath $WebDir
    & npm run dev *>> $Log 2>&1
} -ArgumentList (Join-Path $repoRoot "web"), $frontendLog

$frontendPID = $null
for ($i = 1; $i -le 20; $i++) {
    $conn = Get-NetTCPConnection -LocalPort $frontendPort -State Listen -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($conn) { $frontendPID = $conn.OwningProcess; Write-Host "[frontend] Port listening (attempt $i)"; break }
    Start-Sleep -Milliseconds 500
}
if ($frontendPID) {
    $frontendPID | Out-File -FilePath (Join-Path $pidDir "frontend.pid") -NoNewline -Encoding ASCII
    Write-Host "[frontend] PID $frontendPID"
}

Write-Host "=== Servers started ==="
