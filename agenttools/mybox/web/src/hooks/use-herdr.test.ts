import { renderHook, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { api } from '../api/client'
import { useHerdrOverview } from './use-herdr'

vi.mock('../api/client', () => ({
  api: {
    getHerdrOverview: vi.fn(),
  },
}))

describe('useHerdrOverview', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('normalizes missing tabs and panes arrays from the API', async () => {
    vi.mocked(api.getHerdrOverview).mockResolvedValue({
      available: true,
      workspaces: [],
      agents: [],
      tabs: null,
      panes: null,
    } as never)

    const { result } = renderHook(() => useHerdrOverview(60_000))

    await waitFor(() => expect(result.current.loading).toBe(false))
    expect(result.current.overview?.tabs).toEqual([])
    expect(result.current.overview?.panes).toEqual([])
  })
})
