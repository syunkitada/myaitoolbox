import dns from 'node:dns/promises'
import fs from 'node:fs/promises'
import http from 'node:http'
import net from 'node:net'
import process from 'node:process'
import { chromium } from 'playwright'
import { WebSocketServer, WebSocket } from 'ws'

const args = process.argv.slice(2)
const arg = (name, fallback = '') => {
  const index = args.indexOf(name)
  return index >= 0 ? args[index + 1] ?? fallback : fallback
}
const port = Number(arg('--port', '0'))
const token = arg('--token')
const testFixtureURL = process.env.MYBOX_BROWSER_TEST_FIXTURE_URL?.trim() || ''
const testFixturePath = process.env.MYBOX_BROWSER_TEST_FIXTURE_FILE?.trim() || ''
const viewport = { width: 1280, height: 800 }
const maxObservationText = 20000

if (Boolean(testFixtureURL) !== Boolean(testFixturePath)) {
  throw new Error('MYBOX_BROWSER_TEST_FIXTURE_URL and MYBOX_BROWSER_TEST_FIXTURE_FILE must be configured together')
}
if (testFixtureURL) {
  let parsedFixtureURL
  try { parsedFixtureURL = new URL(testFixtureURL) } catch { parsedFixtureURL = null }
  if (parsedFixtureURL?.origin !== 'https://mybox.test' || parsedFixtureURL.pathname !== '/' || parsedFixtureURL.search || parsedFixtureURL.hash || parsedFixtureURL.username || parsedFixtureURL.password) {
    throw new Error('MYBOX_BROWSER_TEST_FIXTURE_URL must be exactly https://mybox.test/')
  }
}

const sessions = new Map()
let socket = null
let networkProxyPromise = null
let networkProxyServer = null

function isTestFixtureURL(rawURL) {
  return Boolean(testFixtureURL) && rawURL === testFixtureURL
}

async function loadTestFixture() {
  if (!testFixturePath) return null
  return fs.readFile(testFixturePath, 'utf8')
}

function send(message) {
  if (socket?.readyState === WebSocket.OPEN) socket.send(JSON.stringify(message))
}

function sendResult(sessionId, requestId, data, error) {
  send({ type: 'result', session_id: sessionId, request_id: requestId, data, error })
}

function sessionEvent(session, type, extra = {}) {
  send({ type, session_id: session.id, state: session.state, url: session.page?.url() ?? session.url, title: session.title, ...extra })
}

function isPrivateIPv4(address) {
  const octets = address.split('.').map(Number)
  if (octets.length !== 4 || octets.some((n) => !Number.isInteger(n))) return false
  const [a, b] = octets
  return a === 0 || a === 10 || a === 127 || (a === 100 && b >= 64 && b <= 127) || (a === 169 && b === 254) || (a === 172 && b >= 16 && b <= 31) || (a === 192 && (b === 0 || b === 168)) || (a === 198 && (b === 18 || b === 19)) || (a === 203 && b === 0) || a >= 224
}

function isPrivateIPv6(address) {
  const normalized = address.toLowerCase()
  if (normalized === '::1' || normalized === '::' || normalized.startsWith('fc') || normalized.startsWith('fd') || normalized.startsWith('fe80:')) return true
  if (normalized.startsWith('::ffff:')) {
    const mapped = normalized.slice('::ffff:'.length)
    if (net.isIP(mapped) === 4) return isPrivateIPv4(mapped)
    const words = mapped.split(':')
    if (words.length === 2 && words.every((word) => /^[0-9a-f]{1,4}$/.test(word))) {
      const octets = words.flatMap((word) => {
        const value = Number.parseInt(word, 16)
        return [value >> 8, value & 0xff]
      })
      return isPrivateIPv4(octets.join('.'))
    }
    // Fail closed for an IPv4-mapped form we do not recognize.
    return true
  }
  return false
}

