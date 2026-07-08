# GlyphDeck validation harness — Milestone 3 smoke test
# Usage: .\scripts\validation\run-m3-smoke.ps1
# Starts servers, runs Playwright smoke test, stops servers.

$ErrorActionPreference = "Stop"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoRoot = Resolve-Path (Join-Path $scriptDir "..\..")
$valDir = Join-Path $repoRoot ".glyphdeck\validation\m3_5"
$logDir = Join-Path $valDir "logs"
$screenshotDir = Join-Path $valDir "screenshots"
$pidDir = Join-Path $valDir "pids"
$scriptsDir = Join-Path $valDir "scripts"

Write-Host "=== GlyphDeck M3 Smoke Test ==="
Write-Host "Artifact root: $valDir"
Write-Host ""

# ── Setup ────────────────────────────────────────────────────────────────────
$env:GLYPHDECK_DEV_TOOLS = "1"
$env:GLYPHDECK_PROJECT_PATH = (Get-Location).Path
Write-Host "[setup] GLYPHDECK_DEV_TOOLS=1"
Write-Host "[setup] GLYPHDECK_PROJECT_PATH=$env:GLYPHDECK_PROJECT_PATH"

New-Item -ItemType Directory -Path $logDir -Force | Out-Null
New-Item -ItemType Directory -Path $screenshotDir -Force | Out-Null
New-Item -ItemType Directory -Path $pidDir -Force | Out-Null
New-Item -ItemType Directory -Path $scriptsDir -Force | Out-Null
Write-Host "[setup] Artifact directories ready."

# Delete stale screenshots.
Get-ChildItem -LiteralPath $screenshotDir -Filter "*.png" -ErrorAction SilentlyContinue |
    Remove-Item -Force -ErrorAction SilentlyContinue
Write-Host "[setup] Stale screenshots removed."

# ── Start servers ────────────────────────────────────────────────────────────
Write-Host ""
Write-Host "[harness] Starting dev servers..."
$startScript = Join-Path $scriptDir "start-dev.ps1"
& $startScript
Write-Host ""

