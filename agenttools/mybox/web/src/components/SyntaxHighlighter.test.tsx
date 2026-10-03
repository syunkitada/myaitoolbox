import { describe, it, expect, vi } from 'vitest'
import { fireEvent, render } from '@testing-library/react'
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

  it('linkifies relative file paths when enabled', () => {
    window.history.pushState({}, '', '/projects/demo/herdr')
    const { container } = render(
      <SyntaxHighlighter
        text="See data/portfolio/benefits-and-dividends.md for details."
        linkFilePaths
      />,
    )

    const link = container.querySelector('a.syntax-file-link')
    expect(link).not.toBeNull()
    expect(link).toHaveTextContent('data/portfolio/benefits-and-dividends.md')
    expect(link).toHaveAttribute(
      'href',
      '/projects/demo/dashboard/files/data/portfolio/benefits-and-dividends.md',
    )

    window.history.replaceState({}, '', '/')
  })

  it('delegates file links without following the browser href', () => {
    window.history.pushState({}, '', '/projects/demo/herdr')
    const onFilePathClick = vi.fn()
    const { container } = render(
      <SyntaxHighlighter
        text="See data/portfolio/benefits-and-dividends.md for details."
        linkFilePaths
        onFilePathClick={onFilePathClick}
      />,
    )

    fireEvent.click(container.querySelector('a.syntax-file-link')!)

    expect(onFilePathClick).toHaveBeenCalledWith('data/portfolio/benefits-and-dividends.md')
    expect(window.location.pathname).toBe('/projects/demo/herdr')
    window.history.replaceState({}, '', '/')
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
