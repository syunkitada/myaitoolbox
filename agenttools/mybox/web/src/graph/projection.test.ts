import { describe, it, expect } from 'vitest'
import {
  dirId,
  fileId,
  externalId,
  nodeKindOf,
  nodePathOf,
  isAncestorId,
  ancestorsOf,
  ROOT_NODE_ID,
} from './nodeId'
import { buildFileTree, computeProjection, allFilePaths, isMarkdownFile, visibleMarkdownPaths } from './tree'
import { resolveLinkTarget, parseMarkdownLinks, normalizeAlias } from './links'
import { buildProjection } from './projection'
import { GraphController } from './reconcile'
import type { ExplorerState, TreeInputEntry } from './types'

const list: TreeInputEntry[] = [
  { path: 'README.md', kind: 'file' },
  { path: 'docs/guide.md', kind: 'file' },
  { path: 'docs/ref/api.md', kind: 'file' },
  { path: 'docs/assets/logo.png', kind: 'file' },
  { path: 'notes.txt', kind: 'file' },
  { path: 'knowledge', kind: 'dir' },
  { path: 'knowledge/phase1.md', kind: 'file' },
]

function emptyState(): ExplorerState {
  return { expandedNodeIds: new Set(), selectedNodeId: null }
}

describe('nodeId', () => {
  it('round-trips paths through prefixes', () => {
    expect(nodePathOf(dirId('docs'))).toBe('docs')
    expect(nodePathOf(fileId('docs/guide.md'))).toBe('docs/guide.md')
    expect(nodePathOf(externalId('somewhere/else.md'))).toBe('somewhere/else.md')
  })

  it('classifies ids', () => {
    expect(nodeKindOf(dirId('a'))).toBe('directory')
    expect(nodeKindOf(fileId('a.md'))).toBe('file')
    expect(nodeKindOf(externalId('a.md'))).toBe('external')
    expect(nodeKindOf('garbage')).toBe(null)
  })

  it('computes ancestry', () => {
    const guide = fileId('docs/ref/guide.md')
    expect(isAncestorId(dirId(''), guide)).toBe(true)
    expect(isAncestorId(dirId('docs'), guide)).toBe(true)
    expect(isAncestorId(dirId('docs/ref'), guide)).toBe(true)
    expect(isAncestorId(dirId('docs/other'), guide)).toBe(false)
  })

  it('lists ancestors including the root', () => {
    expect(ancestorsOf(fileId('docs/ref/api.md'))).toEqual([
      dirId('docs/ref'),
      dirId('docs'),
      dirId(''),
    ])
    expect(ancestorsOf(dirId('docs'))).toEqual([dirId('')])
    expect(ancestorsOf(ROOT_NODE_ID)).toEqual([ROOT_NODE_ID])
  })
})

describe('buildFileTree', () => {
  it('marks only markdown files', () => {
    const tree = buildFileTree(list)
    expect(isMarkdownFile('docs/guide.md')).toBe(true)
    expect(isMarkdownFile('notes.txt')).toBe(false)
    expect(allFilePaths(tree).sort()).toEqual([
      'README.md',
      'docs/assets/logo.png',
      'docs/guide.md',
      'docs/ref/api.md',
      'knowledge/phase1.md',
      'notes.txt',
    ])
  })

  it('builds nested directory structures', () => {
    const tree = buildFileTree(list)
    const docs = tree.find((n) => n.path === 'docs')
    expect(docs?.kind).toBe('dir')
    expect(docs?.children?.find((n) => n.name === 'guide.md')?.markdown).toBe(true)
    expect(docs?.children?.find((n) => n.name === 'assets')).toBeDefined()
  })

  it('handles a bare directory entry with trailing leaf', () => {
    const tree = buildFileTree([{ path: 'knowledge/phase1.md', kind: 'file' }])
    const knowledge = tree.find((n) => n.path === 'knowledge')
    expect(knowledge).toBeDefined()
    expect(knowledge?.kind).toBe('dir')
  })

  it('does not nest a directory inside itself', () => {
    const tree = buildFileTree([
      { path: 'knowledge', kind: 'dir' },
      { path: 'knowledge/index.md', kind: 'file' },
    ])
    const atRoot = tree.filter((n) => n.name === 'knowledge')
    expect(atRoot).toHaveLength(1)
    expect(atRoot[0].kind).toBe('dir')
    expect(atRoot[0].children?.map((c) => c.name)).toEqual(['index.md'])
  })
})

