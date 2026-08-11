import { useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import ForceGraph2D, { ForceGraphMethods } from 'react-force-graph-2d'
import { GraphData, api } from '../api/client'
import { encodePath, projectUrl } from '../utils/routes'
import { installLabelCollision } from '../utils/graphCollision'

interface Node extends Object {
  id: string
  label: string
  type?: string
}

interface Link {
  source: string | Node
  target: string | Node
}

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

  useEffect(() => {
    if (fgRef.current) {
      installLabelCollision(fgRef.current, {
        nodeRadius: 16,
        dirCharWidth: 4,
        dirPadding: 16,
      })
      fgRef.current.d3ReheatSimulation()
    }
  }, [data])

  if (error) return <div className="page error-banner">{error}</div>
  if (!data) return <div className="page">Loading…</div>

  const nodes = data.nodes.map((n) => ({ ...n }))
  const links: Link[] = (data.links ?? []).map((l) => ({ ...l }))

  return (
    <div className="page">
      <h1>Graph</h1>
      {data.nodes.length === 0 ? (
        <p className="muted">No files to graph yet.</p>
      ) : (
        <div className="graph-container">
          <ForceGraph2D
            ref={fgRef}
            graphData={{ nodes: nodes as Node[], links }}
            nodeLabel={(n) => (n as Node).label}
            nodeColor={(n) => ((n as Node).type === 'task' ? '#4a6' : '#4a7fd4')}
            nodeCanvasObjectMode={() => 'replace'}
            nodeCanvasObject={(n, ctx) => {
              const node = n as Node & { x: number; y: number }
              if (node.type === 'dir') {
                const h = 24
                ctx.font = '7px sans-serif'
                const w = ctx.measureText(node.label).width + 16
                ctx.beginPath()
                ctx.rect(node.x - w / 2, node.y - h / 2, w, h)
                ctx.fillStyle = 'rgba(74, 127, 212, 0.06)'
                ctx.fill()
                ctx.strokeStyle = '#8a9bb0'
                ctx.lineWidth = 1.2
                ctx.stroke()
                ctx.fillStyle = '#333'
                ctx.textAlign = 'center'
                ctx.fillText(node.label, node.x, node.y + 2.5)
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
              else if (node.type === 'task') navigate(projectUrl(`/tasks/${node.id}`))
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
