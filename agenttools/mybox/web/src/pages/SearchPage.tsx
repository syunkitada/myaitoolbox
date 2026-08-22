import { useEffect, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { SearchResult, api } from '../api/client'
import { encodePath, projectUrl } from '../utils/routes'
import { SearchBar } from '../components/SearchBar'
import { Button } from '../components/ui/button'
import { StatusBadge } from '../components/badges'

interface SearchPageProps {
  navigate: (path: string) => void
}

export function SearchPage({ navigate }: SearchPageProps) {
  const [params, setParams] = useSearchParams()
  const q = params.get('q') ?? ''
  const [draft, setDraft] = useState(q)
  const [results, setResults] = useState<SearchResult[]>([])
  const [type, setType] = useState<'task' | 'knowledge' | ''>('')
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    setDraft(q)
  }, [q])

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
    const trimmed = query.trim()
    if (trimmed) setParams({ q: trimmed })
  }

  return (
    <div className="page p-4 md:p-6">
      <h1 className="text-2xl font-bold">Search</h1>
      <div className="toolbar my-3 flex flex-wrap gap-2">
        <SearchBar value={draft} onChange={setDraft} onSubmit={submit} autoFocus />
        <select
          value={type}
          onChange={(e) => setType(e.target.value as typeof type)}
          className="h-9 rounded-md border border-input bg-card px-3 text-sm text-foreground transition-[color,box-shadow] outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50"
        >
          <option value="">all</option>
          <option value="task">tasks</option>
          <option value="knowledge">knowledge</option>
        </select>
      </div>
      {loading && <p className="muted text-sm text-muted-foreground">Searching…</p>}
      <ul className="search-results m-0 flex list-none flex-col gap-2 p-0">
        {results.map((r, i) => (
          <li key={`${r.type}-${r.path}-${i}`} className="card search-result rounded-lg border bg-card p-3">
            <Button
              variant="link"
              size="xs"
              className="h-auto p-0"
              onClick={() => {
                if (r.type === 'task')
                  navigate(projectUrl(`/dashboard/files/tasks/${encodePath(r.id ?? r.path)}/task.md`))
                else navigate(projectUrl(`/dashboard/files/knowledge/${encodePath(r.path)}.md`))
              }}
            >
              {r.title}
            </Button>
            <StatusBadge status={r.type} />
            <div className="muted text-sm text-muted-foreground">{r.path}</div>
            {r.snippet && (
              <div className="snippet mt-1 text-[13px] whitespace-pre-wrap text-muted-foreground">{r.snippet}</div>
            )}
          </li>
        ))}
        {!loading && q.trim() && results.length === 0 && (
          <li className="muted text-muted-foreground">No results for “{q}”.</li>
        )}
      </ul>
    </div>
  )
}
