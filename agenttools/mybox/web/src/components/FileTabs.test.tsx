import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { FileTabs } from './FileTabs'

describe('FileTabs', () => {
  const tabs = ['notes/a.md', '_tasks/b.md']

  it('renders the favorites button and file name labels with tooltips', () => {
    const onSelect = vi.fn()
    const onClose = vi.fn()
    render(<FileTabs tabs={[]} active="" favorites={[]} onSelect={onSelect} onClose={onClose} />)
    expect(screen.getByRole('button', { name: 'Favorites' })).toBeInTheDocument()

    render(<FileTabs tabs={tabs} active="notes/a.md" favorites={[]} onSelect={onSelect} onClose={onClose} />)
    expect(screen.getByRole('button', { name: 'a.md' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'b.md' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'b.md' })).toHaveAttribute('title', '_tasks/b.md')
  })

  it('opens favorites and navigates to a selected file', () => {
    const onSelect = vi.fn()
    render(
      <FileTabs
        tabs={[]}
        active=""
        favorites={['docs/guide.md']}
        onSelect={onSelect}
        onClose={() => undefined}
      />,
    )

    fireEvent.click(screen.getByRole('button', { name: 'Favorites' }))
    expect(screen.getByRole('dialog', { name: 'Favorites' })).toHaveTextContent('docs/guide.md')

    fireEvent.click(screen.getByRole('button', { name: 'docs/guide.md' }))
    expect(onSelect).toHaveBeenCalledWith('docs/guide.md')
    expect(screen.queryByRole('dialog', { name: 'Favorites' })).not.toBeInTheDocument()
  })

  it('calls onSelect when a tab is clicked', () => {
    const onSelect = vi.fn()
    render(<FileTabs tabs={tabs} active="" favorites={[]} onSelect={onSelect} onClose={() => undefined} />)
    fireEvent.click(screen.getByRole('button', { name: 'b.md' }))
    expect(onSelect).toHaveBeenCalledWith('_tasks/b.md')
  })

  it('calls onClose for the matching tab', () => {
    const onClose = vi.fn()
    render(<FileTabs tabs={tabs} active="" favorites={[]} onSelect={() => undefined} onClose={onClose} />)
    fireEvent.click(screen.getByRole('button', { name: 'Close a.md' }))
    expect(onClose).toHaveBeenCalledWith('notes/a.md')
  })
})