describe('computeProjection', () => {
  const tree = buildFileTree(list)

  it('shows only top-level entries when nothing is expanded', () => {
    const { nodes, edges } = computeProjection(tree, emptyState())
    const ids = new Set(nodes.map((n) => n.id))
    expect(ids.has(fileId('README.md'))).toBe(true)
    expect(ids.has(dirId('docs'))).toBe(true)
    expect(ids.has(dirId('knowledge'))).toBe(true)
    expect(ids.has(fileId('docs/guide.md'))).toBe(false)
    expect(nodes.every((n) => n.kind !== 'external')).toBe(true)
    expect(edges.length).toBe(3)
    expect(edges.every((e) => e.kind === 'contains')).toBe(true)
  })

  it('exposes children of expanded directories', () => {
    const state = emptyState()
    state.expandedNodeIds.add(dirId('docs'))
    const { nodes, edges } = computeProjection(tree, state)
    const ids = new Set(nodes.map((n) => n.id))
    expect(ids.has(fileId('docs/guide.md'))).toBe(true)
    expect(ids.has(fileId('docs/assets/logo.png'))).toBe(false)
    expect(ids.has(dirId('docs/ref'))).toBe(true)
    const contains = edges.filter((e) => e.kind === 'contains')
    expect(contains.some((e) => e.target === fileId('docs/guide.md'))).toBe(true)
  })

  it('does not add non-markdown files as nodes', () => {
    const state = emptyState()
    state.expandedNodeIds.add(dirId('docs'))
    state.expandedNodeIds.add(dirId('docs/assets'))
    state.expandedNodeIds.add(dirId('docs/ref'))
    const ids = new Set(computeProjection(tree, state).nodes.map((n) => n.id))
    expect(ids.has(fileId('docs/assets/logo.png'))).toBe(false)
  })

  it('exposes visibleMarkdownPaths', () => {
    const state = emptyState()
    expect(visibleMarkdownPaths(tree, state)).toContain('README.md')
  })
})

describe('resolveLinkTarget', () => {
  const ctx = {
    knownMarkdown: new Set(['docs/guide.md', 'README.md', 'knowledge/phase1.md']),
  }

  it('resolves exact paths and .md suffixes', () => {
    expect(resolveLinkTarget('docs/guide.md', ctx)).toBe('docs/guide.md')
    expect(resolveLinkTarget('docs/guide', ctx)).toBe('docs/guide.md')
  })

  it('resolves wiki-style titles via basename', () => {
    expect(resolveLinkTarget('phase1', ctx)).toBe('knowledge/phase1.md')
  })

  it('returns null for ambiguous basenames', () => {
    const dupe = {
      knownMarkdown: new Set(['a/notes.md', 'b/notes.md']),
    }
    expect(resolveLinkTarget('notes', dupe)).toBe(null)
  })

  it('resolves via alias map', () => {
    const withAlias = {
      knownMarkdown: ctx.knownMarkdown,
      aliases: new Map([['the guide', 'docs/guide.md']]),
    }
    expect(resolveLinkTarget('The Guide', withAlias)).toBe('docs/guide.md')
    expect(normalizeAlias('The Guide')).toBe('the guide')
  })

  it('resolves relative markdown links and rejects urls', () => {
    expect(resolveLinkTarget('./guide', ctx, 'docs/readme.md')).toBe('docs/guide.md')
    expect(resolveLinkTarget('../README.md', ctx, 'docs/readme.md')).toBe('README.md')
    expect(resolveLinkTarget('https://example.com/x', ctx)).toBe(null)
  })
})

describe('parseMarkdownLinks', () => {
  const ctx = { knownMarkdown: new Set(['docs/guide.md', 'README.md', 'knowledge/phase1.md']) }

  it('extracts markdown and wiki links', () => {
    const content = '[guide](./docs/guide.md) and [[Phase1]] and [[missing]]'
    const refs = parseMarkdownLinks(content, 'README.md', ctx)
    expect(refs.map((r) => r.targetPath).sort()).toEqual([
      'docs/guide.md',
      'knowledge/phase1.md',
    ])
  })

  it('skips self references and duplicates', () => {
    const content = '[[README]] and [[README]] and [self](./README.md)'
    expect(parseMarkdownLinks(content, 'README.md', ctx)).toEqual([])
  })
})

