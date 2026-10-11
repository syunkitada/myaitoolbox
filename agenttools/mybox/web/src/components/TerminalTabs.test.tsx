import { describe, expect, it, vi } from 'vitest'
import {
  terminalHostHasLayout,
  terminalHostWidthIsAligned,
  terminalHostLayoutIsStable,
  terminalFitIsReady,
  terminalLayoutRefitDecision,
  terminalRefitRetryAllowed,
} from './TerminalTabs'

vi.mock('@xterm/xterm', () => ({ Terminal: class {} }))
vi.mock('@xterm/addon-fit', () => ({ FitAddon: class {} }))

describe('terminal host layout checks', () => {
  it('does not treat a hidden zero-sized host as ready for fitting', () => {
    expect(terminalHostHasLayout({ clientWidth: 0, clientHeight: 0 })).toBe(false)
    expect(terminalHostHasLayout({ clientWidth: 0, clientHeight: 240 })).toBe(false)
    expect(terminalHostHasLayout({ clientWidth: 640, clientHeight: 0 })).toBe(false)
  })

  it('accepts a host with a measurable layout', () => {
    expect(terminalHostHasLayout({ clientWidth: 640, clientHeight: 240 })).toBe(true)
  })

  it('rejects a host that is temporarily narrower than the terminal panel', () => {
    expect(terminalHostWidthIsAligned(640, 644)).toBe(true)
    expect(terminalHostWidthIsAligned(640, 320)).toBe(false)
    expect(terminalHostWidthIsAligned(0, 640)).toBe(false)
  })

  it('waits for two consecutive frames with the same measurable layout', () => {
    expect(terminalHostLayoutIsStable(null, { clientWidth: 640, clientHeight: 240 })).toBe(false)
    expect(
      terminalHostLayoutIsStable(
        { clientWidth: 480, clientHeight: 240 },
        { clientWidth: 640, clientHeight: 240 },
      ),
    ).toBe(false)
    expect(
      terminalHostLayoutIsStable(
        { clientWidth: 640, clientHeight: 240 },
        { clientWidth: 640, clientHeight: 240 },
      ),
    ).toBe(true)
  })

  it('does not fit a resize observer measurement until the layout is stable', () => {
    expect(
      terminalLayoutRefitDecision(null, { clientWidth: 640, clientHeight: 240 }, 0),
    ).toEqual({ fit: false, retry: true, attempts: 1 })
    expect(
      terminalLayoutRefitDecision(
        { clientWidth: 640, clientHeight: 240 },
        { clientWidth: 720, clientHeight: 240 },
        1,
      ),
    ).toEqual({ fit: false, retry: true, attempts: 2 })
    expect(
      terminalLayoutRefitDecision(
        { clientWidth: 720, clientHeight: 240 },
        { clientWidth: 720, clientHeight: 240 },
        2,
      ),
    ).toEqual({ fit: true, retry: false, attempts: 2 })
  })

  it('defers fitting when the maximum attempts are reached without a stable layout', () => {
    expect(
      terminalLayoutRefitDecision(
        { clientWidth: 640, clientHeight: 240 },
        { clientWidth: 720, clientHeight: 240 },
        30,
      ),
    ).toEqual({ fit: false, retry: false, attempts: 30 })
  })

  it('only treats a fit as ready when xterm has usable proposed dimensions', () => {
    expect(terminalFitIsReady(undefined, { cols: 80, rows: 24 })).toBe(false)
    expect(
      terminalFitIsReady(
        { cols: 80, rows: 24 },
        { cols: 79, rows: 24 },
      ),
    ).toBe(false)
    expect(
      terminalFitIsReady(
        { cols: 80, rows: 24 },
        { cols: 80, rows: 24 },
      ),
    ).toBe(true)
  })

  it('does not schedule background retries for inactive or zero-sized hosts', () => {
    expect(terminalRefitRetryAllowed({ clientWidth: 0, clientHeight: 240 }, true)).toBe(false)
    expect(terminalRefitRetryAllowed({ clientWidth: 640, clientHeight: 0 }, true)).toBe(false)
    expect(terminalRefitRetryAllowed({ clientWidth: 640, clientHeight: 240 }, false)).toBe(false)
    expect(terminalRefitRetryAllowed({ clientWidth: 640, clientHeight: 240 }, true, false)).toBe(false)
    expect(terminalRefitRetryAllowed({ clientWidth: 640, clientHeight: 240 }, true)).toBe(true)
  })
})
