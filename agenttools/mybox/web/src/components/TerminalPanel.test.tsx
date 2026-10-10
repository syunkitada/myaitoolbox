import { describe, expect, it, vi } from 'vitest'
import { nextTerminalId, normalizePersistedTerminalIds } from './TerminalPanel'

vi.mock('./TerminalTabs', () => ({ TerminalTabs: () => null }))

describe('terminal tab ids', () => {
  it('reuses the smallest available display number', () => {
    expect(nextTerminalId([
      { id: 2, title: 'Terminal 2', sessionId: 'session-2' },
      { id: 4, title: 'Terminal 4', sessionId: 'session-4' },
    ])).toBe(1)

    expect(nextTerminalId([
      { id: 1, title: 'Terminal 1', sessionId: 'session-1' },
      { id: 2, title: 'Terminal 2', sessionId: 'session-2' },
    ])).toBe(3)
  })

  it('repairs duplicate restored ids while preserving sessions', () => {
    const state = {
      demo: {
        tabs: [
          { id: 1, title: 'Terminal 1', sessionId: 'session-a' },
          { id: 1, title: 'Terminal 1', sessionId: 'session-b' },
        ],
        activeId: 1,
        collapsed: false,
        visible: true,
      },
    }

    const normalized = normalizePersistedTerminalIds(state)

    expect(normalized.demo.tabs.map((tab) => tab.id)).toEqual([1, 2])
    expect(normalized.demo.tabs.map((tab) => tab.sessionId)).toEqual(['session-a', 'session-b'])
    expect(normalized.demo.tabs[1].title).toBe('Terminal 2')
    expect(normalized.demo.activeId).toBe(2)
  })
})
