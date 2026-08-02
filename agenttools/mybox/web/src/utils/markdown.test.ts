import { describe, it, expect } from 'vitest'
import { extractWikiLinks, renderWikiLinks, resolveWikiPath, normalizePath } from './markdown'

describe('extractWikiLinks', () => {
  it('extracts [[path]] links', () => {
    const links = extractWikiLinks('see [[notes/alpha]] and [[notes/beta.md|Beta]]')
    expect(links).toEqual([
      { target: 'notes/alpha', alias: null, raw: '[[notes/alpha]]' },
      { target: 'notes/beta.md', alias: 'Beta', raw: '[[notes/beta.md|Beta]]' },
    ])
  })

  it('returns empty for no links', () => {
    expect(extractWikiLinks('plain text')).toEqual([])
  })
})

describe('resolveWikiPath', () => {
  it('strips .md suffix', () => {
    expect(resolveWikiPath('notes/foo.md')).toBe('notes/foo')
  })
})

describe('normalizePath', () => {
  it('lowercases and strips extension', () => {
    expect(normalizePath('Notes/Alpha.MD')).toBe('notes/alpha')
  })
})

describe('renderWikiLinks', () => {
  const pathOf = (t: string) => (t === 'notes/alpha' ? 'notes/alpha' : null)

  it('turns resolved links into hash anchors', () => {
    const out = renderWikiLinks('[[notes/alpha]]', pathOf)
    expect(out).toContain('[notes/alpha](#/knowledge/notes%2Falpha)')
  })

  it('uses alias as label', () => {
    const out = renderWikiLinks('[[notes/alpha|Alpha Doc]]', pathOf)
    expect(out).toContain('[Alpha Doc](#/knowledge/notes%2Falpha)')
  })

  it('leaves unresolved links as plain label', () => {
    expect(renderWikiLinks('[[notes/missing]]', pathOf)).toBe('notes/missing')
  })
})
