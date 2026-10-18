import { test, expect, type APIRequestContext, type Page } from '@playwright/test'
import { spawn, type ChildProcessWithoutNullStreams } from 'node:child_process'
import { readFile, unlink } from 'node:fs/promises'
import { createHash } from 'node:crypto'
import readline from 'node:readline'
import path from 'node:path'

const project = 'browser-e2e'
const secondaryProject = 'browser-e2e-secondary'
const mcpProject = 'browser-mcp-e2e'
const browserFixtureURL = 'https://mybox.test/'
const browserWorkerPIDFileBase = path.resolve(process.cwd(), 'tests/e2e/browser-worker.pid')

function browserWorkerPIDFile(targetProject: string) {
  const suffix = createHash('sha256').update(targetProject).digest('hex').slice(0, 16)
  return `${browserWorkerPIDFileBase}.${suffix}`
}
const projectHeaders = { 'X-Project': project }
const agentHeaders = { ...projectHeaders, 'X-Browser-Actor': 'agent' }
const secondaryAgentHeaders = { 'X-Project': secondaryProject, 'X-Browser-Actor': 'agent' }

type BrowserSession = {
  id: string
  owner: string
  state: string
  url: string
  approval_mode: 'manual' | 'auto_safe'
  pending_approval?: { approval_id: string }
}

type MCPResponse = {
  id?: number
  result?: {
    isError?: boolean
    structuredContent?: { data?: unknown; error?: { message?: string } | null }
    [key: string]: unknown
  }
  error?: { message?: string }
}

class StdioMCPClient {
  private readonly child: ChildProcessWithoutNullStreams
  private readonly lines: readline.Interface
  private readonly pending = new Map<number, { resolve: (value: MCPResponse) => void; reject: (error: Error) => void; timer: NodeJS.Timeout }>()
  private nextID = 1

  constructor() {
    const mcpRoot = path.resolve(process.cwd(), '../../mymcpserve')
    this.child = spawn('go', ['run', './cmd/mymcpserve', 'browser', '--transport', 'stdio'], {
      cwd: mcpRoot,
      env: { ...process.env, MYBOX_URL: 'http://127.0.0.1:19090', MYBOX_BROWSER_AGENT_TOKEN: '' },
      stdio: 'pipe',
    })
    this.lines = readline.createInterface({ input: this.child.stdout })
    this.lines.on('line', (line) => this.handleLine(line))
    this.child.stderr.resume()
    this.child.on('error', (error) => this.rejectPending(error))
    this.child.on('exit', (code, signal) => this.rejectPending(new Error(`mymcpserve exited (${code ?? signal ?? 'unknown'})`)))
  }

  private handleLine(line: string) {
    let message: MCPResponse
    try {
      message = JSON.parse(line) as MCPResponse
    } catch {
      return
    }
    if (typeof message.id !== 'number') return
    const request = this.pending.get(message.id)
    if (!request) return
    this.pending.delete(message.id)
    clearTimeout(request.timer)
    request.resolve(message)
  }

  private rejectPending(error: Error) {
    for (const [id, request] of this.pending) {
      clearTimeout(request.timer)
      request.reject(error)
      this.pending.delete(id)
    }
  }

  request(method: string, params: unknown = {}): Promise<MCPResponse> {
    const id = this.nextID++
    return new Promise((resolve, reject) => {
      const timer = setTimeout(() => {
        this.pending.delete(id)
        reject(new Error(`MCP request timed out: ${method}`))
      }, 30000)
      this.pending.set(id, { resolve, reject, timer })
      this.child.stdin.write(`${JSON.stringify({ jsonrpc: '2.0', id, method, params })}\n`)
    })
  }

  notify(method: string, params: unknown = {}) {
    this.child.stdin.write(`${JSON.stringify({ jsonrpc: '2.0', method, params })}\n`)
  }

  async initialize() {
    const response = await this.request('initialize', {
      protocolVersion: '2025-11-25',
      capabilities: {},
      clientInfo: { name: 'mybox-e2e', version: '0.1.0' },
    })
    expect(response.error).toBeUndefined()
    this.notify('notifications/initialized')
  }

  async close() {
    this.lines.close()
    this.child.stdin.end()
    if (this.child.exitCode !== null) return
    await Promise.race([
      new Promise<void>((resolve) => this.child.once('exit', () => resolve())),
      new Promise<void>((resolve) => setTimeout(resolve, 2000)),
    ])
    if (this.child.exitCode === null) this.child.kill('SIGTERM')
  }
}

