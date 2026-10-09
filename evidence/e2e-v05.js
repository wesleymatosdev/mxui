const { chromium } = require('playwright');

(async () => {
  const browser = await chromium.launch({executablePath: "/Users/wesleymatos/Library/Caches/ms-playwright/chromium_headless_shell-1243/chrome-headless-shell-mac-arm64/chrome-headless-shell"});
  const page = await browser.newPage({ viewport: { width: 1100, height: 800 } });
  await page.goto('http://localhost:8791/');
  await page.waitForSelector('#go');
  await page.click('#go');

  await page.waitForFunction(() => {
    const cards = [...document.querySelectorAll('.msg')];
    return cards.some(m => m.textContent.includes('discord.js says: deploy v3.0.0?') &&
      m.querySelector('.mxui-btn.primary') && !m.querySelector('.mxui-btn.primary').disabled);
  }, { timeout: 20000 });
  console.log('DISCORD-CARD-RENDERED');

  const card = page.locator('.msg', { hasText: 'discord.js says: deploy v3.0.0?' }).last();
  await card.locator('.mxui-btn.primary').click();

  await page.waitForFunction(() =>
    [...document.querySelectorAll('.msg .txt')].some(t =>
      t.textContent.includes('discord.js deploy — ✅ Approved via interaction')),
    { timeout: 25000 });
  console.log('DISCORD-INTERACTION-ROUNDTRIP-PASS');
  await page.screenshot({ path: '/Users/wesleymatos/projects/personal/mxui/evidence/v05-01-discordjs-roundtrip.png' });
  await browser.close();
  console.log('V05-E2E-PASS');
})().catch(e => { console.error('E2E-FAIL:', e.message); process.exit(1); });
