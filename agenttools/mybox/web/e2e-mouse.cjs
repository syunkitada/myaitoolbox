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
    window.__sent = []
    window.__mdEvents = []
    const origSend = WebSocket.prototype.send
    WebSocket.prototype.send = function(d) { try { window.__sent.push(String(d)) } catch {} return origSend.apply(this, arguments) }
  })

  await page.goto(ROOT, { waitUntil: 'domcontentloaded' })
  await page.evaluate(p => localStorage.setItem('mybox_terminals_v1', JSON.stringify(p)),
    { [PROJECT]: { tabs: [{ id: 1, title: 'E2E', command: cmd, sessionId: session }], activeId: 1, collapsed: false, visible: true } })
  await page.goto(`${ROOT}/projects/${PROJECT}`, { waitUntil: 'domcontentloaded' })
  await page.waitForSelector('.terminal-xterm', { timeout: 20000 })
  await page.waitForTimeout(2500)

  const sentMouse = () => page.evaluate(() => window.__sent.filter(s => s.indexOf('u001b[<') !== -1).length)

  // BASELINE: focus textarea then click on screen
  const box1 = await page.locator('.terminal-xterm').boundingBox()
  console.log('terminal box:', JSON.stringify(box1))
  await page.evaluate(() => document.querySelector('.xterm-helper-textarea')?.focus())
  await page.mouse.click(box1.x + Math.min(box1.width / 2, 400), box1.y + Math.min(box1.height / 2, 90))
  await page.waitForTimeout(500)
  const base = await sentMouse()
  console.log('BASELINE after click:', base)

  // RELOAD
  await page.reload({ waitUntil: 'domcontentloaded' })
  await page.waitForSelector('.terminal-xterm', { timeout: 20000 })
  await page.waitForTimeout(3500)

  // focus textarea, then click
  await page.evaluate(() => document.querySelector('.xterm-helper-textarea')?.focus())
  await page.waitForTimeout(200)
  const pre = await sentMouse()
  const box2 = await page.locator('.terminal-xterm').boundingBox()
  await page.mouse.click(box2.x + Math.min(box2.width / 2, 400), box2.y + Math.min(box2.height / 2, 90))
  await page.waitForTimeout(500)
  const post = await sentMouse()
  console.log('RELOAD after focus+click:', pre, '->', post, 'delta:', post - pre)

  // Also try: click via locator directly on .xterm-screen
  const screen = page.locator('.xterm-screen')
  const sr = await screen.boundingBox()
  await screen.click({ position: { x: Math.min(sr.width / 2, 200), y: Math.min(sr.height / 2, 80) } })
  await page.waitForTimeout(500)
  const afterScreen = await sentMouse()
  console.log('RELOAD after screen click:', afterScreen, 'delta:', afterScreen - post)

  // Also try: dispatch mousedown event via JS
  await page.evaluate(() => {
    const s = document.querySelector('.xterm-screen')
    const r = s.getBoundingClientRect()
    s.dispatchEvent(new MouseEvent('mousedown', { bubbles: true, cancelable: true, clientX: r.left + 80, clientY: r.top + 30, button: 0, buttons: 1 }))
    s.dispatchEvent(new MouseEvent('mouseup', { bubbles: true, cancelable: true, clientX: r.left + 80, clientY: r.top + 30, button: 0 }))
  })
  await page.waitForTimeout(500)
  const afterJS = await sentMouse()
  console.log('RELOAD after JS mousedown:', afterJS, 'delta:', afterJS - afterScreen)

  await browser.close()
})().catch(e => { console.error('ERR:', e); process.exit(1) })
