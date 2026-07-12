# GlyphDeck v0.1.2 release-candidate validation teardown.
# Stops only the exact recorded validation binary after PID/path/port verification.
$ErrorActionPreference = "Stop"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoRoot = [System.IO.Path]::GetFullPath((Join-Path $scriptDir "..\.."))
$validationRoot = [System.IO.Path]::GetFullPath((Join-Path $repoRoot ".glyphdeck\validation"))
$valDir = [System.IO.Path]::GetFullPath((Join-Path $validationRoot "mvp"))
$pidDir = [System.IO.Path]::GetFullPath((Join-Path $valDir "pids"))
$pidPath = [System.IO.Path]::GetFullPath((Join-Path $pidDir "backend.pid"))
$portPath = [System.IO.Path]::GetFullPath((Join-Path $pidDir "backend-port.txt"))
$binaryPath = [System.IO.Path]::GetFullPath((Join-Path $repoRoot "dist\glyphdeck.exe"))
$prefix = $validationRoot.TrimEnd('\') + '\'

function Assert-NoValidationReparsePoints([string]$path) {
  $current = [System.IO.Path]::GetFullPath($path)
  if (-not $current.StartsWith($prefix, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "Validation path escapes .glyphdeck\validation: $current"
  }

  while ($true) {
    if (Test-Path -LiteralPath $current) {
      $item = Get-Item -LiteralPath $current -Force -ErrorAction Stop
      if ($item.Attributes.HasFlag([System.IO.FileAttributes]::ReparsePoint)) {
        throw "Refusing validation cleanup through reparse point: $current"
      }
    }

    if ($current.Equals($repoRoot, [System.StringComparison]::OrdinalIgnoreCase)) {
      return
    }

    $parent = Split-Path -Parent $current
    if ([string]::IsNullOrWhiteSpace($parent) -or $parent.Equals($current, [System.StringComparison]::OrdinalIgnoreCase)) {
      throw "Unable to verify validation path ancestry: $path"
    }
    $current = $parent
  }
}

foreach ($path in @($pidDir, $pidPath, $portPath)) {
  if (-not $path.StartsWith($prefix, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "Validation path escapes .glyphdeck\validation: $path"
  }
  Assert-NoValidationReparsePoints $path
}

if (-not (Test-Path -LiteralPath $pidPath)) {
  Remove-Item -LiteralPath $portPath -Force -ErrorAction SilentlyContinue
  Write-Host "[mvp] No tracked validation backend PID."
  return
}

$pidText = (Get-Content -LiteralPath $pidPath -Raw -ErrorAction Stop).Trim()
$backendPid = 0
if (-not [int]::TryParse($pidText, [ref]$backendPid) -or $backendPid -le 0) {
  Remove-Item -LiteralPath $pidPath -Force -ErrorAction SilentlyContinue
  Remove-Item -LiteralPath $portPath -Force -ErrorAction SilentlyContinue
  throw "Tracked backend PID file is invalid."
}

$port = 0
if (Test-Path -LiteralPath $portPath) {
  $portText = (Get-Content -LiteralPath $portPath -Raw -ErrorAction Stop).Trim()
  if (-not [int]::TryParse($portText, [ref]$port) -or $port -le 0) {
    throw "Tracked backend port file is invalid."
  }
}

$processInfo = Get-CimInstance Win32_Process -Filter "ProcessId = $backendPid" -ErrorAction SilentlyContinue
if (-not $processInfo) {
  Remove-Item -LiteralPath $pidPath -Force -ErrorAction SilentlyContinue
  Remove-Item -LiteralPath $portPath -Force -ErrorAction SilentlyContinue
  Write-Host "[mvp] Recorded backend PID $backendPid is already gone."
  return
}

$actualPath = if ($processInfo.ExecutablePath) { [System.IO.Path]::GetFullPath($processInfo.ExecutablePath) } else { "" }
if (-not $actualPath.Equals($binaryPath, [System.StringComparison]::OrdinalIgnoreCase)) {
  throw "Refusing to stop PID $($backendPid): it is not the tracked release binary."
}

if ($port -le 0) {
  throw "Refusing to stop PID $backendPid without its tracked dynamic port."
}
$listener = Get-NetTCPConnection -LocalPort $port -State Listen -ErrorAction SilentlyContinue | Select-Object -First 1
if (-not $listener -or $listener.OwningProcess -ne $backendPid) {
  throw "Refusing to stop PID $($backendPid): it does not own recorded validation port $port."
}

try {
  Invoke-WebRequest -Uri "http://127.0.0.1:$port/api/dev/stop-all-app-owned-servers" -Method Post -TimeoutSec 10 -UseBasicParsing -ErrorAction Stop | Out-Null
  Write-Host "[mvp] App-owned OpenCode servers stopped through the validation API."
} catch {
  Write-Host "[mvp] App-owned server API unavailable during backend teardown."
}

Stop-Process -Id $backendPid -ErrorAction Stop
Remove-Item -LiteralPath $pidPath -Force -ErrorAction SilentlyContinue
Remove-Item -LiteralPath $portPath -Force -ErrorAction SilentlyContinue
Write-Host "[mvp] Stopped tracked release binary PID $backendPid."
