import type { ForceGraphMethods } from 'react-force-graph-2d'

export interface CollisionOptions {
  nodeRadius: number
  dirCharWidth: number
  dirPadding: number
}

interface PositionedNode {
  x?: number
  y?: number
  label?: string
  type?: string | null
}

export function installLabelCollision<N extends PositionedNode>(
  fg: ForceGraphMethods<N>,
  opts: CollisionOptions,
): void {
  const radius = (n: PositionedNode) => {
    if (n.type === 'dir') {
      const textWidth = (n.label ?? '').length * opts.dirCharWidth
      return textWidth / 2 + opts.dirPadding / 2 + 2
    }
    return opts.nodeRadius
  }
  let nodes: Array<PositionedNode & { x?: number; y?: number }> = []
  const force = (alpha: number) => {
    const n = nodes.length
    for (let i = 0; i < n; i++) {
      const a = nodes[i]
      if (a.x == null || a.y == null) continue
      const ra = radius(a)
      for (let j = i + 1; j < n; j++) {
        const b = nodes[j]
        if (b.x == null || b.y == null) continue
        const rb = radius(b)
        const min = ra + rb
        if (min <= 0) continue
        let dx = b.x - a.x
        let dy = b.y - a.y
        let d2 = dx * dx + dy * dy
        if (d2 < 1e-4) {
          d2 = 1e-4
          dx = 1
          dy = 0
        }
        const d = Math.sqrt(d2)
        if (d < min) {
          const v = ((min - d) / d) * alpha
          const cx = dx * v
          const cy = dy * v
          a.x -= cx
          a.y -= cy
          b.x += cx
          b.y += cy
        }
      }
    }
  }
  force.initialize = (ns: Array<PositionedNode & { x?: number; y?: number }>) => {
    nodes = ns
  }
  fg.d3Force('collide', force)
}
