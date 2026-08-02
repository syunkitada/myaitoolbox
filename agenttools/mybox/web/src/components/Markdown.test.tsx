import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { Markdown } from './Markdown'

describe('Markdown', () => {
  it('renders markdown to sanitized html', () => {
    render(<Markdown text={'# Hello\n\n**bold** text'} />)
    expect(screen.getByRole('heading', { level: 1, name: 'Hello' })).toBeInTheDocument()
    expect(screen.getByText('bold', { selector: 'strong' })).toBeInTheDocument()
  })

  it('strips raw html', () => {
    render(<Markdown text={'<script>alert(1)</script>safe'} />)
    expect(document.querySelector('script')).not.toBeInTheDocument()
    expect(document.querySelector('p')?.textContent).toContain('safe')
  })

  it('renders resolved wiki links as anchors', () => {
    const pathOf = (t: string) => (t === 'notes/alpha' ? 'notes/alpha' : null)
    render(<Markdown text={'see [[notes/alpha|Alpha]]'} pathOf={pathOf} />)
    const link = screen.getByRole('link', { name: 'Alpha' })
    expect(link).toHaveAttribute('href', '#/knowledge/notes%2Falpha')
  })

  it('leaves broken wiki links as plain text', () => {
    const pathOf = () => null
    render(<Markdown text={'see [[notes/missing]]'} pathOf={pathOf} />)
    expect(screen.getByText(/notes\/missing/)).toBeInTheDocument()
    expect(screen.queryByRole('link')).not.toBeInTheDocument()
  })
})
