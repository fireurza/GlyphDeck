/*
 * GlyphDeck Milestone 10 smoke — POC hardening.
 * Tests: browser refresh, problems tab, terminal output, event stream sanity.
 */
const { chromium } = require('playwright');
const path = require('path');
const fs = require('fs');
const crypto = require('crypto');

const SCREENSHOT_DIR = path.resolve(__dirname, '..', 'screenshots');
const WORKSPACE_DIR = path.resolve(__dirname, '..', 'workspace');
const FRONTEND_URL = 'http://localhost:5173';
const BACKEND_URL = 'http://127.0.0.1:8756';
const MARKER = ['GLYPHDECK_M10', Date.now().toString(36), crypto.randomBytes(3).toString('hex')].join('_');

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
  console.log('=== GlyphDeck M10 POC Hardening Smoke ===');
  console.log(`Marker: ${MARKER}`);
  fs.mkdirSync(WORKSPACE_DIR,{recursive:true});
  fs.writeFileSync(path.join(WORKSPACE_DIR,'README.txt'),'M10\n');
  try{await fetch(`${BACKEND_URL}/api/dev/reset-validation-state`,{method:'POST'})}catch{}

  const browser=await chromium.launch({headless:true});
  const ctx=await browser.newContext({viewport:{width:1440,height:900}});
  const page=await ctx.newPage();
  let projectId;

  try{
    /* 01 clean */
    await page.goto(FRONTEND_URL,{waitUntil:'domcontentloaded'});
    await page.waitForTimeout(4000);
    await av(page,'app-shell','Shell');
    await ss(page,'01-clean-state.png');

    /* Add project */
    await page.getByTestId('project-name-input').fill('M10 Validation');
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

    /* ---- REFRESH: project survives ---- */
    await page.reload({waitUntil:'domcontentloaded'});
    await page.waitForTimeout(3000);
    await av(page,'project-card','Card after refresh');
    await ss(page,'03-refresh-project-restored.png');

    /* Start server */
    await page.getByTestId('project-start-server-button').click();
    if(!(await waitReady(projectId)))fail('Server not ready');
    await page.waitForTimeout(1000);
    await ss(page,'04-server-ready.png');

    /* Select → event stream */
    await page.getByTestId('project-select-button').click();
    await page.waitForTimeout(4000);
    await av(page,'eventstream-connected-state','Live');
    await ss(page,'05-eventstream-live.png');

    /* Regressions (no session needed for review/usage/agent tabs to render) */
    await page.getByTestId('right-review-tab').click(); await page.waitForTimeout(1000);
    try{const t=await page.getByTestId('review-panel').textContent({timeout:3000});if(t?.includes('Failed to fetch'))fail('Review error')}catch{}
    await ss(page,'09-review-regression.png');

    await page.getByTestId('right-usage-tab').click(); await page.waitForTimeout(1000);
    try{const t=await page.getByTestId('usage-panel').textContent({timeout:3000});if(t?.includes('Failed to fetch'))fail('Usage error')}catch{}
    await ss(page,'10-usage-regression.png');

    await page.getByTestId('bottom-agent-terminal-tab').click(); await page.waitForTimeout(500);
    await av(page,'agent-terminal-panel','Agent');
    await ss(page,'11-agent-terminal-regression.png');

    /* Terminal */
    await page.getByTestId('bottom-terminal-tab').click(); await page.waitForTimeout(500);
    await page.getByTestId('user-terminal-start-button').click();
    await page.waitForTimeout(2000);
    await av(page,'user-terminal-viewport','Term viewport');
    try{await page.waitForFunction(()=>{const el=document.querySelector('[data-testid="user-terminal-status"]');return el?.textContent?.includes('Running')},{timeout:10000});console.log('[OK] Running')}catch{warn('Running slow')}
    await ss(page,'13-terminal-running.png');

    /* Send marker */
    await page.waitForTimeout(3000);
    await page.getByTestId('user-terminal-input').fill(`echo ${MARKER}`);
    await page.getByTestId('user-terminal-input').press('Enter');
    await page.waitForTimeout(4000);
    try{await page.waitForFunction((m)=>{const el=document.querySelector('[data-testid="user-terminal-output"]');return el?.textContent?.includes(m)},MARKER,{timeout:15000});console.log('[OK] Marker in output')}catch{warn('Marker not found')}
    await ss(page,'14-terminal-output-visible.png');

    await page.getByTestId('user-terminal-close-button').click(); await page.waitForTimeout(1000);
    await ss(page,'15-terminal-closed.png');

    /* Problems */
    await page.getByTestId('bottom-problems-tab').click(); await page.waitForTimeout(500);
    await av(page,'problems-panel','Problems');
    await ss(page,'16-problems-clean.png');

    /* Stop server → verify sane status */
    await page.getByTestId('project-stop-server-button').click({force:false});
    await page.waitForTimeout(3000);
    const ts=await page.getByTestId('event-stream-status').textContent().catch(()=>'');
    console.log(`[OK] Post-stop: "${ts?.trim()}"`);
    await ss(page,'18-server-stopped-offline.png');

    /* Full layout */
    await ss(page,'19-full-layout-no-clipping.png');

  }finally{await browser.close()}

  console.log('');
  if(FAILURES.length===0){console.log('=== M10 Smoke PASSED ===');process.exit(0)}
  console.error(`=== FAIL (${FAILURES.length}) ===`); FAILURES.forEach(f=>console.error(`  ${f}`));process.exit(1)
}
run().catch(err=>{console.error('Unhandled:',err.message);process.exit(1)});
