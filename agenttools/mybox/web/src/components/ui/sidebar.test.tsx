import { fireEvent, render, screen } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { SidebarContent, SidebarProvider, useSidebar } from './sidebar'

function SidebarStateProbe() {
  const { state } = useSidebar()
  return <output data-testid="sidebar-state">{state}</output>
}

beforeEach(() => {
  Object.defineProperty(window, 'matchMedia', {
    configurable: true,
    value: vi.fn().mockImplementation((query: string) => ({
      matches: false,
      media: query,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    })),
  })
})

describe('SidebarContent', () => {
  it('scrolls vertically without creating a horizontal scrollbar', () => {
    render(<SidebarContent>Sidebar content</SidebarContent>)

    const content = screen.getByText('Sidebar content').closest('[data-slot="sidebar-content"]')
    expect(content).not.toBeNull()
    expect(content).toHaveClass('overflow-y-auto', 'overflow-x-hidden')
    expect(content).not.toHaveClass('overflow-auto')
  })
})

describe('SidebarProvider', () => {
  it('does not toggle from Ctrl+B', () => {
    render(
      <SidebarProvider>
        <SidebarStateProbe />
      </SidebarProvider>,
    )

    fireEvent.keyDown(window, { key: 'b', ctrlKey: true })

    expect(screen.getByTestId('sidebar-state')).toHaveTextContent('expanded')
  })
})
