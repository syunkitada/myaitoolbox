import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { useEffect, useState } from 'react'
import type Graph from 'graphology'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { api } from '../../api/client'
import type { GraphEdgeAttributes, GraphNodeAttributes, NodeId } from '../../graph/types'
import { GraphWorkspace } from './GraphWorkspace'

type GraphWithAttrs = Graph<GraphNodeAttributes, GraphEdgeAttributes>

vi.mock('./GraphView', () => ({
  GraphView: ({
    graph,
    selectedNodeId,
    onNodeSelect,
    onOpenFile,
    onExpandAll,
    onCollapseAll,
  }: {
    graph: GraphWithAttrs
    selectedNodeId: NodeId | null
    onNodeSelect: (id: NodeId) => void
    onOpenFile?: (path: string) => void
    onExpandAll?: (id: NodeId) => void
    onCollapseAll?: (id: NodeId) => void
  }) => {
    const [, setVersion] = useState(0)
    useEffect(() => {
      const refresh = () => setVersion((previous) => previous + 1)
      graph.on('nodeAdded', refresh)
      graph.on('nodeDropped', refresh)
      return () => {
        graph.off('nodeAdded', refresh)
        graph.off('nodeDropped', refresh)
      }
    }, [graph])
    return (
      <div>
        <div data-testid="graph-nodes">{graph.nodes().map((id) => graph.getNodeAttribute(id, 'path')).join(',')}</div>
        <div data-testid="graph-selected">{selectedNodeId ?? ''}</div>
        <button onClick={() => onNodeSelect('file:docs/guide.md')}>Select guide</button>
        <button onClick={() => onOpenFile?.('docs/guide.md')}>Open guide</button>
        <button onClick={() => onExpandAll?.('dir:docs')}>Expand docs subtree</button>
        <button onClick={() => onCollapseAll?.('dir:docs')}>Collapse docs subtree</button>
      </div>
    )
  },
}))

function Harness({ onOpenFile = vi.fn() }: { onOpenFile?: (path: string) => void }) {
  const [expandedPaths, setExpandedPaths] = useState(new Set<string>())
  const [refreshKey, setRefreshKey] = useState(0)
  return (
    <>
      <button type="button" onClick={() => setRefreshKey((previous) => previous + 1)}>Refresh graph files</button>
      <GraphWorkspace
        project="demo"
        refreshKey={refreshKey}
        expandedPaths={expandedPaths}
        onExpandedPathsChange={setExpandedPaths}
        onLoadDir={vi.fn()}
        onOpenFile={onOpenFile}
      />
    </>
  )
}

describe('GraphWorkspace', () => {
  afterEach(() => {
    vi.restoreAllMocks()
    localStorage.clear()
  })

  it('projects the Files tree expansion state and reveals a selected graph node', async () => {
    vi.spyOn(api, 'listFiles').mockResolvedValue([
      { path: 'docs', name: 'docs', kind: 'dir' },
      { path: 'docs/guide.md', name: 'guide.md', kind: 'file' },
    ])
    vi.spyOn(api, 'getFileContent').mockResolvedValue({ path: 'docs/guide.md', content: '# Guide' })

    render(<Harness />)

    expect(await screen.findByText((_, element) => element?.textContent === '1 nodes · 1 edges')).toBeInTheDocument()
    expect(api.getFileContent).not.toHaveBeenCalled()

    fireEvent.click(screen.getByRole('button', { name: 'Select guide' }))

    await waitFor(() => expect(screen.getByTestId('graph-nodes')).toHaveTextContent('docs/guide.md'))
    expect(screen.getByTestId('graph-selected')).toHaveTextContent('file:docs/guide.md')
  })

  it('opens a graph file through the Files view callback', async () => {
    const onOpenFile = vi.fn()
    vi.spyOn(api, 'listFiles').mockResolvedValue([
      { path: 'docs', name: 'docs', kind: 'dir' },
      { path: 'docs/guide.md', name: 'guide.md', kind: 'file' },
    ])
    vi.spyOn(api, 'getFileContent').mockResolvedValue({ path: 'docs/guide.md', content: '# Guide' })

    render(<Harness onOpenFile={onOpenFile} />)

    await screen.findByTestId('graph-nodes')
    act(() => fireEvent.click(screen.getByRole('button', { name: 'Open guide' })))
    expect(onOpenFile).toHaveBeenCalledWith('docs/guide.md')
  })

  it('expands and collapses a directory subtree through the shared Files state', async () => {
    vi.spyOn(api, 'listFiles').mockResolvedValue([
      { path: 'docs', name: 'docs', kind: 'dir' },
      { path: 'docs/guide.md', name: 'guide.md', kind: 'file' },
    ])
    vi.spyOn(api, 'getFileContent').mockResolvedValue({ path: 'docs/guide.md', content: '# Guide' })

    render(<Harness />)

    await screen.findByTestId('graph-nodes')
    expect(screen.getByTestId('graph-nodes')).not.toHaveTextContent('docs/guide.md')

    fireEvent.click(screen.getByRole('button', { name: 'Expand docs subtree' }))
    await waitFor(() => expect(screen.getByTestId('graph-nodes')).toHaveTextContent('docs/guide.md'))

    fireEvent.click(screen.getByRole('button', { name: 'Collapse docs subtree' }))
    await waitFor(() => expect(screen.getByTestId('graph-nodes')).not.toHaveTextContent('docs/guide.md'))
  })

  it('reloads the graph tree after Files changes', async () => {
    const listFiles = vi.spyOn(api, 'listFiles')
      .mockResolvedValueOnce([{ path: 'docs', name: 'docs', kind: 'dir' }])
      .mockResolvedValueOnce([
        { path: 'docs', name: 'docs', kind: 'dir' },
        { path: 'notes.md', name: 'notes.md', kind: 'file' },
      ])
    vi.spyOn(api, 'getFileContent').mockResolvedValue({ path: 'notes.md', content: '# Notes' })

    render(<Harness />)

    expect(await screen.findByText((_, element) => element?.textContent === '1 nodes · 1 edges')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Refresh graph files' }))

    expect(await screen.findByText((_, element) => element?.textContent === '2 nodes · 2 edges')).toBeInTheDocument()
    expect(listFiles).toHaveBeenCalledTimes(2)
  })
})
