import { useEffect, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { SearchResult, api } from '../api/client'
import { encodePath, projectUrl } from '../utils/routes'
import { SearchBar } from '../components/SearchBar'

interface SearchPageProps {
  navigate: (path: string) => void
}

export function SearchPage({ navigate }: SearchPageProps) {
  const [params, setParams] = useSearchParams()
  const q = params.get('q') ?? ''
  const [results, setResults] = useState<SearchResult[]>([])
  const [type, setType] = useState<'task' | 'knowledge' | ''>('')
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    if (!q.trim()) {
      setResults([])
      return
    }
    setLoading(true)
    void api
      .search(q.trim(), type || undefined)
      .then(setResults)
      .finally(() => setLoading(false))
  }, [q, type])

  const submit = (query: string) => {
    setParams({ q: query })
  }

  return (
    <div className="page">
      <h1>Search</h1>
      <div className="toolbar">
        <SearchBar value={q} onChange={() => undefined} onSubmit={submit} autoFocus />
        <select value={type} onChange={(e) => setType(e.target.value as typeof type)}>
          <option value="">all</option>
          <option value="task">tasks</option>
          <option value="knowledge">knowledge</option>
        </select>
      </div>
      {loading && <p className="muted">Searching…</p>}
      <ul className="search-results">
        {results.map((r, i) => (
          <li key={`${r.type}-${r.path}-${i}`} className="card search-result">
            <button
              className="link-btn"
              onClick={() => {
                if (r.type === 'task') navigate(projectUrl(`/tasks/${r.id ?? r.path}`))
                else navigate(projectUrl(`/knowledge/${encodePath(r.path)}`))
              }}
            >
              {r.title}
            </button>
            <span className={`badge status-${r.type}`}>{r.type}</span>
            <div className="muted">{r.path}</div>
            {r.snippet && <div className="snippet">{r.snippet}</div>}
          </li>
        ))}
        {!loading && q.trim() && results.length === 0 && (
          <li className="muted">No results for “{q}”.</li>
        )}
      </ul>
    </div>
  )
}