function mcpData(response: MCPResponse) {
  expect(response.error).toBeUndefined()
  expect(response.result?.isError).not.toBe(true)
  const envelope = response.result?.structuredContent
  expect(envelope?.error ?? null).toBeNull()
  return envelope?.data
}

async function getSession(request: APIRequestContext, targetProject = project): Promise<BrowserSession | null> {
  const response = await request.get(`/api/browser/sessions?project=${encodeURIComponent(targetProject)}`, {
    headers: { 'X-Project': targetProject },
  })
  expect(response.ok()).toBe(true)
  const data = await response.json() as { session: BrowserSession | null }
  return data.session
}

async function waitForSession(page: Page): Promise<BrowserSession> {
  let session: BrowserSession | null = null
  await expect.poll(async () => {
    session = await getSession(page.request)
    return session?.state ?? ''
  }).toMatch(/^(running|user_control)$/)
  return session!
}

async function agentCommand(
  page: Page,
  sessionID: string,
  command: string,
  payload?: unknown,
) {
  return page.request.post(`/api/browser/sessions/${encodeURIComponent(sessionID)}/commands`, {
    headers: agentHeaders,
    data: { command, payload },
  })
}

async function agentCommandForProject(
  page: Page,
  targetProject: string,
  sessionID: string,
  command: string,
  payload?: unknown,
) {
  return page.request.post(`/api/browser/sessions/${encodeURIComponent(sessionID)}/commands`, {
    headers: { 'X-Project': targetProject, 'X-Browser-Actor': 'agent' },
    data: { command, payload },
  })
}

async function userCommand(
  page: Page,
  sessionID: string,
  command: string,
  payload?: unknown,
) {
  return page.request.post(`/api/browser/sessions/${encodeURIComponent(sessionID)}/commands`, {
    headers: projectHeaders,
    data: { command, payload },
  })
}

async function closeBrowser(request: APIRequestContext, targetProject = project) {
  const headers = { 'X-Project': targetProject }
  const session = await getSession(request, targetProject)
  if (session) {
    const response = await request.post(`/api/browser/sessions/${encodeURIComponent(session.id)}/close`, {
      headers,
    })
    expect([204, 404]).toContain(response.status())
  }
  const reset = await request.post('/api/browser/profiles/reset', {
    headers,
    data: { project: targetProject },
  })
  expect(reset.status()).toBe(204)
}

async function stopBrowserWorker(targetProject = project) {
  const pid = Number((await readFile(browserWorkerPIDFile(targetProject), 'utf8')).trim())
  expect(Number.isInteger(pid)).toBe(true)
  expect(pid).toBeGreaterThan(1)
  process.kill(pid, 'SIGKILL')
}

