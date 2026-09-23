import { describe, expect, it, vi, afterEach } from 'vitest'
import { aggregateAgentStatus, renderStatusIcon } from './use-agent-favicon'
import type { HerdrOverview } from '../api/client'

function overviewWith(statuses: string[], available = true): HerdrOverview {
  return {
    available,
    workspaces: [],
    tabs: [],
    panes: [],
    agents: statuses.map((status, i) => ({
      name: `agent${i}`,
      status,
      workspace_id: 'w1',
      pane_id: `p${i}`,
    })),
  }
}

describe('aggregateAgentStatus', () => {
  it('returns unknown without an overview', () => {
    expect(aggregateAgentStatus(null)).toBe('unknown')
    expect(aggregateAgentStatus(undefined)).toBe('unknown')
  })

  it('returns unknown when herdr is unavailable', () => {
    expect(aggregateAgentStatus(overviewWith(['idle'], false))).toBe('unknown')
  })

  it('returns unknown with no agents', () => {
    expect(aggregateAgentStatus(overviewWith([]))).toBe('unknown')
    expect(aggregateAgentStatus(overviewWith(['']))).toBe('unknown')
  })

  it('prefers blocked over every other status', () => {
    expect(aggregateAgentStatus(overviewWith(['working', 'blocked', 'idle']))).toBe('blocked')
  })

  it('prefers done (unseen) over working', () => {
    expect(aggregateAgentStatus(overviewWith(['done', 'working', 'idle']))).toBe('done')
  })

  it('prefers working over idle and unknown', () => {
    expect(aggregateAgentStatus(overviewWith(['idle', 'working', 'unknown']))).toBe('working')
  })

  it('prefers idle over unknown', () => {
    expect(aggregateAgentStatus(overviewWith(['idle', 'unknown']))).toBe('idle')
  })
})

describe('renderStatusIcon', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('degrades to null where canvas is unsupported (jsdom)', () => {
    // jsdom's real getContext logs "Not implemented"; stub it instead.
    vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue(null)
    expect(renderStatusIcon('working')).toBeNull()
  })
})
