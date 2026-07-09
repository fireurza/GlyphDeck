/*
 * GlyphDeck Milestone 14 smoke — Terminal reliability / SSE buffering fix.
 * Hard assertion: terminal marker output MUST be visible.
 */
const { chromium } = require('playwright');
const path = require('path');
const fs = require('fs');
const crypto = require('crypto');

const SCREENSHOT_DIR = path.resolve(__dirname, '..', 'screenshots');
const WORKSPACE_DIR = path.resolve(__dirname, '..', 'workspace');
const FRONTEND_URL = 'http://127.0.0.1:8756';
const BACKEND_URL = 'http://127.0.0.1:8756';
const MARKER = ['GLYPHDECK_M14', Date.now().toString(36), crypto.randomBytes(3).toString('hex')].join('_');

const FAILURES = [];
function fail(m) { FAILURES.push(m); console.error(`[FAIL] ${m}`); }
function warn(m) { console.warn(`[WARN] ${m}`); }
async function av(page,tid,label){try{await page.getByTestId(tid).waitFor({state:'visible',timeout:30000});console.log(`[OK] ${label}`)}catch(e){fail(`${label}: ${e.message}`)}}
async function ss(page,n){await page.screenshot({path:path.join(SCREENSHOT_DIR,n),fullPage:false});console.log(`[SCREENSHOT] ${n}`)}
async function apiGet(route){try{const r=await fetch(`${BACKEND_URL}${route}`);return r.ok?r.json():null}catch{return null}}

async function run(){
  console.log('=== GlyphDeck M14 Terminal Reliability Smoke ===');
  console.log(`Marker: ${MARKER}`);
  fs.mkdirSync(WORKSPACE_DIR,{recursive:true});
  fs.writeFileSync(path.join(WORKSPACE_DIR,'README.txt'),'M14\n');
  try{await fetch(`${BACKEND_URL}/api/dev/reset-validation-state`,{method:'POST'})}catch{}

  const browser=await chromium.launch({headless:true});
  const ctx=await browser.newContext({viewport:{width:1440,height:900}});
  const page=await ctx.newPage();
  let projectId;

  try{
    await page.goto(FRONTEND_URL,{waitUntil:'domcontentloaded'});
    await page.waitForTimeout(4000);
    await av(page,'app-shell','Shell');
    await ss(page,'01-clean-state.png');

    /* Add project */
    await page.getByTestId('project-name-input').fill('M14 Validation');
    await page.getByTestId('project-path-input').fill(WORKSPACE_DIR);
    await page.getByTestId('project-trusted-checkbox').click();
    await page.getByTestId('add-project-button').click();
    await page.waitForTimeout(1000);
    await av(page,'project-card','Card');
    const projects=await apiGet('/api/projects');
    const proj=projects?.projects?.find(p=>p.path===WORKSPACE_DIR);
    projectId=proj?.id; if(!projectId)fail('No project');
    await page.getByTestId('project-select-button').click();
    await page.waitForTimeout(500);
    await ss(page,'02-project-added.png');

    /* Start server */
    await page.getByTestId('project-start-server-button').click();
    let ready=false;
    const dl=Date.now()+60000;
    while(Date.now()<dl){const s=await apiGet(`/api/projects/${encodeURIComponent(projectId)}/server`);if(s?.status==='ready'){ready=true;break}await new Promise(r=>setTimeout(r,1000))}
    if(!ready)fail('Server not ready');
    await page.waitForTimeout(1000);
    await ss(page,'03-server-ready.png');

    /* Event stream */
    await page.getByTestId('project-select-button').click();
    await page.waitForTimeout(4000);
    await av(page,'eventstream-connected-state','Live');
    await ss(page,'04-eventstream-live.png');

    /* Session */
    await page.getByTestId('create-session-button').click();
    await page.waitForTimeout(3000);
    try{await page.waitForSelector('[data-testid="session-item"]',{timeout:15000});await page.locator('[data-testid="session-item"]').first().click();console.log('[OK] Session')}catch(e){fail(`Session: ${e.message}`)}
    await av(page,'active-session-heading','Heading');
    await ss(page,'05-session-created.png');

    /* ====== TERMINAL: hard assertion on marker output ====== */
    await page.getByTestId('bottom-terminal-tab').click();await page.waitForTimeout(500);
    await page.getByTestId('user-terminal-start-button').click();
    await page.waitForTimeout(2000);
    await av(page,'user-terminal-viewport','Term viewport');
    try{await page.waitForFunction(()=>{const el=document.querySelector('[data-testid="user-terminal-status"]');return el?.textContent?.includes('Running')},{timeout:10000});console.log('[OK] Running')}catch{warn('Running slow')}
    await ss(page,'06-terminal-running.png');

    /* Send echo marker and verify output appears */
    await page.waitForTimeout(1000);
    await page.getByTestId('user-terminal-input').fill(`echo ${MARKER}`);
    await page.getByTestId('user-terminal-input').press('Enter');
    /* Wait for marker to appear in terminal output — this MUST work now */
    try{await page.waitForFunction((m)=>{const el=document.querySelector('[data-testid="user-terminal-output"]');return el?.textContent?.includes(m)},MARKER,{timeout:10000});console.log('[OK] Terminal marker VISIBLE')}catch(e){fail(`Terminal marker NOT visible: ${e.message}`)}
    await ss(page,'07-terminal-marker-visible.png');

    /* Send a second command to confirm streaming works */
    await page.getByTestId('user-terminal-input').fill(`Get-Location`);
    await page.getByTestId('user-terminal-input').press('Enter');
    await page.waitForTimeout(3000);
    await ss(page,'08-terminal-output.png');

    await page.getByTestId('user-terminal-close-button').click();await page.waitForTimeout(1000);
    await ss(page,'09-terminal-closed.png');

    /* Regressions */
    await page.getByTestId('right-review-tab').click();await page.waitForTimeout(1000);
    try{const t=await page.getByTestId('review-panel').textContent({timeout:3000});if(t?.includes('Failed to fetch'))fail('Review error')}catch{}
    await ss(page,'10-review.png');

    await page.getByTestId('right-usage-tab').click();await page.waitForTimeout(1000);
    try{const t=await page.getByTestId('usage-panel').textContent({timeout:3000});if(t?.includes('Failed to fetch'))fail('Usage error')}catch{}
    await ss(page,'11-usage.png');

    await page.getByTestId('bottom-agent-terminal-tab').click();await page.waitForTimeout(500);
    await av(page,'agent-terminal-panel','Agent');
    await ss(page,'12-agent-terminal.png');

    await page.getByTestId('bottom-settings-tab').click();await page.waitForTimeout(500);
    await av(page,'settings-panel','Settings');
    await ss(page,'13-settings.png');

    await page.getByTestId('bottom-problems-tab').click();await page.waitForTimeout(500);
    await av(page,'problems-panel','Problems');
    await ss(page,'14-problems.png');

    /* Stop server */
    await page.getByTestId('project-stop-server-button').click({force:false});
    await page.waitForTimeout(3000);
    await ss(page,'15-post-stop.png');
    await ss(page,'16-full-layout.png');

  }finally{await browser.close()}

  console.log('');
  if(FAILURES.length===0){console.log('=== M14 Smoke PASSED ===');process.exit(0)}
  console.error(`=== FAIL (${FAILURES.length}) ===`);FAILURES.forEach(f=>console.error(`  ${f}`));process.exit(1)
}
run().catch(err=>{console.error('Unhandled:',err.message);process.exit(1)});
