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
})
