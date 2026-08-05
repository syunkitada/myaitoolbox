import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import ForceGraph2D from 'react-force-graph-2d'
import { GraphData, api } from '../api/client'

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

  useEffect(() => {
    void api
      .getGraph()
      .then(setData)
      .catch((e) => setError(e instanceof Error ? e.message : String(e)))
  }, [])

  if (error) return <div className="page error-banner">{error}</div>
  if (!data) return <div className="page">Loading…</div>

  const nodes = data.nodes.map((n) => ({ ...n }))
  const links: Link[] = (data.links ?? []).map((l) => ({ ...l }))

  return (
    <div className="page">
      <h1>Graph</h1>
      {data.nodes.length === 0 ? (
        <p className="muted">No knowledge to graph yet.</p>
      ) : (
        <div className="graph-container">
          <ForceGraph2D
            graphData={{ nodes: nodes as Node[], links }}
            nodeLabel={(n) => (n as Node).label}
            nodeColor={(n) => ((n as Node).type === 'task' ? '#4a6' : '#4a7fd4')}
            nodeCanvasObjectMode={() => 'replace'}
            nodeCanvasObject={(n, ctx) => {
              const node = n as Node & { x: number; y: number }
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
              if (node.type === 'task') navigate(`/tasks/${node.id}`)
              else navigate(`/knowledge/${encodeURIComponent(node.id)}`)
            }}
            linkColor={() => '#ccc'}
          />
        </div>
      )}
    </div>
  )
}
