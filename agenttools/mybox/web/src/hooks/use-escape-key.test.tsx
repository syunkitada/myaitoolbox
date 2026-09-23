import { describe, expect, it, vi } from 'vitest'
import { fireEvent, render } from '@testing-library/react'
import { useEscapeKey } from './use-escape-key'

function EscapeTarget({ onEscape, enabled = true }: { onEscape: () => void; enabled?: boolean }) {
  useEscapeKey(onEscape, enabled)
  return null
}

describe('useEscapeKey', () => {
  it('handles Escape only while enabled', () => {
    const onEscape = vi.fn()
    const { rerender } = render(<EscapeTarget onEscape={onEscape} enabled={false} />)

    fireEvent.keyDown(document, { key: 'Escape' })
    expect(onEscape).not.toHaveBeenCalled()

    rerender(<EscapeTarget onEscape={onEscape} />)
    fireEvent.keyDown(document, { key: 'Escape' })
    expect(onEscape).toHaveBeenCalledOnce()
  })

  it('dispatches Escape to the most recently registered target', () => {
    const firstEscape = vi.fn()
    const secondEscape = vi.fn()
    const first = render(<EscapeTarget onEscape={firstEscape} />)
    const second = render(<EscapeTarget onEscape={secondEscape} />)

    fireEvent.keyDown(document, { key: 'Escape' })
    expect(secondEscape).toHaveBeenCalledOnce()
    expect(firstEscape).not.toHaveBeenCalled()

    second.unmount()
    fireEvent.keyDown(document, { key: 'Escape' })
    expect(firstEscape).toHaveBeenCalledOnce()

    first.unmount()
    fireEvent.keyDown(document, { key: 'Escape' })
    expect(firstEscape).toHaveBeenCalledOnce()
  })
})
