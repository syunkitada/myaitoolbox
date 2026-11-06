import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterAll, beforeAll, describe, expect, it, vi } from 'vitest'
import { Mermaid } from './Mermaid'

const originalPointerEvent = window.PointerEvent

beforeAll(() => {
  class PointerEventForTests extends MouseEvent {
    readonly pointerId: number

    constructor(type: string, init: PointerEventInit = {}) {
      super(type, init)
      this.pointerId = init.pointerId ?? 1
    }
  }

  Object.defineProperty(window, 'PointerEvent', { configurable: true, value: PointerEventForTests })
})

afterAll(() => {
  if (originalPointerEvent) {
    Object.defineProperty(window, 'PointerEvent', { configurable: true, value: originalPointerEvent })
  } else {
    Reflect.deleteProperty(window, 'PointerEvent')
  }
})

vi.mock('mermaid', () => ({
  default: {
    initialize: vi.fn(),
    render: vi.fn(async () => ({
      svg: '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100"><rect width="100" height="100" /></svg>',
    })),
  },
}))

async function openExpandedDiagram(code: string) {
  render(<Mermaid code={code} />)
  fireEvent.click(await screen.findByRole('button', { name: 'Expand Mermaid diagram' }))

  const viewport = screen.getByRole('region', { name: 'Mermaid diagram viewport' })
  Object.defineProperty(viewport, 'getBoundingClientRect', {
    configurable: true,
    value: () => ({ left: 0, top: 0, right: 400, bottom: 300, width: 400, height: 300, x: 0, y: 0, toJSON: () => undefined }),
  })

  return { viewport, surface: screen.getByTestId('mermaid-zoom-surface') }
}

describe('Mermaid expanded view', () => {
  it('zooms toward the mouse position when the wheel is scrolled', async () => {
    const { viewport, surface } = await openExpandedDiagram('flowchart LR; A-->B')

    fireEvent.wheel(viewport, { deltaY: -120, clientX: 300, clientY: 250 })

    const match = surface.style.transform.match(/translate\(([-\d.]+)px, ([-\d.]+)px\) scale\(([-\d.]+)\)/)
    expect(match).not.toBeNull()
    expect(Number(match?.[1])).toBeLessThan(0)
    expect(Number(match?.[2])).toBeLessThan(0)
    expect(Number(match?.[3])).toBeGreaterThan(1)
  })

  it('moves the diagram while the pointer is dragged', async () => {
    const { viewport, surface } = await openExpandedDiagram('flowchart LR; C-->D')

    fireEvent.pointerDown(viewport, { button: 0, pointerId: 1, clientX: 100, clientY: 120 })
    fireEvent.pointerMove(viewport, { pointerId: 1, clientX: 145, clientY: 160 })

    await waitFor(() => expect(surface.style.transform).toBe('translate(45px, 40px) scale(1)'))
    fireEvent.click(screen.getByRole('button', { name: 'Reset Mermaid view' }))
    expect(surface.style.transform).toBe('translate(0px, 0px) scale(1)')
    fireEvent.pointerUp(viewport, { pointerId: 1 })
  })
})
