const { chromium } = require('playwright');
const fs = require('fs');

const BASE = 'http://127.0.0.1:8756';
const OUT = '.glyphdeck/validation/remote-lifecycle/screenshots';
const PW = process.env.GLYPHDECK_VALIDATION_PASSWORD || 'admin';

(async () => {
  fs.mkdirSync(OUT, { recursive: true });

  const browser = await chromium.launch({ headless: true });
  const page = await browser.newPage({ viewport: { width: 1280, height: 720 } });

  // Login
  await page.goto(BASE);
  const loginInput = page.locator('[data-testid="login-password-input"]');
  if (await loginInput.isVisible({ timeout: 3000 }).catch(() => false)) {
    await loginInput.fill(PW);
    await page.locator('[data-testid="login-submit-button"]').click();
    await page.waitForSelector('[data-testid="top-version-label"]', { timeout: 15000 });
  }

  // Wait for activity rail to be visible after login
  await page.waitForSelector('.activity-rail', { timeout: 10000 });
  await page.waitForTimeout(500);

  // Switch to Servers view via activity rail
  await page.locator('[data-testid="activity-servers-button"]').click();
  await page.waitForTimeout(500);

  // 01 – Empty state
  await page.screenshot({ path: `${OUT}/01-empty-servers.png`, fullPage: false });
  console.log('[SCREENSHOT] 01-empty-servers.png');

  // Add an SSH target
  const addBtn = page.locator('[data-testid="server-add-button"]');
  if (await addBtn.isVisible()) {
    await addBtn.click();
    await page.waitForTimeout(300);
  }

  // 02 – Add form visible
  await page.screenshot({ path: `${OUT}/02-add-ssh-form.png`, fullPage: false });
  console.log('[SCREENSHOT] 02-add-ssh-form.png');

  // Select SSH type and submit to trigger validation
  await page.locator('[data-testid="server-add-type"]').selectOption('ssh_alias');
  await page.locator('[data-testid="server-add-submit"]').click();
  await page.waitForTimeout(300);

  // 03 – Validation error (name required, SSH alias required)
  await page.screenshot({ path: `${OUT}/03-validation-error.png`, fullPage: false });
  console.log('[SCREENSHOT] 03-validation-error.png');

  // Fill valid data and add
  await page.locator('[data-testid="server-add-name"]').fill('Test SSH Server');
  await page.locator('[data-testid="server-add-ssh-alias"]').fill('testbox');
  await page.locator('[data-testid="server-add-submit"]').click();
  await page.waitForTimeout(1500);

  // 04 – Saved offline target
  await page.screenshot({ path: `${OUT}/04-saved-offline.png`, fullPage: false });
  console.log('[SCREENSHOT] 04-saved-offline.png');

  // 05 – Narrow layout
  await page.setViewportSize({ width: 480, height: 720 });
  await page.waitForTimeout(500);
  await page.screenshot({ path: `${OUT}/05-narrow-layout.png`, fullPage: false });
  console.log('[SCREENSHOT] 05-narrow-layout.png');

  await browser.close();
  console.log('Done.');
})().catch((err) => {
  console.error('Screenshot capture failed:', err.message);
  process.exit(1);
});
