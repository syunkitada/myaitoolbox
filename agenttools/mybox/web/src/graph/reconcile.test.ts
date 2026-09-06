import { describe, expect, it } from 'vitest'
import { GraphController } from './reconcile'
import type { GraphProjection, NodeId } from './types'

function projectionFor(ids: NodeId[]): GraphProjection {
  return {
    nodes: ids.map((id) => ({ id, kind: 'file', label: id, path: `${id}.md` })),
    edges: [],
  }
}

function positionsOf(controller: GraphController): Record<NodeId, { x: number; y: number }> {
  const out: Record<NodeId, { x: number; y: number }> = {}
  const graph = controller.graph
  graph.forEachNode((id) => {
    out[id] = {
      x: graph.getNodeAttribute(id, 'x') as number,
      y: graph.getNodeAttribute(id, 'y') as number,
    }
  })
  return out
}

describe('GraphController persisted layout', () => {
  it('restores persisted positions exactly when layout is skipped', () => {
    const ids = ['a', 'b', 'c']
    const persisted: Record<NodeId, { x: number; y: number }> = {
      a: { x: 12, y: -3 },
      b: { x: -50, y: 40 },
      c: { x: 100, y: 7 },
    }
    const controller = new GraphController(persisted)
    controller.sync(projectionFor(ids), { layout: false })

    for (const id of ids) {
      const attrs = controller.graph.getNodeAttributes(id)
      expect(attrs.x).toBe(persisted[id].x)
      expect(attrs.y).toBe(persisted[id].y)
    }
  })

  it('export/import round-trips positions for a later layout-free sync', () => {
    const ids = ['x', 'y', 'z']
    const first = new GraphController()
    first.sync(projectionFor(ids), { layout: true, maxLayoutIterations: 100 })
    const exported = first.exportPositions()

    const second = new GraphController(exported)
    second.sync(projectionFor(ids), { layout: false })

    const firstPos = positionsOf(first)
    const secondPos = positionsOf(second)
    for (const id of ids) {
      expect(secondPos[id].x).toBeCloseTo(firstPos[id].x, 5)
      expect(secondPos[id].y).toBeCloseTo(firstPos[id].y, 5)
    }
  })

  it('keeps restored positions when additional nodes are synced', () => {
    const controller = new GraphController({ 'dir:docs': { x: 40, y: 40 }, 'file:doc.md': { x: 41, y: 41 } })
    controller.sync(projectionFor(['dir:docs', 'file:doc.md']), { layout: false })
    expect(controller.graph.getNodeAttribute('dir:docs', 'x')).toBe(40)
    expect(controller.graph.getNodeAttribute('dir:docs', 'y')).toBe(40)
    expect(controller.graph.getNodeAttribute('file:doc.md', 'x')).toBe(41)
    expect(controller.graph.getNodeAttribute('file:doc.md', 'y')).toBe(41)
  })

  it('restores a persisted fixed flag and keeps a fixed node in place', () => {
    const controller = new GraphController({ a: { x: 5, y: 5, fixed: true }, b: { x: -5, y: -5 } })
    controller.sync(projectionFor(['a', 'b']), { layout: true, maxLayoutIterations: 100 })
    expect(controller.graph.getNodeAttribute('a', 'fixed')).toBe(true)
    expect(controller.graph.getNodeAttribute('a', 'x')).toBeCloseTo(5, 5)
    expect(controller.graph.getNodeAttribute('a', 'y')).toBeCloseTo(5, 5)
    expect(controller.exportPositions()['a'].fixed).toBe(true)
  })
})