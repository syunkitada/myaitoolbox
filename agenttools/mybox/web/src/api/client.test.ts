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

  it('sends query params and parses json', async () => {
    mockFetch(200, [{ type: 'task', path: 'a', title: 'A' }])
    await api.search('foo', 'task')
    expect(globalThis.fetch).toHaveBeenCalledWith('/api/search?q=foo&type=task', expect.any(Object))
  })

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

  it('throws ApiError with server message', async () => {
    mockFetch(403, { error: 'server is read-only' })
    await expect(api.createTask({ name: 'T' })).rejects.toThrow(
      new ApiError(403, 'server is read-only'),
    )
  })

  it('returns undefined for 204', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({ ok: true, status: 204, json: async () => ({}) } as Response)
    await expect(api.archiveTask('x')).resolves.toBeUndefined()
  })
})
