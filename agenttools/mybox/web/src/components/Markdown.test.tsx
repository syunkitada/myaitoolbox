import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { Markdown } from './Markdown'

describe('Markdown', () => {
  it('renders markdown to sanitized html', () => {
    render(<Markdown text={'# Hello\n\n**bold** text'} />)
    expect(screen.getByRole('heading', { level: 1, name: 'Hello' })).toBeInTheDocument()
    expect(screen.getByText('bold', { selector: 'strong' })).toBeInTheDocument()
  })

  it('strips dangerous raw html', () => {
    render(<Markdown text={'<script>alert(1)</script>safe'} />)
    expect(document.querySelector('script')).not.toBeInTheDocument()
    expect(document.body.textContent).toContain('safe')
  })

  it('renders safe embedded html', () => {
    render(<Markdown text={'before <em class="foo">embedded</em> after'} />)
    expect(document.querySelector('em.foo')).toHaveTextContent('embedded')
  })

  it('renders resolved wiki links as anchors', () => {
    const pathOf = (t: string) => (t === 'notes/alpha' ? 'notes/alpha' : null)
    window.history.pushState({}, '', '/projects/proj/dashboard/files/notes/alpha')
    render(<Markdown text={'see [[notes/alpha|Alpha]]'} pathOf={pathOf} />)
    const link = screen.getByRole('link', { name: 'Alpha' })
    expect(link).toHaveAttribute('href', '/projects/proj/dashboard/files/notes/alpha')
  })

  it('leaves broken wiki links as plain text', () => {
    const pathOf = () => null
    render(<Markdown text={'see [[notes/missing]]'} pathOf={pathOf} />)
    expect(screen.getByText(/notes\/missing/)).toBeInTheDocument()
    expect(screen.queryByRole('link')).not.toBeInTheDocument()
  })
})