async function resolvePublicHTTPS(rawURL, allowBlank = false) {
  if (allowBlank && rawURL === 'about:blank') return
  if (isTestFixtureURL(rawURL)) return
  let parsed
  try {
    parsed = new URL(rawURL)
  } catch {
    throw new Error('URL must be a valid public HTTPS URL')
  }
  if (parsed.protocol !== 'https:' && parsed.protocol !== 'wss:') throw new Error('only public HTTPS or WSS URLs are allowed')
  if (parsed.username || parsed.password) throw new Error('URL credentials are not allowed')
  const hostname = parsed.hostname.replace(/^\[|\]$/g, '').toLowerCase()
  if (!hostname || hostname === 'localhost' || hostname.endsWith('.localhost') || hostname.endsWith('.local')) {
    throw new Error('private and local network targets are not allowed')
  }
  if (net.isIP(hostname) === 4 && isPrivateIPv4(hostname)) throw new Error('private and local network targets are not allowed')
  if (net.isIP(hostname) === 6 && isPrivateIPv6(hostname)) throw new Error('private and local network targets are not allowed')
  if (!net.isIP(hostname)) {
    const addresses = await dns.lookup(hostname, { all: true })
    if (addresses.some(({ address }) => (net.isIP(address) === 4 && isPrivateIPv4(address)) || (net.isIP(address) === 6 && isPrivateIPv6(address)))) {
      throw new Error('hostname resolves to a private or local network address')
    }
    return addresses[0]?.address
  }
  return hostname
}

async function assertPublicHTTPS(rawURL, allowBlank = false) {
  await resolvePublicHTTPS(rawURL, allowBlank)
}

async function ensureNetworkProxy() {
  if (networkProxyPromise) return networkProxyPromise
  networkProxyPromise = new Promise((resolve, reject) => {
    const proxy = http.createServer((request, response) => {
      response.writeHead(403, { 'content-type': 'text/plain' })
      response.end('only HTTPS CONNECT is allowed')
    })
    proxy.on('connect', async (request, clientSocket, head) => {
      let target
      let address
      let targetPort
      try {
        target = new URL(`https://${request.url}`)
        if (target.protocol !== 'https:') throw new Error('only HTTPS CONNECT is allowed')
        targetPort = Number(target.port || 443)
        address = await resolvePublicHTTPS(target.toString())
        if (!address) throw new Error('target address is unavailable')
      } catch {
        clientSocket.destroy()
        return
      }
      const upstream = net.connect({ host: address, port: targetPort })
      const closeSockets = () => {
        clientSocket.destroy()
        upstream.destroy()
      }
      upstream.once('error', closeSockets)
      clientSocket.once('error', () => upstream.destroy())
      upstream.once('connect', () => {
        clientSocket.write('HTTP/1.1 200 Connection Established\r\n\r\n')
        if (head.length > 0) upstream.write(head)
        clientSocket.pipe(upstream)
        upstream.pipe(clientSocket)
      })
    })
    proxy.on('clientError', (_error, clientSocket) => clientSocket.destroy())
    proxy.listen(0, '127.0.0.1', () => {
      networkProxyServer = proxy
      const address = proxy.address()
      resolve({ server: proxy, url: `http://127.0.0.1:${address.port}` })
    })
    proxy.once('error', reject)
  })
  return networkProxyPromise
}

function approvalRequired(ref, action) {
  if (action === 'press') {
    if (['enter', ' ', 'space', 'spacebar'].includes(String(ref.key ?? '').toLowerCase())) return true
    if (ref.tag === 'a' || ref.tag === 'button' || ref.role === 'link' || ref.role === 'button') return true
    if (['submit', 'button', 'checkbox', 'radio', 'reset'].includes(String(ref.type ?? '').toLowerCase())) return true
  }
  if (action !== 'click') return false
  return Boolean(ref.requires_approval)
}

function autoApprovalAllowed(ref, action) {
  if (action === 'navigate') return true
  return action === 'click' && Boolean(ref.auto_approvable)
}

