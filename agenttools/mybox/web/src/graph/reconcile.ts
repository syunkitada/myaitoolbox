import { DirectedGraph } from 'graphology'
import forceLayout from 'graphology-layout-force'
import type {
  GraphEdgeAttributes,
  GraphEdgeKind,
  GraphNodeAttributes,
  GraphNodeKind,
  GraphProjection,
  NodeId,
} from './types'
import type { PersistedPosition } from '../state/graphViewState'
import { dirId, nodePathOf } from './nodeId'

type EdgeAttributes = GraphEdgeAttributes

const EDGE_SEP = '\u0000'

function edgeKey(source: NodeId, target: NodeId, kind: GraphEdgeKind): string {
  return `${source}${EDGE_SEP}${target}${EDGE_SEP}${kind}`
}

export function nodeStyle(
  kind: GraphNodeKind,
  label: string,
  path: string,
  selected: boolean,
  related: boolean,
): GraphNodeAttributes {
  const baseColor =
    kind === 'directory' ? '#3b82f6' : kind === 'external' ? '#94a3b8' : '#0ea5e9'
  const size = kind === 'directory' ? 16 : kind === 'external' ? 8 : 9
  return {
    kind,
    label,
    path,
    size: selected ? size * 1.6 : size,
    color: selected ? '#e11d48' : related ? baseColor : baseColor,
    labelColor: selected ? '#be123c' : kind === 'directory' ? '#1d4ed8' : '#334155',
  }
}

/**
 * Owns the single graphology instance shared by GraphView. Mutations are
 * applied as diffs so existing node positions (and the sigma renderer) stay
 * stable across Explorer expansion changes.
 */
export class GraphController {
  graph: DirectedGraph<GraphNodeAttributes, EdgeAttributes>

  private positions = new Map<NodeId, PersistedPosition>()

  constructor(initial?: Record<NodeId, PersistedPosition> | null) {
    this.graph = new DirectedGraph<GraphNodeAttributes, EdgeAttributes>()
    if (initial) {
      for (const [id, pos] of Object.entries(initial)) {
        if (typeof pos.x === 'number' && typeof pos.y === 'number') {
          this.positions.set(id, { x: pos.x, y: pos.y, fixed: pos.fixed === true })
        }
      }
    }
  }

  savePositions(): void {
    this.graph.forEachNode((node) => {
      this.positions.set(node, {
        x: Number(this.graph.getNodeAttribute(node, 'x')) || 0,
        y: Number(this.graph.getNodeAttribute(node, 'y')) || 0,
        fixed: this.graph.getNodeAttribute(node, 'fixed') === true,
      })
    })
  }

  restorePositions(): void {
    this.graph.forEachNode((node) => {
      const pos = this.positions.get(node)
      if (!pos) return
      this.graph.setNodeAttribute(node, 'x', pos.x)
      this.graph.setNodeAttribute(node, 'y', pos.y)
      if (pos.fixed) this.graph.setNodeAttribute(node, 'fixed', true)
    })
  }

  exportPositions(): Record<NodeId, PersistedPosition> {
    const out: Record<NodeId, PersistedPosition> = {}
    for (const [id, pos] of this.positions) {
      out[id] = { x: pos.x, y: pos.y, fixed: pos.fixed === true }
    }
    return out
  }

  private nextSeedPosition(id: NodeId): { x: number; y: number } {
    const path = nodePathOf(id)
    const parts = path.split('/').filter(Boolean)
    parts.pop()
    let anchor: { x: number; y: number } | null = null
    for (let i = parts.length - 1; i >= 0; i--) {
      const candidate = dirId(parts.slice(0, i + 1).join('/'))
      const pos = this.positions.get(candidate)
      if (pos) {
        anchor = pos
        break
      }
    }
    if (!anchor) anchor = this.positions.get(dirId('')) ?? { x: 0, y: 0 }
    const jitter = () => (Math.random() - 0.5) * 60
    return { x: anchor.x + jitter(), y: anchor.y + jitter() }
  }

  sync(projection: GraphProjection, opts?: { layout?: boolean; maxLayoutIterations?: number }): void {
    const graph = this.graph
    const next = new Set(projection.nodes.map((n) => n.id))

    for (const node of graph.nodes()) {
      if (!next.has(node)) {
        graph.dropNode(node)
        this.positions.delete(node)
      }
    }

    for (const n of projection.nodes) {
      if (!graph.hasNode(n.id)) {
        const saved = this.positions.get(n.id)
        const seed = saved ?? this.nextSeedPosition(n.id)
        graph.addNode(n.id, {
          ...nodeStyle(n.kind, n.label, n.path, false, false),
          fixed: false,
          x: seed.x,
          y: seed.y,
        })
        this.positions.set(n.id, seed)
      } else {
        graph.mergeNodeAttributes(n.id, nodeStyle(n.kind, n.label, n.path, false, false))
      }
    }

    const desiredEdges = new Set(projection.edges.map((e) => edgeKey(e.source, e.target, e.kind)))
    for (const edge of graph.edges()) {
      if (!desiredEdges.has(edge)) graph.dropEdge(edge)
    }
    for (const e of projection.edges) {
      const key = edgeKey(e.source, e.target, e.kind)
      if (!graph.hasEdge(key)) {
        if (!graph.hasNode(e.source) || !graph.hasNode(e.target)) continue
        graph.addDirectedEdgeWithKey(key, e.source, e.target, { kind: e.kind })
      }
    }

    if (opts?.layout ?? true) {
      this.applyLayout(opts?.maxLayoutIterations ?? 120)
    } else {
      this.restorePositions()
    }
  }

  applyLayout(maxIterations = 120): void {
    const graph = this.graph
    this.restorePositions()
    forceLayout.assign(graph, {
      maxIterations,
      settings: {
        attraction: 0.0005,
        repulsion: 0.1,
        gravity: 0.0001,
        inertia: 0.6,
        maxMove: 200,
      },
      isNodeFixed: (node) => graph.getNodeAttribute(node, 'fixed') === true,
    })
    this.savePositions()
  }

  setNodePosition(id: NodeId, x: number, y: number): void {
    if (!this.graph.hasNode(id)) return
    this.graph.mergeNodeAttributes(id, { x, y, fixed: true })
    this.positions.set(id, { x, y, fixed: true })
  }
}