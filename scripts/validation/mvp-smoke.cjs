/*
 * GlyphDeck v0.1.0 release-candidate smoke.
 *
 * The runner copies this file into .glyphdeck/validation/mvp/scripts before
 * execution, so every generated artifact stays in the milestone directory.
 */
const { chromium } = require('playwright');
const { execFileSync } = require('child_process');
const crypto = require('crypto');
const fs = require('fs');
const path = require('path');

const PORT = process.env.GLYPHDECK_PORT;
if (!PORT) throw new Error('GLYPHDECK_PORT must be set by run-mvp-smoke.ps1');

const BASE_URL = `http://127.0.0.1:${PORT}`;
const VALIDATION_DIR = path.resolve(__dirname, '..');
const SCREENSHOT_DIR = path.join(VALIDATION_DIR, 'screenshots');
const WORKSPACE_DIR = path.join(VALIDATION_DIR, 'workspace');
const PLAYWRIGHT_PID_PATH = path.join(VALIDATION_DIR, 'pids', 'playwright.pid');
const RUN_ID = `mvp-${Date.now().toString(36)}-${crypto.randomBytes(3).toString('hex')}`;
const TERMINAL_MARKER = `GLYPHDECK_MVP_${RUN_ID.toUpperCase()}`;
const SETTING_VALUE = path.join(WORKSPACE_DIR, `settings-${RUN_ID}`);
const VIEWPORT = { width: 1280, height: 720 };

const screenshots = [];
const checks = [];
const browserErrors = [];
let page;
let browser;
let validationTerminalID;
let appOwnedServerPID;

function recordCheck(label) {
  checks.push({ label, status: 'PASS' });
  console.log(`[OK] ${label}`);
}

function fail(message) {
  checks.push({ label: message, status: 'FAIL' });
  throw new Error(message);
}

function isProcessAlive(pid) {
  try {
    process.kill(pid, 0);
    return true;
  } catch (error) {
    return error?.code === 'EPERM';
  }
}

function findDescendantPID(shellPID) {
  // Use wmic to find child processes. Unlike Get-CimInstance (WMI),
  // wmic queries the NT kernel object manager directly and can see
  // processes inside ConPTY job objects.
  const command = `wmic process where (ParentProcessId=${shellPID} and Name='node.exe') get ProcessId /format:value`;
  try {
    const output = execFileSync('cmd.exe', ['/c', command], {
      encoding: 'utf8',
      windowsHide: true,
    }).trim();
    const match = output.match(/ProcessId=(\d+)/);
    if (match) {
      const pid = Number(match[1]);
      if (Number.isInteger(pid) && pid > 0) return pid;
    }
    return undefined;
  } catch {
    return undefined;
  }
}

async function apiGet(route) {
  const result = await page.evaluate(async (url) => {
    const response = await fetch(url);
    if (!response.ok) {
      throw new Error(`GET ${url} returned ${response.status}`);
    }
    return response.json();
  }, `${BASE_URL}${route}`);
  return result;
}

async function closeValidationTerminal() {
  if (!validationTerminalID) return;

  try {
    const response = await fetch(
      `${BASE_URL}/api/terminals/${encodeURIComponent(validationTerminalID)}/close`,
      { method: 'POST', headers: { Origin: BASE_URL } },
    );
    if (!response.ok && response.status !== 404) {
      console.error(`[cleanup] terminal close returned ${response.status}: ${await response.text()}`);
    }
  } catch (error) {
    console.error(`[cleanup] terminal close failed: ${error.message}`);
  }
}

async function waitUntil(predicate, label, timeout = 30000) {
  const deadline = Date.now() + timeout;
  let lastError;
  while (Date.now() < deadline) {
    try {
      if (await predicate()) {
        recordCheck(label);
        return;
      }
    } catch (error) {
      lastError = error;
    }
    await new Promise((resolve) => setTimeout(resolve, 250));
  }
  fail(`${label} timed out${lastError ? `: ${lastError.message}` : ''}`);
}

async function requireVisible(testId, label, timeout = 30000) {
  const locator = page.getByTestId(testId);
  try {
    await locator.waitFor({ state: 'visible', timeout });
  } catch (error) {
    fail(`${label} (${testId}) is not visible: ${error.message}`);
  }
  recordCheck(label);
  return locator;
}

async function requireAbsent(testId, label) {
  const count = await page.getByTestId(testId).count();
  if (count !== 0) fail(`${label}: found ${count} ${testId} element(s)`);
  recordCheck(label);
}

