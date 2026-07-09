# GlyphDeck v0.1.0 release-candidate validation runner.
# All generated state remains under .glyphdeck\validation\mvp.
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
        throw "Refusing validation artifact operation through reparse point: $current"
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
$screenshotDir = Assert-ValidationPath (Join-Path $valDir "screenshots")
$pidDir = Assert-ValidationPath (Join-Path $valDir "pids")
$runtimeScriptsDir = Assert-ValidationPath (Join-Path $valDir "scripts")
$workspaceDir = Assert-ValidationPath (Join-Path $valDir "workspace")
$sourceSmoke = [System.IO.Path]::GetFullPath((Join-Path $scriptDir "mvp-smoke.cjs"))
$runtimeSmoke = Assert-ValidationPath (Join-Path $runtimeScriptsDir "mvp-smoke.cjs")
$smokeLog = Assert-ValidationPath (Join-Path $logDir "smoke.log")
$manifestPath = Assert-ValidationPath (Join-Path $screenshotDir "manifest.md")
$portPath = Assert-ValidationPath (Join-Path $pidDir "backend-port.txt")

foreach ($validationPath in @(
  $valDir,
  $logDir,
  $screenshotDir,
  $pidDir,
  $runtimeScriptsDir,
  $workspaceDir,
  $runtimeSmoke,
  $smokeLog,
  $manifestPath,
  $portPath
)) {
  Assert-NoValidationReparsePoints $validationPath
}

$notepadBefore = @(
  Get-Process -Name "notepad" -ErrorAction SilentlyContinue |
    Select-Object -ExpandProperty Id
)
$exitCode = 1

try {
  New-Item -ItemType Directory -Force -Path $logDir, $screenshotDir, $pidDir, $runtimeScriptsDir | Out-Null

  # Fresh screenshot evidence for this run only.
  Get-ChildItem -LiteralPath $screenshotDir -File -Filter "*.png" -ErrorAction SilentlyContinue |
    Remove-Item -Force
  Remove-Item -LiteralPath $manifestPath -Force -ErrorAction SilentlyContinue

  & (Join-Path $scriptDir "start-dev-mvp.ps1")
  if (-not (Test-Path -LiteralPath $portPath)) {
    throw "MVP startup did not record a backend port."
  }
  $port = (Get-Content -LiteralPath $portPath -Raw -ErrorAction Stop).Trim()
  $parsedPort = 0
  if (-not [int]::TryParse($port, [ref]$parsedPort) -or $parsedPort -le 0) {
    throw "MVP startup recorded an invalid backend port."
  }

  Copy-Item -LiteralPath $sourceSmoke -Destination $runtimeSmoke -Force
  $previousPort = $env:GLYPHDECK_PORT
  try {
    $env:GLYPHDECK_PORT = $port
    $smokeOutput = & node $runtimeSmoke 2>&1
    $smokeExitCode = $LASTEXITCODE
  } finally {
    $env:GLYPHDECK_PORT = $previousPort
  }

  $smokeOutput | Out-File -FilePath $smokeLog -Encoding utf8
  $smokeOutput | ForEach-Object { Write-Host $_ }

  if ($smokeExitCode -ne 0) {
    throw "MVP browser smoke failed with exit code $smokeExitCode."
  }
  if (-not (Test-Path -LiteralPath $manifestPath)) {
    throw "MVP browser smoke did not create a screenshot manifest."
  }
  $screenshotCount = @(Get-ChildItem -LiteralPath $screenshotDir -File -Filter "*.png").Count
  if ($screenshotCount -lt 17) {
    throw "MVP browser smoke captured $screenshotCount screenshots; expected at least 17."
  }

  $exitCode = 0
  Write-Host "[mvp] Browser smoke PASS ($screenshotCount fresh screenshots)."
} catch {
  $_ | Out-String | Out-File -FilePath $smokeLog -Encoding utf8 -Append
  Write-Host "[mvp] Browser smoke FAIL: $($_.Exception.Message)"
  $exitCode = 1
} finally {
  try {
    & (Join-Path $scriptDir "stop-dev-mvp.ps1")
  } catch {
    Write-Host "[mvp] Teardown FAIL: $($_.Exception.Message)"
    $exitCode = 1
  }

  $notepadAfter = @(
    Get-Process -Name "notepad" -ErrorAction SilentlyContinue |
      Select-Object -ExpandProperty Id
  )
  $newNotepad = @($notepadAfter | Where-Object { $_ -notin $notepadBefore })
  if ($newNotepad.Count -gt 0) {
    Write-Host "[guard] Forbidden host action: new notepad.exe PID(s): $($newNotepad -join ', ')"
    foreach ($newPid in $newNotepad) {
      $process = Get-Process -Id $newPid -ErrorAction SilentlyContinue
      if ($process -and $process.ProcessName -eq "notepad") {
        try {
          Stop-Process -Id $newPid -Force -ErrorAction Stop
          Write-Host "[guard] Closed new notepad.exe PID $newPid."
        } catch {
          Write-Host "[guard] Unable to close new notepad.exe PID $newPid."
        }
      }
    }
    $exitCode = 1
  }
}

if ($exitCode -eq 0) {
  Write-Host "Result: PASS"
} else {
  Write-Host "Result: FAIL"
}
exit $exitCode
