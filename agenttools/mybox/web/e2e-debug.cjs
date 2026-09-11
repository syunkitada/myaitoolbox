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
  page.on('console', m => { if (m.type()==='error') console.log('[err]', m.text().slice(0,300)) })

  await page.addInitScript(() => {
    window.__sent = []
    window.__debug = {}
    const origSend = WebSocket.prototype.send
    WebSocket.prototype.send = function(d) {
      try { window.__sent.push(String(d)) } catch {}
      return origSend.apply(this, arguments)
    }
  })

  await page.goto(ROOT, { waitUntil: 'domcontentloaded' })
  await page.evaluate(p => localStorage.setItem('mybox_terminals_v1', JSON.stringify(p)),
    { [PROJECT]: { tabs: [{ id: 1, title: 'E2E', command: cmd, sessionId: session }], activeId: 1, collapsed: false, visible: true } })
  await page.goto(`${ROOT}/projects/${PROJECT}`, { waitUntil: 'domcontentloaded' })
  await page.waitForSelector('.terminal-xterm', { timeout: 20000 })
  await page.waitForTimeout(2500)

  const sentMouse = () => page.evaluate(() => window.__sent.filter(s => s.indexOf('u001b[<') !== -1).length)

  // --- FIRST LOAD: baseline ---
  const box1 = await page.locator('.terminal-xterm').boundingBox()
  await page.mouse.click(box1.x + Math.min(box1.width/2, 400), box1.y + Math.min(box1.height/2, 90))
  await page.waitForTimeout(500)
  const base = await sentMouse()
  console.log('BASELINE:', base)

  // --- RELOAD ---
  await page.reload({ waitUntil: 'domcontentloaded' })
  await page.waitForSelector('.terminal-xterm', { timeout: 20000 })
  await page.waitForTimeout(3500)

  // Debug: check all elements at the click point, and check xterm internals
  const box2 = await page.locator('.terminal-xterm').boundingBox()
  const clickX = box2.x + Math.min(box2.width/2, 400)
  const clickY = box2.y + Math.min(box2.height/2, 90)

  const debug = await page.evaluate(({cx, cy}) => {
    const result = {}

    // 1. What elements are at click point?
    const elementsAtPoint = document.elementsFromPoint(cx, cy)
    result.elementsAtPoint = elementsAtPoint.map(e => ({
      tag: e.tagName, class: e.className?.toString()?.slice(0,80), id: e.id, role: e.getAttribute('role'),
      pointerEvents: getComputedStyle(e).pointerEvents, zIndex: getComputedStyle(e).zIndex
    }))

    // 2. Check the xterm screen element
    const screen = document.querySelector('.xterm-screen')
    if (screen) {
      const sr = screen.getBoundingClientRect()
      result.screenRect = { x: sr.x, y: sr.y, w: sr.width, h: sr.height }
      result.screenPointerEvents = getComputedStyle(screen).pointerEvents
      result.screenZIndex = getComputedStyle(screen).zIndex
      // Check if screen has any mousedown listeners (we can't see them directly, but check event handlers)
      result.screenChildren = screen.childElementCount
      result.screenHTML = screen.outerHTML.slice(0, 500)
    }

    // 3. Check xterm's textarea (hidden input)
    const textarea = document.querySelector('.xterm-helper-textarea')
    if (textarea) {
      const ts = textarea.getBoundingClientRect()
      result.textareaRect = { x: ts.x, y: ts.y, w: ts.width, h: ts.height }
      result.textareaPointerEvents = getComputedStyle(textarea).pointerEvents
      result.textareaFocused = document.activeElement === textarea
    }

    // 4. Check if terminal container has any overlays
    const termContainer = document.querySelector('.terminal-xterm')
    if (termContainer) {
      const children = termContainer.children
      result.containerChildren = []
      for (let i = 0; i < children.length; i++) {
        const c = children[i]
        result.containerChildren.push({
          tag: c.tagName, class: c.className?.toString()?.slice(0,80),
          pointerEvents: getComputedStyle(c).pointerEvents,
          position: getComputedStyle(c).position,
          zIndex: getComputedStyle(c).zIndex
        })
      }
    }

    // 5. Try to find the xterm Terminal instance
    // xterm stores on the .xterm element
    const xtermEl = document.querySelector('.xterm')
    if (xtermEl) {
      // Check React fiber
      const fiberKey = Object.keys(xtermEl).find(k => k.startsWith('__reactFiber$'))
      result.hasFiber = !!fiberKey

      // Try to access xterm's coreMouseService via the element's internal properties
      const coreKey = Object.keys(xtermEl).find(k => !k.startsWith('__'))
    }

    return result
  }, { cx: clickX, cy: clickY })

  console.log('DEBUG after reload:', JSON.stringify(debug, null, 2))

  // 6. Dispatch mousedown directly on .xterm-screen and check textarea value change
  const preTextarea = await page.evaluate(() => document.querySelector('.xterm-helper-textarea')?.value || '')
  await page.evaluate(({cx, cy}) => {
    const screen = document.querySelector('.xterm-screen')
    if (screen) {
      const sr = screen.getBoundingClientRect()
      const localX = cx - sr.left
      const localY = cy - sr.top
      // Dispatch mousedown
      const md = new MouseEvent('mousedown', {
        bubbles: true, cancelable: true, view: window,
        clientX: cx, clientY: cy, screenX: cx, screenY: cy,
        button: 0, buttons: 1
      })
      screen.dispatchEvent(md)
      // Dispatch mouseup
      const mu = new MouseEvent('mouseup', {
        bubbles: true, cancelable: true, view: window,
        clientX: cx, clientY: cy, screenX: cx, screenY: cy,
        button: 0, buttons: 0
      })
      screen.dispatchEvent(mu)
    }
  }, { cx: clickX, cy: clickY })
  await page.waitForTimeout(500)
  const postTextarea = await page.evaluate(() => document.querySelector('.xterm-helper-textarea')?.value || '')
  const afterJS = await sentMouse()
  console.log('TEXTAREA before:', JSON.stringify(preTextarea), 'after:', JSON.stringify(postTextarea))
  console.log('SENT MOUSE after JS dispatch:', afterJS)

  await browser.close()
})().catch(e => { console.error('ERR:', e); process.exit(1) })
