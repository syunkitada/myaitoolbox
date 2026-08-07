import { describe, it, expect } from 'vitest'
import {
  extractWikiLinks,
  renderWikiLinks,
  resolveWikiPath,
  normalizePath,
  resolveMarkdownLink,
  extractFrontmatterTags,
  splitFrontmatter,
  parseFrontmatter,
  serializeFrontmatter,
  buildMarkdown,
} from './markdown'

describe('splitFrontmatter', () => {
  it('splits frontmatter and body', () => {
    const split = splitFrontmatter('---\ntitle: X\nstatus: doing\n---\n\n# Body\n')
    expect(split.has).toBe(true)
    expect(split.frontmatter).toBe('title: X\nstatus: doing')
    expect(split.body).toBe('# Body\n')
  })

  it('returns whole text as body when no frontmatter', () => {
    const split = splitFrontmatter('# just body')
    expect(split.has).toBe(false)
    expect(split.frontmatter).toBe('')
    expect(split.body).toBe('# just body')
  })

  it('handles files with trailing newlines and no body', () => {
    const split = splitFrontmatter('---\ntags: [a]\n---\n\n')
    expect(split.frontmatter).toBe('tags: [a]')
    expect(split.body).toBe('')
  })
})

describe('parseFrontmatter', () => {
  it('parses scalars and lists', () => {
    const parsed = parseFrontmatter('title: X\nstatus: doing\ntags:\n  - a\n  - b')
    expect(parsed.ok).toBe(true)
    expect(parsed.data).toEqual({ title: 'X', status: 'doing', tags: ['a', 'b'] })
  })

  it('returns empty data for empty frontmatter', () => {
    expect(parseFrontmatter('')).toEqual({ ok: true, data: {} })
  })

  it('returns ok=false for invalid yaml', () => {
    expect(parseFrontmatter('status: [unclosed').ok).toBe(false)
  })
})

describe('serializeFrontmatter + buildMarkdown', () => {
  it('round-trips through buildMarkdown', () => {
    const fm = serializeFrontmatter({ title: 'X', status: 'doing', tags: ['a', 'b'] })
    expect(fm).toContain('title: X')
    expect(fm).toContain('status: doing')
    expect(fm).toContain('a')
    const doc = buildMarkdown(fm, '# Body\n')
    expect(doc).toBe(`---\n${fm}\n---\n\n# Body\n`)
    expect(parseFrontmatter(splitFrontmatter(doc).frontmatter).data).toEqual({
      title: 'X',
      status: 'doing',
      tags: ['a', 'b'],
    })
  })

  it('drops empty fields', () => {
    expect(serializeFrontmatter({ title: '', status: 'done', tags: [] })).toBe('status: done')
  })

  it('returns body only when frontmatter is empty', () => {
    expect(buildMarkdown('', '# Body\n')).toBe('# Body\n')
  })
})

describe('extractFrontmatterTags', () => {
  it('parses inline tags list', () => {
    expect(extractFrontmatterTags('---\ntitle: X\ntags: [a, b, "c d"]\n---\n\nbody')).toEqual(['a', 'b', 'c d'])
  })

  it('parses block tags list', () => {
    expect(extractFrontmatterTags('---\ntags:\n  - alpha\n  - beta\n---\n\nbody')).toEqual(['alpha', 'beta'])
  })

  it('returns empty when no frontmatter or tags', () => {
    expect(extractFrontmatterTags('plain text')).toEqual([])
    expect(extractFrontmatterTags('---\ntitle: X\n---\n\nbody')).toEqual([])
  })
})


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

describe('resolveMarkdownLink', () => {
  it('resolves ./target.md relative to the note directory', () => {
    expect(resolveMarkdownLink('./hoge.md', 'golang/golang_architecture')).toBe('golang/hoge')
  })

  it('resolves ../parent.md one level up', () => {
    expect(resolveMarkdownLink('../index.md', 'golang/golang_architecture')).toBe('index')
  })

  it('resolves bare relative target in the note directory', () => {
    expect(resolveMarkdownLink('phase6.md', 'notes/phase6')).toBe('notes/phase6')
  })

  it('resolves from a root note to a sibling file', () => {
    expect(resolveMarkdownLink('foo.md', 'index')).toBe('foo')
  })

  it('ignores absolute, scheme, anchor, and non-md targets', () => {
    expect(resolveMarkdownLink('/hoge.md', 'notes/x')).toBeNull()
    expect(resolveMarkdownLink('https://example.com/hoge.md', 'notes/x')).toBeNull()
    expect(resolveMarkdownLink('#heading', 'notes/x')).toBeNull()
    expect(resolveMarkdownLink('./image.png', 'notes/x')).toBeNull()
    expect(resolveMarkdownLink('', 'notes/x')).toBeNull()
  })
})
