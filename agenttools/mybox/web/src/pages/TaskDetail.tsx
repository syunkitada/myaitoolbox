import { FormEvent, useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import { Task, TaskPriority, TaskStatus, api } from '../api/client'
import { projectUrl } from '../utils/routes'
import { Markdown } from '../components/Markdown'

const statuses: TaskStatus[] = ['todo', 'doing', 'blocked', 'review', 'done']
const priorities: TaskPriority[] = ['low', 'medium', 'high', 'urgent']

interface TaskDetailProps {
  navigate: (path: string) => void
}

export function TaskDetail({ navigate }: TaskDetailProps) {
  const { id } = useParams()
  const [task, setTask] = useState<Task | null>(null)
  const [editing, setEditing] = useState(false)
  const [form, setForm] = useState({
    name: '',
    status: 'todo' as TaskStatus,
    priority: 'medium' as TaskPriority,
    assignee: '',
    due: '',
    tags: '',
  })
  const [error, setError] = useState<string | null>(null)

  const load = () => {
    if (!id) return
    void api.getTask(id).then((t) => {
      setTask(t)
      setForm({
        name: t.title,
        status: t.status,
        priority: t.priority,
        assignee: t.assignee ?? '',
        due: t.due ?? '',
        tags: (t.tags ?? []).join(', '),
      })
    })
  }

  useEffect(load, [id])

  if (!task) return <div className="page">Loading…</div>

  const submit = (e: FormEvent) => {
    e.preventDefault()
    if (!id) return
    void api
      .updateTask(id, {
        name: form.name.trim(),
        status: form.status,
        priority: form.priority,
        assignee: form.assignee.trim() || undefined,
        due: form.due || undefined,
        tags: form.tags
          .split(',')
          .map((s) => s.trim())
          .filter(Boolean),
      })
      .then(() => {
        setEditing(false)
        load()
      })
      .catch((err) => setError(err instanceof Error ? err.message : String(err)))
  }

  const archive = () => {
    if (!id) return
    void api.archiveTask(id).then(() => navigate(projectUrl('/tasks')))
  }

  return (
    <div className="page">
      <div className="page-header">
        <button className="ghost" onClick={() => navigate(projectUrl('/tasks'))}>
          ← Tasks
        </button>
        {!editing && (
          <div className="actions">
            <button className="ghost" onClick={() => setEditing(true)}>
              Edit
            </button>
            {!task.archived && (
              <button className="ghost" onClick={archive}>
                Archive
              </button>
            )}
          </div>
        )}
      </div>
      {error && <div className="error-banner">{error}</div>}
      {editing ? (
        <form className="card form" onSubmit={submit}>
          <label>
            Name
            <input
              value={form.name}
              onChange={(e) => setForm({ ...form, name: e.target.value })}
              required
            />
          </label>
          <div className="form-row">
            <label>
              Status
              <select
                value={form.status}
                onChange={(e) => setForm({ ...form, status: e.target.value as TaskStatus })}
              >
                {statuses.map((s) => (
                  <option key={s} value={s}>
                    {s}
                  </option>
                ))}
              </select>
            </label>
            <label>
              Priority
              <select
                value={form.priority}
                onChange={(e) => setForm({ ...form, priority: e.target.value as TaskPriority })}
              >
                {priorities.map((p) => (
                  <option key={p} value={p}>
                    {p}
                  </option>
                ))}
              </select>
            </label>
          </div>
          <div className="form-row">
            <label>
              Assignee
              <input
                value={form.assignee}
                onChange={(e) => setForm({ ...form, assignee: e.target.value })}
              />
            </label>
            <label>
              Due
              <input
                type="date"
                value={form.due}
                onChange={(e) => setForm({ ...form, due: e.target.value })}
              />
            </label>
          </div>
          <label>
            Tags (comma separated)
            <input value={form.tags} onChange={(e) => setForm({ ...form, tags: e.target.value })} />
          </label>
          <div className="actions">
            <button className="primary" type="submit">
              Save
            </button>
            <button type="button" className="ghost" onClick={() => setEditing(false)}>
              Cancel
            </button>
          </div>
        </form>
      ) : (
        <div className="card">
          <h1>{task.title}</h1>
          <div className="meta-line">
            <span className={`badge status-${task.status}`}>{task.status}</span>
            <span className={`badge priority-${task.priority}`}>{task.priority}</span>
            {task.assignee && <span className="muted">assignee: {task.assignee}</span>}
            {task.due && <span className="muted">due: {task.due}</span>}
          </div>
          {(task.tags ?? []).map((t) => (
            <span key={t} className="badge tag">
              {t}
            </span>
          ))}
          {task.body && (
            <div className="markdown-body">
              <Markdown text={task.body} />
            </div>
          )}
        </div>
      )}
    </div>
  )
}
