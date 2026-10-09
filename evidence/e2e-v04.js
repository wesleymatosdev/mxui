const { chromium } = require('playwright');

(async () => {
  const browser = await chromium.launch({executablePath: "/Users/wesleymatos/Library/Caches/ms-playwright/chromium_headless_shell-1243/chrome-headless-shell-mac-arm64/chrome-headless-shell"});
  const page = await browser.newPage({ viewport: { width: 1100, height: 800 } });
  await page.goto('http://localhost:8791/');
  await page.waitForSelector('#go');
  await page.click('#go');

  // wait for the webhook-bot's card to render with enabled buttons
  await page.waitForFunction(() => {
    const cards = [...document.querySelectorAll('.msg')];
    return cards.some(m => m.textContent.includes('Webhook-bot deploy card') &&
      m.querySelector('.mxui-btn.primary') && !m.querySelector('.mxui-btn.primary').disabled);
  }, { timeout: 20000 });
  console.log('WEBHOOK-CARD-RENDERED');

  const card = page.locator('.msg', { hasText: 'Webhook-bot deploy card' }).last();
  await card.locator('.mxui-btn.primary').click();
  await page.waitForFunction(() =>
    [...document.querySelectorAll('.msg .txt')].some(t =>
      t.textContent.includes('Webhook-bot deploy card — ✅ Approved by')),
    { timeout: 25000 });
  console.log('WEBHOOK-APPROVE-ROUNDTRIP-PASS');
  await page.screenshot({ path: '/Users/wesleymatos/projects/personal/mxui/evidence/v04-01-webhook-approve.png' });

  await browser.close();
  console.log('V04-E2E-PASS');
})().catch(e => { console.error('E2E-FAIL:', e.message); process.exit(1); });
