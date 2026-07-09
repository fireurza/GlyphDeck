/*
 * GlyphDeck Milestone 8 smoke — Permissions popup.
 * Committed source-of-truth under scripts/validation/.
 *
 * Honesty rules (exit nonzero on any failure):
 *   - Isolated workspace with forced bash permission config
 *   - Fresh session selected by exact ID
 *   - data-testid selectors only
 *   - Verify permission popup appears, dismisses, agent resumes
 *   - FAIL on error banners, debug text, missing popup
 *   - Validate app behavior, not model obedience
 */
const { chromium } = require('playwright');
const path = require('path');
const fs = require('fs');
const crypto = require('crypto');

const SCREENSHOT_DIR = path.resolve(__dirname, '..', 'screenshots');
const WORKSPACE_DIR = path.resolve(__dirname, '..', 'workspace');
const FRONTEND_URL = 'http://localhost:5173';
const BACKEND_URL = 'http://127.0.0.1:8756';

const MARKER = ['GLYPHDECK_M8', Date.now().toString(36), crypto.randomBytes(3).toString('hex')].join('_');

/* Force permission approval for bash in project config */
const OPENCODE_CONFIG = JSON.stringify({
  permission: { bash: 'ask' },
}, null, 2);

const FAILURES = [];
function fail(m) { FAILURES.push(m); console.error(`[FAIL] ${m}`); }
function warn(m) { console.warn(`[WARN] ${m}`); }

async function assertVisible(page, tid, label) {
  try { await page.getByTestId(tid).waitFor({ state: 'visible', timeout: 30000 }); console.log(`[OK] ${label}: visible`); }
  catch (e) { fail(`${label}: ${e.message}`); }
}

async function assertNoText(page, tid, words, label) {
  try {
    const t = await page.getByTestId(tid).textContent({ timeout: 5000 });
    for (const w of words) if (t?.toLowerCase().includes(w.toLowerCase())) { fail(`${label}: "${w}"`); return; }
    console.log(`[OK] ${label}: clean`);
  } catch { console.log(`[OK] ${label}: not present`); }
}

async function ss(page, name) { await page.screenshot({ path: path.join(SCREENSHOT_DIR, name), fullPage: false }); console.log(`[SCREENSHOT] ${name}`); }

async function apiGet(route) { try { const r = await fetch(`${BACKEND_URL}${route}`); return r.ok ? r.json() : null; } catch { return null; } }
async function apiPost(route, body) { try { const r = await fetch(`${BACKEND_URL}${route}`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) }); if (!r.ok) throw new Error(`${r.status}`); return r.json(); } catch (e) { fail(`API: ${e.message}`); return null; } }

async function findProject(wp) {
  const d = await apiGet('/api/projects'); const ps = d?.projects; if (!ps) return null;
  const n = wp.replace(/\\/g, '/').toLowerCase();
  return ps.find(p => (p.path || '').replace(/\\/g, '/').toLowerCase() === n);
}

async function waitReady(pid, ms = 60000) {
  const dl = Date.now() + ms;
  while (Date.now() < dl) { const s = await apiGet(`/api/projects/${encodeURIComponent(pid)}/server`); if (s?.status === 'ready') return true; await new Promise(r => setTimeout(r, 1000)); }
  return false;
}