async function labelInteractive(page, session) {
  session.observationID = crypto.randomUUID()
  session.revision += 1
  await page.evaluate(() => {
    document.querySelectorAll('[data-mybox-ref]').forEach((node) => node.removeAttribute('data-mybox-ref'))
  }).catch(() => undefined)
  const elements = await page.locator('a,button,input,textarea,select,[role="button"],[role="link"],[contenteditable="true"]').evaluateAll((nodes) => nodes.slice(0, 1000).map((node, index) => {
    const tag = node.tagName.toLowerCase()
    const type = node.getAttribute('type') || ''
    const autocomplete = node.getAttribute('autocomplete') || ''
    const identity = `${type} ${autocomplete} ${node.getAttribute('name') || ''} ${node.getAttribute('id') || ''} ${node.getAttribute('aria-label') || ''}`.toLowerCase()
    const sensitive = type.toLowerCase() === 'password' || /password|one-time-code|credit-card|card-number|security-code|cvc|cvv|social-security|ssn|api[-_ ]?key|secret|token/.test(identity)
    const text = (node.innerText || node.getAttribute('aria-label') || node.getAttribute('placeholder') || '').trim().replace(/\s+/g, ' ').slice(0, 200)
    const dangerous = /\b(submit|send|save|delete|remove|buy|purchase|checkout|confirm|order|publish|post|sign|pay|apply|cancel|unsubscribe|log\s*out|logout|sign\s*out|revoke|transfer|withdraw|share|invite|disconnect|erase|reset|approve|reject)\b/i.test(`${text} ${type}`)
    const submit = tag === 'button' || type === 'submit' || type === 'button'
    const potentiallySideEffectful = tag === 'a' || tag === 'button' || node.getAttribute('role') === 'link' || node.getAttribute('role') === 'button' || ['submit', 'button', 'checkbox', 'radio', 'reset'].includes(type)
    const href = tag === 'a' ? node.getAttribute('href')?.trim() || '' : ''
    const safeLink = tag === 'a' && href !== '' && /^https?:$/.test(new URL(node.href, document.baseURI).protocol) && !node.hasAttribute('download') && (!node.target || node.target.toLowerCase() === '_self')
    const menuButton = (tag === 'button' || node.getAttribute('role') === 'button') && !node.closest('form') && ['menu', 'listbox', 'tree', 'dialog'].includes((node.getAttribute('aria-haspopup') || '').toLowerCase())
    const disclosureButton = (tag === 'button' || node.getAttribute('role') === 'button') && !node.closest('form') && node.hasAttribute('aria-expanded')
    const auto_approvable = !dangerous && Boolean(text) && (safeLink || menuButton || disclosureButton)
    const ref = `r${index + 1}`
    node.setAttribute('data-mybox-ref', ref)
    return { ref, tag, type, text, role: node.getAttribute('role') || '', sensitive, requires_approval: dangerous || potentiallySideEffectful || (submit && node.closest('form') !== null), auto_approvable }
  }))
  session.refs = new Map(elements.map((ref) => [ref.ref, ref]))
  let ariaSnapshot = ''
  if (typeof page.ariaSnapshot === 'function') {
    try { ariaSnapshot = (await page.ariaSnapshot({ timeout: 2000 })).slice(0, maxObservationText) } catch { /* page may be mid-navigation */ }
  }
  let text = ''
  try { text = (await page.locator('body').innerText({ timeout: 2000 })).slice(0, maxObservationText) } catch { /* ignore */ }
  return {
    observation_id: session.observationID,
    revision: session.revision,
    url: page.url(),
    title: await page.title().catch(() => ''),
    aria_snapshot: ariaSnapshot,
    text,
    refs: elements,
  }
}

function ensureReference(session, payload) {
  if (!payload?.observation_id || payload.observation_id !== session.observationID) throw new Error('observation is stale; call observe again')
  const ref = session.refs.get(payload.ref)
  if (!ref) throw new Error('unknown browser reference; call observe again')
  return ref
}

async function emitObservation(session) {
  const data = await labelInteractive(session.page, session)
  session.title = data.title
  session.url = data.url
  return data
}

function sendFrame(session, image) {
  if (socket?.readyState !== WebSocket.OPEN) return
  try {
    const sessionID = Buffer.from(session.id)
    const header = Buffer.alloc(4)
    header.writeUInt32BE(sessionID.length, 0)
    socket.send(Buffer.concat([header, sessionID, image]))
  } catch { /* the worker connection may be closing */ }
}

