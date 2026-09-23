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

})
