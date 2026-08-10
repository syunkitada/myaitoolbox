import { useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Task, TaskStatus, api } from '../api/client'
import { projectUrl } from '../utils/routes'
import { SearchBar } from '../components/SearchBar'

const statuses: Array<TaskStatus | ''> = ['', 'todo', 'doing', 'blocked', 'review', 'done']

export function TaskList() {
  const [tasks, setTasks] = useState<Task[]>([])
  const [status, setStatus] = useState<TaskStatus | ''>('')
  const [tag, setTag] = useState('')
  const [q, setQ] = useState('')
  const [error, setError] = useState<string | null>(null)
  const navigate = useNavigate()

  const load = () => {
    void api
      .listTasks({ status: status || undefined, tag: tag || undefined })
      .then(setTasks)
      .catch((e) => setError(e instanceof Error ? e.message : String(e)))
  }

  useEffect(load, [status, tag])

  const filtered = useMemo(() => {
    if (!q.trim()) return tasks
    const needle = q.trim().toLowerCase()
    return tasks.filter(
      (t) =>
        t.title.toLowerCase().includes(needle) ||
        (t.tags ?? []).some((x) => x.toLowerCase().includes(needle)),
    )
  }, [tasks, q])

  return (
    <div className="page">
      <div className="page-header">
        <h1>Tasks</h1>
        <button
          className="primary"
          onClick={() => {
            const name = window.prompt('New task name')
            if (name && name.trim()) {
              void api.createTask({ name: name.trim() }).then((t) => navigate(projectUrl(`/tasks/${t.id}`)))
            }
          }}
        >
          New task
        </button>
      </div>
      {error && <div className="error-banner">{error}</div>}
      <div className="toolbar">
        <SearchBar value={q} onChange={setQ} onSubmit={() => undefined} placeholder="Filter…" />
        <select value={status} onChange={(e) => setStatus(e.target.value as TaskStatus | '')}>
          {statuses.map((s) => (
            <option key={s} value={s}>
              {s === '' ? 'all statuses' : s}
            </option>
          ))}
        </select>
        <input
          value={tag}
          onChange={(e) => setTag(e.target.value)}
          placeholder="tag"
          aria-label="filter by tag"
        />
      </div>
      <ul className="task-list">
        {filtered.map((t) => (
          <li key={t.id} className="task-row">
            <button className="link-btn" onClick={() => navigate(projectUrl(`/tasks/${t.id}`))}>
              {t.title}
            </button>
            <span className={`badge status-${t.status}`}>{t.status}</span>
            <span className={`badge priority-${t.priority}`}>{t.priority}</span>
            {(t.tags ?? []).map((tg) => (
              <span key={tg} className="badge tag">
                {tg}
              </span>
            ))}
          </li>
        ))}
        {filtered.length === 0 && <li className="muted">No tasks.</li>}
      </ul>
    </div>
  )
}