test.describe('embedded browser', () => {
  test.afterEach(async ({ page }) => {
    await closeBrowser(page.request, mcpProject)
    await closeBrowser(page.request, secondaryProject)
    await closeBrowser(page.request)
    await Promise.all([project, secondaryProject, mcpProject].map((targetProject) => unlink(browserWorkerPIDFile(targetProject)).catch(() => undefined)))
  })

  test('MCP stdio controls the real browser and respects user approval and takeover', async ({ page }) => {
    const mcp = new StdioMCPClient()
    try {
      await mcp.initialize()
      const listed = await mcp.request('tools/list')
      const tools = (listed.result?.tools ?? []) as Array<{ name: string; inputSchema?: { properties?: Record<string, unknown> } }>
      expect(tools.map((tool) => tool.name)).toEqual(expect.arrayContaining([
        'browser_start_session',
        'browser_observe',
        'browser_navigate',
        'browser_close_session',
      ]))
      const startTool = tools.find((tool) => tool.name === 'browser_start_session')
      expect(startTool?.inputSchema?.properties).not.toHaveProperty('url')

      const started = await mcp.request('tools/call', {
        name: 'browser_start_session',
        arguments: { project: mcpProject },
      })
      const session = mcpData(started) as BrowserSession
      expect(session.owner).toBe('agent')

      const navigation = await mcp.request('tools/call', {
        name: 'browser_navigate',
        arguments: { project: mcpProject, session_id: session.id, url: browserFixtureURL },
      })
      expect((mcpData(navigation) as { status: string }).status).toBe('approval_required')

      await expect.poll(async () => (await getSession(page.request, mcpProject))?.pending_approval?.approval_id ?? '')
        .not.toBe('')
      const approvalID = (await getSession(page.request, mcpProject))?.pending_approval?.approval_id
      expect(approvalID).not.toBe('')
      const approved = await page.request.post(`/api/browser/sessions/${encodeURIComponent(session.id)}/approvals/${encodeURIComponent(approvalID!)}/approve`, {
        headers: { 'X-Project': mcpProject },
      })
      expect(approved.status()).toBe(200)

      await expect.poll(async () => (await getSession(page.request, mcpProject))?.url).toBe(browserFixtureURL)
      const observed = await mcp.request('tools/call', {
        name: 'browser_observe',
        arguments: { project: mcpProject, session_id: session.id },
      })
      expect((mcpData(observed) as { title: string; text: string }).title).toBe('Mybox browser fixture')

      const takeover = await page.request.post(`/api/browser/sessions/${encodeURIComponent(session.id)}/control/takeover`, {
        headers: { 'X-Project': mcpProject },
      })
      expect(takeover.status()).toBe(200)
      const blocked = await mcp.request('tools/call', {
        name: 'browser_observe',
        arguments: { project: mcpProject, session_id: session.id },
      })
      expect(blocked.result?.isError).toBe(true)
      expect(blocked.result?.structuredContent?.error?.message).toContain('controlled by user')

      const returned = await page.request.post(`/api/browser/sessions/${encodeURIComponent(session.id)}/control/return`, {
        headers: { 'X-Project': mcpProject },
      })
      expect(returned.status()).toBe(200)
      const observedAfterReturn = await mcp.request('tools/call', {
        name: 'browser_observe',
        arguments: { project: mcpProject, session_id: session.id },
      })
      expect((mcpData(observedAfterReturn) as { text: string }).text).toContain('Deterministic browser fixture')
    } finally {
      await mcp.close()
    }
  })

  test('replays the latest browser frame after the UI reconnects', async ({ page }) => {
    await page.goto(`/projects/${project}/browser`)
    await page.getByRole('button', { name: 'Start browser' }).click()

    const frame = page.getByRole('img', { name: 'Live browser page' })
    await expect(frame).toBeVisible()
    await expect.poll(() => frame.evaluate((element) => (element as HTMLImageElement).naturalWidth)).toBeGreaterThan(0)
    const previousFrameURL = await frame.getAttribute('src')

    await page.reload()

    await expect(frame).toBeVisible()
    await expect.poll(() => frame.getAttribute('src')).not.toBe(previousFrameURL)
    await expect.poll(() => frame.evaluate((element) => (element as HTMLImageElement).naturalWidth)).toBeGreaterThan(0)
  })

  test('refreshes the live frame after an AI browser action', async ({ page }) => {
    await page.goto(`/projects/${project}/browser`)
    await page.getByRole('button', { name: 'Start browser' }).click()
    const session = await waitForSession(page)
    const frame = page.getByRole('img', { name: 'Live browser page' })
    await expect.poll(() => frame.evaluate((element) => (element as HTMLImageElement).naturalWidth)).toBeGreaterThan(0)

    await page.getByRole('button', { name: 'Return to AI' }).click()
    await expect.poll(async () => (await getSession(page.request))?.owner).toBe('agent')
    const previousFrameURL = await frame.getAttribute('src')

    const action = await agentCommand(page, session.id, 'wait', { ms: 50 })
    expect(action.status()).toBe(200)
    await expect.poll(() => frame.getAttribute('src')).not.toBe(previousFrameURL)
  })

  test('a timed-out command closes only its session', async ({ page }) => {
    const primary = await page.request.post('/api/browser/sessions', {
      headers: agentHeaders,
      data: { project, url: 'about:blank' },
    })
    expect(primary.status()).toBe(201)
    const primarySession = await primary.json() as BrowserSession

    const secondary = await page.request.post('/api/browser/sessions', {
      headers: secondaryAgentHeaders,
      data: { project: secondaryProject, url: 'about:blank' },
    })
    expect(secondary.status()).toBe(201)
    const secondarySession = await secondary.json() as BrowserSession

    const navigation = await agentCommandForProject(page, secondaryProject, secondarySession.id, 'navigate', { url: browserFixtureURL })
    expect(navigation.status()).toBe(200)
    expect(((await navigation.json()) as { status: string }).status).toBe('approval_required')
    await expect.poll(async () => (await getSession(page.request, secondaryProject))?.pending_approval?.approval_id ?? '')
      .not.toBe('')
    const approvalID = (await getSession(page.request, secondaryProject))?.pending_approval?.approval_id
    expect(approvalID).not.toBe('')
    const approved = await page.request.post(`/api/browser/sessions/${encodeURIComponent(secondarySession.id)}/approvals/${encodeURIComponent(approvalID!)}/approve`, {
      headers: { 'X-Project': secondaryProject },
    })
    expect(approved.status()).toBe(200)
    await expect.poll(async () => (await getSession(page.request, secondaryProject))?.url).toBe(browserFixtureURL)

    const timedOut = await agentCommand(page, primarySession.id, 'wait', { ms: 10000 })
    expect(timedOut.status()).toBe(500)
    expect((await timedOut.json()).error).toBe('browser command timed out')

    await expect.poll(async () => getSession(page.request)).toBeNull()
    const surviving = await getSession(page.request, secondaryProject)
    expect(surviving?.id).toBe(secondarySession.id)
    expect(surviving?.state).toBe('running')

    const observed = await agentCommandForProject(page, secondaryProject, secondarySession.id, 'observe')
    expect(observed.status()).toBe(200)
    expect(((await observed.json()) as { title: string }).title).toBe('Mybox browser fixture')
  })

  test('worker failure is visible and requires an explicit restart', async ({ page }) => {
    await page.goto(`/projects/${project}/browser`)
    await expect(page.getByTestId('web-browser-page')).toBeVisible()
    await page.getByRole('button', { name: 'Start browser' }).click()
    await expect(page.getByRole('button', { name: 'Close' })).toBeVisible()

    const session = await waitForSession(page)
    await page.getByRole('switch', { name: 'Auto-approve safe actions' }).click()
    await expect.poll(async () => (await getSession(page.request))?.approval_mode).toBe('auto_safe')
    await stopBrowserWorker()

    await expect.poll(async () => (await getSession(page.request))?.state ?? '').toBe('error')
    await expect(page.getByText('browser worker disconnected', { exact: true })).toBeVisible()
    await expect(page.getByRole('button', { name: 'Restart browser' })).toBeVisible()

    const rejected = await agentCommand(page, session.id, 'observe')
    expect(rejected.status()).toBe(500)
    expect((await rejected.json()).error).toContain('restart the browser session')

    await page.getByRole('button', { name: 'Restart browser' }).click()
    await expect.poll(async () => (await getSession(page.request))?.state ?? '').toBe('user_control')
    const restarted = await getSession(page.request)
    expect(restarted?.id).not.toBe(session.id)
    expect(restarted?.approval_mode).toBe('manual')
  })

  test('browser audit log shows project events and filters by session and result', async ({ page }) => {
    await page.goto(`/projects/${project}/browser`)
    await expect(page.getByTestId('browser-audit-panel')).toBeVisible()
    await page.getByRole('button', { name: 'Start browser' }).click()
    await expect(page.getByRole('button', { name: 'Close' })).toBeVisible()

    const session = await waitForSession(page)
    const auditPanel = page.getByTestId('browser-audit-panel')
    await auditPanel.getByRole('button', { name: 'Refresh audit log' }).click()
    await expect(page.getByTestId('browser-audit-events')).toContainText('session_created')

    await auditPanel.getByLabel('Audit session ID').fill(session.id)
    await auditPanel.getByLabel('Audit result').selectOption('success')
    await auditPanel.getByRole('button', { name: 'Apply filters' }).click()
    await expect(page.getByTestId('browser-audit-events')).toContainText('session_created')
    await expect(page.getByTestId('browser-audit-events')).not.toContainText('worker_disconnected')

  })

  test('worker failure does not affect another project', async ({ page }) => {
    const primaryResponse = await page.request.post('/api/browser/sessions', {
      headers: agentHeaders,
      data: { project, url: 'about:blank' },
    })
    expect(primaryResponse.status()).toBe(201)
    const primarySession = await primaryResponse.json() as BrowserSession

    const secondaryResponse = await page.request.post('/api/browser/sessions', {
      headers: secondaryAgentHeaders,
      data: { project: secondaryProject, url: 'about:blank' },
    })
    expect(secondaryResponse.status()).toBe(201)
    const secondarySession = await secondaryResponse.json() as BrowserSession

    await expect.poll(async () => (await getSession(page.request))?.state ?? '').toBe('running')
    await expect.poll(async () => (await getSession(page.request, secondaryProject))?.state ?? '').toBe('running')
    await stopBrowserWorker(project)

    await expect.poll(async () => (await getSession(page.request))?.state ?? '').toBe('error')
    await expect.poll(async () => (await getSession(page.request, secondaryProject))?.state ?? '').toBe('running')
    const observed = await agentCommandForProject(page, secondaryProject, secondarySession.id, 'observe')
    expect(observed.status()).toBe(200)
    expect(((await observed.json()) as { url: string }).url).toBe('about:blank')
    expect(primarySession.id).not.toBe(secondarySession.id)
  })

  test('browser commands require the matching project scope', async ({ page }) => {
    const created = await page.request.post('/api/browser/sessions', {
      headers: agentHeaders,
      data: { project, url: 'about:blank' },
    })
    expect(created.status()).toBe(201)
    const session = await created.json() as BrowserSession

    const wrongProject = await page.request.post(`/api/browser/sessions/${encodeURIComponent(session.id)}/commands`, {
      headers: { 'X-Project': secondaryProject, 'X-Browser-Actor': 'agent' },
      data: { command: 'observe' },
    })
    expect(wrongProject.status()).toBe(404)

    const missingProject = await page.request.post(`/api/browser/sessions/${encodeURIComponent(session.id)}/commands`, {
      headers: { 'X-Browser-Actor': 'agent' },
      data: { command: 'observe' },
    })
    expect(missingProject.status()).toBe(404)
  })

  test('AI navigation approval survives a UI reconnect and can be rejected', async ({ page }) => {
    await page.goto(`/projects/${project}/browser`)
    await expect(page.getByTestId('web-browser-page')).toBeVisible()
    await page.getByRole('button', { name: 'Start browser' }).click()
    await expect(page.getByRole('button', { name: 'Close' })).toBeVisible()

    const session = await waitForSession(page)
    await page.getByRole('button', { name: 'Return to AI' }).click()
    await expect(page.getByRole('button', { name: 'Take over' })).toBeVisible()

    const response = await agentCommand(page, session.id, 'navigate', { url: browserFixtureURL })
    expect(response.status()).toBe(200)
    const result = await response.json() as { status: string; approval_id: string }
    expect(result.status).toBe('approval_required')
    expect(result.approval_id).not.toBe('')

    await page.reload()
    await expect(page.getByText('Approval required')).toBeVisible()
    await page.getByRole('button', { name: 'Reject' }).click()

    await expect(page.getByText('Approval required')).toHaveCount(0)
    await expect.poll(async () => (await getSession(page.request))?.state).toBe('running')
    expect((await getSession(page.request))?.owner).toBe('agent')
    await expect.poll(async () => (await getSession(page.request))?.url).toBe('about:blank')
  })

  test('AI navigation approval executes after user approval and updates the session', async ({ page }) => {
    await page.goto(`/projects/${project}/browser`)
    await expect(page.getByTestId('web-browser-page')).toBeVisible()
    await page.getByRole('button', { name: 'Start browser' }).click()
    await expect(page.getByRole('button', { name: 'Close' })).toBeVisible()

    const session = await waitForSession(page)
    await page.getByRole('button', { name: 'Return to AI' }).click()
    await expect(page.getByRole('button', { name: 'Take over' })).toBeVisible()

    const response = await agentCommand(page, session.id, 'navigate', { url: browserFixtureURL })
    expect(response.status()).toBe(200)
    const result = await response.json() as { status: string; approval_id: string }
    expect(result.status).toBe('approval_required')
    expect(result.approval_id).not.toBe('')

    await page.reload()
    await expect(page.getByText('Approval required')).toBeVisible()
    await page.getByRole('button', { name: 'Approve' }).click()

    await expect(page.getByText('Approval required')).toHaveCount(0)
    await expect.poll(async () => (await getSession(page.request))?.url).toBe(browserFixtureURL)
    const updated = await getSession(page.request)
    expect(updated?.state).toBe('running')
    expect(updated?.owner).toBe('agent')

    const observed = await agentCommand(page, session.id, 'observe')
    expect(observed.status()).toBe(200)
    const observation = await observed.json() as { title: string; text: string }
    expect(observation.title).toBe('Mybox browser fixture')
    expect(observation.text).toContain('Deterministic browser fixture')
  })

  test('AI click approval executes against an observed fixture reference', async ({ page }) => {
    await page.goto(`/projects/${project}/browser`)
    await expect(page.getByTestId('web-browser-page')).toBeVisible()
    await page.getByRole('button', { name: 'Start browser' }).click()
    await expect(page.getByRole('button', { name: 'Close' })).toBeVisible()

    const session = await waitForSession(page)
    await page.getByRole('button', { name: 'Return to AI' }).click()
    await expect(page.getByRole('button', { name: 'Take over' })).toBeVisible()

    const navigation = await agentCommand(page, session.id, 'navigate', { url: browserFixtureURL })
    expect(navigation.status()).toBe(200)
    await page.reload()
    await expect(page.getByText('Approval required')).toBeVisible()
    await page.getByRole('button', { name: 'Approve' }).click()
    await expect(page.getByText('Approval required')).toHaveCount(0)
    await expect.poll(async () => (await getSession(page.request))?.url).toBe(browserFixtureURL)

    const observed = await agentCommand(page, session.id, 'observe')
    expect(observed.status()).toBe(200)
    const observation = await observed.json() as {
      observation_id: string
      refs: Array<{ ref: string; text: string }>
    }
    const fixtureButton = observation.refs.find((ref) => ref.text === 'Fixture action')
    expect(fixtureButton).toBeDefined()

    const click = await agentCommand(page, session.id, 'click', {
      observation_id: observation.observation_id,
      ref: fixtureButton!.ref,
    })
    expect(click.status()).toBe(200)
    expect((await click.json()).status).toBe('approval_required')

    await page.reload()
    await expect(page.getByText('Approval required')).toBeVisible()
    await page.getByRole('button', { name: 'Approve' }).click()
    await expect(page.getByText('Approval required')).toHaveCount(0)

    await expect.poll(async () => {
      const next = await agentCommand(page, session.id, 'observe')
      if (next.status() !== 200) return ''
      return ((await next.json()) as { text: string }).text
    }).toContain('Fixture action completed')
  })

  test('auto approval mode runs safe actions but keeps form submission approval-gated', async ({ page }) => {
    await page.goto(`/projects/${project}/browser`)
    await page.getByRole('button', { name: 'Start browser' }).click()
    const session = await waitForSession(page)

    await page.getByLabel('Browser URL').fill(browserFixtureURL)
    await page.getByTestId('web-browser-page').getByRole('button', { name: 'Go' }).click()
    await expect.poll(async () => (await getSession(page.request))?.url).toBe(browserFixtureURL)
    await expect.poll(async () => (await userCommand(page, session.id, 'observe')).status()).toBe(200)
    await page.getByRole('button', { name: 'Return to AI' }).click()
    await expect(page.getByRole('button', { name: 'Take over' })).toBeVisible()

    const before = await getSession(page.request)
    expect(before?.approval_mode).toBe('manual')
    await page.getByRole('switch', { name: 'Auto-approve safe actions' }).click()
    await expect.poll(async () => (await getSession(page.request))?.approval_mode).toBe('auto_safe')
    expect((await getSession(page.request))?.owner).toBe('agent')

    const navigation = await agentCommand(page, session.id, 'navigate', { url: browserFixtureURL })
    expect(navigation.status(), await navigation.text()).toBe(200)
    expect((await navigation.json()).status).not.toBe('approval_required')

    const firstObservation = await agentCommand(page, session.id, 'observe')
    const observation = await firstObservation.json() as {
      observation_id: string
      refs: Array<{ ref: string; text: string; auto_approvable: boolean }>
    }
    const noDestination = observation.refs.find((ref) => ref.text === 'No destination')
    expect(noDestination?.auto_approvable).toBe(false)
    const safeButton = observation.refs.find((ref) => ref.text === 'Fixture action')
    expect(safeButton).toBeDefined()
    const safeClick = await agentCommand(page, session.id, 'click', {
      observation_id: observation.observation_id,
      ref: safeButton!.ref,
    })
    expect(safeClick.status()).toBe(200)
    expect((await safeClick.json()).status).not.toBe('approval_required')
    const afterSafeClick = await agentCommand(page, session.id, 'observe')
    expect(((await afterSafeClick.json()) as { text: string }).text).toContain('Fixture action completed')

    const secondObservation = await agentCommand(page, session.id, 'observe')
    const refreshed = await secondObservation.json() as {
      observation_id: string
      refs: Array<{ ref: string; text: string }>
    }
    const submitButton = refreshed.refs.find((ref) => ref.text === 'Submit message')
    expect(submitButton).toBeDefined()
    const submitClick = await agentCommand(page, session.id, 'click', {
      observation_id: refreshed.observation_id,
      ref: submitButton!.ref,
    })
    expect(submitClick.status()).toBe(200)
    expect((await submitClick.json()).status).toBe('approval_required')
    await expect.poll(async () => (await getSession(page.request))?.pending_approval?.approval_id ?? '').not.toBe('')
    await page.reload()
    await expect(page.getByRole('switch', { name: 'Auto-approve safe actions' })).toBeChecked()
    await expect(page.getByText('Approval required')).toBeVisible()
  })

  test('failed approved action clears the pending approval after the page observation changes', async ({ page }) => {
    await page.goto(`/projects/${project}/browser`)
    await page.getByRole('button', { name: 'Start browser' }).click()
    const session = await waitForSession(page)

    await page.getByLabel('Browser URL').fill(browserFixtureURL)
    await page.getByTestId('web-browser-page').getByRole('button', { name: 'Go' }).click()
    await expect.poll(async () => (await getSession(page.request))?.url).toBe(browserFixtureURL)
    await expect.poll(async () => (await userCommand(page, session.id, 'observe')).status()).toBe(200)
    await page.getByRole('button', { name: 'Return to AI' }).click()

    await expect.poll(async () => (await agentCommand(page, session.id, 'observe')).status()).toBe(200)
    const observed = await agentCommand(page, session.id, 'observe')
    expect(observed.status()).toBe(200)
    const observation = await observed.json() as {
      observation_id: string
      refs: Array<{ ref: string; text: string }>
    }
    const actionButton = observation.refs.find((ref) => ref.text === 'Fixture action')
    expect(actionButton).toBeDefined()
    const click = await agentCommand(page, session.id, 'click', {
      observation_id: observation.observation_id,
      ref: actionButton!.ref,
    })
    expect(click.status()).toBe(200)
    const pending = await click.json() as { status: string }
    expect(pending.status).toBe('approval_required')

    await page.getByRole('button', { name: 'Take over' }).click()
    const refreshedObservation = await userCommand(page, session.id, 'observe')
    expect(refreshedObservation.status()).toBe(200)
    const approvalID = (await getSession(page.request))?.pending_approval?.approval_id
    expect(approvalID).toBeTruthy()

    const approval = await page.request.post(`/api/browser/sessions/${encodeURIComponent(session.id)}/approvals/${encodeURIComponent(approvalID!)}/approve`, {
      headers: projectHeaders,
    })
    expect(approval.status()).toBe(500)
    await expect.poll(async () => (await getSession(page.request))?.pending_approval).toBeNull()
    await expect.poll(async () => (await getSession(page.request))?.state).toBe('user_control')
  })

  test('AI can fill a safe field and must approve pressing Enter to submit it', async ({ page }) => {
    await page.goto(`/projects/${project}/browser`)
    await expect(page.getByTestId('web-browser-page')).toBeVisible()
    await page.getByRole('button', { name: 'Start browser' }).click()
    await expect(page.getByRole('button', { name: 'Close' })).toBeVisible()

    const session = await waitForSession(page)
    await page.getByRole('button', { name: 'Return to AI' }).click()
    await expect(page.getByRole('button', { name: 'Take over' })).toBeVisible()

    const navigation = await agentCommand(page, session.id, 'navigate', { url: browserFixtureURL })
    expect(navigation.status()).toBe(200)
    await page.reload()
    await expect(page.getByText('Approval required')).toBeVisible()
    await page.getByRole('button', { name: 'Approve' }).click()
    await expect(page.getByText('Approval required')).toHaveCount(0)

    const observed = await agentCommand(page, session.id, 'observe')
    expect(observed.status()).toBe(200)
    const observation = await observed.json() as {
      observation_id: string
      refs: Array<{ ref: string; text: string }>
    }
    const fixtureInput = observation.refs.find((ref) => ref.text === 'Fixture message')
    expect(fixtureInput).toBeDefined()
    const sensitiveInput = observation.refs.find((ref) => ref.text === 'One time code')
    expect(sensitiveInput).toBeDefined()

    const sensitiveFill = await agentCommand(page, session.id, 'fill', {
      observation_id: observation.observation_id,
      ref: sensitiveInput!.ref,
      value: '123456',
    })
    expect(sensitiveFill.status()).toBe(500)
    expect((await sensitiveFill.json()).error).toContain('sensitive fields')

    const fill = await agentCommand(page, session.id, 'fill', {
      observation_id: observation.observation_id,
      ref: fixtureInput!.ref,
      value: 'AI draft',
    })
    expect(fill.status()).toBe(200)
    expect(await fill.json()).toEqual({ ok: true })

    const filled = await agentCommand(page, session.id, 'observe')
    expect(filled.status()).toBe(200)
    const filledObservation = await filled.json() as { observation_id: string; text: string; refs: Array<{ ref: string; text: string }> }
    expect(filledObservation.text).toContain('Message: AI draft')
    const latestInput = filledObservation.refs.find((ref) => ref.text === 'Fixture message')
    expect(latestInput).toBeDefined()

    const press = await agentCommand(page, session.id, 'press', {
      observation_id: filledObservation.observation_id,
      ref: latestInput!.ref,
      key: 'Enter',
    })
    expect(press.status()).toBe(200)
    expect((await press.json()).status).toBe('approval_required')

    await page.reload()
    await expect(page.getByText('Approval required')).toBeVisible()
    await page.getByRole('button', { name: 'Approve' }).click()
    await expect(page.getByText('Approval required')).toHaveCount(0)

    await expect.poll(async () => {
      const next = await agentCommand(page, session.id, 'observe')
      if (next.status() !== 200) return ''
      return ((await next.json()) as { text: string }).text
    }).toContain('Submitted: AI draft')
  })

  test('user takeover input can type into the deterministic fixture', async ({ page }) => {
    const created = await page.request.post('/api/browser/sessions', {
      headers: agentHeaders,
      data: { project, url: 'about:blank' },
    })
    expect(created.status()).toBe(201)

    await page.goto(`/projects/${project}/browser`)
    await expect(page.getByTestId('web-browser-page')).toBeVisible()
    const session = await waitForSession(page)
    await page.getByRole('button', { name: 'Take over' }).click()
    await expect(page.getByRole('button', { name: 'Return to AI' })).toBeVisible()

    await page.getByLabel('Browser URL').fill(browserFixtureURL)
    await page.getByTestId('web-browser-page').getByRole('button', { name: 'Go' }).click()
    await expect.poll(async () => (await getSession(page.request))?.url).toBe(browserFixtureURL)

    const mouseDown = await userCommand(page, session.id, 'input', { kind: 'mouse', event: 'down', x: 80, y: 230 })
    expect(mouseDown.status()).toBe(200)
    const mouseUp = await userCommand(page, session.id, 'input', { kind: 'mouse', event: 'up', x: 80, y: 230 })
    expect(mouseUp.status()).toBe(200)
    const typed = await userCommand(page, session.id, 'input', { kind: 'text', text: 'User draft' })
    expect(typed.status()).toBe(200)

    const observed = await userCommand(page, session.id, 'observe')
    expect(observed.status()).toBe(200)
    expect(((await observed.json()) as { text: string }).text).toContain('Message: User draft')
  })

  test('user takeover blocks agent commands until control is returned', async ({ page }) => {
    await page.request.post('/api/browser/sessions', {
      headers: agentHeaders,
      data: { project, url: 'about:blank' },
    }).then((response) => expect(response.status()).toBe(201))
    await page.goto(`/projects/${project}/browser`)
    await expect(page.getByRole('button', { name: 'Take over' })).toBeVisible()

    const session = await waitForSession(page)
    await page.getByRole('button', { name: 'Take over' }).click()
    await expect(page.getByRole('button', { name: 'Return to AI' })).toBeVisible()

    const blocked = await agentCommand(page, session.id, 'observe')
    expect(blocked.status()).toBe(409)
    expect(await blocked.json()).toEqual({ error: 'browser session is controlled by user' })

    await page.getByRole('button', { name: 'Return to AI' }).click()
    await expect(page.getByRole('button', { name: 'Take over' })).toBeVisible()
    const observed = await agentCommand(page, session.id, 'observe')
    expect(observed.status()).toBe(200)
  })

  test('agent input is rejected while user input remains available', async ({ page }) => {
    const created = await page.request.post('/api/browser/sessions', {
      headers: agentHeaders,
      data: { project, url: 'about:blank' },
    })
    expect(created.status()).toBe(201)
    const session = await created.json() as BrowserSession

    const agentInput = await agentCommand(page, session.id, 'input', { kind: 'mouse', event: 'move', x: 10, y: 10 })
    expect(agentInput.status()).toBe(403)
    expect(await agentInput.json()).toEqual({ error: 'browser input is user-only' })

    const takeover = await page.request.post(`/api/browser/sessions/${encodeURIComponent(session.id)}/control/takeover`, {
      headers: projectHeaders,
    })
    expect(takeover.status()).toBe(200)

    const userInput = await page.request.post(`/api/browser/sessions/${encodeURIComponent(session.id)}/commands`, {
      headers: projectHeaders,
      data: { command: 'input', payload: { kind: 'mouse', event: 'move', x: 10, y: 10 } },
    })
    expect(userInput.status()).toBe(200)
  })

  test('worker rejects private navigation targets', async ({ page }) => {
    await page.goto(`/projects/${project}/browser`)
    await page.getByRole('button', { name: 'Start browser' }).click()
    await expect(page.getByRole('button', { name: 'Close' })).toBeVisible()
    const session = await waitForSession(page)

    for (const url of ['http://127.0.0.1:19090', 'https://localhost/', 'https://user:pass@example.com/', 'https://[::ffff:7f00:1]:19090/']) {
      const response = await page.request.post(`/api/browser/sessions/${encodeURIComponent(session.id)}/commands`, {
        headers: projectHeaders,
        data: { command: 'navigate', payload: { url } },
      })
      expect(response.status()).not.toBe(200)
      expect((await response.json()).error).toMatch(/public HTTPS|private|credentials/)
    }
  })
})