describe('buildProjection', () => {
  const tree = buildFileTree(list)
  const ctx = { knownMarkdown: new Set(['README.md', 'docs/guide.md', 'knowledge/phase1.md']) }

  it('creates external nodes for references to hidden markdown files', () => {
    const state = emptyState()
    const contents = new Map([
      ['README.md', '- links to [[Phase1]] and [[docs/guide]]\n'],
    ])
    const projection = buildProjection({ tree, state, contents, linkContext: ctx })
    const ids = new Set(projection.nodes.map((n) => n.id))
    expect(ids.has(externalId('knowledge/phase1.md'))).toBe(true)
    expect(ids.has(externalId('docs/guide.md'))).toBe(true)
    const links = projection.edges.filter((e) => e.kind === 'link')
    expect(links).toHaveLength(2)
    expect(links.every((e) => e.source === fileId('README.md'))).toBe(true)
  })

  it('reuses visible file nodes when target is expanded', () => {
    const state = emptyState()
    state.expandedNodeIds.add(dirId('knowledge'))
    state.expandedNodeIds.add(dirId('docs'))
    const contents = new Map([['README.md', '[[Phase1]] and [[guide]]\n']])
    const projection = buildProjection({ tree, state, contents, linkContext: ctx })
    const ids = new Set(projection.nodes.map((n) => n.id))
    expect(ids.has(fileId('knowledge/phase1.md'))).toBe(true)
    expect(ids.has(externalId('knowledge/phase1.md'))).toBe(false)
  })
})

describe('GraphController', () => {
  it('reconciles node and edge diffs while preserving positions', () => {
    const controller = new GraphController()
    const projection1 = {
      nodes: [
        { id: ROOT_NODE_ID, kind: 'directory' as const, label: '', path: '' },
        { id: dirId('docs'), kind: 'directory' as const, label: 'docs', path: 'docs' },
        { id: fileId('README.md'), kind: 'file' as const, label: 'README.md', path: 'README.md' },
      ],
      edges: [
        { source: ROOT_NODE_ID, target: dirId('docs'), kind: 'contains' as const },
        { source: ROOT_NODE_ID, target: fileId('README.md'), kind: 'contains' as const },
      ],
    }
    controller.sync(projection1, { layout: true })
    expect(controller.graph.order).toBe(3)
    expect(controller.graph.size).toBe(2)

    const pos = controller.graph.getNodeAttribute(fileId('README.md'), 'x') as number
    controller.sync(projection1, { layout: false })
    expect(controller.graph.getNodeAttribute(fileId('README.md'), 'x')).toBe(pos)

    const projection2 = {
      nodes: [
        { id: ROOT_NODE_ID, kind: 'directory' as const, label: '', path: '' },
        {
          id: fileId('notes/other.md'),
          kind: 'external' as const,
          label: 'notes/other.md',
          path: 'notes/other.md',
        },
      ],
      edges: [{ source: ROOT_NODE_ID, target: fileId('notes/other.md'), kind: 'link' as const }],
    }
    controller.sync(projection2, { layout: false })
    expect(controller.graph.order).toBe(2)
    expect(controller.graph.hasNode(fileId('README.md'))).toBe(false)
    expect(controller.graph.hasDirectedEdge(ROOT_NODE_ID, fileId('notes/other.md'))).toBe(true)
  })

  it('keeps dragged nodes fixed through layout', () => {
    const controller = new GraphController()
    const projection = {
      nodes: [{ id: fileId('a.md'), kind: 'file' as const, label: 'a', path: 'a.md' }],
      edges: [],
    }
    controller.sync(projection, { layout: true })
    const before = controller.graph.getNodeAttribute(fileId('a.md'), 'x') as number
    controller.setNodePosition(fileId('a.md'), 123.5, 456.5)
    controller.applyLayout(200)
    expect(controller.graph.getNodeAttribute(fileId('a.md'), 'x')).toBeCloseTo(123.5, 3)
    expect(controller.graph.getNodeAttribute(fileId('a.md'), 'x')).toBeCloseTo(123.5, 3)
    expect(before).not.toBeCloseTo(123.5, 3)
  })
})