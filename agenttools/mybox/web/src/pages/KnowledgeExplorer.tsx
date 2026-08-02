import { useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Knowledge, api } from '../api/client'
import { SearchBar } from '../components/SearchBar'

export function KnowledgeExplorer() {
  const [list, setList] = useState<Knowledge[]>([])
  const [tag, setTag] = useState('')
  const [q, setQ] = useState('')
  const [error, setError] = useState<string | null>(null)
  const navigate = useNavigate()

  const load = () => {
    void api
      .listKnowledge(tag ? { tag } : {})
      .then(setList)
      .catch((e) => setError(e instanceof Error ? e.message : String(e)))
  }

  useEffect(load, [tag])

  const allTags = useMemo(
    () => Array.from(new Set(list.flatMap((k) => k.tags ?? []))).sort(),
    [list],
  )

  const filtered = useMemo(() => {
    if (!q.trim()) return list
    const needle = q.trim().toLowerCase()
    return list.filter(
      (k) =>
        k.path.toLowerCase().includes(needle) ||
        k.title.toLowerCase().includes(needle) ||
        (k.tags ?? []).some((t) => t.toLowerCase().includes(needle)),
    )
  }, [list, q])

  return (
    <div className="page">
      <div className="page-header">
        <h1>Knowledge</h1>
        <button
          className="primary"
          onClick={() => {
            const path = window.prompt('New knowledge path (e.g. notes/foo)')
            if (path && path.trim()) {
              void api.createKnowledge(path.trim()).then((k) => {
                navigate(`/knowledge/${encodeURIComponent(k.path)}`)
              })
            }
          }}
        >
          New note
        </button>
      </div>
      {error && <div className="error-banner">{error}</div>}
      <div className="toolbar">
        <SearchBar value={q} onChange={setQ} onSubmit={() => undefined} placeholder="Filter…" />
        <select value={tag} onChange={(e) => setTag(e.target.value)}>
          <option value="">all tags</option>
          {allTags.map((t) => (
            <option key={t} value={t}>
              {t}
            </option>
          ))}
        </select>
      </div>
      <ul className="file-list">
        {filtered.map((k) => (
          <li key={k.path}>
            <button className="link-btn" onClick={() => navigate(`/knowledge/${encodeURIComponent(k.path)}`)}>
              {k.path}
            </button>
            <span className="muted">{k.title}</span>
            {(k.tags ?? []).map((t) => (
              <span key={t} className="badge tag">
                {t}
              </span>
            ))}
          </li>
        ))}
        {filtered.length === 0 && <li className="muted">No knowledge.</li>}
      </ul>
    </div>
  )
}
