import { describe, expect, it } from 'vitest'
import { installLabelCollision } from './graphCollision'

interface MockNode {
  x?: number
  y?: number
  label?: string
  type?: string | null
}

function simulate(nodes: MockNode[], opts: Parameters<typeof installLabelCollision>[1]) {
  let force: ((alpha: number) => void) & { initialize?: (n: MockNode[]) => void } | undefined
  const fg = {
    d3Force: (_name: string, f: (typeof force) | null) => {
      force = f ?? undefined
      return fg
    },
  }
  installLabelCollision(fg as never, opts)
  if (!force) throw new Error('no force installed')
  force.initialize?.(nodes)
  for (let tick = 0; tick < 500; tick++) {
    const alpha = Math.max(0.001, 1 - tick / 300)
    force(alpha)
  }
  return nodes
}

const minDist = (a: MockNode, b: MockNode) =>
  Math.hypot((a.x ?? 0) - (b.x ?? 0), (a.y ?? 0) - (b.y ?? 0))

describe('installLabelCollision', () => {
  it('keeps plain nodes apart by the configured node radius', () => {
    const nodes: MockNode[] = [
      { x: 0, y: 0, label: 'a' },
      { x: 1, y: 1, label: 'b' },
      { x: 2, y: 2, label: 'c' },
    ]
    simulate(nodes, { nodeRadius: 16, dirCharWidth: 4, dirPadding: 16 })
    expect(minDist(nodes[0], nodes[1])).toBeGreaterThanOrEqual(31)
    expect(minDist(nodes[0], nodes[2])).toBeGreaterThanOrEqual(31)
  })

  it('keeps directory nodes at frame-width separation from plain nodes', () => {
    const nodes: MockNode[] = [
      { x: 0, y: 0, label: 'knowledge', type: 'dir' },
      { x: 1, y: 0, label: 'note' },
    ]
    simulate(nodes, { nodeRadius: 16, dirCharWidth: 4, dirPadding: 16 })
    expect(minDist(nodes[0], nodes[1])).toBeGreaterThanOrEqual(42)
  })

  it('does not move nodes that are already far apart', () => {
    const nodes: MockNode[] = [
      { x: 0, y: 0, label: 'a' },
      { x: 500, y: 500, label: 'b' },
    ]
    simulate(nodes, { nodeRadius: 16, dirCharWidth: 4, dirPadding: 16 })
    expect(nodes[0].x).toBe(0)
    expect(nodes[0].y).toBe(0)
    expect(nodes[1].x).toBe(500)
    expect(nodes[1].y).toBe(500)
  })
})
