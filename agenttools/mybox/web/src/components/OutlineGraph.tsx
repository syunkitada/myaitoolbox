import { useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import ForceGraph2D, { ForceGraphMethods } from 'react-force-graph-2d'
import { GraphData, api } from '../api/client'
import { encodePath, projectUrl } from '../utils/routes'
import { installLabelCollision } from '../utils/graphCollision'

interface GraphNode {
  id: string
  label: string
  type?: string | null
}

interface GraphLink {
  source: string | GraphNode
  target: string | GraphNode
}

function linkEndpoints(l: GraphLink): [string, string] {
  const s = typeof l.source === 'string' ? l.source : l.source.id
  const t = typeof l.target === 'string' ? l.target : l.target.id
  return [s, t]
}

export function OutlineGraph({
  path,
  root,
  onNodeClick,
}: {
  path: string
  root?: string
  onNodeClick?: (node: GraphNode) => void
}) {
  const navigate = useNavigate()
  const [data, setData] = useState<GraphData | null>(null)
  const containerRef = useRef<HTMLDivElement>(null)
  const [size, setSize] = useState<{ w: number; h: number }>({ w: 300, h: 220 })

  useEffect(() => {
    let cancelled = false
    void api
      .getGraph(root)
      .then((d) => {
        if (!cancelled) setData(d)
      })
      .catch(() => undefined)
    return () => {
      cancelled = true
    }
  }, [path, root])

  useEffect(() => {
    const el = containerRef.current
    if (!el) return
    const update = () => {
      if (el.clientWidth > 0) {
        setSize({ w: el.clientWidth, h: el.clientHeight || 220 })
      }
    }
    update()
    const ro = new ResizeObserver(update)
    ro.observe(el)
    return () => ro.disconnect()
  }, [])

  const graphData = useMemo(() => {
    if (!data) return null
    const nodes: GraphNode[] = data.nodes.map((n) => ({ ...n }))
    const links: GraphLink[] = (data.links ?? []).map((l) => ({ ...l }))
    const current = nodeIdOf(path, root)
    if (nodes.some((n) => n.id === current)) {
      const linked = new Set<string>([current])
      for (const l of links) {
        const [s, t] = linkEndpoints(l)
        if (s === current) linked.add(t)
        if (t === current) linked.add(s)
      }
      const subNodes = nodes.filter((n) => linked.has(n.id))
      const subLinks = links.filter((l) => {
        const [s, t] = linkEndpoints(l)
        return linked.has(s) && linked.has(t)
      })
      return { nodes: subNodes, links: subLinks, linked, current }
    }
    const linked = new Set<string>([current])
    for (const l of links) {
      const [s, t] = linkEndpoints(l)
      if (s === current) linked.add(t)
      if (t === current) linked.add(s)
    }
    return { nodes, links, linked, current }
  }, [data, path, root])

  const fgRef = useRef<ForceGraphMethods<any, any> | undefined>(undefined)

  useEffect(() => {
    if (fgRef.current) {
      installLabelCollision(fgRef.current, {
        nodeRadius: 12,
        dirCharWidth: 3,
        dirPadding: 12,
      })
      fgRef.current.d3ReheatSimulation()
    }
  }, [graphData])

  useEffect(() => {
    if (!graphData) return
    const fg = fgRef.current
    if (!fg) return
    const t = window.setTimeout(() => {
      const nodes = graphData.nodes as Array<{ id: string; x?: number; y?: number }>
      const cur = nodes.find((n) => n.id === graphData.current)
      if (cur && typeof cur.x === 'number' && typeof cur.y === 'number') {
        fg.centerAt(cur.x, cur.y, 400)
        fg.zoom(1.6, 400)
      }
    }, 500)
    return () => window.clearTimeout(t)
  }, [graphData])

  if (!graphData) return <div className="outline-graph" />

  const handleClick =
    onNodeClick ??
    ((n: GraphNode) => {
      if (n.type === 'task') navigate(projectUrl(`/tasks/${n.id}`))
      else if (n.type === 'dir') navigate(projectUrl(`/dashboard/files/${encodePath(n.id)}`))
      else {
        const id = n.id.endsWith('.md') ? n.id : `${n.id}.md`
        navigate(projectUrl(`/dashboard/files/${encodePath(id)}`))
      }
    })

  return (
    <div className="outline-graph" ref={containerRef}>
      <ForceGraph2D
        ref={fgRef}
        graphData={{ nodes: graphData.nodes, links: graphData.links }}
        width={size.w}
        height={size.h}
        nodeLabel={(n) => (n as GraphNode).label}
        nodeCanvasObjectMode={() => 'replace'}
        nodeCanvasObject={(n, ctx) => {
          const node = n as GraphNode & { x: number; y: number }
          const isCurrent = node.id === graphData.current
          const isLinked = graphData.linked.has(node.id)
          if (node.type === 'dir') {
            const h = 18
            ctx.font = '5px sans-serif'
            const w = ctx.measureText(node.label).width + 12
            ctx.beginPath()
            ctx.rect(node.x - w / 2, node.y - h / 2, w, h)
            ctx.fillStyle = isCurrent ? 'rgba(224, 83, 61, 0.08)' : 'rgba(74, 127, 212, 0.08)'
            ctx.fill()
            ctx.strokeStyle = isCurrent ? '#e0533d' : '#8a9bb0'
            ctx.lineWidth = 1
            ctx.stroke()
            if (isCurrent || isLinked) {
              ctx.fillStyle = '#6b7684'
              ctx.textAlign = 'center'
              ctx.fillText(node.label, node.x, node.y + 3)
            }
            return
          }
          const size = isCurrent ? 5 : isLinked ? 3.5 : 1.5
          ctx.beginPath()
          ctx.arc(node.x, node.y, size, 0, 2 * Math.PI)
          ctx.fillStyle = isCurrent ? '#e0533d' : isLinked ? '#4a7fd4' : '#d8dee9'
          ctx.fill()
          if (isCurrent || isLinked) {
            ctx.font = '5px sans-serif'
            ctx.fillStyle = '#6b7684'
            ctx.textAlign = 'center'
            ctx.fillText(node.label, node.x, node.y - size - 2)
          }
        }}
        linkColor={(l) => {
          const [s, t] = linkEndpoints(l as unknown as GraphLink)
          return s === graphData.current || t === graphData.current ? '#9ab8d8' : '#e5e9f0'
        }}
        onNodeClick={(n) => handleClick(n as GraphNode)}
      />
    </div>
  )
}

function nodeIdOf(path: string, root?: string): string {
  const stripped = path.replace(/\.md$/i, '')
  return root && stripped && !stripped.startsWith(root + '/')
    ? `${root}/${stripped}`
    : stripped
}