# ── Call reset endpoint ──────────────────────────────────────────────────────
$backendPort = if ($env:GLYPHDECK_PORT) { $env:GLYPHDECK_PORT } else { "8756" }
Write-Host "[reset] Calling POST /api/dev/reset-validation-state..."
try {
    $null = Invoke-WebRequest -Uri "http://127.0.0.1:${backendPort}/api/dev/reset-validation-state" `
        -Method POST -TimeoutSec 5 -UseBasicParsing -ErrorAction Stop
    Write-Host "[reset] Validation state reset."
} catch {
    Write-Host "[reset] Endpoint not available: $($_.Exception.Message)"
    Write-Host "[reset] Continuing — state may not be clean."
}

# ── Write Playwright script ──────────────────────────────────────────────────
$playwrightScript = Join-Path $scriptsDir "smoke-test.cjs"

$playwrightCode = @'
const { chromium } = require('playwright');
const path = require('path');
const fs = require('fs');

const outDir = path.join(__dirname, '..', 'screenshots');
if (!fs.existsSync(outDir)) fs.mkdirSync(outDir, { recursive: true });

const BASE = 'http://localhost:5173';
const projectPath = process.env.GLYPHDECK_PROJECT_PATH || 'C:\\Users\\Fireurza\\Documents\\Code\\GlyphDeck';

async function waitFor(predicate, timeoutMs = 30000, pollMs = 500) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    if (await predicate()) return true;
    await new Promise(r => setTimeout(r, pollMs));
  }
  return false;
}

(async () => {
  const browser = await chromium.launch({ headless: true });
  const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 } });
  const page = await ctx.newPage();

  await page.goto(BASE, { waitUntil: 'networkidle' });
  await page.waitForTimeout(1000);

  // 1. Clean state
  await page.screenshot({ path: path.join(outDir, '01-clean-state.png'), fullPage: true });
  console.log('01-clean-state');

  // 2. Add project
  await page.getByTestId('project-name-input').fill('GlyphDeck');
  await page.getByTestId('project-path-input').fill(projectPath);
  await page.getByTestId('project-trusted-checkbox').check();
  await page.getByTestId('add-project-button').click();
  await page.waitForTimeout(1000);
  await page.screenshot({ path: path.join(outDir, '02-project-added.png') });
  console.log('02-project-added');

  // 3. Start server and wait for ready
  await page.getByTestId('project-start-server-button').click();
  const ready = await waitFor(async () => {
    try {
      const status = await page.getByTestId('server-status').textContent();
      return status && status.includes('Ready');
    } catch { return false; }
  }, 30000);
  console.log('Server ready:', ready);
  await page.screenshot({ path: path.join(outDir, '03-server-ready.png') });
  console.log('03-server-ready');

  // 4. Select project (click Select button)
  await page.getByTestId('project-select-button').click();
  await page.waitForTimeout(500);

  // 5. Create session
  await page.getByTestId('create-session-button').click();
  await page.waitForTimeout(1500);
  await page.screenshot({ path: path.join(outDir, '04-session-created.png') });
  console.log('04-session-created');

  // 6. Click session item
  const sessionItems = page.getByTestId('session-item');
  const count = await sessionItems.count();
  if (count > 0) {
    await sessionItems.first().click();
    await page.waitForTimeout(500);
  }

  // 7. Type and send prompt
  await page.getByTestId('prompt-composer-input').fill('Inspect this repo and list the validation commands from README.');
  await page.screenshot({ path: path.join(outDir, '05-prompt-sent.png') });
  console.log('05-prompt-sent');
  await page.getByTestId('prompt-send-button').click();
  console.log('Send clicked');

  // 8. Wait for assistant response
  const responseAppeared = await waitFor(async () => {
    return (await page.getByTestId('transcript-assistant-message').count()) > 0;
  }, 120000);
  console.log('Assistant response:', responseAppeared);
  await page.screenshot({ path: path.join(outDir, '06-assistant-response-visible.png') });
  console.log('06-assistant-response');

  // 9. Stop server
  await page.getByTestId('project-stop-server-button').click();
  await page.waitForTimeout(2000);
  await page.screenshot({ path: path.join(outDir, '07-server-stopped.png') });
  console.log('07-server-stopped');

  // 10. Full layout
  await page.screenshot({ path: path.join(outDir, '08-full-layout.png'), fullPage: true });
  console.log('08-full-layout');

  await browser.close();
  console.log('DONE');
})().catch(e => { console.error('FATAL:', e.message); process.exit(1); });
'@

Set-Content -LiteralPath $playwrightScript -Value $playwrightCode -Encoding UTF8 -NoNewline
Write-Host "[playwright] Script written to $playwrightScript"

# ── Run Playwright ───────────────────────────────────────────────────────────
Write-Host ""
Write-Host "[playwright] Running smoke test..."
$playwrightResult = & node $playwrightScript 2>&1
$playwrightExit = $LASTEXITCODE

Write-Host "--- Playwright output ---"
$playwrightResult | ForEach-Object { Write-Host $_ }
Write-Host "--- End Playwright output ---"
Write-Host ""

if ($playwrightExit -eq 0) {
    Write-Host "[playwright] Smoke test PASSED (exit 0)"
} else {
    Write-Host "[playwright] Smoke test FAILED (exit $playwrightExit)"
}

# ── Stop servers ─────────────────────────────────────────────────────────────
Write-Host ""
Write-Host "[harness] Stopping dev servers..."
$stopScript = Join-Path $scriptDir "stop-dev.ps1"
& $stopScript

# ── Report ───────────────────────────────────────────────────────────────────
Write-Host ""
Write-Host "=== GlyphDeck M3 Smoke Test Complete ==="
if ($playwrightExit -eq 0) {
    Write-Host "Result: PASS"
} else {
    Write-Host "Result: FAIL"
}
Write-Host "Screenshots: $screenshotDir"
Write-Host "Logs: $logDir"
exit $playwrightExit
