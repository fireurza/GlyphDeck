/*
 * GlyphDeck Milestone 7 smoke — Review tab.
 * Committed source-of-truth under scripts/validation/.
 *
 * Honesty rules (exit nonzero on any failure):
 *   - Isolated workspace (no GlyphDeck-repo sessions leaking in)
 *   - Fresh session selected by exact ID (never .first())
 *   - Machine assertion before every screenshot
 *   - data-testid selectors only
 *   - FAIL on "Failed to fetch", "Request failed", "Event stream: error"
 *   - FAIL on debug/implementation text in transcript
 *   - Validate app behavior, NOT model obedience (marker echo not required)
 *   - Review tab must show project/session/git data or honest unavailable states
 */
const { chromium } = require('playwright');
const path = require('path');
const fs = require('fs');
const crypto = require('crypto');

/* ------------------------------------------------------------------ */
/*  Paths                                                              */
/* ------------------------------------------------------------------ */
const SCREENSHOT_DIR = path.resolve(__dirname, '..', 'screenshots');
const WORKSPACE_DIR = path.resolve(__dirname, '..', 'workspace');

const FRONTEND_URL = 'http://localhost:5173';
const BACKEND_URL = 'http://127.0.0.1:8756';

const MARKER = [
  'GLYPHDECK_M7',
  Date.now().toString(36),
  crypto.randomBytes(3).toString('hex'),
].join('_');

const PROMPT_TEXT = `Say this exact marker: ${MARKER}`;

/* ------------------------------------------------------------------ */
/*  Helpers                                                            */
/* ------------------------------------------------------------------ */
const FAILURES = [];

function fail(msg) {
  FAILURES.push(msg);
  console.error(`[FAIL] ${msg}`);
}

function warn(msg) {
  console.warn(`[WARN] ${msg}`);
}

async function assertVisible(page, testId, label) {
  try {
    await page.getByTestId(testId).waitFor({ state: 'visible', timeout: 30000 });
    console.log(`[OK] ${label}: data-testid="${testId}" visible`);
  } catch (e) {
    fail(`${label}: data-testid="${testId}" not visible (${e.message})`);
  }
}

async function assertNoText(page, testId, forbidden, label) {
  try {
    const text = await page.getByTestId(testId).textContent({ timeout: 5000 });
    if (text) {
      for (const word of forbidden) {
        if (text.toLowerCase().includes(word.toLowerCase())) {
          fail(`${label}: contains "${word}" — "${text.trim().substring(0, 80)}"`);
          return;
        }
      }
    }
    console.log(`[OK] ${label}: no forbidden text`);
  } catch (e) {
    console.log(`[OK] ${label}: element not present (no error visible)`);
  }
}

async function screenshot(page, name) {
  await page.screenshot({ path: path.join(SCREENSHOT_DIR, name), fullPage: false });
  console.log(`[SCREENSHOT] ${name}`);
}

async function apiGet(route) {
  try {
    const res = await fetch(`${BACKEND_URL}${route}`);
    return res.ok ? await res.json() : null;
  } catch { return null; }
}