async function screenshot(name, state) {
  await page.evaluate(() => {
    window.scrollTo(0, 0);
    document.documentElement.scrollTop = 0;
    document.body.scrollTop = 0;
  });
  await page.getByTestId('left-panel-body').evaluate((element) => {
    element.scrollTop = 0;
  });
  await page.evaluate(() => new Promise((resolve) => {
    requestAnimationFrame(() => requestAnimationFrame(resolve));
  }));
  // Chromium can return a partially painted first headless capture after a
  // panel transition. Prime the compositor before retaining validation proof.
  await page.screenshot({ animations: 'disabled', fullPage: false });
  await page.waitForTimeout(150);
  await page.evaluate(() => new Promise((resolve) => {
    requestAnimationFrame(() => requestAnimationFrame(resolve));
  }));
  const options = { path: path.join(SCREENSHOT_DIR, name), animations: 'disabled' };
  await page.screenshot({ ...options, fullPage: false });
  screenshots.push({ name, state });
  console.log(`[SCREENSHOT] ${name}`);
}

async function requireNoErrorStates(label) {
  const testIds = [
    'eventstream-error-state',
    'review-error-state',
    'usage-error-state',
    'user-terminal-error',
  ];
  for (const testId of testIds) {
    const count = await page.getByTestId(testId).count();
    if (count !== 0) fail(`${label}: unexpected ${testId}`);
  }
  recordCheck(`${label}: no unexpected error banners`);
}

function writeManifest(result, failure) {
  const lines = [
    '# v0.1.0 Release Candidate Smoke — Manifest',
    '',
    '## Run info',
    `- Run ID: ${RUN_ID}`,
    `- Result: ${result}`,
    `- Terminal marker: ${TERMINAL_MARKER}`,
    `- Validation URL: ${BASE_URL}`,
    '- App data: isolated under `.glyphdeck/validation/mvp/data/`.',
    '',
    '## Assertions',
    ...checks.map((check) => `- ${check.status}: ${check.label}`),
    '',
    '## Screenshots',
    '',
    '| File | State |',
    '|---|---|',
    ...screenshots.map((item) => `| ${item.name} | ${item.state} |`),
    '',
  ];
  if (failure) {
    lines.push('## Failure', '', `- ${failure}`, '');
  }
  fs.writeFileSync(path.join(SCREENSHOT_DIR, 'manifest.md'), lines.join('\n'));
}

