import { afterEach, describe, expect, it } from 'vitest'
import {
  loadExplorerExpandedPaths,
  loadGraphMode,
  saveExplorerExpandedPaths,
  saveGraphMode,
} from './graphViewState'

describe('graph and shared explorer view state', () => {
  afterEach(() => localStorage.clear())

  it('stores graph mode separately for each project', () => {
    saveGraphMode('alpha', true)
    saveGraphMode('beta', false)

    expect(loadGraphMode('alpha')).toBe(true)
    expect(loadGraphMode('beta')).toBe(false)
    expect(loadGraphMode('missing')).toBe(false)
  })

  it('stores shared expanded directory paths per project', () => {
    saveExplorerExpandedPaths('alpha', new Set(['docs', 'docs/design']))

    expect(loadExplorerExpandedPaths('alpha')).toEqual(new Set(['docs', 'docs/design']))
    expect(loadExplorerExpandedPaths('beta')).toBeNull()
  })

  it('migrates the existing graph explorer expansion state when no shared state exists', () => {
    localStorage.setItem(
      'mybox_graph_explorer_state',
      JSON.stringify({ alpha: { expanded: ['dir:docs', 'dir:docs/design'], selected: 'file:docs/design/a.md' } }),
    )

    expect(loadExplorerExpandedPaths('alpha')).toEqual(new Set(['docs', 'docs/design']))
    expect(loadExplorerExpandedPaths('alpha')).toEqual(new Set(['docs', 'docs/design']))
    expect(JSON.parse(localStorage.getItem('mybox_files_explorer_expanded') ?? '{}')).toEqual({
      alpha: ['docs', 'docs/design'],
    })
  })

  it('prefers an explicitly saved empty expansion state over legacy graph state', () => {
    localStorage.setItem(
      'mybox_graph_explorer_state',
      JSON.stringify({ alpha: { expanded: ['dir:docs'], selected: null } }),
    )
    saveExplorerExpandedPaths('alpha', new Set())

    expect(loadExplorerExpandedPaths('alpha')).toEqual(new Set())
  })
})
