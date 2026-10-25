import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import type { Dispatch, SetStateAction } from 'react'
import { api } from '../../api/client'
import { GraphView } from './GraphView'
import { GraphController } from '../../graph/reconcile'
import { allFilePaths, buildFileTree, isMarkdownFile, subtreeDirIds } from '../../graph/tree'
import { buildProjection, missingContentPaths } from '../../graph/projection'
import { ancestorsOf, dirId, nodeKindOf, nodePathOf } from '../../graph/nodeId'
import { loadLayout, saveLayout } from '../../state/graphViewState'
import type { NodeId, TreeInputEntry } from '../../graph/types'

function setsEqual(a: Set<string>, b: Set<string>): boolean {
  if (a.size !== b.size) return false
  for (const value of a) if (!b.has(value)) return false
  return true
}

export interface GraphWorkspaceProps {
  project: string
  refreshKey: number
  expandedPaths: Set<string>
  onExpandedPathsChange: Dispatch<SetStateAction<Set<string>>>
  onLoadDir: (path: string) => void | Promise<void>
  onOpenFile: (path: string) => void
}

export function GraphWorkspace({
  project,
  refreshKey,
  expandedPaths,
  onExpandedPathsChange,
  onLoadDir,
  onOpenFile,
}: GraphWorkspaceProps) {
  const [treeInput, setTreeInput] = useState<TreeInputEntry[]>([])
  const [contents, setContents] = useState<Map<string, string>>(new Map())
  const [selectedNodeId, setSelectedNodeId] = useState<NodeId | null>(null)
  const [error, setError] = useState<string | null>(null)
  const inFlight = useRef<Set<string>>(new Set())
  const treeGeneration = useRef(0)
  const controllerRef = useRef<GraphController | null>(null)
  const prevNodeSet = useRef<Set<string> | null>(null)
  const appliedPersistedLayout = useRef(false)
  if (!controllerRef.current) controllerRef.current = new GraphController(loadLayout(project))
  const controller = controllerRef.current

  const tree = useMemo(() => buildFileTree(treeInput), [treeInput])
  const expandedNodeIds = useMemo(
    () => new Set([...expandedPaths].map(dirId)),
    [expandedPaths],
  )
  const linkContext = useMemo(() => {
    const knownMarkdown = new Set(allFilePaths(tree).filter(isMarkdownFile))
    return { knownMarkdown }
  }, [tree])
  const graphState = useMemo(
    () => ({ expandedNodeIds, selectedNodeId }),
    [expandedNodeIds, selectedNodeId],
  )
  const projection = useMemo(
    () => buildProjection({ tree, state: graphState, contents, linkContext }),
    [tree, graphState, contents, linkContext],
  )

  useEffect(() => {
    let cancelled = false
    const generation = ++treeGeneration.current
    inFlight.current.clear()
    setContents(new Map())
    void api
      .listFiles()
      .then((list) => {
        if (!cancelled && treeGeneration.current === generation) {
          setTreeInput(list.map((entry) => ({ path: entry.path, kind: entry.kind })))
          setError(null)
        }
      })
      .catch((loadError) => {
        if (!cancelled && treeGeneration.current === generation) {
          setError(loadError instanceof Error ? loadError.message : String(loadError))
        }
      })
    return () => {
      cancelled = true
    }
  }, [refreshKey])

  useEffect(() => {
    const visible = projection.nodes.filter((node) => node.kind === 'file').map((node) => node.path)
    const missing = missingContentPaths(visible, contents, inFlight.current)
    if (missing.length === 0) return
    const generation = treeGeneration.current
    for (const path of missing) {
      // Mark paths before starting requests so another completion does not
      // cause still-pending files to be fetched again.
      inFlight.current.add(path)
      void api
        .getFileContent(path)
        .then((result) => {
          if (treeGeneration.current !== generation) return
          setContents((previous) => {
            const next = new Map(previous)
            next.set(path, result.content)
            return next
          })
        })
        .catch(() => undefined)
        .finally(() => {
          if (treeGeneration.current === generation) inFlight.current.delete(path)
        })
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [projection])

  useEffect(() => {
    const ids = new Set(projection.nodes.map((node) => node.id))
    const nodeSetUnchanged = prevNodeSet.current !== null && setsEqual(prevNodeSet.current, ids)
    prevNodeSet.current = ids

    const persistedLayout = loadLayout(project)
    const storedCoversCurrent =
      !!persistedLayout && ids.size > 0 && [...ids].every((id) => id in persistedLayout)

    let layout = !nodeSetUnchanged
    if (storedCoversCurrent && !appliedPersistedLayout.current) {
      layout = false
      appliedPersistedLayout.current = true
    }

    controller.sync(projection, { layout, maxLayoutIterations: 100 })
    if (controller.graph.order > 0) saveLayout(project, controller.exportPositions())
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [projection])

  useEffect(() => {
    return () => {
      if (controller.graph.order > 0) saveLayout(project, controller.exportPositions())
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [project])

  const revealNode = useCallback(
    (id: NodeId) => {
      setSelectedNodeId(id)
      if (nodeKindOf(id) === 'external') return
      const paths = ancestorsOf(id).map(nodePathOf).filter(Boolean)
      if (paths.length === 0) return
      onExpandedPathsChange((previous) => {
        const next = new Set(previous)
        for (const path of paths) next.add(path)
        return next
      })
      for (const path of paths) void onLoadDir(path)
    },
    [onExpandedPathsChange, onLoadDir],
  )

  const handleRelayout = useCallback(() => controller.applyLayout(300), [controller])

  const handleNodeMove = useCallback(
    (id: NodeId, x: number, y: number) => {
      controller.setNodePosition(id, x, y)
      const positions = controller.exportPositions()
      if (Object.keys(positions).length > 0) saveLayout(project, positions)
    },
    [controller, project],
  )

  const handleExpandAll = useCallback(
    (id: NodeId) => {
      const paths = subtreeDirIds(tree, nodePathOf(id)).map(nodePathOf)
      if (paths.length === 0) return
      onExpandedPathsChange((previous) => new Set([...previous, ...paths]))
      for (const path of paths) void onLoadDir(path)
    },
    [tree, onExpandedPathsChange, onLoadDir],
  )

  const handleCollapseAll = useCallback(
    (id: NodeId) => {
      const paths = new Set(subtreeDirIds(tree, nodePathOf(id)).map(nodePathOf))
      onExpandedPathsChange((previous) => new Set([...previous].filter((path) => !paths.has(path))))
    },
    [tree, onExpandedPathsChange],
  )

  const nodeCount = projection.nodes.length
  const edgeCount = projection.edges.length

  return (
    <div className="page relative flex h-full min-h-0 flex-col overflow-hidden">
      {error && (
        <div className="error-banner m-2 flex items-center justify-between rounded-md border border-red-200 bg-red-50 px-3.5 py-2.5 text-sm text-red-700">
          {error}
        </div>
      )}
      <header className="flex shrink-0 items-center justify-between gap-3 border-b border-border px-4 py-3">
        <div className="flex items-baseline gap-3">
          <h1 className="text-2xl font-bold">Graph</h1>
          {nodeCount > 0 && (
            <span className="text-xs text-muted-foreground">
              {nodeCount} nodes · {edgeCount} edges
            </span>
          )}
        </div>
        <p className="hidden text-xs text-muted-foreground md:block">
          Files folder expansion controls what the graph shows.
        </p>
      </header>
      <main className="min-h-0 flex-1">
        {projection.nodes.length === 0 ? (
          <div className="flex h-full items-center justify-center text-sm text-muted-foreground">
            No files to graph yet.
          </div>
        ) : (
          <GraphView
            graph={controller.graph}
            project={project}
            selectedNodeId={selectedNodeId}
            onNodeSelect={revealNode}
            onRelayout={handleRelayout}
            onNodeMove={handleNodeMove}
            onOpenFile={onOpenFile}
            onExpandAll={handleExpandAll}
            onCollapseAll={handleCollapseAll}
          />
        )}
      </main>
    </div>
  )
}
