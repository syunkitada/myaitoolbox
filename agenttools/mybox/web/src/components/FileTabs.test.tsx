import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { FileTabs } from './FileTabs'

describe('FileTabs', () => {
  const tabs = ['notes/a.md', 'tasks/b.md']

  it('renders file name labels with tooltips and does not render when empty', () => {
    const onSelect = vi.fn()
    const onClose = vi.fn()
    render(<FileTabs tabs={[]} active="" onSelect={onSelect} onClose={onClose} />)
    expect(screen.queryByRole('button')).toBeNull()

    render(<FileTabs tabs={tabs} active="notes/a.md" onSelect={onSelect} onClose={onClose} />)
    expect(screen.getByRole('button', { name: 'a.md' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'b.md' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'b.md' })).toHaveAttribute('title', 'tasks/b.md')
  })

  it('calls onSelect when a tab is clicked', () => {
    const onSelect = vi.fn()
    render(<FileTabs tabs={tabs} active="" onSelect={onSelect} onClose={() => undefined} />)
    fireEvent.click(screen.getByRole('button', { name: 'b.md' }))
    expect(onSelect).toHaveBeenCalledWith('tasks/b.md')
  })

  it('calls onClose for the matching tab', () => {
    const onClose = vi.fn()
    render(<FileTabs tabs={tabs} active="" onSelect={() => undefined} onClose={onClose} />)
    fireEvent.click(screen.getByRole('button', { name: 'Close a.md' }))
    expect(onClose).toHaveBeenCalledWith('notes/a.md')
  })
})