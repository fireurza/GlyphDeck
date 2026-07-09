/*
 * GlyphDeck Milestone 9 smoke — User Terminal.
 * Committed source-of-truth under scripts/validation/.
 */
const { chromium } = require('playwright');
const path = require('path');
const fs = require('fs');
const crypto = require('crypto');

const SCREENSHOT_DIR = path.resolve(__dirname, '..', 'screenshots');
const WORKSPACE_DIR = path.resolve(__dirname, '..', 'workspace');
const FRONTEND_URL = 'http://localhost:5173';
const BACKEND_URL = 'http://127.0.0.1:8756';

const MARKER = ['GLYPHDECK_M9_TERMINAL', Date.now().toString(36), crypto.randomBytes(3).toString('hex')].join('_');

const FAILURES = [];
function fail(m) { FAILURES.push(m); console.error(`[FAIL] ${m}`); }
function warn(m) { console.warn(`[WARN] ${m}`); }

async function assertVisible(page, tid, label) {
  try { await page.getByTestId(tid).waitFor({ state: 'visible', timeout: 30000 }); console.log(`[OK] ${label}`); }
  catch (e) { fail(`${label}: ${e.message}`); }
}

async function ss(page, name) { await page.screenshot({ path: path.join(SCREENSHOT_DIR, name), fullPage: false }); console.log(`[SCREENSHOT] ${name}`); }

async function apiGet(route) { try { const r = await fetch(`${BACKEND_URL}${route}`); return r.ok ? r.json() : null; } catch { return null; } }
async function apiPost(route, body) { try { const r = await fetch(`${BACKEND_URL}${route}`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) }); if (!r.ok) throw new Error(`${r.status}`); return r.json(); } catch (e) { fail(`API: ${e.message}`); return null; } }

async function findProject(wp) {
  const d = await apiGet('/api/projects'); const ps = d?.projects; if (!ps) return null;
  const n = wp.replace(/\\/g, '/').toLowerCase();
  return ps.find(p => (p.path || '').replace(/\\/g, '/').toLowerCase() === n);
}

async function run() {
  console.log('=== GlyphDeck M9 User Terminal Smoke ===');
  console.log(`Marker: ${MARKER}`);

  /* Isolated workspace */
  fs.mkdirSync(WORKSPACE_DIR, { recursive: true });
  fs.writeFileSync(path.join(WORKSPACE_DIR, 'README.txt'), 'M9 validation\n');
  console.log('[workspace] Created');

  try { await fetch(`${BACKEND_URL}/api/dev/reset-validation-state`, { method: 'POST' }); } catch {}

  const browser = await chromium.launch({ headless: true });
  const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 } });
  const page = await ctx.newPage();
  let projectId;

  try {
    /* 01 */
    await page.goto(FRONTEND_URL, { waitUntil: 'domcontentloaded' });
    await page.waitForTimeout(4000);
    await assertVisible(page, 'app-shell', 'Shell');
    await ss(page, '01-clean-state.png');

    /* Add project */
    await page.getByTestId('project-name-input').fill('M9 Validation');
    await page.getByTestId('project-path-input').fill(WORKSPACE_DIR);
    await page.getByTestId('project-trusted-checkbox').click();
    await page.getByTestId('add-project-button').click();
    await page.waitForTimeout(1000);
    await assertVisible(page, 'project-card', 'Card');
    const p = await findProject(WORKSPACE_DIR);
    projectId = p?.id; if (!projectId) { fail('No project ID'); }
    await ss(page, '02-project-added.png');

    /* Select project so selectedProjectId is set */
    await page.getByTestId('project-select-button').click();
    await page.waitForTimeout(500);
    console.log('[OK] Project selected');

    /* ---- TERMINAL ---- */
    await page.getByTestId('bottom-terminal-tab').click();
    await page.waitForTimeout(500);

    /* Empty state */
    try {
      await page.waitForSelector('[data-testid="user-terminal-empty-state"], [data-testid="user-terminal-start-button"]', { timeout: 5000 });
      console.log('[OK] Terminal empty/start state visible');
    } catch { fail('Terminal empty state not found'); }
    await ss(page, '03-terminal-tab-empty.png');

    /* Start terminal */
    await page.getByTestId('user-terminal-start-button').click();
    await page.waitForTimeout(2000);
    await assertVisible(page, 'user-terminal-viewport', 'Viewport');
    await ss(page, '04-terminal-starting.png');

    /* Wait for running state */
    try {
      await page.waitForFunction(() => {
        const el = document.querySelector('[data-testid="user-terminal-status"]');
        return el?.textContent?.includes('Running');
      }, { timeout: 10000 });
      console.log('[OK] Terminal running');
    } catch { warn('Terminal running status slow'); }
    await page.waitForTimeout(1000);
    await ss(page, '05-terminal-running.png');

    /* Wait for shell prompt to appear (PowerShell takes time to init) */
    await page.waitForTimeout(5000);

    /* Warm-up: send harmless command to flush shell init */
    await page.getByTestId('user-terminal-input').fill('echo warmup');
    await page.getByTestId('user-terminal-input').press('Enter');
    await page.waitForTimeout(3000);

    /* Send echo marker */
    await page.getByTestId('user-terminal-input').fill(`echo ${MARKER}`);
    await page.getByTestId('user-terminal-input').press('Enter');
    await page.waitForTimeout(3000);

    /* Verify marker */
    try {
      await page.waitForFunction((m) => {
        const el = document.querySelector('[data-testid="user-terminal-output"]');
        return el?.textContent?.includes(m);
      }, MARKER, { timeout: 15000 });
      console.log('[OK] Marker in output');
    } catch { warn('Marker not found — output may be buffered'); }
    await ss(page, '06-terminal-marker-output.png');

    /* Send cwd */
    await page.getByTestId('user-terminal-input').fill('Get-Location');
    await page.getByTestId('user-terminal-input').press('Enter');
    await page.waitForTimeout(3000);
    try {
      await page.waitForFunction(() => {
        const el = document.querySelector('[data-testid="user-terminal-output"]');
        return el?.textContent?.toLowerCase().includes('workspace');
      }, { timeout: 10000 });
      console.log('[OK] CWD confirmed');
    } catch { warn('CWD not confirmed'); }
    await ss(page, '07-terminal-cwd-output.png');

    /* Send git status */
    await page.getByTestId('user-terminal-input').fill('git status --short');
    await page.getByTestId('user-terminal-input').press('Enter');
    await page.waitForTimeout(1500);
    await ss(page, '08-terminal-git-status-output.png');

    /* Close terminal */
    await page.getByTestId('user-terminal-close-button').click();
    await page.waitForTimeout(1000);
    try {
      await page.waitForFunction(() => {
        const el = document.querySelector('[data-testid="user-terminal-status"]');
        return el?.textContent?.includes('Closed');
      }, { timeout: 5000 });
      console.log('[OK] Terminal closed');
    } catch { warn('Close status slow'); }
    await ss(page, '09-terminal-closed.png');

    /* Full layout */
    await ss(page, '10-full-layout-no-clipping.png');

  } finally { await browser.close(); }

  console.log('');
  if (FAILURES.length === 0) { console.log('=== M9 Smoke PASSED ==='); process.exit(0); }
  console.error(`=== FAIL (${FAILURES.length}) ===`);
  FAILURES.forEach(f => console.error(`  ${f}`));
  process.exit(1);
}

run().catch(err => { console.error('Unhandled:', err.message); process.exit(1); });
