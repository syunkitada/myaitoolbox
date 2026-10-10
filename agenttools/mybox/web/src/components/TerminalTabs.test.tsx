import { describe, expect, it, vi } from 'vitest'
import { terminalHostHasLayout } from './TerminalTabs'

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
})
