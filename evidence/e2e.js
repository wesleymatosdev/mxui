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
  await page.waitForSelector('#msgs', { state: 'visible', timeout: 15000 });
  await page.waitForSelector('.mxui-btn', { timeout: 15000 });
  await page.screenshot({ path: '/Users/wesleymatos/projects/personal/mxui/evidence/01-buttons-rendered.png' });

  const nbtn = await page.locator('.mxui-btn').count();
  const nmsg = await page.locator('.msg').count();
  console.log(`RENDER: ${nmsg} messages, ${nbtn} buttons`);

  await page.locator('.mxui-btn.primary', { hasText: 'Approve' }).first().click();
  await page.waitForFunction(() =>
    [...document.querySelectorAll('.msg .txt')].some(t => t.textContent.includes('Approved')),
    { timeout: 20000 });
  await page.screenshot({ path: '/Users/wesleymatos/projects/personal/mxui/evidence/02-after-approve.png' });
  const approved = await page.evaluate(() =>
    [...document.querySelectorAll('.msg .txt')]
      .filter(t => t.textContent.includes('Approved')).map(t => t.textContent));
  console.log('APPROVED-TEXTS:', JSON.stringify(approved.slice(-2)));
  console.log('STATUS:', await page.locator('#status').innerText());
  console.log('CONSOLE:', JSON.stringify(logs.slice(-8), null, 1));
  await browser.close();
  console.log('E2E-PASS');
})().catch(e => { console.error('E2E-FAIL:', e.message); process.exit(1); });
