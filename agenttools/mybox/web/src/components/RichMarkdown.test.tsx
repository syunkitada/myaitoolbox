import { describe, it, expect } from 'vitest'
import { render } from '@testing-library/react'
import { RichMarkdown, extractOutline } from './RichMarkdown'

describe('RichMarkdown', () => {
  it('assigns slug ids to headings matching extractOutline', () => {
    const text = '# Hello World\n\n## Overview\n\n## See [[notes/alpha|Alpha]]\n'
    render(<RichMarkdown text={text} />)
    const outline = extractOutline(text)
    expect(outline.length).toBeGreaterThan(0)
    for (const h of outline) {
      const el = document.getElementById(h.id)
      expect(el).not.toBeNull()
      expect(el?.tagName).toBe(`H${h.level}`)
    }
  })

  it('resolves relative file links with a custom linkUrl', () => {
    const { container } = render(
      <RichMarkdown
        text="see [golang_project_structure](./golang_project_structure.md)"
        relativeTo="golang/golang_architecture"
        linkUrl={(resolved) => `/projects/proj/dashboard/files/${resolved}`}
      />,
    )
    const link = container.querySelector('a')
    expect(link?.getAttribute('href')).toBe(
      '/projects/proj/dashboard/files/golang/golang_project_structure',
    )
  })

  it('keeps the .md extension on file links in files mode', () => {
    const { container } = render(
      <RichMarkdown
        text="see [golang_project_structure](./golang_project_structure.md)"
        relativeTo="golang/golang_architecture"
        linkUrl={(resolved) => `/projects/proj/dashboard/files/${resolved}`}
        preserveExtension
      />,
    )
    const link = container.querySelector('a')
    expect(link?.getAttribute('href')).toBe(
      '/projects/proj/dashboard/files/golang/golang_project_structure.md',
    )
  })

  it('resolves directory links in files mode', () => {
    const { container } = render(
      <RichMarkdown
        text="see [config](./xdgconfig/) and [image](./logo.png)"
        relativeTo="golang/golang_architecture"
        linkUrl={(resolved) => `/projects/proj/dashboard/files/${resolved}`}
      />,
    )
    const links = container.querySelectorAll('a')
    const hrefs = Array.from(links).map((a) => a.getAttribute('href'))
    expect(hrefs).toContain('/projects/proj/dashboard/files/golang/xdgconfig')
    expect(hrefs).toContain('/projects/proj/dashboard/files/golang/logo.png')
  })

  it('resolves relative image embeds to the raw file URL', () => {
    const { container } = render(
      <RichMarkdown
        text="![kddi](assets/9433_kddi.png)"
        relativeTo="golang/golang_architecture"
        imageUrl={(resolved) => `/api/files/raw?path=${encodeURIComponent(resolved)}`}
      />,
    )
    const img = container.querySelector('img')
    expect(img?.getAttribute('src')).toBe(
      '/api/files/raw?path=golang%2Fassets%2F9433_kddi.png',
    )
  })

  it('keeps absolute and external image URLs unchanged', () => {
    const { container } = render(
      <RichMarkdown
        text={'![ext](https://example.com/x.png)\n\n![root](/assets/y.png)'}
        relativeTo="notes/foo"
        imageUrl={(resolved) => `/api/files/raw?path=${resolved}`}
      />,
    )
    const imgs = Array.from(container.querySelectorAll('img')).map((i) => i.getAttribute('src'))
    expect(imgs).toEqual(['https://example.com/x.png', '/assets/y.png'])
  })

  it('does not resolve directory links without a linkUrl', () => {
    const { container } = render(
      <RichMarkdown text="see [config](./xdgconfig/)" relativeTo="golang/golang_architecture" />,
    )
    const link = container.querySelector('a')
    expect(link?.getAttribute('href')).toBe('./xdgconfig/')
  })
})
