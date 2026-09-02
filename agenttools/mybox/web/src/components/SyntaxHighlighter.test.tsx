import { describe, it, expect } from 'vitest'
import { render } from '@testing-library/react'
import { SyntaxHighlighter } from './SyntaxHighlighter'

describe('SyntaxHighlighter', () => {
  it('wraps highlighted output in a pre/code block', () => {
    const { container } = render(<SyntaxHighlighter text="const x = 1" language="javascript" />)
    expect(container.querySelector('pre')).not.toBeNull()
    expect(container.querySelector('code')).not.toBeNull()
  })

  it('linkifies http and https urls', () => {
    const { container } = render(
      <SyntaxHighlighter text="see https://example.com/a and http://sub.example.org/x" />,
    )
    const links = Array.from(container.querySelectorAll('a'))
    const hrefs = links.map((a) => a.getAttribute('href'))
    expect(hrefs).toContain('https://example.com/a')
    expect(hrefs).toContain('http://sub.example.org/x')
    for (const a of links) {
      expect(a.getAttribute('target')).toBe('_blank')
      expect(a.getAttribute('rel')).toContain('noopener')
    }
  })

  it('adds token spans when a grammar is available', () => {
    const { container } = render(
      <SyntaxHighlighter text={'const x = 1'} language="javascript" />,
    )
    expect(container.querySelectorAll('.token')).not.toHaveLength(0)
  })

  it('escapes html in plaintext source', () => {
    const { container } = render(<SyntaxHighlighter text={'<script>alert(1)</script>'} />)
    expect(container.querySelector('script')).toBeNull()
    expect(container.querySelector('code')?.textContent).toContain('<script>alert(1)</script>')
  })
})