async function publishCurrentFrame(session) {
  try {
    sendFrame(session, await session.page.screenshot({ type: 'jpeg', quality: 72 }))
  } catch { /* a preview failure must not fail the browser action */ }
}

async function attachPage(session, page) {
  session.page = page
  session.cdp = await session.context.newCDPSession(page)
  session.cdp.on('Page.screencastFrame', async (event) => {
    try {
      send({ type: 'frame', session_id: session.id, page_id: session.pageID, width: viewport.width, height: viewport.height })
      sendFrame(session, Buffer.from(event.data, 'base64'))
      await session.cdp.send('Page.screencastFrameAck', { sessionId: event.sessionId })
    } catch { /* the page or worker is closing */ }
  })
  await session.cdp.send('Page.startScreencast', { format: 'jpeg', quality: 72, maxWidth: viewport.width, maxHeight: viewport.height, everyNthFrame: 1 })
  page.on('framenavigated', async (frame) => {
    if (frame !== page.mainFrame()) return
    session.url = page.url()
    session.title = await page.title().catch(() => '')
    session.state = session.owner === 'user' ? 'user_control' : 'running'
    sessionEvent(session, 'page')
  })
  page.on('close', () => {
    if (session.page === page) session.state = 'error'
  })
}

async function createSession(message) {
  const initialURL = message.payload?.initial_url || 'about:blank'
  await assertPublicHTTPS(initialURL, true)
  const fixtureBody = await loadTestFixture()
  const networkProxy = await ensureNetworkProxy()
  let context
  let session
  try {
    context = await chromium.launchPersistentContext(message.payload.profile_dir, {
      headless: true,
      viewport,
      deviceScaleFactor: 1,
      acceptDownloads: false,
      serviceWorkers: 'block',
      proxy: { server: networkProxy.url },
    })
    session = {
      id: message.session_id,
      project: message.payload.project,
      context,
      page: null,
      cdp: null,
      pageID: crypto.randomUUID(),
      refs: new Map(),
      observationID: '',
      revision: 0,
      pending: new Map(),
      owner: message.payload.owner || 'agent',
      approvalMode: 'manual',
      state: 'starting',
      url: initialURL,
      title: '',
    }
    sessions.set(session.id, session)
    context.on('page', async (page) => {
      if (!session.page) await attachPage(session, page)
    })
    await context.route('**/*', async (route) => {
      try {
        const requestURL = route.request().url()
        if (isTestFixtureURL(requestURL)) {
          await route.fulfill({ status: 200, contentType: 'text/html; charset=utf-8', body: fixtureBody })
          return
        }
        await assertPublicHTTPS(requestURL, true)
        await route.continue()
      } catch {
        await route.abort('blockedbyclient')
      }
    })
    await context.routeWebSocket('**/*', async (route) => {
      try {
        await assertPublicHTTPS(route.url())
        await route.connectToServer()
      } catch {
        await route.close({ code: 1008, reason: 'private or non-HTTPS WebSocket target' })
      }
    })
    const page = context.pages()[0] || await context.newPage()
    await attachPage(session, page)
    if (initialURL !== 'about:blank') await page.goto(initialURL, { waitUntil: 'domcontentloaded', timeout: 15000 })
    session.state = session.owner === 'user' ? 'user_control' : 'running'
    session.url = page.url()
    session.title = await page.title().catch(() => '')
    sessionEvent(session, 'session')
    await publishCurrentFrame(session)
    return session
  } catch (error) {
    if (session) sessions.delete(session.id)
    if (context) await context.close().catch(() => undefined)
    throw error
  }
}

async function closeSession(session) {
  try {
    await session.context.close()
  } finally {
    sessions.delete(session.id)
    session.state = 'closed'
    sessionEvent(session, 'session')
  }
}

