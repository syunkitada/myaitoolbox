import { useEffect, useMemo, useState } from 'react'
import { useParams } from 'react-router-dom'
import { Knowledge, api } from '../api/client'
import { RichMarkdown, extractOutline } from '../components/RichMarkdown'
import { extractWikiLinks, normalizePath } from '../utils/markdown'

interface KnowledgeViewProps {
  navigate: (path: string) => void
  refreshMeta: () => Promise<void>
  favorites: string[]
}

export function KnowledgeView({ navigate, refreshMeta, favorites }: KnowledgeViewProps) {
  const params = useParams()
  const path = (params['*'] ?? '').trim()

  const [list, setList] = useState<Knowledge[]>([])
  const [content, setContent] = useState<string>('')
  const [title, setTitle] = useState('')
  const [editing, setEditing] = useState(false)
  const [draft, setDraft] = useState('')
  const [isFav, setIsFav] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [saved, setSaved] = useState(false)

  const byPath = useMemo(() => {
    const m = new Map<string, Knowledge>()
    for (const k of list) m.set(normalizePath(k.path), k)
    return m
  }, [list])

  const pathOf = (target: string) => byPath.get(normalizePath(target))?.path ?? null

  const backlinks = useMemo(
    () => list.filter((k) => (k.wiki_links ?? []).some((l) => normalizePath(l) === normalizePath(path))),
    [list, path],
  )

  const load = () => {
    if (!path) return
    setError(null)
    void api
      .listKnowledge()
      .then((l) => {
        setList(l)
        const cur = l.find((k) => k.path === path)
        if (cur) setTitle(cur.title)
      })
      .catch((e) => setError(e instanceof Error ? e.message : String(e)))
    void api
      .getKnowledgeContent(path)
      .then((c) => {
        setContent(c.content)
        setDraft(c.content)
      })
      .catch((e) => setError(e instanceof Error ? e.message : String(e)))
    void api.recordRecent(path).then(() => void refreshMeta()).catch(() => undefined)
  }

  useEffect(() => {
    load()
    setIsFav(favorites.includes(path))
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [path])

  const fav = (enabled: boolean) => {
    if (!path) return
    void api
      .setFavorite(path, enabled)
      .then(() => {
        setIsFav(enabled)
        void refreshMeta()
      })
      .catch((e) => setError(e instanceof Error ? e.message : String(e)))
  }

  const save = () => {
    if (!path) return
    setSaved(false)
    void api
      .saveKnowledgeContent(path, draft)
      .then(() => {
        setContent(draft)
        setEditing(false)
        setSaved(true)
        load()
        setTimeout(() => setSaved(false), 2000)
      })
      .catch((e) => setError(e instanceof Error ? e.message : String(e)))
  }

  const rename = () => {
    if (!path) return
    const name = window.prompt('New title', title)
    if (name && name.trim()) {
      void api.renameKnowledge(path, name.trim()).then(() => load())
    }
  }

  const move = () => {
    if (!path) return
    const newPath = window.prompt('New path', path)
    if (newPath && newPath.trim() && newPath.trim() !== path) {
      void api.moveKnowledge(path, newPath.trim()).then(() => navigate(`/knowledge/${encodeURIComponent(newPath.trim())}`))
    }
  }

  const links = useMemo(() => extractWikiLinks(content), [content])

  if (!path) {
    return (
      <div className="page">
        <h1>Knowledge</h1>
        <p className="muted">Select a note from the explorer, or open one from Favorites / Recent.</p>
      </div>
    )
  }

  return (
    <div className="page">
      <div className="page-header">
        <button className="ghost" onClick={() => navigate('/knowledge')}>
          ← Explorer
        </button>
        <div className="actions">
          <button className={isFav ? 'ghost active' : 'ghost'} onClick={() => fav(!isFav)}>
            {isFav ? '★ Favorite' : '☆ Favorite'}
          </button>
          <button className="ghost" onClick={rename}>
            Rename
          </button>
          <button className="ghost" onClick={move}>
            Move
          </button>
          {!editing && (
            <button className="primary" onClick={() => setEditing(true)}>
              Edit
            </button>
          )}
        </div>
      </div>
      {error && <div className="error-banner">{error}</div>}
      {saved && <div className="notice">Saved.</div>}
      {editing ? (
        <div className="card">
          <div className="actions">
            <button className="primary" onClick={save}>
              Save
            </button>
            <button className="ghost" onClick={() => setDraft(content)}>
              Cancel
            </button>
          </div>
          <textarea
            className="editor"
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            aria-label="Markdown editor"
          />
        </div>
      ) : (
        <div className="card knowledge-view">
          <div className="knowledge-body">
            <div className="knowledge-main">
              <h1>{title || path.split('/').pop()}</h1>
              <div className="meta-line">
                <span className="muted">{path}</span>
                {(list.find((k) => k.path === path)?.tags ?? []).map((t) => (
                  <span key={t} className="badge tag">
                    {t}
                  </span>
                ))}
              </div>
              <RichMarkdown text={content} pathOf={pathOf} />
            </div>
            <aside className="outline">
              <div className="outline-title">Outline</div>
              {extractOutline(content).map((h) => (
                <a
                  key={h.id}
                  href={`#${h.id}`}
                  className={`outline-link level-${h.level}`}
                  onClick={(e) => {
                    e.preventDefault()
                    document.getElementById(h.id)?.scrollIntoView({ behavior: 'smooth' })
                  }}
                >
                  {h.text}
                </a>
              ))}
            </aside>
          </div>
          {links.length > 0 && (
            <div className="card-section">
              <h3>Links</h3>
              <ul>
                {links.map((l) => {
                  const resolved = pathOf(l.target)
                  return (
                    <li key={l.raw}>
                      {resolved ? (
                        <button
                          className="link-btn"
                          onClick={() => navigate(`/knowledge/${encodeURIComponent(resolved)}`)}
                        >
                          {l.alias ?? l.target}
                        </button>
                      ) : (
                        <span className="broken-link">{l.alias ?? l.target}</span>
                      )}
                    </li>
                  )
                })}
              </ul>
            </div>
          )}
          {backlinks.length > 0 && (
            <div className="card-section">
              <h3>Backlinks</h3>
              <ul>
                {backlinks.map((b) => (
                  <li key={b.path}>
                    <button
                      className="link-btn"
                      onClick={() => navigate(`/knowledge/${encodeURIComponent(b.path)}`)}
                    >
                      {b.path}
                    </button>
                  </li>
                ))}
              </ul>
            </div>
          )}
        </div>
      )}
    </div>
  )
}
