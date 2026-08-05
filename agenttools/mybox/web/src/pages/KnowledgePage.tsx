import { ReactNode, useEffect, useMemo, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { Knowledge, api } from '../api/client'
import { SearchBar } from '../components/SearchBar'
import { RichMarkdown, extractOutline } from '../components/RichMarkdown'
import { OutlineGraph } from '../components/OutlineGraph'
import { extractWikiLinks, normalizePath } from '../utils/markdown'

interface KnowledgePageProps {
  refreshMeta: () => Promise<void>
  favorites: string[]
}

interface TreeFile {
  kind: 'file'
  name: string
  path: string
}

interface TreeDir {
  kind: 'dir'
  name: string
  dirPath: string
  children: TreeNode[]
}

type TreeNode = TreeFile | TreeDir

function buildTree(list: Knowledge[]): TreeNode[] {
  const root: TreeNode[] = []
  const sorted = [...list].sort((a, b) => a.path.localeCompare(b.path))
  for (const k of sorted) {
    const parts = k.path.split('/')
    let cur = root
    let prefix = ''
    for (let i = 0; i < parts.length - 1; i++) {
      const part = parts[i]
      prefix = prefix ? `${prefix}/${part}` : part
      let dir = cur.find((n): n is TreeDir => n.kind === 'dir' && n.name === part)
      if (!dir) {
        dir = { kind: 'dir', name: part, dirPath: prefix, children: [] }
        cur.push(dir)
      }
      cur = dir.children
    }
    cur.push({ kind: 'file', name: parts[parts.length - 1], path: k.path })
  }
  return root
}

interface ExplorerProps {
  list: Knowledge[]
  selected: string
  onSelect: (path: string) => void
  onNew: () => void
  onClose: () => void
}

function Explorer({ list, selected, onSelect, onNew, onClose }: ExplorerProps) {
  const [q, setQ] = useState('')
  const [tag, setTag] = useState('')
  const [collapsed, setCollapsed] = useState<Set<string>>(new Set())

  const allTags = useMemo(
    () => Array.from(new Set(list.flatMap((k) => k.tags ?? []))).sort(),
    [list],
  )

  const filtering = q.trim() !== '' || tag !== ''

  const filtered = useMemo(() => {
    const needle = q.trim().toLowerCase()
    return list.filter(
      (k) =>
        (tag === '' || (k.tags ?? []).includes(tag)) &&
        (!needle ||
          k.path.toLowerCase().includes(needle) ||
          k.title.toLowerCase().includes(needle) ||
          (k.tags ?? []).some((t) => t.toLowerCase().includes(needle))),
    )
  }, [list, q, tag])

  const tree = useMemo(() => buildTree(list), [list])

  const toggle = (dirPath: string) => {
    setCollapsed((prev) => {
      const next = new Set(prev)
      if (next.has(dirPath)) next.delete(dirPath)
      else next.add(dirPath)
      return next
    })
  }

  const items: ReactNode[] = []
  const renderNodes = (nodes: TreeNode[], depth: number) => {
    for (const node of nodes) {
      const pad = depth * 14
      if (node.kind === 'dir') {
        const open = !collapsed.has(node.dirPath)
        items.push(
          <li
            key={`dir:${node.dirPath}`}
            className="knowledge-tree-row"
            style={{ paddingLeft: pad }}
          >
            <button
              className="knowledge-caret"
              aria-label={open ? `Collapse ${node.name}` : `Expand ${node.name}`}
              onClick={() => toggle(node.dirPath)}
            >
              {open ? '▾' : '▸'}
            </button>
            <span className="knowledge-dir-name">{node.name}</span>
          </li>,
        )
        if (open) renderNodes(node.children, depth + 1)
      } else {
        items.push(
          <li
            key={`file:${node.path}`}
            className="knowledge-tree-row"
            style={{ paddingLeft: pad + 20 }}
          >
            <button
              className={`knowledge-file${node.path === selected ? ' active' : ''}`}
              onClick={() => onSelect(node.path)}
            >
              {node.name}
            </button>
          </li>,
        )
      }
    }
  }
  renderNodes(tree, 0)

  return (
    <aside className="knowledge-explorer">
      <div className="page-header">
        <button className="ghost mobile-close" onClick={onClose} aria-label="Close explorer">
          ← Back
        </button>
        <h1>Knowledge</h1>
        <button className="primary" onClick={onNew}>
          New
        </button>
      </div>
      <div className="toolbar">
        <SearchBar value={q} onChange={setQ} onSubmit={() => undefined} placeholder="Filter…" />
        <select value={tag} onChange={(e) => setTag(e.target.value)} aria-label="Filter by tag">
          <option value="">all tags</option>
          {allTags.map((t) => (
            <option key={t} value={t}>
              {t}
            </option>
          ))}
        </select>
      </div>
      {list.length === 0 ? (
        <p className="muted">No knowledge yet.</p>
      ) : filtering ? (
        <ul className="file-list">
          {filtered.map((k) => (
            <li key={k.path}>
              <button className="link-btn" onClick={() => onSelect(k.path)}>
                {k.path}
              </button>
            </li>
          ))}
          {filtered.length === 0 && <li className="muted">No matches.</li>}
        </ul>
      ) : (
        <ul className="knowledge-tree">{items}</ul>
      )}
    </aside>
  )
}

interface NotePaneProps {
  path: string
  list: Knowledge[]
  favorites: string[]
  refreshMeta: () => Promise<void>
  onChanged: () => void
}

function NotePane({ path, list, favorites, refreshMeta, onChanged }: NotePaneProps) {
  const navigate = useNavigate()
  const [content, setContent] = useState('')
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

  const byAlias = useMemo(() => {
    const m = new Map<string, string>()
    for (const k of list) {
      if (k.title) m.set(normalizePath(k.title), k.path)
      for (const a of k.aliases ?? []) m.set(normalizePath(a), k.path)
    }
    return m
  }, [list])

  const byBasename = useMemo(() => {
    const m = new Map<string, string>()
    for (const k of list) {
      const base = normalizePath(k.path.split('/').pop() ?? '')
      if (base && !m.has(base)) m.set(base, k.path)
    }
    return m
  }, [list])

  const pathOf = (target: string) =>
    byPath.get(normalizePath(target))?.path ??
    byAlias.get(normalizePath(target)) ??
    byBasename.get(normalizePath(target)) ??
    null

  const backlinks = useMemo(
    () => list.filter((k) => (k.wiki_links ?? []).some((l) => pathOf(l) === path)),
    [list, path],
  )

  const load = () => {
    setError(null)
    void api
      .getKnowledgeContent(path)
      .then((c) => {
        setContent(c.content)
        setDraft(c.content)
      })
      .catch((e) => setError(e instanceof Error ? e.message : String(e)))
    const cur = list.find((k) => k.path === path)
    if (cur) setTitle(cur.title)
    void api.recordRecent(path).then(() => void refreshMeta()).catch(() => undefined)
  }

  useEffect(() => {
    load()
    setEditing(false)
    setSaved(false)
    setIsFav(favorites.includes(path))
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [path])

  const fav = (enabled: boolean) => {
    void api
      .setFavorite(path, enabled)
      .then(() => {
        setIsFav(enabled)
        void refreshMeta()
      })
      .catch((e) => setError(e instanceof Error ? e.message : String(e)))
  }

  const save = () => {
    setSaved(false)
    void api
      .saveKnowledgeContent(path, draft)
      .then(() => {
        setContent(draft)
        setEditing(false)
        setSaved(true)
        onChanged()
        setTimeout(() => setSaved(false), 2000)
      })
      .catch((e) => setError(e instanceof Error ? e.message : String(e)))
  }

  const rename = () => {
    const name = window.prompt('New title', title)
    if (name && name.trim()) {
      void api.renameKnowledge(path, name.trim()).then(() => onChanged())
    }
  }

  const move = () => {
    const newPath = window.prompt('New path', path)
    if (newPath && newPath.trim() && newPath.trim() !== path) {
      void api
        .moveKnowledge(path, newPath.trim())
        .then(() => {
          onChanged()
          navigate(`/knowledge/${encodeURIComponent(newPath.trim())}`)
        })
        .catch((e) => setError(e instanceof Error ? e.message : String(e)))
    }
  }

  const links = useMemo(() => extractWikiLinks(content), [content])
  const tags = list.find((k) => k.path === path)?.tags ?? []

  return (
    <div>
      <div className="knowledge-body">
        <div className="knowledge-main">
          <div className="page-header note-toolbar">
            <button className="ghost mobile-back" onClick={() => navigate('/knowledge')}>
              ← Files
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
              <h1>{title || path.split('/').pop()}</h1>
              <div className="meta-line">
                <span className="muted">{path}</span>
                {(list.find((k) => k.path === path)?.tags ?? []).map((t) => (
                  <span key={t} className="badge tag">
                    {t}
                  </span>
                ))}
              </div>
              <RichMarkdown text={content} pathOf={pathOf} relativeTo={path} />
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
        {!editing && (
          <aside className="outline">
            <div className="outline-section">
              <div className="outline-title">Content</div>
              {extractOutline(content).map((h, i) => (
                <a
                  key={i}
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
            </div>
            <div className="outline-section">
              <div className="outline-title">Tags</div>
              {tags.length === 0 ? (
                <span className="muted outline-none">No tags</span>
              ) : (
                <div className="outline-tags">
                  {tags.map((t) => (
                    <span key={t} className="badge tag">
                      {t}
                    </span>
                  ))}
                </div>
              )}
            </div>
            <div className="outline-section">
              <div className="outline-title">Graph</div>
              <OutlineGraph path={path} />
            </div>
          </aside>
        )}
      </div>
    </div>
  )
}

export function KnowledgePage({ refreshMeta, favorites }: KnowledgePageProps) {
  const params = useParams()
  const selected = (params['*'] ?? '').trim()
  const navigate = useNavigate()
  const [list, setList] = useState<Knowledge[]>([])
  const [error, setError] = useState<string | null>(null)
  const [lastPath, setLastPath] = useState<string | null>(null)

  useEffect(() => {
    if (selected) setLastPath(selected)
  }, [selected])

  const load = () => {
    void api
      .listKnowledge()
      .then(setList)
      .catch((e) => setError(e instanceof Error ? e.message : String(e)))
  }

  useEffect(load, [])

  const handleNew = () => {
    const path = window.prompt('New knowledge path (e.g. notes/foo)')
    if (path && path.trim()) {
      void api
        .createKnowledge(path.trim())
        .then((k) => {
          load()
          navigate(`/knowledge/${encodeURIComponent(k.path)}`)
        })
        .catch((e) => setError(e instanceof Error ? e.message : String(e)))
    }
  }

  const handleSelect = (path: string) => {
    navigate(`/knowledge/${encodeURIComponent(path)}`)
  }

  const handleClose = () => {
    if (lastPath) navigate(`/knowledge/${encodeURIComponent(lastPath)}`)
    else navigate('/')
  }

  return (
    <div className="page">
      <div className={`knowledge-layout${selected ? ' has-selection' : ''}`}>
        <Explorer
          list={list}
          selected={selected}
          onSelect={handleSelect}
          onNew={handleNew}
          onClose={handleClose}
        />
        <div className="knowledge-pane">
          {error && <div className="error-banner">{error}</div>}
          {selected ? (
            <NotePane
              path={selected}
              list={list}
              favorites={favorites}
              refreshMeta={refreshMeta}
              onChanged={load}
            />
          ) : (
            <div className="card knowledge-empty">
              <h2>Knowledge</h2>
              <p className="muted">
                Select a note from the explorer to view it here.
              </p>
              {list.length === 0 && <p className="muted">No knowledge yet — create your first note.</p>}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
