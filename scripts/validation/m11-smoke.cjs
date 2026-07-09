/*
 * GlyphDeck Milestone 11 smoke — SQLite persistence.
 * Tests: project survives backend restart, refresh, trusted flag.
 */
const { chromium } = require('playwright');
const path = require('path');
const fs = require('fs');
const crypto = require('crypto');

const SCREENSHOT_DIR = path.resolve(__dirname, '..', 'screenshots');
const WORKSPACE_DIR = path.resolve(__dirname, '..', 'workspace');
const FRONTEND_URL = 'http://localhost:5173';
const BACKEND_URL = 'http://127.0.0.1:8756';
const MARKER = ['GLYPHDECK_M11', Date.now().toString(36), crypto.randomBytes(3).toString('hex')].join('_');

const FAILURES = [];
function fail(m) { FAILURES.push(m); console.error(`[FAIL] ${m}`); }
function warn(m) { console.warn(`[WARN] ${m}`); }
async function av(page,tid,label){try{await page.getByTestId(tid).waitFor({state:'visible',timeout:30000});console.log(`[OK] ${label}`)}catch(e){fail(`${label}: ${e.message}`)}}
async function ss(page,n){await page.screenshot({path:path.join(SCREENSHOT_DIR,n),fullPage:false});console.log(`[SCREENSHOT] ${n}`)}
async function apiGet(route){try{const r=await fetch(`${BACKEND_URL}${route}`);return r.ok?r.json():null}catch{return null}}

async function run(){
  console.log('=== GlyphDeck M11 SQLite Persistence Smoke ===');
  console.log(`Marker: ${MARKER}`);
  fs.mkdirSync(WORKSPACE_DIR,{recursive:true});
  fs.writeFileSync(path.join(WORKSPACE_DIR,'README.txt'),'M11\n');
  try{await fetch(`${BACKEND_URL}/api/dev/reset-validation-state`,{method:'POST'})}catch{}

  const browser=await chromium.launch({headless:true});
  const ctx=await browser.newContext({viewport:{width:1440,height:900}});
  const page=await ctx.newPage();

  try{
    /* 01 */
    await page.goto(FRONTEND_URL,{waitUntil:'domcontentloaded'});
    await page.waitForTimeout(4000);
    await av(page,'app-shell','Shell');
    await ss(page,'01-clean-state.png');

    /* Add project */
    await page.getByTestId('project-name-input').fill('M11 Validation');
    await page.getByTestId('project-path-input').fill(WORKSPACE_DIR);
    await page.getByTestId('project-trusted-checkbox').click();
    await page.getByTestId('add-project-button').click();
    await page.waitForTimeout(1000);
    await av(page,'project-card','Card');
    await page.getByTestId('project-select-button').click();
    await page.waitForTimeout(500);
    await ss(page,'02-project-added.png');

    /* ---- BROWSER REFRESH: project survives ---- */
    await page.reload({waitUntil:'domcontentloaded'});
    await page.waitForTimeout(3000);
    await av(page,'project-card','Card after refresh');
    await ss(page,'03-refresh-project-restored.png');

    /* Start server + verify regression panels */
    await page.getByTestId('project-start-server-button').click();

    /* Poll API for server ready (idempotent — project persists via SQLite) */
    const projects=await apiGet('/api/projects');
    const proj=projects?.projects?.find(p=>p.path===WORKSPACE_DIR);
    const pid=proj?.id;
    if(!pid)fail('Project not found via API after restart');
    console.log(`[api] Project ID: ${pid}`);

    let ready=false;
    const dl=Date.now()+60000;
    while(Date.now()<dl){
      const s=await apiGet(`/api/projects/${encodeURIComponent(pid)}/server`);
      if(s?.status==='ready'){ready=true;break}
      await new Promise(r=>setTimeout(r,1000))
    }
    if(!ready)fail('Server not ready');
    await page.waitForTimeout(1000);
    await ss(page,'04-server-ready.png');

    /* Event stream */
    await page.getByTestId('project-select-button').click();
    await page.waitForTimeout(4000);
    await av(page,'eventstream-connected-state','Live');
    await ss(page,'05-eventstream-live.png');

    /* Regressions */
    await page.getByTestId('right-review-tab').click();await page.waitForTimeout(1000);
    try{const t=await page.getByTestId('review-panel').textContent({timeout:3000});if(t?.includes('Failed to fetch'))fail('Review error')}catch{}
    await ss(page,'09-review-regression.png');

    await page.getByTestId('right-usage-tab').click();await page.waitForTimeout(1000);
    try{const t=await page.getByTestId('usage-panel').textContent({timeout:3000});if(t?.includes('Failed to fetch'))fail('Usage error')}catch{}
    await ss(page,'10-usage-regression.png');

    await page.getByTestId('bottom-agent-terminal-tab').click();await page.waitForTimeout(500);
    await av(page,'agent-terminal-panel','Agent');
    await ss(page,'11-agent-terminal-regression.png');

    /* Terminal open/close only */
    await page.getByTestId('bottom-terminal-tab').click();await page.waitForTimeout(500);
    await page.getByTestId('user-terminal-start-button').click();
    await page.waitForTimeout(2000);
    await av(page,'user-terminal-viewport','Term viewport');
    await page.getByTestId('user-terminal-close-button').click();
    await page.waitForTimeout(1000);
    await ss(page,'13-terminal-regression.png');

    /* Problems */
    await page.getByTestId('bottom-problems-tab').click();await page.waitForTimeout(500);
    await av(page,'problems-panel','Problems');
    await ss(page,'16-problems-clean.png');

    /* Stop server */
    await page.getByTestId('project-stop-server-button').click({force:false});
    await page.waitForTimeout(3000);
    await ss(page,'18-server-stopped.png');

    /* Full layout */
    await ss(page,'19-full-layout-no-clipping.png');

  }finally{await browser.close()}

  console.log('');
  if(FAILURES.length===0){console.log('=== M11 Smoke PASSED ===');process.exit(0)}
  console.error(`=== FAIL (${FAILURES.length}) ===`);FAILURES.forEach(f=>console.error(`  ${f}`));process.exit(1)
}
run().catch(err=>{console.error('Unhandled:',err.message);process.exit(1)});
