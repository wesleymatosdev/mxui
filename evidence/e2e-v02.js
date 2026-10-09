const { chromium } = require('playwright');

(async () => {
  const browser = await chromium.launch({executablePath: "/Users/wesleymatos/Library/Caches/ms-playwright/chromium_headless_shell-1243/chrome-headless-shell-mac-arm64/chrome-headless-shell"});
  const page = await browser.newPage({ viewport: { width: 1100, height: 800 } });
  const logs = [];
  page.on('console', m => logs.push(`[console.${m.type()}] ${m.text()}`));
  page.on('pageerror', e => logs.push(`[pageerror] ${e}`));

  await page.goto('http://localhost:8791/');
  await page.waitForSelector('#go');
  await page.click('#go');
  await page.waitForSelector('.mxui-btn', { timeout: 15000 });

  // 1. Composer: send a plain message
  await page.fill('#draft', 'hello from the mxui web client composer');
  await page.click('#send');
  await page.waitForFunction(() =>
    [...document.querySelectorAll('.msg .txt')].some(t =>
      t.textContent.includes('hello from the mxui web client composer')),
    { timeout: 10000 });
  console.log('COMPOSER-PASS');

  // 2. Select menu exists with options
  const selCount = await page.locator('.mxui-select').count();
  await page.selectOption('.mxui-select', 'staging');
  console.log('SELECT-RENDER-PASS count=' + selCount);

  // 3. Approve with staging selected
  const last = page.locator('.msg', { has: page.locator('.mxui-select') }).last();
  await last.locator('.mxui-btn.primary').click();
  await page.waitForFunction(() =>
    [...document.querySelectorAll('.msg .txt')].some(t =>
      t.textContent.includes('Approved by') && t.textContent.includes('staging')),
    { timeout: 20000 });
  console.log('APPROVE-WITH-ENV-PASS');
  await page.screenshot({ path: '/Users/wesleymatos/projects/personal/mxui/evidence/v02-01-approve-staging.png' });

  // 4. Slash command
  await page.fill('#draft', '/deploy v2.0.0');
  await page.click('#send');
  await page.waitForFunction(() =>
    [...document.querySelectorAll('.msg .txt')].some(t =>
      t.textContent.includes('Deploying v2.0.0') && t.textContent.includes('ephemeral')),
    { timeout: 20000 });
  console.log('SLASH-COMMAND-PASS');
  await page.screenshot({ path: '/Users/wesleymatos/projects/personal/mxui/evidence/v02-02-slash.png' });

  // 5. Modal: get a fresh card via /card, then click Configure
  await page.fill('#draft', '/card');
  await page.click('#send');
  await page.waitForFunction(() => {
    const cards = [...document.querySelectorAll('.msg')];
    return cards.some(m => m.textContent.includes('fresh card') &&
      m.querySelector('.mxui-btn') && !m.querySelector('.mxui-btn').disabled);
  }, { timeout: 20000 });
  const fresh = page.locator('.msg', { hasText: 'fresh card' }).last();
  await fresh.locator('.mxui-btn', { hasText: 'Configure…' }).click();
  await page.waitForSelector('#modalBack.open', { timeout: 15000 });
  await page.fill('#mf-version', 'v9.9.9');
  await page.fill('#mf-replicas', '5');
  await page.selectOption('#mf-strategy', 'bluegreen');
  await page.screenshot({ path: '/Users/wesleymatos/projects/personal/mxui/evidence/v02-03-modal.png' });
  await page.click('#modalSubmit');
  await page.waitForFunction(() =>
    [...document.querySelectorAll('.msg .txt')].some(t =>
      t.textContent.includes('Scheduled by') && t.textContent.includes('v9.9.9') &&
      t.textContent.includes('bluegreen')),
    { timeout: 20000 });
  console.log('MODAL-ROUNDTRIP-PASS');
  await page.screenshot({ path: '/Users/wesleymatos/projects/personal/mxui/evidence/v02-04-scheduled.png' });

  console.log('CONSOLE:', JSON.stringify(logs.slice(-6)));
  await browser.close();
  console.log('V02-E2E-PASS');
})().catch(e => { console.error('E2E-FAIL:', e.message); process.exit(1); });