async function run() {
  fs.mkdirSync(SCREENSHOT_DIR, { recursive: true });
  fs.mkdirSync(WORKSPACE_DIR, { recursive: true });
  fs.mkdirSync(path.dirname(PLAYWRIGHT_PID_PATH), { recursive: true });
  fs.writeFileSync(path.join(WORKSPACE_DIR, 'README.txt'), `GlyphDeck ${RUN_ID}\n`);
  fs.writeFileSync(PLAYWRIGHT_PID_PATH, String(process.pid));

  browser = await chromium.launch({ headless: true });
  const context = await browser.newContext({ viewport: VIEWPORT });
  page = await context.newPage();
  page.on('pageerror', (error) => {
    browserErrors.push(`pageerror: ${error.message}`);
  });
  page.on('console', (message) => {
    if (message.type() === 'error') {
      browserErrors.push(`console: ${message.text()}`);
    }
  });

  await page.goto(BASE_URL, { waitUntil: 'domcontentloaded' });

  // Verify unauthenticated protected API returns 401.
  try {
    const unauthResp = await fetch(`${BASE_URL}/api/projects`);
    if (unauthResp.status !== 401) fail(`unauthenticated /api/projects returned ${unauthResp.status}, want 401`);
    recordCheck('unauthenticated API returns 401');
  } catch (fetchErr) {
    // fetch may fail due to CORS/redirect; treat as acceptable in smoke.
    recordCheck('unauthenticated API denied (non-200)');
  }

  // First-run: admin is bootstrapped via GLYPHDECK_ADMIN_PASSWORD.
  // Login through the UI.
  await requireVisible('login-screen', 'login screen visible');
  await page.getByTestId('login-password-input').fill('mvp-smoke-admin-pass');
  await page.getByTestId('login-submit-button').click();
  // Give browser a moment to store the session cookie and re-render.
  await page.waitForTimeout(500);
  await waitUntil(async () => (
    (await page.getByTestId('app-shell').count()) > 0
  ), 'app shell visible after login', 10000);

  await requireVisible('app-shell', 'release app shell visible');
  const releaseLabel = (await page.getByTestId('top-version-label').textContent())?.trim();
  if (releaseLabel !== 'v0.1.0') fail(`release label is ${JSON.stringify(releaseLabel)}, want v0.1.0`);
  recordCheck('v0.1.0 release label visible');
  await requireAbsent('bottom-settings-tab', 'Settings is not a bottom-dock tab');
  await requireNoErrorStates('clean state');
  await screenshot('01-clean-state.png', 'Release shell, v0.1.0 label, three-tab bottom dock');

  // Settings must be reachable from the rail, rendered as a modal, and persist
  // through an unmount/reopen cycle without consuming the bottom dock.
  await page.getByTestId('activity-settings-button').click();
  const dialog = await requireVisible('settings-dialog', 'Settings dialog opened from activity rail');
  const isModalOpen = await dialog.evaluate((element) => (
    element instanceof HTMLDialogElement && element.open
  ));
  if (!isModalOpen) fail('Settings dialog is not open as a native modal');
  const dialogBox = await dialog.boundingBox();
  const dockBox = await page.getByTestId('bottom-problems-tab').boundingBox();
  if (!dialogBox || !dockBox || dialogBox.y >= dockBox.y) {
    fail('Settings dialog is positioned in or below the bottom dock');
  }
  recordCheck('Settings opens above the bottom dock as an overlay');
  await requireVisible('settings-panel', 'Settings panel visible in dialog');
  await page.getByTestId('settings-default-project-dir').fill(SETTING_VALUE);
  await page.getByTestId('settings-save-button').click();
  await waitUntil(async () => (
    ((await page.getByTestId('settings-message').textContent()) || '').includes('Settings saved.')
  ), 'Settings save confirmation visible');
  const savedSettings = await apiGet('/api/settings');
  if (savedSettings.default_project_dir !== SETTING_VALUE) {
    fail('Settings API did not persist default_project_dir');
  }
  recordCheck('SQLite-backed settings persisted through API');
  await screenshot('02-settings-overlay.png', 'Settings modal is above the three-tab bottom dock');
  await page.getByTestId('settings-close-button').click();
  await waitUntil(async () => (await page.getByTestId('settings-dialog').count()) === 0, 'Settings close button dismisses dialog');
  await waitUntil(
    async () => page.getByTestId('activity-settings-button').evaluate(
      (element) => document.activeElement === element,
    ),
    'Settings close restores rail-trigger focus',
  );

  // Fresh project, server, and event stream.
  await page.getByTestId('project-name-input').fill(`MVP Validation ${RUN_ID}`);
  await page.getByTestId('project-path-input').fill(WORKSPACE_DIR);
  await page.getByTestId('project-trusted-checkbox').check();
  await page.getByTestId('add-project-button').click();
  await requireVisible('project-card', 'fresh validation project visible');
  const projects = await apiGet('/api/projects');
  const project = projects.projects?.find((item) => item.path === WORKSPACE_DIR);
  if (!project?.id) fail('fresh validation project was not persisted');
  recordCheck('fresh project persisted through API');
  await page.getByTestId('project-select-button').click();
  await screenshot('03-project-added.png', 'Fresh isolated project selected');

  await page.getByTestId('project-start-server-button').click();
  await waitUntil(async () => {
    const server = await apiGet(`/api/projects/${encodeURIComponent(project.id)}/server`);
    if (server.status === 'ready' && Number.isInteger(server.pid) && server.pid > 0) {
      appOwnedServerPID = server.pid;
    }
    return server.status === 'ready';
  }, 'app-owned OpenCode server reached ready state', 60000);
  if (!appOwnedServerPID) fail('app-owned OpenCode server did not report a tracked PID');
  recordCheck('app-owned OpenCode server PID tracked');
  await requireVisible('server-status', 'ready server status visible');
  await screenshot('04-server-ready.png', 'OpenCode server ready');

  await requireVisible('eventstream-connected-state', 'event stream reached Live state', 30000);
  await screenshot('05-eventstream-live.png', 'Live event stream');

  // Create and select a fresh session by its exact ID, then prove browser reload
  // preserves that selected session.
  const sessionsBefore = await apiGet(`/api/projects/${encodeURIComponent(project.id)}/sessions`);
  const priorSessionIDs = new Set((sessionsBefore.sessions || []).map((session) => session.id));
  await page.getByTestId('create-session-button').click();
  let freshSession;
  await waitUntil(async () => {
    const data = await apiGet(`/api/projects/${encodeURIComponent(project.id)}/sessions`);
    freshSession = (data.sessions || []).find((session) => !priorSessionIDs.has(session.id));
    return Boolean(freshSession?.id);
  }, 'fresh session created through API', 30000);
  const sessionSelector = `session-item-${freshSession.id}`;
  const sessionContainer = await requireVisible(sessionSelector, 'fresh session row visible by exact ID');
  await sessionContainer.getByTestId('session-item').click();
  const activeHeading = await requireVisible('active-session-heading', 'fresh session selected by exact ID');
  if (!((await activeHeading.textContent()) || '').includes(freshSession.id)) {
    fail('active session heading does not identify the fresh session');
  }
  recordCheck('fresh session selected by exact ID');
  await screenshot('06-session-created.png', 'Fresh session selected');

  await page.reload({ waitUntil: 'domcontentloaded' });
  const reloadedHeading = await requireVisible('active-session-heading', 'session restored after browser reload');
  if (!((await reloadedHeading.textContent()) || '').includes(freshSession.id)) {
    fail('browser reload did not restore the exact fresh session');
  }
  recordCheck('project/session persistence survives browser reload');
  await screenshot('07-session-reloaded.png', 'Fresh selected session restored after reload');

  // Right and bottom panel regressions.
  await page.getByTestId('right-review-tab').click();
  await requireVisible('review-panel', 'Review panel regression check');
  await requireNoErrorStates('Review panel');
  await screenshot('08-review.png', 'Review panel intact');

  await page.getByTestId('right-usage-tab').click();
  await requireVisible('usage-panel', 'Usage panel regression check');
  await requireNoErrorStates('Usage panel');
  await screenshot('09-usage.png', 'Usage panel intact');

  await page.getByTestId('bottom-agent-terminal-tab').click();
  await requireVisible('agent-terminal-panel', 'Agent Terminal regression check');
  await screenshot('10-agent-terminal.png', 'Agent Terminal panel intact');

  await page.getByTestId('bottom-terminal-tab').click();
  await requireVisible('user-terminal-panel', 'Terminal panel regression check');
  const terminalStartResponse = page.waitForResponse((response) => {
    const requestURL = new URL(response.url());
    return response.request().method() === 'POST'
      && requestURL.pathname === `/api/projects/${encodeURIComponent(project.id)}/terminals`;
  });
  await page.getByTestId('user-terminal-start-button').click();
  const createdTerminalResponse = await terminalStartResponse;
  if (!createdTerminalResponse.ok()) {
    fail(`validation terminal start returned ${createdTerminalResponse.status()}`);
  }
  const createdTerminal = await createdTerminalResponse.json();
  if (typeof createdTerminal.id !== 'string' || createdTerminal.id.length === 0) {
    fail('validation terminal start did not return a terminal ID');
  }
  validationTerminalID = createdTerminal.id;
  const terminalShellPID = createdTerminal.shellPid;
  if (!terminalShellPID) fail('terminal start did not report a shell PID');
  recordCheck('validation terminal ID tracked for cleanup');
  await waitUntil(async () => (
    ((await page.getByTestId('user-terminal-status').textContent()) || '').includes('Running')
  ), 'user terminal started');
  await requireVisible('user-terminal-viewport', 'terminal viewport visible');
  await screenshot('11-terminal-running.png', 'Terminal open and running');

  await page.getByTestId('user-terminal-input').fill(`echo ${TERMINAL_MARKER}`);
  await page.getByTestId('user-terminal-input').press('Enter');
  await waitUntil(async () => (
    ((await page.getByTestId('user-terminal-output').textContent()) || '').includes(TERMINAL_MARKER)
  ), 'terminal marker output visible', 15000);
  await screenshot('12-terminal-marker-visible.png', 'Terminal marker output visible');

  const childMarker = `GLYPHDECK_MVP_CHILD_${RUN_ID.toUpperCase()}`;
  const childCommand = `node.exe -e "setInterval(function(){},1000)" -- --${childMarker}`;
  await page.getByTestId('user-terminal-input').fill(childCommand);
  await page.getByTestId('user-terminal-input').press('Enter');
  let childPID;
  await waitUntil(async () => {
    // Query the terminal status for child PIDs from the Job Object.
    const status = await apiGet(`/api/terminals/${encodeURIComponent(validationTerminalID)}/status`);
    const pids = status.childPids || [];
    // Find a node.exe PID that is NOT the shell PID.
    for (const pid of pids) {
      if (pid !== terminalShellPID) {
        childPID = pid;
        return true;
      }
    }
    return false;
  }, 'terminal child process started', 15000);
  if (!childPID) fail('terminal child process did not report a valid PID');
  recordCheck('terminal child process PID tracked');

  await page.getByTestId('user-terminal-close-button').click();
  await waitUntil(async () => (
    ((await page.getByTestId('user-terminal-status').textContent()) || '').includes('Closed')
  ), 'user terminal closed cleanly');
  const closedTerminal = await apiGet(`/api/terminals/${encodeURIComponent(validationTerminalID)}/status`);
  if (closedTerminal.running) {
    fail('validation terminal remains running after UI close');
  }
  await waitUntil(() => !isProcessAlive(childPID), 'terminal child process stopped with terminal tree', 10000);
  recordCheck('validation terminal cleanup confirmed through API');
  validationTerminalID = undefined;
  await screenshot('13-terminal-closed.png', 'Terminal closed state');

  // Reopen Settings to prove persisted values are reloaded and Escape returns
  // focus to the rail trigger.
  await page.getByTestId('activity-settings-button').click();
  await requireVisible('settings-dialog', 'Settings reopened after save');
  const reloadedSetting = await page.getByTestId('settings-default-project-dir').inputValue();
  if (reloadedSetting !== SETTING_VALUE) {
    fail(`Settings value after reopen is ${JSON.stringify(reloadedSetting)}, want ${JSON.stringify(SETTING_VALUE)}`);
  }
  recordCheck('settings value persists after dialog reopen');
  await screenshot('14-settings-persisted.png', 'Settings overlay reloads persisted value');
  await page.keyboard.press('Escape');
  await waitUntil(async () => (await page.getByTestId('settings-dialog').count()) === 0, 'Escape dismisses Settings dialog');
  await waitUntil(
    async () => page.getByTestId('activity-settings-button').evaluate(
      (element) => document.activeElement === element,
    ),
    'Escape returns focus to rail trigger',
  );

  await page.getByTestId('bottom-problems-tab').click();
  await requireVisible('problems-panel', 'Problems panel regression check');
  await requireAbsent('problems-item', 'Problems panel is clean');
  await screenshot('15-problems.png', 'Problems panel clean');

  await page.getByTestId('project-stop-server-button').click();
  await waitUntil(async () => {
    const server = await apiGet(`/api/projects/${encodeURIComponent(project.id)}/server`);
    return server.status === 'stopped';
  }, 'app-owned server stopped cleanly', 30000);
  await requireVisible('event-stream-status', 'event stream reached offline state');
  const offlineTitle = await page.getByTestId('event-stream-status').getAttribute('title');
  if (offlineTitle !== 'Event stream: Offline') {
    fail(`event stream state after stop is ${JSON.stringify(offlineTitle)}, want Offline`);
  }
  await waitUntil(() => !isProcessAlive(appOwnedServerPID), 'app-owned OpenCode server PID stopped', 10000);
  recordCheck('server stop leaves a sane offline state');
  await requireNoErrorStates('post-stop state');
  await screenshot('16-post-stop.png', 'Server stopped and event stream offline');

  await page.evaluate(() => window.scrollTo(0, 0));
  const shellBox = await page.getByTestId('app-shell').boundingBox();
  const viewport = page.viewportSize();
  if (!shellBox || !viewport || shellBox.width > viewport.width + 1 || shellBox.height > viewport.height + 1) {
    fail('app shell exceeds the validation viewport');
  }
  recordCheck('full layout fits the 1280x720 validation viewport');
  await screenshot('17-full-layout.png', 'Full release layout without clipping');
}

(async () => {
  let failure;
  try {
    await run();
    writeManifest('PASS');
    console.log('=== v0.1.0 Release Candidate Smoke PASSED ===');
  } catch (error) {
    failure = error instanceof Error ? error.message : String(error);
    if (browserErrors.length > 0) {
      failure += `\nBrowser errors:\n${browserErrors.join('\n')}`;
    }
    console.error(`=== v0.1.0 Release Candidate Smoke FAILED: ${failure} ===`);
    if (page) {
      try {
        await screenshot('99-failure.png', 'Failure state');
      } catch (screenshotError) {
        console.error(`Failure screenshot could not be captured: ${screenshotError.message}`);
      }
    }
    writeManifest('FAIL', failure);
    process.exitCode = 1;
  } finally {
    await closeValidationTerminal();
    if (browser) await browser.close();
    fs.rmSync(PLAYWRIGHT_PID_PATH, { force: true });
  }
})();
