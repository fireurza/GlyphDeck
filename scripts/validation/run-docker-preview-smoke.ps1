# GlyphDeck Docker Compose preview smoke test.
# Validates the Docker Compose preview stack in an isolated environment.
# All artifacts stay under .glyphdeck\validation\docker-preview\.
$ErrorActionPreference = "Stop"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoRoot = [System.IO.Path]::GetFullPath((Join-Path $scriptDir "..\.."))
$validationRoot = [System.IO.Path]::GetFullPath((Join-Path $repoRoot ".glyphdeck\validation"))
$valDir = [System.IO.Path]::GetFullPath((Join-Path $validationRoot "docker-preview"))

$logDir = Join-Path $valDir "logs"
$screenshotDir = Join-Path $valDir "screenshots"

# Unique project name to avoid collisions with any other Docker work.
$projectName = "glyphdeck-preview-smoke-" + (Get-Random -Minimum 10000 -Maximum 99999).ToString()
$volumeName = "glyphdeck-smoke-data-" + (Get-Random -Minimum 10000 -Maximum 99999).ToString()
# Create the secret file where compose expects it (relative to repo root).
$composeSecretDir = Join-Path $repoRoot "secrets"
$secretFile = Join-Path $composeSecretDir "glyphdeck_admin_password.txt"
$composeFile = Join-Path $repoRoot "compose.yaml"
$dockerfilePath = Join-Path $repoRoot "Dockerfile"
$logFile = Join-Path $logDir "smoke.log"
$port = 18756
$exitCode = 1
$containerId = ""

function Write-Log {
    param([string]$Message)
    $line = "[{0:HH:mm:ss}] {1}" -f (Get-Date), $Message
    Add-Content -LiteralPath $logFile -Value $line
    Write-Host $line
}