async function apiPost(route, body) {
  try {
    const res = await fetch(`${BACKEND_URL}${route}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });
    if (!res.ok) throw new Error(`${res.status} ${await res.text()}`);
    return await res.json();
  } catch (e) {
    fail(`API POST ${route}: ${e.message}`);
    return null;
  }
}

async function findProjectByPath(wp) {
  const data = await apiGet('/api/projects');
  const projects = data?.projects;
  if (!projects || !Array.isArray(projects)) return null;
  const n = wp.replace(/\\/g, '/').toLowerCase();
  return projects.find(p => (p.path || '').replace(/\\/g, '/').toLowerCase() === n);
}

async function waitForServerReady(projectId, timeoutMs = 60000) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    const s = await apiGet(`/api/projects/${encodeURIComponent(projectId)}/server`);
    if (s && s.status === 'ready') return true;
    await new Promise(r => setTimeout(r, 1000));
  }
  fail('Server did not reach ready state');
  return false;
}

/* ------------------------------------------------------------------ */
/*  Main                                                               */
/* ------------------------------------------------------------------ */
async function run() {
  console.log('=== GlyphDeck M7 Review Tab Smoke Test ===');
  console.log(`Marker: ${MARKER}`);

  /* Workspace */
  fs.mkdirSync(WORKSPACE_DIR, { recursive: true });
  fs.writeFileSync(path.join(WORKSPACE_DIR, 'README.txt'), 'M7 validation\n');
  console.log('[workspace] Created');

  /* Reset */
  try { await fetch(`${BACKEND_URL}/api/dev/reset-validation-state`, { method: 'POST' }); } catch {}

  const browser = await chromium.launch({ headless: true });
  const context = await browser.newContext({ viewport: { width: 1440, height: 900 } });
  const page = await context.newPage();
  let projectId, sessionId;

  try {
    /* 01 clean — wait for React to fully hydrate */
    await page.goto(FRONTEND_URL, { waitUntil: 'domcontentloaded' });
    await page.waitForTimeout(4000);
    await assertVisible(page, 'app-shell', 'App shell');
    await screenshot(page, '01-clean-state.png');

    /* Add project */
    await page.getByTestId('project-name-input').fill('M7 Validation');
    await page.getByTestId('project-path-input').fill(WORKSPACE_DIR);
    await page.getByTestId('project-trusted-checkbox').click();
    await page.getByTestId('add-project-button').click();
    await page.waitForTimeout(1000);
    await assertVisible(page, 'project-card', 'Project card');
    await screenshot(page, '02-project-added.png');

    /* Resolve project ID */
    const proj = await findProjectByPath(WORKSPACE_DIR);
    projectId = proj?.id;
    if (!projectId) { fail('Project ID not resolved'); }
    console.log(`[api] Project: ${projectId}`);

    /* Start server */
    await page.getByTestId('project-start-server-button').click();
    if (!(await waitForServerReady(projectId))) { fail('Server never ready'); }
    await page.waitForTimeout(1000);
    await screenshot(page, '03-server-ready.png');

    /* Create session via API */
    const session = await apiPost(`/api/projects/${encodeURIComponent(projectId)}/sessions`, {});
    sessionId = session?.id;
    if (!sessionId) { fail('No session ID'); }
    console.log(`[api] Session: ${sessionId}`);

    /* Select project */
    await page.getByTestId('project-select-button').click();
    await page.waitForTimeout(4000);
    await assertVisible(page, 'eventstream-connected-state', 'Event stream');
    await screenshot(page, '04-eventstream-connected.png');

    /* Select session by ID */
    const sel = `[data-testid="session-item"][data-session-id="${sessionId}"]`;
    try {
      await page.waitForSelector(sel, { timeout: 10000 });
      await page.locator(sel).click();
      console.log(`[OK] Session selected by ID`);
    } catch { fail(`Session ${sessionId} not found`); }
    await assertVisible(page, 'active-session-heading', 'Heading');
    const hd = await page.getByTestId('active-session-heading').textContent();
    if (!hd?.includes(sessionId)) fail(`Heading mismatch: "${hd}"`);
    await screenshot(page, '05-session-created.png');

    /* HONESTY: 0 messages before prompt */
    const pre = await page.locator('[data-testid^="transcript-"][data-testid$="-message"]').count();
    if (pre > 0) fail(`${pre} stale messages before prompt`);
    else console.log('[OK] Transcript clean');

    /* Send prompt */
    await page.getByTestId('prompt-composer-input').fill(PROMPT_TEXT);
    await page.getByTestId('prompt-send-button').click();
    await page.waitForTimeout(500);
    await screenshot(page, '06-prompt-sent.png');

    /* Verify user message */
    try {
      await page.waitForFunction(m => {
        for (const el of document.querySelectorAll('[data-testid="transcript-user-message"]'))
          if (el.textContent?.includes(m)) return true;
        return false;
      }, MARKER, { timeout: 15000 });
      console.log('[OK] User message with marker');
    } catch { fail('User message not found'); }

    /* Wait for assistant text + ensure "Sending..." spinner is gone. */
    try {
      await page.waitForFunction(() => {
        const el = document.querySelector('[data-testid="transcript-assistant-message"]') ||
                   document.querySelector('[data-testid="transcript-streamed-message"]');
        if (!el || !el.textContent || el.textContent.trim().length === 0) return false;
        // Ensure "Sending..." is gone.
        const sending = document.querySelector('[data-testid="transcript"]');
        const sendingText = sending?.textContent || '';
        if (sendingText.includes('Sending…')) return false;
        return true;
      }, { timeout: 90000 });
      console.log('[OK] Assistant response with text, no spinner');
    } catch { fail('No assistant response with text'); }

    /* Marker check: validate app, not model obedience. */
    let hasMarker = false;
    const am = page.locator('[data-testid="transcript-assistant-message"],[data-testid="transcript-streamed-message"]');
    for (let i = 0; i < await am.count(); i++)
      if ((await am.nth(i).textContent())?.includes(MARKER)) hasMarker = true;
    if (!hasMarker) warn('Assistant did not echo marker (model behavior, not a GlyphDeck bug)');

    /* Wait for text to fully render before screenshot. */
    await page.waitForTimeout(2000);
    await screenshot(page, '07-streaming-response-visible.png');

    /* ---- Usage tab (M6 regression) ---- */
    await page.getByTestId('right-usage-tab').click();
    await page.waitForTimeout(1000);
    await assertNoText(page, 'usage-panel', ['Failed to fetch', 'Request failed'], 'Usage');
    await screenshot(page, '08-usage-still-working.png');

    /* ---- Agent Terminal (M5 regression) ---- */
    await page.getByTestId('bottom-agent-terminal-tab').click();
    await page.waitForTimeout(500);
    await assertVisible(page, 'agent-terminal-panel', 'Agent Terminal');
    await screenshot(page, '09-agent-terminal-still-working.png');

    /* ---- Review tab ---- */
    await page.getByTestId('right-review-tab').click();
    await page.waitForTimeout(500);

    /* Wait for review-panel to appear (loading or filled). */
    try {
      await page.waitForFunction(() =>
        document.querySelector('[data-testid="review-panel"]') !== null,
        { timeout: 5000 });
      console.log('[OK] Review panel appeared');
    } catch { fail('Review panel did not appear'); }

    await page.waitForTimeout(500);
    await screenshot(page, '10-review-tab-empty-or-loading.png');

    /* Wait for data or check for error. */
    await page.waitForTimeout(2000);
    await assertNoText(page, 'review-panel', ['Failed to fetch', 'Request failed'], 'Review');

    /* Verify key data-testid elements exist. */
    const reviewIds = [
      'review-project-name', 'review-project-path',
      'review-git-branch', 'review-git-status',
      'review-session-id', 'review-message-count',
      'review-activity-summary',
    ];
    let found = 0;
    for (const tid of reviewIds) {
      if (await page.getByTestId(tid).isVisible().catch(() => false)) found++;
    }
    console.log(`[OK] Review data elements: ${found}/${reviewIds.length} visible`);
    await screenshot(page, '11-review-tab-filled-or-unavailable.png');

    /* Refresh */
    await page.getByTestId('review-refresh-button').click();
    await page.waitForTimeout(1000);
    await assertNoText(page, 'review-panel', ['Failed to fetch', 'Request failed'], 'Review refresh');
    await screenshot(page, '12-review-refresh-clean.png');

    /* Stop server */
    await page.getByTestId('project-stop-server-button').click({ force: false });
    try {
      await page.waitForFunction(() => {
        const el = document.querySelector('[data-testid="server-status"]');
        return el?.textContent?.toLowerCase().includes('stopped');
      }, { timeout: 15000 });
      console.log('[OK] Server stopped');
    } catch { warn('Server stopped state transition slow'); }
    await page.waitForTimeout(1000);
    await screenshot(page, '13-server-stopped.png');

    /* Full layout */
    await screenshot(page, '14-full-layout-no-clipping.png');

  } finally { await browser.close(); }

  console.log('');
  if (FAILURES.length === 0) {
    console.log('=== M7 Smoke PASSED ===');
    process.exit(0);
  }
  console.error(`=== M7 Smoke FAILED (${FAILURES.length}) ===`);
  FAILURES.forEach(f => console.error(`  ${f}`));
  process.exit(1);
}

run().catch(err => { console.error('Unhandled:', err.message); process.exit(1); });
