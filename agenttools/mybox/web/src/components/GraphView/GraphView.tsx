import { useEffect, useMemo, useRef, useState } from 'react'
import Sigma from 'sigma'
import type Graph from 'graphology'
import { Button } from '../ui/button'
import { ContextMenu } from '../ContextMenu'
import { nodeKindOf, nodePathOf } from '../../graph/nodeId'
import { loadCamera, saveCamera } from '../../state/graphViewState'
import type { GraphEdgeAttributes, GraphNodeAttributes, NodeId } from '../../graph/types'
import { cn } from '@/lib/utils'

type GraphWithAttrs = Graph<GraphNodeAttributes, GraphEdgeAttributes>

function createSigma(graph: GraphWithAttrs, container: HTMLElement) {
  return new Sigma(graph, container, {
    renderLabels: true,
    labelRenderedSizeThreshold: 4,
    labelDensity: 0.5,
    minCameraRatio: 0.05,
    maxCameraRatio: 12,
    enableEdgeEvents: true,
    labelColor: { attribute: 'labelColor' },
  })
}

type GraphSigma = ReturnType<typeof createSigma>

export interface GraphViewProps {
  graph: GraphWithAttrs
  project?: string
  selectedNodeId: NodeId | null
  onNodeSelect: (id: NodeId) => void
  onRelayout: () => void
  onOpenFile?: (path: string) => void
  onExpandAll?: (id: NodeId) => void
  onCollapseAll?: (id: NodeId) => void
}

interface HoverInfo {
  nodeId: NodeId
  x: number
  y: number
}

interface ContextMenuState {
  x: number
  y: number
  nodeId: NodeId
}

const KIND_LABELS: Record<string, string> = {
  directory: 'Directory',
  file: 'Markdown',
  external: 'Hidden Reference',
}

const LEGEND = [
  { color: '#3b82f6', label: 'Directory' },
  { color: '#0ea5e9', label: 'Markdown file' },
  { color: '#94a3b8', label: 'Hidden reference' },
  { color: '#f59e0b', label: 'Link' },
]

function nodeDisplayLabel(graph: GraphWithAttrs, id: NodeId): { label: string; path: string; kind: string } {
  const attrs = graph.getNodeAttributes(id) as Partial<GraphNodeAttributes>
  return {
    label: attrs.label ?? nodePathOf(id).split('/').pop() ?? id,
    path: attrs.path ?? nodePathOf(id),
    kind: attrs.kind ?? nodeKindOf(id) ?? '',
  }
}

