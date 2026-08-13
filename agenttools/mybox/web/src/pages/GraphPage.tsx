import { useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import ForceGraph2D, { ForceGraphMethods } from 'react-force-graph-2d'
import { GraphData, api } from '../api/client'
import { encodePath, projectUrl, taskIdOf } from '../utils/routes'
import { installLabelCollision } from '../utils/graphCollision'

interface Node extends Object {
  id: string
  label: string
  type?: string
}

interface PositionedNode extends Node {
  x?: number
  y?: number
}

interface Link {
  source: string | Node
  target: string | Node
}

const ROOT_ID = ''

export function GraphPage() {
  const [data, setData] = useState<GraphData | null>(null)
  const [error, setError] = useState<string | null>(null)
  const navigate = useNavigate()
  const fgRef = useRef<ForceGraphMethods<Node> | undefined>(undefined)

  useEffect(() => {
    void api
      .getGraph()
      .then(setData)
      .catch((e) => setError(e instanceof Error ? e.message : String(e)))
  }, [])

  const graphData = useMemo(() => {
    if (!data) return null
    const nodes = data.nodes.map((n) => ({ ...n })) as Node[]
    const links: Link[] = (data.links ?? []).map((l) => ({ ...l }))
    return { nodes, links }
  }, [data])

  useEffect(() => {
    if (fgRef.current) {
      installLabelCollision(fgRef.current, {
        nodeRadius: 16,
        dirCharWidth: 4,
        dirPadding: 16,
      })
      fgRef.current.d3ReheatSimulation()
    }
  }, [graphData])

  useEffect(() => {
    if (!graphData) return
    const fg = fgRef.current
    if (!fg) return
    const t = window.setTimeout(() => {
      const root = graphData.nodes.find((n) => n.id === ROOT_ID) as PositionedNode | undefined
      if (root && typeof root.x === 'number' && typeof root.y === 'number') {
        fg.centerAt(root.x, root.y, 400)
      }
    }, 500)
    return () => window.clearTimeout(t)
  }, [graphData])

  if (error) return <div className="page error-banner">{error}</div>
  if (!data) return <div className="page">Loading…</div>
  if (!graphData) return null

  return (
    <div className="page">
      <h1>Graph</h1>
      {data.nodes.length === 0 ? (
        <p className="muted">No files to graph yet.</p>
      ) : (
        <div className="graph-container">
          <ForceGraph2D
            ref={fgRef}
            graphData={graphData}
            nodeLabel={(n) => (n as Node).label}
            nodeColor={(n) => ((n as Node).type === 'task' ? '#4a6' : '#4a7fd4')}
            nodeCanvasObjectMode={() => 'replace'}
            nodeCanvasObject={(n, ctx) => {
              const node = n as PositionedNode & { x: number; y: number }
              const isRoot = node.id === ROOT_ID
              if (node.type === 'dir') {
                const h = isRoot ? 30 : 24
                ctx.font = isRoot ? '8px sans-serif' : '7px sans-serif'
                const w = ctx.measureText(node.label).width + (isRoot ? 26 : 16)
                ctx.beginPath()
                ctx.rect(node.x - w / 2, node.y - h / 2, w, h)
                ctx.fillStyle = isRoot ? 'rgba(74, 127, 212, 0.16)' : 'rgba(74, 127, 212, 0.06)'
                ctx.fill()
                ctx.strokeStyle = isRoot ? '#2e5f9e' : '#8a9bb0'
                ctx.lineWidth = isRoot ? 2 : 1.2
                ctx.stroke()
                ctx.fillStyle = isRoot ? '#2e5f9e' : '#333'
                ctx.textAlign = 'center'
                ctx.fillText(node.label, node.x, node.y + (isRoot ? 3 : 2.5))
                return
              }
              const size = 3.5
              ctx.beginPath()
              ctx.arc(node.x, node.y, size, 0, 2 * Math.PI)
              ctx.fillStyle = node.type === 'task' ? '#4a6' : '#4a7fd4'
              ctx.fill()
              const label = node.label
              ctx.font = '7px sans-serif'
              ctx.fillStyle = '#333'
              ctx.textAlign = 'center'
              ctx.fillText(label, node.x, node.y - 8)
            }}
            onNodeClick={(n) => {
              const node = n as Node
              if (node.type === 'dir')
                navigate(projectUrl(`/dashboard/files/${encodePath(node.id)}`))
              else if (node.type === 'task')
                navigate(projectUrl(`/dashboard/files/tasks/${taskIdOf(node.id)}/task.md`))
              else if (node.type === 'file')
                navigate(projectUrl(`/dashboard/files/${encodePath(node.id)}`))
              else
                navigate(
                  projectUrl(
                    `/dashboard/files/${encodePath(node.id.endsWith('.md') ? node.id : `${node.id}.md`)}`,
                  ),
                )
            }}
            linkColor={() => '#ccc'}
          />
        </div>
      )}
    </div>
  )
}
