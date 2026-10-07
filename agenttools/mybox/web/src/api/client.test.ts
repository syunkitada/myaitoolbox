import { describe, it, expect, vi, afterEach } from 'vitest'
import { api, ApiError } from './client'

describe('api client', () => {
  const originalFetch = globalThis.fetch

  afterEach(() => {
    globalThis.fetch = originalFetch
    vi.restoreAllMocks()
  })

  const mockFetch = (status: number, body: unknown) => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: status < 400,
      status,
      statusText: 'x',
      json: async () => body,
    } as Response)
  }

  it('omits empty query params', async () => {
    mockFetch(200, [])
    await api.listTasks({})
    expect(globalThis.fetch).toHaveBeenCalledWith('/api/tasks', expect.any(Object))
  })

  it('serializes body as json', async () => {
    mockFetch(201, { id: 'x', title: 'T', status: 'todo', priority: 'low' })
    await api.createTask({ name: 'T' })
    expect(globalThis.fetch).toHaveBeenCalledWith(
      '/api/tasks',
      expect.objectContaining({
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name: 'T' }),
      }),
    )
  })

  it('gets the rendered task template', async () => {
    mockFetch(200, { content: '---\ntitle: Task\n---\n' })
    await api.getTaskTemplate('Task')
    expect(globalThis.fetch).toHaveBeenCalledWith(
      '/api/task-template?name=Task',
      expect.any(Object),
    )
  })

  it('attaches to a server-owned file execution stream', () => {
    class FakeWebSocket {
      static latest: FakeWebSocket
      readonly url: string
      onmessage: ((event: MessageEvent) => void) | null = null
      onerror: (() => void) | null = null
      onclose: (() => void) | null = null
      close = vi.fn()

      constructor(url: string) {
        this.url = url
        FakeWebSocket.latest = this
      }
    }

    vi.stubGlobal('WebSocket', FakeWebSocket)
    const output: string[] = []
    const snapshot = vi.fn()
    const error = vi.fn()
    api.executeFileStream('run-1', {
      onOutput: (chunk) => output.push(chunk),
      onSnapshot: snapshot,
      onError: error,
    })

    expect(FakeWebSocket.latest.url).toContain('/api/files/execute/stream?run_id=run-1')
    FakeWebSocket.latest.onmessage?.({ data: JSON.stringify({ type: 'output', data: 'first\n' }) } as MessageEvent)
    expect(output).toEqual(['first\n'])
    expect(snapshot).not.toHaveBeenCalled()

    FakeWebSocket.latest.onmessage?.({
      data: JSON.stringify({
        type: 'state',
        id: 'run-1',
        path: 'scripts/run.sh',
        status: 'completed',
        output: 'first\n',
        exit_code: 0,
        started_at: '1970-01-01T00:00:00Z',
      }),
    } as MessageEvent)
    expect(snapshot).toHaveBeenCalledWith({
      id: 'run-1',
      path: 'scripts/run.sh',
      status: 'completed',
      output: 'first\n',
      exit_code: 0,
      timed_out: undefined,
      error: undefined,
      started_at: '1970-01-01T00:00:00Z',
    })
    expect(error).not.toHaveBeenCalled()
  })

  it('focuses a herdr agent', async () => {
    mockFetch(200, { ok: true })
    await api.focusHerdrAgent('w7:p1')
    expect(globalThis.fetch).toHaveBeenCalledWith(
      '/api/herdr/agents/focus',
      expect.objectContaining({
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ target: 'w7:p1' }),
      }),
    )
  })

  it('schedules and cancels a herdr prompt', async () => {
    const scheduled = {
      id: 'sp-1',
      target: 'w7:p1',
      text: 'send later',
      scheduled_at: '2026-09-29T12:00:00Z',
    }
    mockFetch(201, scheduled)
    await api.createHerdrScheduledPrompt('w7:p1', 'send later', scheduled.scheduled_at)
    expect(globalThis.fetch).toHaveBeenCalledWith(
      '/api/herdr/agents/scheduled-prompts',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({
          target: 'w7:p1',
          text: 'send later',
          scheduled_at: scheduled.scheduled_at,
        }),
      }),
    )

    mockFetch(204, undefined)
    await api.deleteHerdrScheduledPrompt('sp-1')
    expect(globalThis.fetch).toHaveBeenCalledWith(
      '/api/herdr/agents/scheduled-prompts/sp-1',
      expect.objectContaining({ method: 'DELETE' }),
    )
  })

  it('posts a task trigger definition', async () => {
    mockFetch(201, {
      id: 'daily-report',
      type: 'cron',
      directory: '_task_triggers/daily-report',
      task_path: '_task_triggers/daily-report/task.md',
    })
    await api.createTaskTrigger({
      id: 'daily-report',
      task: { name: 'Daily report', agent_kind: 'opencode', prompt: 'do-the-task' },
      trigger: { type: 'cron', cron: '0 9 * * 1-5', timezone: 'Asia/Tokyo' },
    })
    expect(globalThis.fetch).toHaveBeenCalledWith(
      '/api/task-triggers',
      expect.objectContaining({
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          id: 'daily-report',
          task: { name: 'Daily report', agent_kind: 'opencode', prompt: 'do-the-task' },
          trigger: { type: 'cron', cron: '0 9 * * 1-5', timezone: 'Asia/Tokyo' },
        }),
      }),
    )
  })

  it('runs a task trigger', async () => {
    mockFetch(200, {
      id: 'run-1',
      trigger_id: 'manual-report',
      event_id: 'manual:event',
      status: 'dispatched',
      task_id: 'task-1',
    })
    await api.runTaskTrigger('manual-report')
    expect(globalThis.fetch).toHaveBeenCalledWith(
      '/api/task-triggers/manual-report/run',
      expect.objectContaining({ method: 'POST' }),
    )
  })

  it('throws ApiError with server message', async () => {
    mockFetch(400, { error: 'bad request' })
    await expect(api.createTask({ name: 'T' })).rejects.toThrow(
      new ApiError(400, 'bad request'),
    )
  })

  it('returns undefined for 204', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({ ok: true, status: 204, json: async () => ({}) } as Response)
    await expect(api.archiveTask('x')).resolves.toBeUndefined()
  })

  it('posts a create file request', async () => {
    mockFetch(204, undefined)
    await api.createFile('notes/idea.md')
    expect(globalThis.fetch).toHaveBeenCalledWith(
      '/api/files',
      expect.objectContaining({
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ path: 'notes/idea.md' }),
      }),
    )
  })

  it('sends an explicit project override for listFiles', async () => {
    mockFetch(200, [])
    await api.listFiles({ project: 'other' })
    expect(globalThis.fetch).toHaveBeenCalledWith(
      '/api/files',
      expect.objectContaining({ headers: { 'X-Project': 'other' } }),
    )
  })

  it('searches file contents with the hidden-file setting', async () => {
    mockFetch(200, { query: 'foo', results: [], total: 0, truncated: false })
    await api.searchFiles({ q: 'foo bar', showHidden: false, project: 'other' })
    expect(globalThis.fetch).toHaveBeenCalledWith(
      '/api/files/search?q=foo+bar&show_hidden=false',
      expect.objectContaining({ headers: { 'X-Project': 'other' } }),
    )
  })
})
