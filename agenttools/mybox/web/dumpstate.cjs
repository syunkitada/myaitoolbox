const { chromium } = require('@playwright/test')
;(async () => {
  const browser = await chromium.launch({ headless: true })
  const ctx = await browser.newContext()
  const page = await ctx.newPage()
  await page.goto('http://127.0.0.1:1112/', { waitUntil: 'domcontentloaded' })
  await page.evaluate(() => {
    localStorage.setItem('mybox_terminals_v1', JSON.stringify({
      home: { tabs: [{ id: 7, title: 'E2E', command: 'echo HI', sessionId: 'persist-test-123' }], activeId: 7, collapsed: false, visible: true },
      home_ex: { tabs: [], activeId: null, collapsed: false, visible: false },
      mylabo: { tabs: [], activeId: null, collapsed: false, visible: false },
      myaitoolbox: { tabs: [], activeId: null, collapsed: false, visible: false },
    }))
  })
  await page.goto('http://127.0.0.1:1112/projects/home', { waitUntil: 'domcontentloaded' })
  await page.waitForSelector('.terminal-xterm', { timeout: 15000 })
  await page.waitForTimeout(2000)
  const stored = await page.evaluate(() => localStorage.getItem('mybox_terminals_v1'))
  console.log('STORED AFTER APP:', stored)
  await browser.close()
})().catch(e => { console.error(e); process.exit(1) })
