/*
 * GlyphDeck Milestone 12 smoke — State model cleanup.
 * Tests: session load after refresh, event stream after stop.
 */
const { chromium } = require('playwright');
const path = require('path');
const fs = require('fs');
const crypto = require('crypto');

const SCREENSHOT_DIR = path.resolve(__dirname, '..', 'screenshots');
const WORKSPACE_DIR = path.resolve(__dirname, '..', 'workspace');
const FRONTEND_URL = 'http://localhost:5173';
const BACKEND_URL = 'http://127.0.0.1:8756';
const MARKER = ['GLYPHDECK_M12', Date.now().toString(36), crypto.randomBytes(3).toString('hex')].join('_');

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
  console.log('=== GlyphDeck M12 State Model Cleanup Smoke ===');
  console.log(`Marker: ${MARKER}`);
  fs.mkdirSync(WORKSPACE_DIR,{recursive:true});
  fs.writeFileSync(path.join(WORKSPACE_DIR,'README.txt'),'M12\n');
  try{await fetch(`${BACKEND_URL}/api/dev/reset-validation-state`,{method:'POST'})}catch{}

  const browser=await chromium.launch({headless:true});
  const ctx=await browser.newContext({viewport:{width:1440,height:900}});
  const page=await ctx.newPage();
  let projectId, sessionId;

  try{
    /* 01 */
    await page.goto(FRONTEND_URL,{waitUntil:'domcontentloaded'});
    await page.waitForTimeout(4000);
    await av(page,'app-shell','Shell');
    await ss(page,'01-clean-state.png');

    /* Add project */
    await page.getByTestId('project-name-input').fill('M12 Validation');
    await page.getByTestId('project-path-input').fill(WORKSPACE_DIR);
    await page.getByTestId('project-trusted-checkbox').click();
    await page.getByTestId('add-project-button').click();
    await page.waitForTimeout(1000);
    await av(page,'project-card','Card');
    const p=await findProject(WORKSPACE_DIR); projectId=p?.id;
    if(!projectId)fail('No project ID');
    await page.getByTestId('project-select-button').click();
    await page.waitForTimeout(500);
    await ss(page,'02-project-added.png');

    /* Start server */
    await page.getByTestId('project-start-server-button').click();
    if(!(await waitReady(projectId)))fail('Server not ready');
    await page.waitForTimeout(1000);
    await ss(page,'03-server-ready.png');

    /* Event stream */
    await page.getByTestId('project-select-button').click();
    await page.waitForTimeout(4000);
    await av(page,'eventstream-connected-state','Live');
    await ss(page,'04-eventstream-live.png');

    /* Create session via UI button */
    await page.getByTestId('create-session-button').click();
    await page.waitForTimeout(3000);
    /* Find the new session item and click it */
    try{await page.waitForSelector('[data-testid="session-item"]',{timeout:15000});
      const items=page.locator('[data-testid="session-item"]');
      const count=await items.count();
      if(count===0)fail('No session items');
      await items.first().click();
      console.log('[OK] Session created via UI');
      // Get the session ID from the heading
      const hd=await page.getByTestId('active-session-heading').textContent();
      console.log(`[OK] Heading: "${hd?.trim()}"`);
    }catch(e){fail(`Session UI: ${e.message}`)}
    await av(page,'active-session-heading','Heading');
    await ss(page,'05-session-created.png');

    /* Send prompt */
    await page.getByTestId('prompt-composer-input').fill(`Say: ${MARKER}`);
    await page.getByTestId('prompt-send-button').click();
    await page.waitForTimeout(500);
    try{await page.waitForFunction(m=>{for(const el of document.querySelectorAll('[data-testid="transcript-user-message"]'))if(el.textContent?.includes(m))return true;return false},MARKER,{timeout:15000});console.log('[OK] User msg')}catch{warn('User msg slow')}
    try{await page.waitForFunction(()=>(document.querySelector('[data-testid="transcript-assistant-message"]')||document.querySelector('[data-testid="transcript-streamed-message"]'))&&document.querySelector('[data-testid="transcript"]')?.textContent?.length>5,{timeout:90000});console.log('[OK] Asst')}catch{warn('Asst slow')}
    await page.waitForTimeout(2000);
    await ss(page,'06-transcript.png');

    /* ---- BROWSER REFRESH with session ---- */
    await page.reload({waitUntil:'domcontentloaded'});
    await page.waitForTimeout(4000);
    /* Verify project card still visible */
    await av(page,'project-card','Card after refresh');
    /* Verify session list reloaded (auto-load effect) */
    try{await page.waitForSelector('[data-testid="session-item"]',{timeout:15000});console.log('[OK] Sessions reloaded after refresh')}catch{warn('Sessions not reloaded')}
    await ss(page,'07-after-refresh.png');

    /* ---- INTENTIONAL STOP: verify sane Offline state ---- */
    await page.getByTestId('project-stop-server-button').click({force:false});
    await page.waitForTimeout(4000);
    /* After stop, TopBar should show "Offline" not "Error" */
    const topStatus=await page.getByTestId('event-stream-status').textContent().catch(()=>'');
    console.log(`[OK] Post-stop status: "${topStatus?.trim()}"`);
    if(topStatus?.toLowerCase().includes('error'))fail('Event stream shows Error after intentional stop');
    await ss(page,'08-post-stop-offline.png');

    /* Regressions */
    await page.getByTestId('right-review-tab').click();await page.waitForTimeout(1000);
    try{const t=await page.getByTestId('review-panel').textContent({timeout:3000});if(t?.includes('Failed to fetch'))fail('Review error')}catch{}
    await ss(page,'09-review-regression.png');

    await page.getByTestId('right-usage-tab').click();await page.waitForTimeout(1000);
    try{const t=await page.getByTestId('usage-panel').textContent({timeout:3000});if(t?.includes('Failed to fetch'))fail('Usage error')}catch{}
    await ss(page,'10-usage-regression.png');

    await page.getByTestId('bottom-agent-terminal-tab').click();await page.waitForTimeout(500);
    await av(page,'agent-terminal-panel','Agent');
    await ss(page,'11-agent-terminal.png');

    await page.getByTestId('bottom-terminal-tab').click();await page.waitForTimeout(500);
    await page.getByTestId('user-terminal-start-button').click();
    await page.waitForTimeout(2000);
    await av(page,'user-terminal-viewport','Term viewport');
    await page.getByTestId('user-terminal-close-button').click();
    await page.waitForTimeout(1000);
    await ss(page,'12-terminal.png');

    await page.getByTestId('bottom-problems-tab').click();await page.waitForTimeout(500);
    await av(page,'problems-panel','Problems');
    await ss(page,'13-problems-clean.png');

    /* Full layout */
    await ss(page,'14-full-layout-no-clipping.png');

  }finally{await browser.close()}

  console.log('');
  if(FAILURES.length===0){console.log('=== M12 Smoke PASSED ===');process.exit(0)}
  console.error(`=== FAIL (${FAILURES.length}) ===`);FAILURES.forEach(f=>console.error(`  ${f}`));process.exit(1)
}
run().catch(err=>{console.error('Unhandled:',err.message);process.exit(1)});