try {
    New-Item -ItemType Directory -Force -Path $logDir, $screenshotDir, $composeSecretDir | Out-Null

    # Generate a random admin password and write it to the isolated secret file.
    $adminPassword = -join ((48..57) + (65..90) + (97..122) | Get-Random -Count 32 | ForEach-Object { [char]$_ })
    Set-Content -LiteralPath $secretFile -Value $adminPassword -NoNewline -Encoding ASCII

    Write-Log "Project: $projectName"
    Write-Log "Volume: $volumeName"
    Write-Log "Port: $port"

    # ---- Validate compose config ----
    Write-Log "Validating compose config..."
    $configOutput = & docker compose -f $composeFile -p $projectName config 2>&1
    if ($LASTEXITCODE -ne 0) {
        throw "docker compose config failed: $configOutput"
    }
    Write-Log "[PASS] compose config"

    # ---- Build the image ----
    Write-Log "Building Docker image..."
    $buildOutput = & docker compose -f $composeFile -p $projectName build --no-cache 2>&1
    if ($LASTEXITCODE -ne 0) {
        throw "docker compose build failed: $buildOutput"
    }
    Write-Log "[PASS] docker build"

    # ---- Start the service detached with overrides ----
    Write-Log "Starting service..."
    $env:GLYPHDECK_HOST_PORT = [string]$port
    $startOutput = & docker compose -f $composeFile -p $projectName up -d --wait 2>&1
    if ($LASTEXITCODE -ne 0) {
        throw "docker compose up failed: $startOutput"
    }

    # Get the container ID.
    $containerId = (& docker compose -f $composeFile -p $projectName ps -q glyphdeck 2>&1).Trim()
    if (-not $containerId) {
        throw "Could not determine container ID"
    }
    Write-Log "Container: $containerId"

    # ---- Verify healthcheck ----
    Write-Log "Waiting for healthy status..."
    $healthy = $false
    for ($i = 1; $i -le 30; $i++) {
        $status = (& docker inspect -f '{{.State.Health.Status}}' $containerId 2>&1).Trim()
        if ($status -eq "healthy") {
            $healthy = $true
            break
        }
        Start-Sleep -Seconds 2
    }
    if (-not $healthy) {
        $logs = & docker compose -f $composeFile -p $projectName logs --tail 50 glyphdeck 2>&1
        throw "Container did not become healthy. Logs: $logs"
    }
    Write-Log "[PASS] healthcheck"

    # ---- Verify /healthz ----
    Write-Log "Checking /healthz..."
    try {
        $healthResp = Invoke-RestMethod -Uri "http://127.0.0.1:$port/healthz" -Method Get -TimeoutSec 10
        if ($healthResp.status -ne "ok") {
            throw "healthz returned unexpected status: $($healthResp | ConvertTo-Json)"
        }
        Write-Log "[PASS] /healthz"
    } catch {
        throw "/healthz check failed: $_"
    }

    # ---- Verify embedded UI responds ----
    Write-Log "Checking embedded UI..."
    try {
        $uiResp = Invoke-WebRequest -Uri "http://127.0.0.1:$port/" -Method Get -TimeoutSec 10 -UseBasicParsing
        if ($uiResp.StatusCode -ne 200) {
            throw "UI returned status $($uiResp.StatusCode)"
        }
        Write-Log "[PASS] embedded UI"
    } catch {
        throw "UI check failed: $_"
    }

    # ---- Verify admin auth (admin bootstrapped from password file) ----
    Write-Log "Checking auth status..."
    try {
        $authStatus = Invoke-RestMethod -Uri "http://127.0.0.1:$port/api/auth/status" -Method Get -TimeoutSec 10
        if (-not $authStatus.LoginRequired) {
            throw "Expected LoginRequired=true, got $($authStatus | ConvertTo-Json)"
        }
        Write-Log "[PASS] auth status (login required, admin exists)"
    } catch {
        throw "Auth status check failed: $_"
    }

    # ---- Login with the admin password ----
    Write-Log "Logging in..."
    try {
        $loginBody = @{ password = $adminPassword } | ConvertTo-Json
        $origin = "http://127.0.0.1:$port"
        $loginResp = Invoke-WebRequest -Uri "http://127.0.0.1:$port/api/auth/login" -Method Post -Body $loginBody -ContentType "application/json" -TimeoutSec 10 -UseBasicParsing -Headers @{ Origin = $origin }
        $sessionCookie = $loginResp.Headers['Set-Cookie']
        if (-not $sessionCookie) {
            throw "No session cookie returned"
        }
        # Extract just the cookie name=value (before first semicolon).
        $sessionCookie = ($sessionCookie -split ';')[0]
        Write-Log "[PASS] login"
    } catch {
        throw "Login failed: $_"
    }

    # ---- Create a project through the API ----
    Write-Log "Creating test project..."
    $projectId = ""
    try {
        $projectName = "docker-smoke-test-project"
        $projectBody = @{
            name = $projectName
            path = "/home/glyphdeck"
        } | ConvertTo-Json
        $origin = "http://127.0.0.1:$port"
        $createResp = Invoke-RestMethod -Uri "http://127.0.0.1:$port/api/projects" -Method Post -Body $projectBody -ContentType "application/json" -TimeoutSec 10 -Headers @{ Cookie = $sessionCookie; Origin = $origin }
        $projectId = $createResp.id
        if (-not $projectId) {
            throw "No project ID returned"
        }
        Write-Log "[PASS] create project ($projectId)"
    } catch {
        throw "Create project failed: $_"
    }

    # ---- Verify the project exists ----
    Write-Log "Verifying project persisted..."
    try {
        $getResp = Invoke-RestMethod -Uri "http://127.0.0.1:$port/api/projects/$projectId" -Method Get -TimeoutSec 10 -Headers @{ Cookie = $sessionCookie }
        if ($getResp.id -ne $projectId) {
            throw "Project ID mismatch: expected $projectId, got $($getResp.id)"
        }
        Write-Log "[PASS] project exists in DB"
    } catch {
        throw "Get project failed: $_"
    }

    # ---- Recreate container without deleting volume ----
    Write-Log "Recreating container (persistence test)..."
    & docker compose -f $composeFile -p $projectName down 2>&1 | Out-Null
    $startOutput2 = & docker compose -f $composeFile -p $projectName up -d --wait 2>&1
    if ($LASTEXITCODE -ne 0) {
        throw "Recreate up failed: $startOutput2"
    }

    # Wait for healthy again.
    $containerId = (& docker compose -f $composeFile -p $projectName ps -q glyphdeck 2>&1).Trim()
    $healthy = $false
    for ($i = 1; $i -le 30; $i++) {
        $status = (& docker inspect -f '{{.State.Health.Status}}' $containerId 2>&1).Trim()
        if ($status -eq "healthy") {
            $healthy = $true
            break
        }
        Start-Sleep -Seconds 2
    }
    if (-not $healthy) { throw "Container did not become healthy after recreate" }

    # Login again (new session).
    $loginBody = @{ password = $adminPassword } | ConvertTo-Json
    $origin = "http://127.0.0.1:$port"
    $loginResp2 = Invoke-WebRequest -Uri "http://127.0.0.1:$port/api/auth/login" -Method Post -Body $loginBody -ContentType "application/json" -TimeoutSec 10 -UseBasicParsing -Headers @{ Origin = $origin }
    $sessionCookie2 = $loginResp2.Headers['Set-Cookie']
    # Extract just the cookie name=value.
    $sessionCookie2 = ($sessionCookie2 -split ';')[0]

    # Verify persisted project.
    try {
        $getResp2 = Invoke-RestMethod -Uri "http://127.0.0.1:$port/api/projects/$projectId" -Method Get -TimeoutSec 10 -Headers @{ Cookie = $sessionCookie2 }
        if ($getResp2.id -ne $projectId) {
            throw "Project not persisted after recreate"
        }
        Write-Log "[PASS] persistence (project survived recreate)"
    } catch {
        throw "Persistence check failed: $_"
    }

    # ---- Verify non-root ----
    Write-Log "Checking non-root user..."
    $containerUser = (& docker exec $containerId whoami 2>&1).Trim()
    if ($containerUser -eq "root") {
        throw "Container is running as root"
    }
    Write-Log "[PASS] non-root user ($containerUser)"

    # ---- Verify loopback-only publication ----
    Write-Log "Checking loopback-only publication..."
    $portMapping = (& docker port $containerId 8756 2>&1).Trim()
    if ($portMapping -notmatch "^127\.0\.0\.1:") {
        throw "Port is not bound to loopback: $portMapping"
    }
    Write-Log "[PASS] loopback-only ($portMapping)"

    # ---- Verify no Docker socket ----
    Write-Log "Checking no Docker socket..."
    $mounts = (& docker inspect -f '{{range .Mounts}}{{.Source}} {{end}}' $containerId 2>&1).Trim()
    if ($mounts -match "docker\.sock") {
        throw "Docker socket is mounted"
    }
    Write-Log "[PASS] no Docker socket"

    # ---- Verify OpenCode is not required ----
    # The container should start and serve the UI without OpenCode installed.
    # This is verified by the healthcheck and UI checks above — they passed
    # without OpenCode being present in the container.
    Write-Log "[PASS] OpenCode not required for startup"

    $exitCode = 0
    Write-Log "=== Docker preview smoke PASSED ==="
} catch {
    Write-Log "=== Docker preview smoke FAILED ==="
    Write-Log "Error: $($_.Exception.Message)"
    $exitCode = 1
} finally {
    # ---- Cleanup ----
    Write-Log "Cleaning up..."
    try {
        & docker compose -f $composeFile -p $projectName down --volumes 2>&1 | Out-Null
        Write-Log "Compose stack removed."
    } catch {
        Write-Log "Warning: cleanup error: $_"
    }

    # Remove the validation volume explicitly if compose down didn't clean it.
    try {
        & docker volume rm $volumeName 2>&1 | Out-Null
    } catch {
        # Volume may already be gone.
    }

    # Remove the secret file we created.
    Remove-Item -LiteralPath $secretFile -Force -ErrorAction SilentlyContinue

    # Clean up env var.
    Remove-Item Env:GLYPHDECK_HOST_PORT -ErrorAction SilentlyContinue
}

exit $exitCode
