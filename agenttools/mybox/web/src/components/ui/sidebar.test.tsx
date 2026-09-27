import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { SidebarContent } from './sidebar'

describe('SidebarContent', () => {
  it('scrolls vertically without creating a horizontal scrollbar', () => {
    render(<SidebarContent>Sidebar content</SidebarContent>)

    const content = screen.getByText('Sidebar content').closest('[data-slot="sidebar-content"]')
    expect(content).not.toBeNull()
    expect(content).toHaveClass('overflow-y-auto', 'overflow-x-hidden')
    expect(content).not.toHaveClass('overflow-auto')
  })
})