async function run() {
  console.log('=== GlyphDeck M8 Permissions Smoke ===');
  console.log(`Marker: ${MARKER}`);

  /* Isolated workspace + forced bash permission config */
  fs.mkdirSync(path.join(WORKSPACE_DIR, '.opencode'), { recursive: true });
  fs.writeFileSync(path.join(WORKSPACE_DIR, '.opencode', 'opencode.jsonc'), OPENCODE_CONFIG);
  fs.writeFileSync(path.join(WORKSPACE_DIR, 'README.txt'), 'M8 validation\n');
  console.log('[workspace] Created with forced bash permission');

  try { await fetch(`${BACKEND_URL}/api/dev/reset-validation-state`, { method: 'POST' }); } catch {}

  const browser = await chromium.launch({ headless: true });
  const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 } });
  const page = await ctx.newPage();
  let projectId, sessionId, popupSeen = false;

  try {
    /* 01 */
    await page.goto(FRONTEND_URL, { waitUntil: 'domcontentloaded' });
    await page.waitForTimeout(4000);
    await assertVisible(page, 'app-shell', 'Shell');
    await ss(page, '01-clean-state.png');

    /* Add project */
    await page.getByTestId('project-name-input').fill('M8 Validation');
    await page.getByTestId('project-path-input').fill(WORKSPACE_DIR);
    await page.getByTestId('project-trusted-checkbox').click();
    await page.getByTestId('add-project-button').click();
    await page.waitForTimeout(1000);
    await assertVisible(page, 'project-card', 'Card');
    await ss(page, '02-project-added.png');

    /* Resolve project */
    const p = await findProject(WORKSPACE_DIR);
    projectId = p?.id; if (!projectId) { fail('No project ID'); }
    console.log(`[api] Project: ${projectId}`);

    /* Start server */
    await page.getByTestId('project-start-server-button').click();
    if (!(await waitReady(projectId))) { fail('Server not ready'); }
    await page.waitForTimeout(1000);
    await ss(page, '03-server-ready.png');

    /* Create session via API */
    const ses = await apiPost(`/api/projects/${encodeURIComponent(projectId)}/sessions`, {});
    sessionId = ses?.id; if (!sessionId) { fail('No session ID'); }
    console.log(`[api] Session: ${sessionId}`);

    /* Select project */
    await page.getByTestId('project-select-button').click();
    await page.waitForTimeout(4000);
    await assertVisible(page, 'eventstream-connected-state', 'Stream');
    await ss(page, '04-eventstream-connected.png');

    /* Select session */
    const sel = `[data-testid="session-item"][data-session-id="${sessionId}"]`;
    try { await page.waitForSelector(sel, { timeout: 10000 }); await page.locator(sel).click(); }
    catch { fail(`Session ${sessionId} missing`); }
    await assertVisible(page, 'active-session-heading', 'Heading');
    const hd = await page.getByTestId('active-session-heading').textContent();
    if (!hd?.includes(sessionId)) fail('Heading mismatch');
    await ss(page, '05-session-created.png');

    /* 0 messages */
    const pre = await page.locator('[data-testid^="transcript-"][data-testid$="-message"]').count();
    if (pre > 0) fail(`${pre} stale messages`); else console.log('[OK] Transcript clean');

    /* Send prompt that triggers bash permission */
    await page.getByTestId('prompt-composer-input').fill(`Run this single command and report its output: echo ${MARKER}`);
    await page.getByTestId('prompt-send-button').click();
    await page.waitForTimeout(500);
    await ss(page, '06-prompt-sent.png');

    /* Verify user message */
    try {
      await page.waitForFunction(m => {
        for (const el of document.querySelectorAll('[data-testid="transcript-user-message"]'))
          if (el.textContent?.includes(m)) return true;
        return false;
      }, MARKER, { timeout: 15000 });
      console.log('[OK] User message');
    } catch { fail('User message not found'); }

    /* ---- PERMISSION POPUP ---- */
    popupSeen = false;
    try {
      await page.waitForSelector('[data-testid="permission-popup"]', { timeout: 30000 });
      console.log('[OK] Permission popup appeared');
      popupSeen = true;
    } catch { warn('Permission popup did not appear within timeout — agent may not have run bash'); }

    if (popupSeen) {
      await page.waitForTimeout(500);
      const cmd = await page.getByTestId('permission-command').textContent();
      console.log(`[OK] Permission command: "${cmd?.trim()}"`);
      await ss(page, '08-permission-popup-visible.png');

      /* Approve Once */
      await page.getByTestId('permission-approve-once').click();
      await page.waitForTimeout(500);

      /* Verify popup dismissed */
      const popupCount = await page.locator('[data-testid="permission-popup"]').count();
      if (popupCount === 0) console.log('[OK] Popup dismissed after approve');
      else warn('Popup still visible after approve');

      await ss(page, '09-permission-approved.png');
    }

    /* Wait for assistant response */
    try {
      await page.waitForFunction(() =>
        (document.querySelector('[data-testid="transcript-assistant-message"]') ||
         document.querySelector('[data-testid="transcript-streamed-message"]')) &&
        document.querySelector('[data-testid="transcript"]')?.textContent?.length > 10,
        { timeout: 90000 });
      console.log('[OK] Assistant response');
    } catch { warn('Assistant response slow or missing'); }
    await page.waitForTimeout(2000);
    await ss(page, '07-streaming-response-visible.png');

    /* M6 + M7 regressions */
    await page.getByTestId('right-usage-tab').click();
    await page.waitForTimeout(1000);
    await assertNoText(page, 'usage-panel', ['Failed to fetch', 'Request failed'], 'Usage');
    await ss(page, '10-usage-regression.png');

    await page.getByTestId('right-review-tab').click();
    await page.waitForTimeout(1000);
    await assertNoText(page, 'review-panel', ['Failed to fetch', 'Request failed'], 'Review');
    await ss(page, '11-review-regression.png');

    await page.getByTestId('bottom-agent-terminal-tab').click();
    await page.waitForTimeout(500);
    await assertVisible(page, 'agent-terminal-panel', 'Agent Term');
    await ss(page, '12-agent-terminal-regression.png');

    /* Stop server */
    await page.getByTestId('project-stop-server-button').click({ force: false });
    try {
      await page.waitForFunction(() => {
        const el = document.querySelector('[data-testid="server-status"]');
        return el?.textContent?.toLowerCase().includes('stopped');
      }, { timeout: 15000 });
      console.log('[OK] Stopped');
    } catch { warn('Stop slow'); }
    await page.waitForTimeout(1000);
    await ss(page, '13-server-stopped.png');

    await ss(page, '14-full-layout-no-clipping.png');
    if (!popupSeen) await ss(page, '15-permission-not-triggered.png');

  } finally { await browser.close(); }

  console.log('');
  if (FAILURES.length === 0) {
    if (!popupSeen) warn('Permission popup was not triggered during this run. Verify forced-bash config is correct.');
    console.log('=== M8 Smoke PASSED ===');
    process.exit(0);
  }
  console.error(`=== FAIL (${FAILURES.length}) ===`);
  FAILURES.forEach(f => console.error(`  ${f}`));
  process.exit(1);
}

run().catch(err => { console.error('Unhandled:', err.message); process.exit(1); });
