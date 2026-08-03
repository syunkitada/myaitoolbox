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
})
