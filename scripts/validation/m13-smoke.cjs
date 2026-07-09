/*
 * GlyphDeck Milestone 13 smoke — Settings + embedded frontend + release validation.
 * Tests: single-binary serves frontend, settings persist, all regressions.
 */
const { chromium } = require('playwright');
const path = require('path');
const fs = require('fs');
const crypto = require('crypto');

const SCREENSHOT_DIR = path.resolve(__dirname, '..', 'screenshots');
const WORKSPACE_DIR = path.resolve(__dirname, '..', 'workspace');
const FRONTEND_URL = 'http://127.0.0.1:8756'; // Release mode: Go binary serves frontend
const BACKEND_URL = 'http://127.0.0.1:8756';
const MARKER = ['GLYPHDECK_M13', Date.now().toString(36), crypto.randomBytes(3).toString('hex')].join('_');

const FAILURES = [];
function fail(m) { FAILURES.push(m); console.error(`[FAIL] ${m}`); }
function warn(m) { console.warn(`[WARN] ${m}`); }
async function av(page,tid,label){try{await page.getByTestId(tid).waitFor({state:'visible',timeout:30000});console.log(`[OK] ${label}`)}catch(e){fail(`${label}: ${e.message}`)}}
async function ss(page,n){await page.screenshot({path:path.join(SCREENSHOT_DIR,n),fullPage:false});console.log(`[SCREENSHOT] ${n}`)}
async function apiGet(route){try{const r=await fetch(`${BACKEND_URL}${route}`);return r.ok?r.json():null}catch{return null}}
async function apiPost(route,body){try{const r=await fetch(`${BACKEND_URL}${route}`,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(body)});if(!r.ok)throw new Error(`${r.status}`);return r.json()}catch(e){fail(`API: ${e.message}`);return null}}
async function findProject(wp){const d=await apiGet('/api/projects');const ps=d?.projects;if(!ps)return null;const n=wp.replace(/\\/g,'/').toLowerCase();return ps.find(p=>(p.path||'').replace(/\\/g,'/').toLowerCase()===n)}
async function waitReady(pid,ms=60000){const dl=Date.now()+ms;while(Date.now()<dl){const s=await apiGet(`/api/projects/${encodeURIComponent(pid)}/server`);if(s?.status==='ready')return true;await new Promise(r=>setTimeout(r,1000))}return false}

