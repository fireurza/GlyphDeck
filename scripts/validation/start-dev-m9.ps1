# GlyphDeck validation harness - start dev servers (M9)
$ErrorActionPreference = "Stop"
$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoRoot = Resolve-Path (Join-Path $scriptDir "..\..")
$valDir = Join-Path $repoRoot ".glyphdeck\validation\m9"
$logDir = Join-Path $valDir "logs"
$pidDir = Join-Path $valDir "pids"
New-Item -ItemType Directory -Path $logDir,$pidDir -Force | Out-Null
$backendLog = Join-Path $logDir "backend.log"
$frontendLog = Join-Path $logDir "frontend.log"
$bp = if ($env:GLYPHDECK_PORT) { $env:GLYPHDECK_PORT } else { "8756" }
$fp = "5173"
Write-Host "=== GlyphDeck M9 — Start Dev ==="
try { Invoke-WebRequest -Uri "http://127.0.0.1:${bp}/api/dev/reset-validation-state" -Method POST -TimeoutSec 3 -UseBasicParsing -ErrorAction Stop | Out-Null; Write-Host "[cleanup] Reset OK"; Start-Sleep -Milliseconds 500 } catch { Write-Host "[cleanup] No existing" }
foreach ($p in @($bp,$fp)) { $e = Get-NetTCPConnection -LocalPort $p -State Listen -ErrorAction SilentlyContinue | Select -First 1; if ($e) { try { Stop-Process -Id $e.OwningProcess -Force -ErrorAction Stop; Write-Host "[cleanup] Killed PID $($e.OwningProcess)"; Start-Sleep -Milliseconds 500 } catch {} } }
Write-Host "[backend] Starting..."
Start-Job -Name "glyphdeck-m9-backend" -ScriptBlock { param($R,$L,$P) $env:GLYPHDECK_PORT=$P; Set-Location $R; & go run ./cmd/glyphdeck >> $L 2>&1 } -Arg $repoRoot,$backendLog,$bp | Out-Null
$bpid=$null; for ($i=1;$i -le 20;$i++) { try { $r=Invoke-WebRequest -Uri "http://127.0.0.1:${bp}/healthz" -Method GET -TimeoutSec 2 -UseBasicParsing -ErrorAction Stop; if ($r.StatusCode -eq 200) { $c=Get-NetTCPConnection -LocalPort $bp -State Listen -ErrorAction SilentlyContinue|Select -First 1; if ($c){$bpid=$c.OwningProcess}; break } } catch {}; Start-Sleep -Milliseconds 500 }
if ($bpid) { $bpid|Out-File (Join-Path $pidDir "backend.pid") -NoNewline -Encoding ASCII; Write-Host "[backend] PID $bpid" }
Write-Host "[frontend] Starting..."
Start-Job -Name "glyphdeck-m9-frontend" -ScriptBlock { param($W,$L) Set-Location $W; & npm run dev >> $L 2>&1 } -Arg (Join-Path $repoRoot "web"),$frontendLog | Out-Null
$fpid=$null; for ($i=1;$i -le 20;$i++) { $c=Get-NetTCPConnection -LocalPort $fp -State Listen -ErrorAction SilentlyContinue|Select -First 1; if ($c){$fpid=$c.OwningProcess; Write-Host "[frontend] Port listening ($i)"; break }; Start-Sleep -Milliseconds 500 }
if ($fpid) { $fpid|Out-File (Join-Path $pidDir "frontend.pid") -NoNewline -Encoding ASCII; Write-Host "[frontend] PID $fpid" }
Write-Host "=== Ready ==="
