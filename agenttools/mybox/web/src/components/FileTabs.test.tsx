import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { FileTabs } from './FileTabs'

describe('FileTabs', () => {
  const tabs = ['notes/a.md', '_tasks/b.md']

  it('renders the favorites button and file name labels with tooltips', () => {
    const onSelect = vi.fn()
    const onClose = vi.fn()
    render(
      <FileTabs
        tabs={[]}
        active=""
        favorites={[]}
        onSelect={onSelect}
        onClose={onClose}
        onSelectFavorite={() => undefined}
        onRemoveFavorite={() => undefined}
      />,
    )
    expect(screen.getByRole('button', { name: 'Favorites' })).toBeInTheDocument()

    render(
      <FileTabs
        tabs={tabs}
        active="notes/a.md"
        favorites={[]}
        onSelect={onSelect}
        onClose={onClose}
        onSelectFavorite={() => undefined}
        onRemoveFavorite={() => undefined}
      />,
    )
    expect(screen.getByRole('button', { name: 'a.md' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'b.md' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'b.md' })).toHaveAttribute('title', '_tasks/b.md')
  })

  it('opens favorites and navigates to a selected file', () => {
    const onSelectFavorite = vi.fn()
    render(
      <FileTabs
        tabs={[]}
        active=""
        favorites={[{ project: 'docs', path: 'guide.md' }]}
        onSelect={() => undefined}
        onClose={() => undefined}
        onSelectFavorite={onSelectFavorite}
        onRemoveFavorite={() => undefined}
      />,
    )

    fireEvent.click(screen.getByRole('button', { name: 'Favorites' }))
    const dialog = screen.getByRole('dialog', { name: 'Favorites' })
    expect(dialog).toHaveTextContent('guide.md')
    expect(dialog).toHaveTextContent('docs')

    fireEvent.click(screen.getByRole('button', { name: 'docs: guide.md' }))
    expect(onSelectFavorite).toHaveBeenCalledWith({ project: 'docs', path: 'guide.md' })
    expect(screen.queryByRole('dialog', { name: 'Favorites' })).not.toBeInTheDocument()
  })

  it('removes a favorite without selecting the file', () => {
    const onSelectFavorite = vi.fn()
    const onRemoveFavorite = vi.fn()
    render(
      <FileTabs
        tabs={[]}
        active=""
        favorites={[{ project: 'docs', path: 'guide.md' }]}
        onSelect={() => undefined}
        onClose={() => undefined}
        onSelectFavorite={onSelectFavorite}
        onRemoveFavorite={onRemoveFavorite}
      />,
    )

    fireEvent.click(screen.getByRole('button', { name: 'Favorites' }))
    fireEvent.click(screen.getByRole('button', { name: 'Remove docs: guide.md from favorites' }))

    expect(onRemoveFavorite).toHaveBeenCalledWith({ project: 'docs', path: 'guide.md' })
    expect(onSelectFavorite).not.toHaveBeenCalled()
  })

  it('calls onSelect when a tab is clicked', () => {
    const onSelect = vi.fn()
    render(
      <FileTabs
        tabs={tabs}
        active=""
        favorites={[]}
        onSelect={onSelect}
        onClose={() => undefined}
        onSelectFavorite={() => undefined}
        onRemoveFavorite={() => undefined}
      />,
    )
    fireEvent.click(screen.getByRole('button', { name: 'b.md' }))
    expect(onSelect).toHaveBeenCalledWith('_tasks/b.md')
  })

  it('calls onClose for the matching tab', () => {
    const onClose = vi.fn()
    render(
      <FileTabs
        tabs={tabs}
        active=""
        favorites={[]}
        onSelect={() => undefined}
        onClose={onClose}
        onSelectFavorite={() => undefined}
        onRemoveFavorite={() => undefined}
      />,
    )
    fireEvent.click(screen.getByRole('button', { name: 'Close a.md' }))
    expect(onClose).toHaveBeenCalledWith('notes/a.md')
  })
})