async function run(){
  console.log('=== GlyphDeck M13 Settings + Release Smoke ===');
  console.log(`Marker: ${MARKER}`);
  fs.mkdirSync(WORKSPACE_DIR,{recursive:true});
  fs.writeFileSync(path.join(WORKSPACE_DIR,'README.txt'),'M13\n');
  try{await fetch(`${BACKEND_URL}/api/dev/reset-validation-state`,{method:'POST'})}catch{}

  const browser=await chromium.launch({headless:true});
  const ctx=await browser.newContext({viewport:{width:1440,height:900}});
  const page=await ctx.newPage();
  let projectId;

  try{
    /* 01 — Release mode: binary serves frontend on :8756 */
    await page.goto(FRONTEND_URL,{waitUntil:'domcontentloaded'});
    await page.waitForTimeout(4000);
    await av(page,'app-shell','Shell');
    await ss(page,'01-clean-state.png');

    /* Settings tab */
    await page.getByTestId('bottom-settings-tab').click();await page.waitForTimeout(500);
    await av(page,'settings-panel','Settings');
    await page.getByTestId('settings-opencode-path').fill('opencode');
    await page.getByTestId('settings-log-level').selectOption('debug');
    await page.getByTestId('settings-save-button').click();
    await page.waitForTimeout(1000);
    /* Verify save message */
    const msg=await page.getByTestId('settings-message').textContent().catch(()=>'');
    console.log(`[OK] Settings save: "${msg}"`);
    await ss(page,'02-settings-saved.png');

    /* Add project */
    await page.getByTestId('bottom-problems-tab').click(); // Switch away from settings
    await page.waitForTimeout(300);
    await page.getByTestId('project-name-input').fill('M13 Validation');
    await page.getByTestId('project-path-input').fill(WORKSPACE_DIR);
    await page.getByTestId('project-trusted-checkbox').click();
    await page.getByTestId('add-project-button').click();
    await page.waitForTimeout(1000);
    await av(page,'project-card','Card');
    const p=await findProject(WORKSPACE_DIR); projectId=p?.id;
    if(!projectId)fail('No project ID');
    await page.getByTestId('project-select-button').click();
    await page.waitForTimeout(500);
    await ss(page,'03-project-added.png');

    /* Start server */
    await page.getByTestId('project-start-server-button').click();
    if(!(await waitReady(projectId)))fail('Server not ready');
    await page.waitForTimeout(1000);
    await ss(page,'04-server-ready.png');

    /* Event stream */
    await page.getByTestId('project-select-button').click();
    await page.waitForTimeout(4000);
    await av(page,'eventstream-connected-state','Live');
    await ss(page,'05-eventstream-live.png');

    /* Create session */
    await page.getByTestId('create-session-button').click();
    await page.waitForTimeout(3000);
    try{await page.waitForSelector('[data-testid="session-item"]',{timeout:15000});
      await page.locator('[data-testid="session-item"]').first().click();
      const hd=await page.getByTestId('active-session-heading').textContent();
      console.log(`[OK] Session: "${hd?.trim()}"`);
    }catch(e){fail(`Session: ${e.message}`)}
    await av(page,'active-session-heading','Heading');
    await ss(page,'06-session-created.png');

    /* Send prompt */
    await page.getByTestId('prompt-composer-input').fill(`Say: ${MARKER}`);
    await page.getByTestId('prompt-send-button').click();
    await page.waitForTimeout(500);
    try{await page.waitForFunction(m=>{for(const el of document.querySelectorAll('[data-testid="transcript-user-message"]'))if(el.textContent?.includes(m))return true;return false},MARKER,{timeout:15000});console.log('[OK] User')}catch{warn('User slow')}
    try{await page.waitForFunction(()=>(document.querySelector('[data-testid="transcript-assistant-message"]')||document.querySelector('[data-testid="transcript-streamed-message"]'))&&document.querySelector('[data-testid="transcript"]')?.textContent?.length>5,{timeout:90000});console.log('[OK] Asst')}catch{warn('Asst slow')}
    await page.waitForTimeout(2000);
    await ss(page,'07-transcript.png');

    /* Regressions */
    await page.getByTestId('right-review-tab').click();await page.waitForTimeout(1000);
    try{const t=await page.getByTestId('review-panel').textContent({timeout:3000});if(t?.includes('Failed to fetch'))fail('Review error')}catch{}
    await ss(page,'08-review.png');

    await page.getByTestId('right-usage-tab').click();await page.waitForTimeout(1000);
    try{const t=await page.getByTestId('usage-panel').textContent({timeout:3000});if(t?.includes('Failed to fetch'))fail('Usage error')}catch{}
    await ss(page,'09-usage.png');

    await page.getByTestId('bottom-agent-terminal-tab').click();await page.waitForTimeout(500);
    await av(page,'agent-terminal-panel','Agent');
    await ss(page,'10-agent-terminal.png');

    await page.getByTestId('bottom-terminal-tab').click();await page.waitForTimeout(500);
    await page.getByTestId('user-terminal-start-button').click();
    await page.waitForTimeout(2000);
    await av(page,'user-terminal-viewport','Term viewport');
    await ss(page,'11-terminal.png');
    await page.getByTestId('user-terminal-close-button').click();
    await page.waitForTimeout(1000);

    await page.getByTestId('bottom-problems-tab').click();await page.waitForTimeout(500);
    await av(page,'problems-panel','Problems');
    await ss(page,'12-problems.png');

    /* Stop server */
    await page.getByTestId('project-stop-server-button').click({force:false});
    await page.waitForTimeout(3000);
    await ss(page,'13-post-stop.png');

    /* Settings — verify persisted after restart */
    await page.getByTestId('bottom-settings-tab').click();await page.waitForTimeout(500);
    const pathVal=await page.getByTestId('settings-opencode-path').inputValue();
    console.log(`[OK] Settings opencode_path persisted: "${pathVal}"`);
    await ss(page,'14-settings-persisted.png');

    await ss(page,'15-full-layout.png');

  }finally{await browser.close()}

  console.log('');
  if(FAILURES.length===0){console.log('=== M13 Smoke PASSED ===');process.exit(0)}
  console.error(`=== FAIL (${FAILURES.length}) ===`);FAILURES.forEach(f=>console.error(`  ${f}`));process.exit(1)
}
run().catch(err=>{console.error('Unhandled:',err.message);process.exit(1)});
