import { describe, it, expect } from 'vitest'
import { dirname, relativePath, suggestLinkTargets, type LinkPathItem } from './markdown-link-completions'

describe('dirname', () => {
  it('returns empty for root paths', () => {
    expect(dirname('')).toBe('')
    expect(dirname('foo.md')).toBe('')
  })

  it('returns the directory portion', () => {
    expect(dirname('a/b.md')).toBe('a')
    expect(dirname('a/b/c.md')).toBe('a/b')
  })
})

describe('relativePath', () => {
  it('returns the target when no base directory', () => {
    expect(relativePath('', 'notes/arch.md')).toBe('notes/arch.md')
  })

  it('computes a relative path within the same directory', () => {
    expect(relativePath('a', 'a/b.md')).toBe('b.md')
  })

  it('computes a relative path using ..', () => {
    expect(relativePath('a/b', 'a/c.md')).toBe('../c.md')
    expect(relativePath('a/b', 'c.md')).toBe('../../c.md')
  })

  it('computes a relative path going deeper', () => {
    expect(relativePath('a', 'a/b/c.md')).toBe('b/c.md')
  })
})

function item(path: string, kind: 'file' | 'dir' = 'file'): LinkPathItem {
  return { path, kind }
}

describe('suggestLinkTargets', () => {
  const items: LinkPathItem[] = [
    item('notes/arch.md'),
    item('notes/archive.md'),
    item('notes/design.md'),
    item('notes/sub/dep.md'),
    item('tasks/todo.md'),
    item('docs', 'dir'),
    item('docs/readme.md'),
    item('docs/other.md'),
  ]

  it('shows immediate children when typed is empty', () => {
    const results = suggestLinkTargets(undefined, items, '')
    const labels = results.map((r) => r.label)
    expect(labels).toContain('docs/')
    expect(labels).toContain('notes/')
    expect(labels).toContain('tasks/')
    expect(labels).not.toContain('readme.md')
  })

  it('shows immediate children when typed is ./', () => {
    const results = suggestLinkTargets(undefined, items, './')
    const labels = results.map((r) => r.label)
    expect(labels).toContain('docs/')
    expect(labels).toContain('notes/')
    expect(labels).toContain('tasks/')
  })

  it('shows children of a named directory', () => {
    const results = suggestLinkTargets(undefined, items, './docs/')
    const labels = results.map((r) => r.label)
    expect(labels).toContain('readme.md')
    expect(labels).toContain('other.md')
  })

  it('shows candidates relative to the current file directory', () => {
    const results = suggestLinkTargets('notes/arch.md', items, './')
    const labels = results.map((r) => r.label)
    expect(labels).toContain('design.md')
    expect(labels).toContain('archive.md')
    expect(labels).not.toContain('arch.md')
  })

  it('searches by leaf name across the tree', () => {
    const results = suggestLinkTargets(undefined, items, './dep')
    const labels = results.map((r) => r.label)
    expect(labels).toContain('dep.md')
    const dep = results.find((r) => r.label === 'dep.md')
    expect(dep?.insertText).toBe('./notes/sub/dep.md')
  })

  it('returns . insertText when typed starts with ./', () => {
    const results = suggestLinkTargets(undefined, items, './docs/')
    expect(results.length).toBeGreaterThan(0)
    for (const r of results) {
      expect(r.insertText).toMatch(/^\.\//)
    }
  })

  it('returns insertText without . when typed has no dot prefix', () => {
    const results = suggestLinkTargets(undefined, items, 'docs/')
    expect(results.length).toBeGreaterThan(0)
    for (const r of results) {
      expect(r.insertText).not.toMatch(/^\.\//)
    }
  })

  it('excludes the current file', () => {
    const results = suggestLinkTargets('notes/arch.md', items, './arch')
    const labels = results.map((r) => r.label)
    expect(labels).not.toContain('arch.md')
    expect(labels).toContain('archive.md')
  })

  it('marks directories as dir kind', () => {
    const results = suggestLinkTargets(undefined, items, './do')
    const docs = results.find((r) => r.detail === 'docs')
    expect(docs?.kind).toBe('dir')
    expect(docs?.insertText).toBe('./docs/')
  })

  it('filters by dir prefix', () => {
    const results = suggestLinkTargets(undefined, items, './do')
    const labels = results.map((r) => r.label)
    expect(labels).toContain('docs/')
    expect(labels).not.toContain('todo.md')
    const docs = results.find((r) => r.label === 'docs/')
    expect(docs?.insertText).toBe('./docs/')
  })

  it('filters by name prefix within a directory', () => {
    const results = suggestLinkTargets(undefined, items, './to')
    const labels = results.map((r) => r.label)
    expect(labels).toContain('todo.md')
  })

  it('restricts nested browsing by dir prefix', () => {
    const results = suggestLinkTargets(undefined, items, './notes/sub/')
    const labels = results.map((r) => r.label)
    expect(labels).toContain('dep.md')
    expect(labels).not.toContain('arch.md')
    const dep = results.find((r) => r.label === 'dep.md')
    expect(dep?.insertText).toBe('./notes/sub/dep.md')
  })

  it('skips paths that are not linkable', () => {
    const ugly = [item('has space.md'), item('paren(x).md')]
    const results = suggestLinkTargets(undefined, ugly, '')
    expect(results).toHaveLength(0)
  })

  it('sorts the parent .. entry last', () => {
    const itemsWithParent = [
      item('knowledge/index.md'),
      item('knowledge/docs/guide.md'),
      ...items,
    ]
    const results = suggestLinkTargets('knowledge/index.md', itemsWithParent, './')
    const parents = results.filter((r) => r.label === '../')
    expect(parents).toHaveLength(1)
    expect(parents[0]?.insertText).toBe('./../')
    expect(results[results.length - 1]?.label).toBe('../')
  })
})