export function GraphView({
  graph,
  project,
  selectedNodeId,
  onNodeSelect,
  onRelayout,
  onOpenFile,
  onExpandAll,
  onCollapseAll,
}: GraphViewProps) {
  const containerRef = useRef<HTMLDivElement | null>(null)
  const sigmaRef = useRef<GraphSigma | null>(null)
  const dragRef = useRef<{ node: NodeId } | null>(null)
  const [hover, setHover] = useState<HoverInfo | null>(null)
  const [dragging, setDragging] = useState(false)
  const [menu, setMenu] = useState<ContextMenuState | null>(null)

  useEffect(() => {
    const container = containerRef.current
    if (!container) return
    const sigma = createSigma(graph, container)
    sigmaRef.current = sigma

    const camera = sigma.getCamera()
    const savedView = project ? loadCamera(project) : null
    if (savedView) {
      camera.setState({ x: savedView.x, y: savedView.y, angle: savedView.angle, ratio: savedView.ratio })
    }

    const mouseCaptor = sigma.getMouseCaptor()

    sigma.on('clickNode', ({ node, preventSigmaDefault }) => {
      preventSigmaDefault()
      onNodeSelect(node)
    })
    sigma.on('doubleClickNode', ({ node, preventSigmaDefault }) => {
      preventSigmaDefault()
      const kind = nodeKindOf(node)
      if (kind === 'file' || kind === 'external') onOpenFile?.(nodePathOf(node))
    })
    sigma.on('rightClickNode', ({ node, event }) => {
      if (nodeKindOf(node) !== 'directory') return
      if (typeof event.original.preventDefault === 'function') event.original.preventDefault()
      const rect = container.getBoundingClientRect()
      setMenu({ x: rect.left + event.x, y: rect.top + event.y, nodeId: node })
    })
    sigma.on('clickStage', () => setMenu(null))
    sigma.on('downNode', ({ node, preventSigmaDefault }) => {
      preventSigmaDefault()
      dragRef.current = { node }
      setDragging(true)
    })
    mouseCaptor.on('mousemovebody', (event) => {
      const drag = dragRef.current
      if (!drag) return
      const pos = sigma.viewportToGraph({ x: event.x, y: event.y })
      graph.mergeNodeAttributes(drag.node, { x: pos.x, y: pos.y, fixed: true })
    })
    const stopDrag = () => {
      dragRef.current = null
      setDragging(false)
    }
    sigma.on('upNode', (e) => {
      e.preventSigmaDefault()
      stopDrag()
    })
    mouseCaptor.on('mouseup', stopDrag)

    sigma.on('enterNode', ({ node }) => {
      const data = sigma.getNodeDisplayData(node)
      if (!data) return
      const viewport = sigma.graphToViewport({ x: data.x, y: data.y })
      setHover({ nodeId: node, x: viewport.x, y: viewport.y })
    })
    sigma.on('leaveNode', () => setHover(null))
    sigma.on('leaveStage', () => setHover(null))

    const persistCamera = () => {
      if (!project) return
      const state = camera.getState()
      saveCamera(project, { x: state.x, y: state.y, angle: state.angle, ratio: state.ratio })
    }
    camera.on('updated', persistCamera)

    return () => {
      persistCamera()
      sigma.kill()
      sigmaRef.current = null
      setHover(null)
    }
  }, [graph]) // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    const sigma = sigmaRef.current
    if (!sigma) return

    const selected = selectedNodeId
    const neighborIds = new Set<NodeId>()
    if (selected && graph.hasNode(selected)) {
      graph.forEachOutboundEdge(selected, (_edge, _attrs, _source, target) => {
        neighborIds.add(target)
      })
      graph.forEachInboundEdge(selected, (_edge, _attrs, source, _target) => {
        neighborIds.add(source)
      })
    }

    const baseColor = (color?: unknown): string => (typeof color === 'string' ? color : '#0ea5e9')

    sigma.setSetting('nodeReducer', (node, data) => {
      const size = (data.size as number) ?? 9
      if (node === selected) {
        return { ...data, size: size * 1.6, color: '#e11d48', highlighted: true }
      }
      if (!selected) return { ...data }
      if (neighborIds.has(node)) {
        return { ...data, size, color: baseColor(data.color), highlighted: true }
      }
      return { ...data, size: Math.max(size * 0.7, 4), color: '#d3dce4' }
    })
    sigma.setSetting('edgeReducer', (edge, data) => {
      const touches = selected && graph.hasNode(selected) ? graph.hasExtremity(edge, selected) : false
      if (touches) {
        return { ...data, color: '#f59e0b', size: 2 }
      }
      return { ...data, color: selected ? '#dfe5ec' : '#d7dee6', size: selected ? 0.5 : 1 }
    })
    sigma.refresh()
  }, [selectedNodeId, graph])

  const hoverInfo = useMemo(() => {
    if (!hover || !graph.hasNode(hover.nodeId)) return null
    const { label, path, kind } = nodeDisplayLabel(graph, hover.nodeId)
    return { ...hover, label, path, kind }
  }, [hover, graph])

  const zoomIn = () => void sigmaRef.current?.getCamera().animatedZoom({ duration: 200 })
  const zoomOut = () => void sigmaRef.current?.getCamera().animatedUnzoom({ duration: 200 })
  const fitView = () => void sigmaRef.current?.getCamera().animatedReset({ duration: 300 })

  return (
    <div className="graph-view relative h-full w-full min-h-0 overflow-hidden">
      <div ref={containerRef} className="graph-canvas h-full w-full" />
      {menu && (
        <ContextMenu
          x={menu.x}
          y={menu.y}
          onClose={() => setMenu(null)}
          items={[
            { label: 'Expand all', onSelect: () => onExpandAll?.(menu.nodeId) },
            { label: 'Collapse all', onSelect: () => onCollapseAll?.(menu.nodeId) },
          ]}
        />
      )}
      <div className="absolute top-2 right-3 z-10 flex items-center gap-1.5 rounded-lg border bg-card/95 px-2 py-1.5 shadow-sm">
        <Button variant="ghost" size="sm" className="h-8 cursor-pointer px-2" onClick={zoomIn} aria-label="Zoom in" title="Zoom in">
          +
        </Button>
        <Button variant="ghost" size="sm" className="h-8 cursor-pointer px-2" onClick={zoomOut} aria-label="Zoom out" title="Zoom out">
          −
        </Button>
        <Button variant="ghost" size="sm" className="h-8 cursor-pointer px-2" onClick={fitView} aria-label="Fit graph" title="Fit graph">
          ⌖
        </Button>
        <Button variant="ghost" size="sm" className="h-8 cursor-pointer px-2" onClick={onRelayout} aria-label="Re-layout graph" title="Re-layout graph">
          ⟳
        </Button>
      </div>
      <div className="pointer-events-none absolute bottom-3 left-3 z-10 flex flex-col gap-1 rounded-md border bg-card/95 px-3 py-2 text-xs shadow-sm">
        {LEGEND.map((item) => (
          <span key={item.label} className="flex items-center gap-2 text-muted-foreground">
            <span className="inline-block size-2.5 rounded-full" style={{ backgroundColor: item.color }} />
            {item.label}
          </span>
        ))}
      </div>
      {hoverInfo && graph.hasNode(hoverInfo.nodeId) && (
        <div
          className="pointer-events-none absolute z-20 max-w-64 rounded-md border bg-popover px-3 py-2 text-xs text-popover-foreground shadow-md"
          style={{
            left: hoverInfo.x + 12,
            top: hoverInfo.y - 10,
          }}
        >
          <div className={cn('font-semibold')}>
            {KIND_LABELS[hoverInfo.kind] ?? hoverInfo.kind} · {hoverInfo.label}
          </div>
          <div className="mt-0.5 truncate text-muted-foreground" title={hoverInfo.path}>
            {hoverInfo.path}
          </div>
        </div>
      )}
      {dragging && (
        <div className="pointer-events-none absolute right-3 bottom-3 rounded-md border bg-card/95 px-3 py-1.5 text-xs text-muted-foreground shadow-sm">
          Dragging node — position is kept.
        </div>
      )}
    </div>
  )
}