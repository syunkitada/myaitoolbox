import { useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import ForceGraph2D from 'react-force-graph-2d'
import { GraphData, api } from '../api/client'

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

export function OutlineGraph({ path }: { path: string }) {
  const navigate = useNavigate()
  const [data, setData] = useState<GraphData | null>(null)
  const containerRef = useRef<HTMLDivElement>(null)
  const [size, setSize] = useState<{ w: number; h: number }>({ w: 300, h: 220 })

  useEffect(() => {
    let cancelled = false
    void api
      .getGraph()
      .then((d) => {
        if (!cancelled) setData(d)
      })
      .catch(() => undefined)
    return () => {
      cancelled = true
    }
  }, [path])

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
    const linked = new Set<string>([path])
    for (const l of links) {
      const [s, t] = linkEndpoints(l)
      if (s === path) linked.add(t)
      if (t === path) linked.add(s)
    }
    return { nodes, links, linked }
  }, [data, path])

  if (!graphData) return <div className="outline-graph" />

  return (
    <div className="outline-graph" ref={containerRef}>
      <ForceGraph2D
        graphData={{ nodes: graphData.nodes, links: graphData.links }}
        width={size.w}
        height={size.h}
        nodeLabel={(n) => (n as GraphNode).label}
        nodeCanvasObjectMode={() => 'replace'}
        nodeCanvasObject={(n, ctx) => {
          const node = n as GraphNode & { x: number; y: number }
          const isCurrent = node.id === path
          const isLinked = graphData.linked.has(node.id)
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
          return s === path || t === path ? '#9ab8d8' : '#e5e9f0'
        }}
        onNodeClick={(n) => {
          const node = n as GraphNode
          if (node.type === 'task') navigate(`/tasks/${node.id}`)
          else navigate(`/knowledge/${encodeURIComponent(node.id)}`)
        }}
      />
    </div>
  )
}
