import { describe, expect, it } from 'vitest'
import { buildFileTree, subtreeDirIds } from './tree'

const list = [
  { path: 'README.md', kind: 'file' as const },
  { path: 'knowledge', kind: 'dir' as const },
  { path: 'knowledge/index.md', kind: 'file' as const },
  { path: 'knowledge/docs', kind: 'dir' as const },
  { path: 'knowledge/docs/guide.md', kind: 'file' as const },
  { path: 'knowledge/docs/recipes', kind: 'dir' as const },
  { path: 'knowledge/docs/recipes/pizza.md', kind: 'file' as const },
]

const tree = buildFileTree(list)

describe('subtreeDirIds', () => {
  it('collects a nested directory and its descendants', () => {
    expect(subtreeDirIds(tree, 'knowledge/docs')).toEqual([
      'dir:knowledge/docs',
      'dir:knowledge/docs/recipes',
    ])
  })

  it('includes the root of the subtree itself', () => {
    expect(subtreeDirIds(tree, 'knowledge')).toEqual([
      'dir:knowledge',
      'dir:knowledge/docs',
      'dir:knowledge/docs/recipes',
    ])
  })

  it('returns an empty array for unknown directories', () => {
    expect(subtreeDirIds(tree, 'missing')).toEqual([])
    expect(subtreeDirIds(tree, '')).toEqual([])
  })
})