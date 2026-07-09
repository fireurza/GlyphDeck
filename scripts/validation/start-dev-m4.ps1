# GlyphDeck validation harness — start dev servers as hidden background jobs (M4)
# Usage: .\scripts\validation\start-dev-m4.ps1
# Sets GLYPHDECK_DEV_TOOLS=1 if the env var is already 1.
# M4-specific: artifact paths under .glyphdeck\validation\m4\

$ErrorActionPreference = "Stop"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoRoot = Resolve-Path (Join-Path $scriptDir "..\..")
$valDir = Join-Path $repoRoot ".glyphdeck\validation\m4_recovery"
$logDir = Join-Path $valDir "logs"
$pidDir = Join-Path $valDir "pids"

# Ensure artifact directories exist.
New-Item -ItemType Directory -Path $logDir -Force | Out-Null
New-Item -ItemType Directory -Path $pidDir -Force | Out-Null

$backendLog = Join-Path $logDir "backend.log"
$frontendLog = Join-Path $logDir "frontend.log"

# Port defaults.
$backendPort = if ($env:GLYPHDECK_PORT) { $env:GLYPHDECK_PORT } else { "8756" }
$frontendPort = "5173"

Write-Host "=== GlyphDeck M4 Validation Harness — Start Dev ==="
Write-Host "Repo root: $repoRoot"
Write-Host "Backend port: $backendPort"
Write-Host "Frontend port: $frontendPort"
Write-Host ""

# ── Start backend ────────────────────────────────────────────────────────────
Write-Host "[backend] Starting Go backend as background job..."

$backendJob = Start-Job -Name "glyphdeck-m4-backend" -ScriptBlock {
    param($RepoRoot, $LogFile, $Port)
    $env:GLYPHDECK_PORT = $Port
    # Preserve GLYPHDECK_DEV_TOOLS if caller set it.
    if ($env:GLYPHDECK_DEV_TOOLS -ne "1") {
        Remove-Item Env:\GLYPHDECK_DEV_TOOLS -ErrorAction SilentlyContinue
    }
    Set-Location -LiteralPath $RepoRoot
    & go run ./cmd/glyphdeck *>> $LogFile 2>&1
} -ArgumentList $repoRoot, $backendLog, $backendPort

$backendPID = $null
$maxAttempts = 20
$healthURL = "http://127.0.0.1:${backendPort}/healthz"

for ($i = 1; $i -le $maxAttempts; $i++) {
    try {
        $resp = Invoke-WebRequest -Uri $healthURL -Method GET -TimeoutSec 2 -UseBasicParsing -ErrorAction Stop
        if ($resp.StatusCode -eq 200) {
            Write-Host "[backend] Health check OK (attempt $i)"
            # Extract PID from the job child process (go run spawns child).
            $backendPID = try {
                (Get-Job -Name "glyphdeck-m4-backend" | Receive-Job -Keep 2>$null)
                $child = Get-WmiObject Win32_Process -Filter "ParentProcessId=$($backendJob.Id)" -ErrorAction SilentlyContinue |
                    Where-Object { $_.Name -match '^(go|glyphdeck)' } |
                    Select-Object -First 1
                if ($child) { $child.ProcessId } else { $null }
            } catch { $null }

            # Fallback: find by port listener.
            if (-not $backendPID) {
                $conn = Get-NetTCPConnection -LocalPort $backendPort -State Listen -ErrorAction SilentlyContinue |
                    Select-Object -First 1
                if ($conn) { $backendPID = $conn.OwningProcess }
            }
            break
        }
    } catch { }
    Start-Sleep -Milliseconds 500
}

if ($backendPID) {
    $backendPID | Out-File -FilePath (Join-Path $pidDir "backend.pid") -NoNewline -Encoding ASCII
    Write-Host "[backend] PID $backendPID written to pids/backend.pid"
} else {
    Write-Host "[backend] WARNING: Could not determine backend PID. Job ID: $($backendJob.Id)"
    $backendJob.Id | Out-File -FilePath (Join-Path $pidDir "backend.pid") -NoNewline -Encoding ASCII
    Write-Host "[backend] Saved job ID $($backendJob.Id) as fallback."
}

# ── Start frontend ───────────────────────────────────────────────────────────
Write-Host ""
Write-Host "[frontend] Starting Vite frontend as background job..."

$frontendJob = Start-Job -Name "glyphdeck-m4-frontend" -ScriptBlock {
    param($WebDir, $LogFile)
    Set-Location -LiteralPath $WebDir
    & npm run dev *>> $LogFile 2>&1
} -ArgumentList (Join-Path $repoRoot "web"), $frontendLog

$frontendPID = $null

for ($i = 1; $i -le $maxAttempts; $i++) {
    $conn = Get-NetTCPConnection -LocalPort $frontendPort -State Listen -ErrorAction SilentlyContinue |
        Select-Object -First 1
    if ($conn) {
        $frontendPID = $conn.OwningProcess
        Write-Host "[frontend] Port $frontendPort listening (attempt $i)"
        break
    }
    Start-Sleep -Milliseconds 500
}

if ($frontendPID) {
    $frontendPID | Out-File -FilePath (Join-Path $pidDir "frontend.pid") -NoNewline -Encoding ASCII
    Write-Host "[frontend] PID $frontendPID written to pids/frontend.pid"
} else {
    Write-Host "[frontend] WARNING: Could not determine frontend PID. Job ID: $($frontendJob.Id)"
    $frontendJob.Id | Out-File -FilePath (Join-Path $pidDir "frontend.pid") -NoNewline -Encoding ASCII
    Write-Host "[frontend] Saved job ID $($frontendJob.Id) as fallback."
}

# ── Summary ──────────────────────────────────────────────────────────────────
Write-Host ""
Write-Host "=== M4 Dev servers started ==="
Write-Host "Backend : http://127.0.0.1:${backendPort}  (job: glyphdeck-m4-backend)"
Write-Host "Frontend: http://localhost:${frontendPort}   (job: glyphdeck-m4-frontend)"
Write-Host "Logs    : $logDir"
Write-Host "PIDs    : $pidDir"