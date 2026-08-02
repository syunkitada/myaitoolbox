import { useEffect, useState } from 'react'
import { Meta, api, Task } from '../api/client'

interface DashboardProps {
  meta: Meta | null
  navigate: (path: string) => void
}

export function Dashboard({ meta, navigate }: DashboardProps) {
  const [tasks, setTasks] = useState<Task[]>([])

  useEffect(() => {
    void api.listTasks().then(setTasks).catch(() => setTasks([]))
  }, [])

  const open = tasks.filter((t) => t.status !== 'done')
  const done = tasks.filter((t) => t.status === 'done')

  return (
    <div className="page">
      <h1>Dashboard</h1>
      {meta && (
        <p>
          Project: <strong>{meta.project}</strong> ({meta.projects.join(', ')})
        </p>
      )}
      <div className="stats">
        <div className="stat">
          <span className="stat-value">{open.length}</span>
          <span className="stat-label">open tasks</span>
        </div>
        <div className="stat">
          <span className="stat-value">{done.length}</span>
          <span className="stat-label">done</span>
        </div>
        <div className="stat">
          <span className="stat-value">{(meta?.tags ?? []).length}</span>
          <span className="stat-label">tags</span>
        </div>
        <div className="stat">
          <span className="stat-value">{(meta?.favorites ?? []).length}</span>
          <span className="stat-label">favorites</span>
        </div>
      </div>
      <div className="card">
        <h2>Open tasks</h2>
        <ul className="task-list">
          {open.slice(0, 10).map((t) => (
            <li key={t.id}>
              <button className="link-btn" onClick={() => navigate(`/tasks/${t.id}`)}>
                {t.title}
              </button>
              <span className={`badge status-${t.status}`}>{t.status}</span>
            </li>
          ))}
          {open.length === 0 && <li className="muted">No open tasks.</li>}
        </ul>
      </div>
      {meta && meta.recent_files != null && meta.recent_files.length > 0 && (
        <div className="card">
          <h2>Recent</h2>
          <ul className="task-list">
            {meta.recent_files.slice(0, 10).map((p) => (
              <li key={p}>
                <button className="link-btn" onClick={() => navigate(`/knowledge/${encodeURIComponent(p)}`)}>
                  {p}
                </button>
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  )
}
