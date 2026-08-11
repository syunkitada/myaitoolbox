import { describe, it, expect } from 'vitest'
import {
  extractWikiLinks,
  renderWikiLinks,
  resolveWikiPath,
  normalizePath,
  resolveMarkdownLink,
  buildDirListing,
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
  const hrefOf = (r: string) => `/projects/proj/dashboard/files/${r}`

  it('turns resolved links into path anchors', () => {
    const out = renderWikiLinks('[[notes/alpha]]', pathOf, hrefOf)
    expect(out).toContain('[notes/alpha](/projects/proj/dashboard/files/notes/alpha)')
  })

  it('uses alias as label', () => {
    const out = renderWikiLinks('[[notes/alpha|Alpha Doc]]', pathOf, hrefOf)
    expect(out).toContain('[Alpha Doc](/projects/proj/dashboard/files/notes/alpha)')
  })

  it('leaves unresolved links as plain label', () => {
    expect(renderWikiLinks('[[notes/missing]]', pathOf, hrefOf)).toBe('notes/missing')
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

  it('preserves the .md extension when preserveExtension is set', () => {
    expect(resolveMarkdownLink('./hoge.md', 'golang/golang_architecture', true)).toBe('golang/hoge.md')
    expect(resolveMarkdownLink('../index.md', 'golang/golang_architecture', true)).toBe('index.md')
    expect(resolveMarkdownLink('foo.md', 'index', true)).toBe('foo.md')
  })

  it('ignores absolute, scheme, anchor, and non-md targets', () => {
    expect(resolveMarkdownLink('/hoge.md', 'notes/x')).toBeNull()
    expect(resolveMarkdownLink('https://example.com/hoge.md', 'notes/x')).toBeNull()
    expect(resolveMarkdownLink('#heading', 'notes/x')).toBeNull()
    expect(resolveMarkdownLink('./image.png', 'notes/x')).toBeNull()
    expect(resolveMarkdownLink('', 'notes/x')).toBeNull()
  })
})

describe('resolveMarkdownLink directory links', () => {
  const opts = { resolveDirectories: true }

  it('resolves ./dir/ relative to the note directory', () => {
    expect(resolveMarkdownLink('./xdgconfig/', 'golang/golang_architecture', true, opts)).toBe(
      'golang/xdgconfig',
    )
  })

  it('resolves ../parent/ one level up', () => {
    expect(resolveMarkdownLink('../config/', 'golang/sub/note', true, opts)).toBe('golang/config')
  })

  it('resolves a dir target from a directory context', () => {
    expect(resolveMarkdownLink('sub/', 'xdgconfig/', true, opts)).toBe('xdgconfig/sub')
  })

  it('does not resolve directory links without the option', () => {
    expect(resolveMarkdownLink('./xdgconfig/', 'notes/x', true)).toBeNull()
  })
})

describe('resolveMarkdownLink relativeTo directory', () => {
  it('keeps the directory segment when relativeTo ends with a slash', () => {
    expect(resolveMarkdownLink('guide.md', 'xdgconfig/', true)).toBe('xdgconfig/guide.md')
    expect(resolveMarkdownLink('guide.md', 'xdgconfig/', false)).toBe('xdgconfig/guide')
  })

  it('resolves non-md files when resolveAnyFile is set', () => {
    expect(resolveMarkdownLink('logo.png', 'notes/guide', true, { resolveAnyFile: true })).toBe(
      'notes/logo.png',
    )
  })
})

describe('buildDirListing', () => {
  const entries = [
    { path: 'xdgconfig', name: 'xdgconfig', kind: 'dir' as const },
    { path: 'xdgconfig/README.md', name: 'README.md', kind: 'file' as const },
    { path: 'xdgconfig/config.toml', name: 'config.toml', kind: 'file' as const },
    { path: 'xdgconfig/sub', name: 'sub', kind: 'dir' as const },
    { path: 'other.md', name: 'other.md', kind: 'file' as const },
  ]

  it('links directories with a trailing slash and files plainly', () => {
    const out = buildDirListing('xdgconfig', entries)
    expect(out).toContain('# xdgconfig')
    expect(out).toContain('- [sub/](sub/)')
    expect(out).toContain('- [README.md](README.md)')
    expect(out).toContain('- [config.toml](config.toml)')
    expect(out).not.toContain('other.md')
  })

  it('renders directories before files', () => {
    const out = buildDirListing('xdgconfig', entries)
    expect(out.indexOf('- [sub/](sub/)')).toBeLessThan(out.indexOf('- [README.md](README.md)'))
  })

  it('handles the project root', () => {
    const out = buildDirListing('', entries)
    expect(out).toContain('# Files')
    expect(out).toContain('- [xdgconfig/](xdgconfig/)')
  })

  it('marks empty directories', () => {
    const out = buildDirListing('xdgconfig/empty', entries)
    expect(out).toContain('(empty)')
  })
})
