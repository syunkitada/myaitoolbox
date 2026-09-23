import { useState } from 'react'
import { describe, it, expect } from 'vitest'
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { RichMarkdown, extractOutline } from './RichMarkdown'

function renderMd(ui: React.ReactElement) {
  return render(<MemoryRouter>{ui}</MemoryRouter>)
}

function CopyableMarkdown({ text }: { text: string }) {
  const [copyDialogOpen, setCopyDialogOpen] = useState(false)
  return (
    <>
      <button type="button" onClick={() => setCopyDialogOpen(true)} aria-label="Copy file contents" />
      <RichMarkdown
        text={text}
        copyDialogOpen={copyDialogOpen}
        onCopyDialogClose={() => setCopyDialogOpen(false)}
      />
    </>
  )
}

describe('RichMarkdown', () => {
  it('assigns slug ids to headings matching extractOutline', () => {
    const text = '# Hello World\n\n## Overview\n\n## See Alpha\n'
    renderMd(<RichMarkdown text={text} />)
    const outline = extractOutline(text)
    expect(outline.length).toBeGreaterThan(0)
    for (const h of outline) {
      const el = document.getElementById(h.id)
      expect(el).not.toBeNull()
      expect(el?.tagName).toBe(`H${h.level}`)
    }
  })

  it('resolves anchors to headings containing inline code and underscores', () => {
    const text = '[`list_alerts`](#list_alerts)\n\n### `list_alerts`'
    const { container } = renderMd(<RichMarkdown text={text} />)

    expect(container.querySelector('h3')).toHaveAttribute('id', 'list_alerts')
    expect(container.querySelector('a')).not.toHaveClass('dead-anchor')
    expect(container.querySelector('.dead-link-mark')).not.toBeInTheDocument()
    expect(extractOutline(text)).toEqual([
      { level: 3, id: 'list_alerts', text: 'list_alerts' },
    ])
  })

  it('resolves relative file links with a custom linkUrl', () => {
    const { container } = renderMd(
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
    const { container } = renderMd(
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
    const { container } = renderMd(
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
    const { container } = renderMd(
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
    const { container } = renderMd(
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
    const { container } = renderMd(
      <RichMarkdown text="see [config](./xdgconfig/)" relativeTo="golang/golang_architecture" />,
    )
    const link = container.querySelector('a')
    expect(link?.getAttribute('href')).toBe('./xdgconfig/')
  })

  it.each([
    ['```', '```\n## Not a heading\n```'],
    ['~~~', '~~~\n### Also not a heading\n~~~'],
  ])('extractOutline skips headings inside %s fences', (_label, fenced) => {
    const outline = extractOutline(`# Real heading\n\n${fenced}\n\n## Another real heading\n`)
    expect(outline.map((h) => h.text)).toEqual(['Real heading', 'Another real heading'])
  })

  it('extractOutline resumes after a fenced block closes', () => {
    const outline = extractOutline('# A\n\n```\n# B\n```\n\n## C\n')
    expect(outline.map((h) => ({ level: h.level, text: h.text }))).toEqual([
      { level: 1, text: 'A' },
      { level: 2, text: 'C' },
    ])
  })

  it('extractOutline handles CRLF line endings', () => {
    const outline = extractOutline(
      '# Hoge\r\n\r\n## 1. Hoge\r\n\r\n# Piyo\r\n\r\n## 2. Piyo\r\n',
    )
    expect(outline.map((h) => ({ level: h.level, text: h.text, id: h.id }))).toEqual([
      { level: 1, text: 'Hoge', id: 'hoge' },
      { level: 2, text: '1. Hoge', id: '1-hoge' },
      { level: 1, text: 'Piyo', id: 'piyo' },
      { level: 2, text: '2. Piyo', id: '2-piyo' },
    ])
  })

  it('assigns slug ids to CRLF headings matching extractOutline', () => {
    const text = '# Hoge\r\n\r\n## 1. Hoge\r\n\r\n# Piyo\r\n\r\n## 2. Piyo\r\n'
    renderMd(<RichMarkdown text={text} />)
    const outline = extractOutline(text)
    expect(outline.length).toBe(4)
    for (const h of outline) {
      const el = document.getElementById(h.id)
      expect(el).not.toBeNull()
      expect(el?.tagName).toBe(`H${h.level}`)
    }
  })

  it('extractOutline accepts up to three leading spaces like markdown-it', () => {
    const outline = extractOutline('# One\n\n  ## Two\n\n    # Not a heading (indented code)\n')
    expect(outline.map((h) => ({ level: h.level, text: h.text }))).toEqual([
      { level: 1, text: 'One' },
      { level: 2, text: 'Two' },
    ])
  })

  it('does not mark Japanese anchor links as dead', () => {
    const { container } = renderMd(
      <RichMarkdown text={'[memo](#メモ)\n\n# メモ\n\nあああ'} />,
    )
    const link = container.querySelector('a')
    expect(link).not.toBeNull()
    expect(link?.classList.contains('dead-anchor')).toBe(false)
    expect(container.querySelector('.dead-link-mark')).toBeNull()
  })

  it('renders task list markers as checkboxes and reports toggles', async () => {
    const onTaskToggle = vi.fn().mockResolvedValue(true)
    const { container } = renderMd(
      <RichMarkdown
        text={'- [ ] first task\n- [x] completed task'}
        onTaskToggle={onTaskToggle}
      />,
    )

    const checkboxes = container.querySelectorAll<HTMLInputElement>('input[data-task-checkbox]')
    expect(checkboxes).toHaveLength(2)
    expect(checkboxes[0]).not.toBeChecked()
    expect(checkboxes[1]).toBeChecked()
    expect(container.textContent).toContain('first task')
    expect(container.textContent).not.toContain('[ ]')

    fireEvent.click(checkboxes[0])
    await waitFor(() => expect(onTaskToggle).toHaveBeenCalledWith(0, true))
    expect(checkboxes[0]).toBeChecked()
  })

  it('reverts a checkbox when saving the task fails', async () => {
    const onTaskToggle = vi.fn().mockResolvedValue(false)
    const { container } = renderMd(
      <RichMarkdown text={'- [ ] task'} onTaskToggle={onTaskToggle} />,
    )
    const checkbox = container.querySelector<HTMLInputElement>('input[data-task-checkbox]')!

    fireEvent.click(checkbox)
    await waitFor(() => expect(checkbox).not.toBeChecked())
  })

  it('highlights fenced code blocks with prism tokens', () => {
    const { container } = renderMd(<RichMarkdown text={'```javascript\nconst x = 1\n```'} />)
    expect(container.querySelectorAll('.token').length).toBeGreaterThan(0)
    const code = container.querySelector('pre.language-javascript code')
    expect(code?.textContent).toContain('const x = 1')
  })

  it('copies fenced code from its code block button', async () => {
    const originalClipboard = navigator.clipboard
    const writeText = vi.fn().mockResolvedValue(undefined)
    Object.defineProperty(navigator, 'clipboard', {
      configurable: true,
      value: { writeText },
    })

    try {
      renderMd(<RichMarkdown text={'```javascript\nconst x = 1\n```'} />)
      const button = screen.getByRole('button', { name: 'Copy code' })

      fireEvent.click(button)

      await waitFor(() => expect(writeText).toHaveBeenCalledWith('const x = 1\n'))
      expect(button).toHaveTextContent('Copied')
    } finally {
      Object.defineProperty(navigator, 'clipboard', {
        configurable: true,
        value: originalClipboard,
      })
    }
  })

  it('previews and copies markdown as text from the single copy button', async () => {
    const originalClipboard = navigator.clipboard
    const writeText = vi.fn().mockResolvedValue(undefined)
    Object.defineProperty(navigator, 'clipboard', {
      configurable: true,
      value: { writeText },
    })

    try {
      const text = '# Hello\n\nThis is the note.'
      renderMd(<CopyableMarkdown text={text} />)
      fireEvent.click(screen.getByRole('button', { name: 'Copy file contents' }))

      const dialog = screen.getByRole('dialog', { name: 'Copy file contents' })
      expect(within(dialog).getByRole('tab', { name: 'Text' })).toHaveAttribute('aria-selected', 'true')
      expect(within(dialog).getByRole('textbox', { name: 'Text copy preview' })).toHaveValue(text)
      expect(writeText).not.toHaveBeenCalled()

      fireEvent.click(within(dialog).getByRole('button', { name: 'Copy' }))
      await waitFor(() => expect(writeText).toHaveBeenCalledWith(text))
      expect(within(dialog).getByRole('button', { name: 'Copied' })).toBeInTheDocument()
    } finally {
      Object.defineProperty(navigator, 'clipboard', {
        configurable: true,
        value: originalClipboard,
      })
    }
  })

  it('switches the modal preview and copy target to Jira wiki markup', async () => {
    const originalClipboard = navigator.clipboard
    const writeText = vi.fn().mockResolvedValue(undefined)
    const user = userEvent.setup()
    Object.defineProperty(navigator, 'clipboard', {
      configurable: true,
      value: { writeText },
    })

    try {
      renderMd(<CopyableMarkdown text={'# Hello\n\n**bold**'} />)
      await user.click(screen.getByRole('button', { name: 'Copy file contents' }))
      const dialog = screen.getByRole('dialog', { name: 'Copy file contents' })
      await user.click(within(dialog).getByRole('tab', { name: 'Jira' }))
      expect(within(dialog).getByRole('textbox', { name: 'Jira copy preview' })).toHaveValue(
        'h1. Hello\n\n*bold*',
      )
      await user.click(within(dialog).getByRole('button', { name: 'Copy' }))

      await waitFor(() => expect(writeText).toHaveBeenCalledWith('h1. Hello\n\n*bold*'))
    } finally {
      Object.defineProperty(navigator, 'clipboard', {
        configurable: true,
        value: originalClipboard,
      })
    }
  })
})
