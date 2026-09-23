import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { api } from '../api/client'
import { ExplorerStateProvider, useExplorerState } from '../state/explorerState'
import { ExplorerTree } from '../components/Explorer/ExplorerTree'
import { GraphView } from '../components/GraphView/GraphView'
import { GraphController } from '../graph/reconcile'
import { buildFileTree, allFilePaths, isMarkdownFile, subtreeDirIds } from '../graph/tree'
import { buildProjection } from '../graph/projection'
import { nodeKindOf, nodePathOf } from '../graph/nodeId'
import { encodePath, projectUrl } from '../utils/routes'
import { loadLayout, saveLayout } from '../state/graphViewState'
import type { NodeId, TreeInputEntry } from '../graph/types'

function setsEqual(a: Set<string>, b: Set<string>): boolean {
  if (a.size !== b.size) return false
  for (const v of a) if (!b.has(v)) return false
  return true
}

function KnowledgeGraphBody() {
  const { expandedNodeIds, selectedNodeId, toggleNode, selectNode, revealNode, expandDirs, collapseDirs } =
    useExplorerState()
  const navigate = useNavigate()
  const { project } = useParams()
  const projectKey = project ?? ''

  const [treeInput, setTreeInput] = useState<TreeInputEntry[]>([])
  const [contents, setContents] = useState<Map<string, string>>(new Map())
  const [error, setError] = useState<string | null>(null)
  const inFlight = useRef<Set<string>>(new Set())
  const controllerRef = useRef<GraphController | null>(null)
  const prevNodeSet = useRef<Set<string> | null>(null)
  const appliedPersistedLayout = useRef(false)
  if (!controllerRef.current) controllerRef.current = new GraphController(loadLayout(projectKey))
  const controller = controllerRef.current

  const tree = useMemo(() => buildFileTree(treeInput), [treeInput])

  useEffect(() => {
    void api
      .listFiles()
      .then((list) => setTreeInput(list.map((e) => ({ path: e.path, kind: e.kind }))))
      .catch((e) => setError(e instanceof Error ? e.message : String(e)))
  }, [])

  const linkContext = useMemo(() => {
    const knownMarkdown = new Set(allFilePaths(tree).filter(isMarkdownFile))
    return { knownMarkdown }
  }, [tree])

  const explorerState = useMemo(
    () => ({ expandedNodeIds, selectedNodeId }),
    [expandedNodeIds, selectedNodeId],
  )

  const projection = useMemo(
    () => buildProjection({ tree, state: explorerState, contents, linkContext }),
    [tree, explorerState, contents, linkContext],
  )

  useEffect(() => {
    const visible = projection.nodes.filter((n) => n.kind === 'file').map((n) => n.path)
    const missing = visible.filter((p) => !contents.has(p) && !inFlight.current.has(p))
    if (missing.length === 0) return
    for (const p of missing) {
      void api
        .getFileContent(p)
        .then((res) => {
          setContents((prev) => {
            const next = new Map(prev)
            next.set(p, res.content)
            return next
          })
        })
        .catch(() => undefined)
        .finally(() => inFlight.current.delete(p))
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [projection])

  useEffect(() => {
    const ids = new Set(projection.nodes.map((n) => n.id))
    const nodeSetUnchanged = prevNodeSet.current !== null && setsEqual(prevNodeSet.current, ids)
    prevNodeSet.current = ids

    const persistedLayout = loadLayout(projectKey)
    const storedCoversCurrent =
      !!persistedLayout &&
      ids.size > 0 &&
      [...ids].every((id) => id in persistedLayout)

    let layout = !nodeSetUnchanged
    if (storedCoversCurrent && !appliedPersistedLayout.current) {
      layout = false
      appliedPersistedLayout.current = true
    }

    controller.sync(projection, { layout, maxLayoutIterations: 100 })
    if (controller.graph.order > 0) {
      saveLayout(projectKey, controller.exportPositions())
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [projection])

  useEffect(() => {
    return () => {
      if (controller.graph.order > 0) saveLayout(projectKey, controller.exportPositions())
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [projectKey])

  const handleGraphSelect = useCallback(
    (id: NodeId) => {
      const kind = nodeKindOf(id)
      if (kind === 'external') selectNode(id)
      else revealNode(id)
    },
    [selectNode, revealNode],
  )

  const handleRelayout = useCallback(() => {
    controller.applyLayout(300)
  }, [controller])

  const handleNodeMove = useCallback(
    (id: NodeId, x: number, y: number) => {
      controller.setNodePosition(id, x, y)
      const positions = controller.exportPositions()
      if (Object.keys(positions).length > 0) saveLayout(projectKey, positions)
    },
    [controller, projectKey],
  )

  const handleExplorerSelect = useCallback(
    (id: NodeId) => {
      selectNode(id)
    },
    [selectNode],
  )

  const openFile = useCallback(
    (path: string) => {
      navigate(projectUrl(`/dashboard/files/${encodePath(path)}`))
    },
    [navigate],
  )

  const handleExpandAll = useCallback(
    (id: NodeId) => {
      const dirs = subtreeDirIds(tree, nodePathOf(id))
      if (dirs.length > 0) expandDirs(dirs)
    },
    [tree, expandDirs],
  )

  const handleCollapseAll = useCallback(
    (id: NodeId) => {
      const dirs = subtreeDirIds(tree, nodePathOf(id))
      if (dirs.length > 0) collapseDirs(dirs)
    },
    [tree, collapseDirs],
  )

  const nodeCount = projection.nodes.length
  const edgeCount = projection.edges.length

  return (
    <div className="page relative h-full min-h-0 overflow-hidden">
      {error && (
        <div className="error-banner m-2 flex items-center justify-between rounded-md border border-red-200 bg-red-50 px-3.5 py-2.5 text-sm text-red-700">
          {error}
        </div>
      )}
      <div className="flex h-full min-h-0 flex-col">
        <header className="flex items-center justify-between gap-3 border-b border-border px-4 py-3">
          <div className="flex items-baseline gap-3">
            <h1 className="text-2xl font-bold">Graph</h1>
            {nodeCount > 0 && (
              <span className="text-xs text-muted-foreground">
                {nodeCount} nodes · {edgeCount} edges
              </span>
            )}
          </div>
          <p className="hidden text-xs text-muted-foreground md:block">
            Explorer expansion state controls what the graph shows.
          </p>
        </header>
        <div className="flex min-h-0 flex-1 overflow-hidden">
          <aside className="explorer-pane hidden w-[260px] shrink-0 overflow-hidden border-r border-border max-md:hidden lg:block">
            <ExplorerTree
              tree={tree}
              expandedNodeIds={expandedNodeIds}
              selectedNodeId={selectedNodeId}
              onToggle={toggleNode}
              onSelect={handleExplorerSelect}
              onOpenFile={openFile}
              onExpandAllDir={handleExpandAll}
              onCollapseAllDir={handleCollapseAll}
            />
          </aside>
          <main className="min-w-0 flex-1">
            {projection.nodes.length === 0 ? (
              <div className="flex h-full items-center justify-center text-sm text-muted-foreground">
                No files to graph yet.
              </div>
            ) : (
              <GraphView
                graph={controller.graph}
                project={projectKey}
                selectedNodeId={selectedNodeId}
                onNodeSelect={handleGraphSelect}
                onRelayout={handleRelayout}
                onNodeMove={handleNodeMove}
                onOpenFile={openFile}
                onExpandAll={handleExpandAll}
                onCollapseAll={handleCollapseAll}
              />
            )}
          </main>
        </div>
      </div>
    </div>
  )
}

export function KnowledgeGraphPage() {
  const { project } = useParams()
  return (
    <ExplorerStateProvider project={project}>
      <KnowledgeGraphBody />
    </ExplorerStateProvider>
  )
}