async function perform(session, action, payload) {
  const page = session.page
  if (!page) throw new Error('browser page is not ready')
  if (action === 'observe') return emitObservation(session)
  if (action === 'navigate') {
    await assertPublicHTTPS(payload.url)
    await page.goto(payload.url, { waitUntil: 'domcontentloaded', timeout: 15000 })
    return emitObservation(session)
  }
  if (action === 'click') {
    const ref = ensureReference(session, payload)
    const locator = page.locator(`[data-mybox-ref="${ref.ref}"]`).first()
    await locator.click({ timeout: 10000 })
    await page.waitForTimeout(250)
    return { ok: true, url: page.url(), title: await page.title().catch(() => '') }
  }
  if (action === 'fill') {
    const ref = ensureReference(session, payload)
    if (ref.sensitive) throw new Error('sensitive fields must be filled by the user')
    await page.locator(`[data-mybox-ref="${ref.ref}"]`).first().fill(String(payload.value ?? ''), { timeout: 10000 })
    return { ok: true }
  }
  if (action === 'press') {
    const ref = ensureReference(session, payload)
    await page.locator(`[data-mybox-ref="${ref.ref}"]`).first().press(String(payload.key), { timeout: 10000 })
    await page.waitForTimeout(250)
    return { ok: true, url: page.url(), title: await page.title().catch(() => '') }
  }
  if (action === 'scroll') {
    await page.mouse.wheel(Number(payload.delta_x || 0), Number(payload.delta_y || 0))
    await page.waitForTimeout(100)
    return { ok: true }
  }
  if (action === 'wait') {
    const timeout = Math.min(10000, Math.max(0, Number(payload.ms || 500)))
    await page.waitForTimeout(timeout)
    return { ok: true, waited_ms: timeout }
  }
  if (action === 'screenshot') {
    const image = await page.screenshot({ type: 'jpeg', quality: 72 })
    sendFrame(session, image)
    return { mime_type: 'image/jpeg', data: image.toString('base64') }
  }
  throw new Error(`unsupported browser command: ${action}`)
}

async function command(message) {
  const session = sessions.get(message.session_id)
  if (message.command === 'start_session') {
    if (session) return { ok: true, state: session.state }
    const created = await createSession(message)
    return { ok: true, state: created.state, url: created.url }
  }
  if (!session) throw new Error('browser session not found in worker')
  if (message.command === 'close_session') {
    await closeSession(session)
    return { ok: true }
  }
  if (message.command === 'set_approval_mode') {
    if (message.actor !== 'user') throw new Error('browser approval mode is user-only')
    const mode = message.payload?.mode
    if (mode !== 'manual' && mode !== 'auto_safe') throw new Error('browser approval mode must be manual or auto_safe')
    session.approvalMode = mode
    sessionEvent(session, 'approval_mode', { approval_mode: mode })
    return { approval_mode: mode }
  }
  if (message.command === 'approve_pending' || message.command === 'reject_pending') {
    const approvalID = message.payload?.approval_id
    const pending = session.pending.get(approvalID)
    if (!pending) throw new Error('browser approval is missing or expired')
    session.pending.delete(approvalID)
    if (message.command === 'reject_pending') {
      session.state = session.owner === 'user' ? 'user_control' : 'running'
      sessionEvent(session, 'approval_rejected', { approval_id: approvalID })
      return { ok: true, rejected: true }
    }
    try {
      const data = await perform(session, pending.action, pending.payload)
      if (pending.action !== 'screenshot') await publishCurrentFrame(session)
      session.state = session.owner === 'user' ? 'user_control' : 'running'
      sessionEvent(session, 'approval_approved', { approval_id: approvalID })
      return data
    } catch (error) {
      session.state = session.owner === 'user' ? 'user_control' : 'running'
      sessionEvent(session, 'approval_failed', { approval_id: approvalID })
      throw error
    }
  }
  if (message.command === 'input') {
    if (message.actor === 'agent') throw new Error('browser input is user-only')
    return input(session, message.payload || {})
  }
  const action = message.command
  const payload = message.payload || {}
  if (action === 'navigate' && message.actor === 'agent' && session.approvalMode !== 'auto_safe') {
    const approvalID = crypto.randomUUID()
    session.pending.set(approvalID, { action, payload })
    session.state = 'approval_pending'
    sessionEvent(session, 'approval_required', { approval_id: approvalID, action, target: payload.url || 'navigation' })
    return { status: 'approval_required', approval_id: approvalID, action, target: payload.url || 'navigation', url: session.page.url() }
  }
  if (action === 'click' || action === 'press') {
    const ref = ensureReference(session, payload)
    if (approvalRequired({ ...ref, key: payload.key }, action) && !(session.approvalMode === 'auto_safe' && autoApprovalAllowed(ref, action))) {
      const approvalID = crypto.randomUUID()
      session.pending.set(approvalID, { action, payload })
      session.state = 'approval_pending'
      sessionEvent(session, 'approval_required', { approval_id: approvalID, action, target: ref.text || ref.ref })
      return { status: 'approval_required', approval_id: approvalID, action, target: ref.text || ref.ref, url: session.page.url() }
    }
  }
  const data = await perform(session, action, payload)
  if (action !== 'screenshot') await publishCurrentFrame(session)
  return data
}

