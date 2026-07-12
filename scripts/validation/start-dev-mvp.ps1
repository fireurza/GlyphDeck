# GlyphDeck v0.1.2 release-candidate validation startup.
# Starts only a tracked, isolated release binary; never reuses or kills a port owner.
$ErrorActionPreference = "Stop"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoRoot = [System.IO.Path]::GetFullPath((Join-Path $scriptDir "..\.."))
$validationRoot = [System.IO.Path]::GetFullPath((Join-Path $repoRoot ".glyphdeck\validation"))
$valDir = [System.IO.Path]::GetFullPath((Join-Path $validationRoot "mvp"))

function Assert-ValidationPath([string]$path) {
  $resolved = [System.IO.Path]::GetFullPath($path)
  $prefix = $validationRoot.TrimEnd('\') + '\'
  if (-not $resolved.StartsWith($prefix, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "Validation path escapes .glyphdeck\validation: $resolved"
  }
  return $resolved
}

function Assert-NoValidationReparsePoints([string]$path) {
  $current = Assert-ValidationPath $path
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

$logDir = Assert-ValidationPath (Join-Path $valDir "logs")
$pidDir = Assert-ValidationPath (Join-Path $valDir "pids")
$dataDir = Assert-ValidationPath (Join-Path $valDir "data")
$launchDir = Assert-ValidationPath (Join-Path $valDir "launch")
$backendLog = Assert-ValidationPath (Join-Path $logDir "backend.log")
$backendErrorLog = Assert-ValidationPath (Join-Path $logDir "backend-error.log")
$pidPath = Assert-ValidationPath (Join-Path $pidDir "backend.pid")
$portPath = Assert-ValidationPath (Join-Path $pidDir "backend-port.txt")
$binaryPath = [System.IO.Path]::GetFullPath((Join-Path $repoRoot "dist\glyphdeck.exe"))

if (-not (Test-Path -LiteralPath $binaryPath -PathType Leaf)) {
  throw "Release binary is missing: $binaryPath. Run scripts\build.ps1 first."
}

# Verify every validation write/delete path before creating artifacts or asking
# teardown to remove stale PID files.
foreach ($validationDirectory in @($logDir, $pidDir, $dataDir, $launchDir)) {
  Assert-NoValidationReparsePoints $validationDirectory
}
New-Item -ItemType Directory -Force -Path $logDir, $pidDir | Out-Null

# A prior tracked run is cleaned only by its recorded PID and matching dynamic port.
if ((Test-Path -LiteralPath $pidPath) -or (Test-Path -LiteralPath $portPath)) {
  & (Join-Path $scriptDir "stop-dev-mvp.ps1")
}

# This is an isolated, repo-local test data directory. Verify containment before
# deleting stale validation data from a prior run.
if (Test-Path -LiteralPath $dataDir) {
  Remove-Item -LiteralPath $dataDir -Recurse -Force
}
New-Item -ItemType Directory -Force -Path $dataDir | Out-Null
New-Item -ItemType Directory -Force -Path $launchDir | Out-Null

$listener = [System.Net.Sockets.TcpListener]::new([System.Net.IPAddress]::Loopback, 0)
try {
  $listener.Start()
  $port = $listener.LocalEndpoint.Port
} finally {
  $listener.Stop()
}

$oldPort = $env:GLYPHDECK_PORT
$oldDataDir = $env:GLYPHDECK_DATA_DIR
$oldDevTools = $env:GLYPHDECK_DEV_TOOLS
$oldAdminPass = $env:GLYPHDECK_ADMIN_PASSWORD
try {
  $env:GLYPHDECK_PORT = [string]$port
  $env:GLYPHDECK_DATA_DIR = $dataDir
  $env:GLYPHDECK_DEV_TOOLS = "1"
  $env:GLYPHDECK_ADMIN_PASSWORD = "mvp-smoke-admin-pass"

  $psi = New-Object System.Diagnostics.ProcessStartInfo
  $psi.FileName = $binaryPath
  $psi.WorkingDirectory = $launchDir
  $psi.UseShellExecute = $false
  $psi.CreateNoWindow = $true
  $psi.WindowStyle = [System.Diagnostics.ProcessWindowStyle]::Hidden
  $psi.RedirectStandardOutput = $true
  $psi.RedirectStandardError = $true
  $p = New-Object System.Diagnostics.Process
  $p.StartInfo = $psi
  $p.Start() | Out-Null

  # Capture stdout/stderr to log files.
  $stdoutJob = Start-Job -ScriptBlock { param($reader, $path) $reader.BaseStream.CopyTo([System.IO.File]::OpenWrite($path)) } -ArgumentList $p.StandardOutput, $backendLog
  $stderrJob = Start-Job -ScriptBlock { param($reader, $path) $reader.BaseStream.CopyTo([System.IO.File]::OpenWrite($path)) } -ArgumentList $p.StandardError, $backendErrorLog
  $process = $p
} finally {
  $env:GLYPHDECK_PORT = $oldPort
  $env:GLYPHDECK_DATA_DIR = $oldDataDir
  $env:GLYPHDECK_DEV_TOOLS = $oldDevTools
  $env:GLYPHDECK_ADMIN_PASSWORD = $oldAdminPass
}

[System.IO.File]::WriteAllText($pidPath, [string]$process.Id)
[System.IO.File]::WriteAllText($portPath, [string]$port)

$ready = $false
for ($attempt = 1; $attempt -le 40; $attempt++) {
  try {
    $response = Invoke-WebRequest -Uri "http://127.0.0.1:$port/healthz" -Method Get -TimeoutSec 2 -UseBasicParsing -ErrorAction Stop
    $owner = Get-NetTCPConnection -LocalPort $port -State Listen -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($response.StatusCode -eq 200 -and $owner -and $owner.OwningProcess -eq $process.Id) {
      $ready = $true
      break
    }
  } catch {
    # The app is still starting; the timeout below produces the useful failure.
  }
  Start-Sleep -Milliseconds 500
}

if (-not $ready) {
  & (Join-Path $scriptDir "stop-dev-mvp.ps1")
  throw "Release binary did not reach healthz on the tracked validation port $port."
}

Write-Host "[mvp] Started release binary PID $($process.Id) on 127.0.0.1:$port"
