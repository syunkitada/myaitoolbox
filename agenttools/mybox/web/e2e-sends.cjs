const { chromium } = require('@playwright/test')
const PORT = '1112', PROJECT = 'home', ROOT = `http://127.0.0.1:${PORT}`
const session = 'e2eherdr-' + Date.now()
const pyLines = [
  `import os,sys,tty`,
  `ESC=bytes([27])`,
  `os.write(1,ESC+b"[?1000h"+ESC+b"[?1002h"+ESC+b"[?1003h"+ESC+b"[?1006h"+ESC+b"[?2004h"+ESC+b"[?25l")`,
  `os.write(1,b"STUB-READY\\\\r\\\\n")`,
  `tty.setraw(0)`,
  `while True:`,
  `\\tb=os.read(0,1)`,
  `\\tif not b: break`,
  `\\tos.write(1,b"IN:"+b.hex().encode()+b"\\\\r\\\\n")`,
]
const cmd = `python3 -c $'${pyLines.join('\\n')}'`

;(async () => {
  const browser = await chromium.launch({ headless: true })
  const ctx = await browser.newContext({ viewport: { width: 1280, height: 900 } })
  const page = await ctx.newPage()
  page.on('console', m => { if (m.type()==='error') console.log('[err]', m.text().slice(0,200)) })

  await page.addInitScript(() => {
    window.__allSends = []
    const origSend = WebSocket.prototype.send
    WebSocket.prototype.send = function(d) {
      try { window.__allSends.push({ t: Date.now(), d: String(d), b: typeof d }) } catch {}
      return origSend.apply(this, arguments)
    }
  })

  await page.goto(ROOT, { waitUntil: 'domcontentloaded' })
  await page.evaluate(p => localStorage.setItem('mybox_terminals_v1', JSON.stringify(p)),
    { [PROJECT]: { tabs: [{ id: 1, title: 'E2E', command: cmd, sessionId: session }], activeId: 1, collapsed: false, visible: true } })
  await page.goto(`${ROOT}/projects/${PROJECT}`, { waitUntil: 'domcontentloaded' })
  await page.waitForSelector('.terminal-xterm', { timeout: 20000 })
  await page.waitForTimeout(2500)

  const box1 = await page.locator('.terminal-xterm').boundingBox()
  await page.mouse.click(box1.x + Math.min(box1.width/2, 400), box1.y + Math.min(box1.height/2, 90))
  await page.waitForTimeout(600)
  const sends1 = await page.evaluate(() => window.__allSends.map(s => s.d))
  console.log('=== BASELINE (first load) ===')
  console.log('total sends:', sends1.length)
  console.log('ESC-prefixed:', JSON.stringify(sends1.filter(s => s.includes('u001b['))))
  console.log('SGR mouse (u001b[<):', sends1.filter(s => s.includes('u001b[<')).length)
  console.log('all raw:', JSON.stringify(sends1.map(s => JSON.stringify(s))))

  // RELOAD
  await page.reload({ waitUntil: 'domcontentloaded' })
  await page.waitForSelector('.terminal-xterm', { timeout: 20000 })
  await page.waitForTimeout(3500)
  const pre = await page.evaluate(() => window.__allSends.length)
  const box2 = await page.locator('.terminal-xterm').boundingBox()
  await page.mouse.click(box2.x + Math.min(box2.width/2, 400), box2.y + Math.min(box2.height/2, 90))
  await page.waitForTimeout(600)
  const sends2 = await page.evaluate(pre => window.__allSends.slice(pre).map(s => s.d), pre)
  console.log('\n=== AFTER RELOAD (sends during click) ===')
  console.log('total sends during click:', sends2.length)
  sends2.forEach((s,i) => console.log(`  [${i}] ${JSON.stringify(s)}`))

  await browser.close()
})().catch(e => { console.error('ERR:', e); process.exit(1) })