async function input(session, payload) {
  if (!session.cdp) throw new Error('browser input is not ready')
  if (payload.kind === 'mouse') {
    const type = payload.event === 'up' ? 'mouseReleased' : payload.event === 'move' ? 'mouseMoved' : 'mousePressed'
    await session.cdp.send('Input.dispatchMouseEvent', { type, x: Number(payload.x), y: Number(payload.y), button: payload.button || 'left', clickCount: Number(payload.click_count || 1) })
    return { ok: true }
  }
  if (payload.kind === 'wheel') {
    await session.cdp.send('Input.dispatchMouseEvent', { type: 'mouseWheel', x: Number(payload.x || 0), y: Number(payload.y || 0), deltaX: Number(payload.delta_x || 0), deltaY: Number(payload.delta_y || 0) })
    return { ok: true }
  }
  if (payload.kind === 'text') {
    await session.cdp.send('Input.insertText', { text: String(payload.text || '') })
    return { ok: true }
  }
  if (payload.kind === 'key') {
    await session.cdp.send('Input.dispatchKeyEvent', { type: payload.event === 'up' ? 'keyUp' : 'keyDown', key: payload.key, code: payload.code, text: payload.text, modifiers: Number(payload.modifiers || 0) })
    return { ok: true }
  }
  throw new Error('unsupported browser input')
}

const server = http.createServer((request, response) => {
  if (request.url === '/health') {
    response.writeHead(200, { 'content-type': 'application/json' })
    response.end(JSON.stringify({ ok: true, sessions: sessions.size }))
    return
  }
  response.writeHead(404)
  response.end()
})
const wss = new WebSocketServer({ noServer: true, maxPayload: 16 << 20 })
server.on('upgrade', (request, socketForUpgrade, head) => {
  if (request.url !== '/worker' || request.headers.authorization !== `Bearer ${token}`) {
    socketForUpgrade.destroy()
    return
  }
  wss.handleUpgrade(request, socketForUpgrade, head, (client) => wss.emit('connection', client, request))
})
wss.on('connection', (client) => {
  if (socket) socket.close()
  socket = client
  client.on('message', async (data) => {
    let message = {}
    try {
      message = JSON.parse(data.toString())
      if (message.type !== 'command') return
      const result = await command(message)
      sendResult(message.session_id, message.request_id, result)
    } catch (error) {
      sendResult(message.session_id, message.request_id, undefined, error instanceof Error ? error.message : String(error))
    }
  })
  client.on('close', () => { if (socket === client) socket = null })
})

server.listen(port, '127.0.0.1', () => {
  const address = server.address()
  process.stdout.write(`${JSON.stringify({ type: 'ready', port: address.port })}\n`)
})

async function shutdown() {
  for (const session of sessions.values()) await session.context.close().catch(() => undefined)
  if (networkProxyServer) networkProxyServer.close()
  server.close()
  process.exit(0)
}
process.on('SIGTERM', shutdown)
process.on('SIGINT', shutdown)
