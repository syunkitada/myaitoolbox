import { ReactNode, useCallback, useEffect, useMemo, useState } from 'react'
import { api, FileEntry } from '../api/client'
import { RichMarkdown, extractOutline } from '../components/RichMarkdown'
import { SearchBar } from '../components/SearchBar'

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

function buildTree(list: FileEntry[]): TreeNode[] {
  const root: TreeNode[] = []
  const sorted = [...list].sort((a, b) => {
    if (a.kind !== b.kind) return a.kind === 'dir' ? -1 : 1
    return a.name.localeCompare(b.name)
  })
  for (const e of sorted) {
    const parts = e.path.split('/')
    let cur = root
    let prefix = ''
    for (let i = 0; i < parts.length - (e.kind === 'dir' ? 0 : 1); i++) {
      const part = parts[i]
      prefix = prefix ? `${prefix}/${part}` : part
      let dir = cur.find((n): n is TreeDir => n.kind === 'dir' && n.name === part)
      if (!dir) {
        dir = { kind: 'dir', name: part, dirPath: prefix, children: [] }
        cur.push(dir)
      }
      cur = dir.children
    }
    if (e.kind === 'file') {
      cur.push({ kind: 'file', name: parts[parts.length - 1], path: e.path })
    }
  }
  return root
}

interface ExplorerProps {
  entries: FileEntry[]
  selected: string
  onSelect: (path: string) => void
  onMoveFile: (filePath: string, dirPath: string) => void
}

function Explorer({ entries, selected, onSelect, onMoveFile }: ExplorerProps) {
  const [q, setQ] = useState('')
  const [collapsed, setCollapsed] = useState<Set<string>>(new Set())
  const [dragOverDir, setDragOverDir] = useState<string | null>(null)

  const filtering = q.trim() !== ''

  const filtered = useMemo(() => {
    const needle = q.trim().toLowerCase()
    if (!needle) return entries
    return entries.filter((e) => e.path.toLowerCase().includes(needle))
  }, [entries, q])

  const tree = useMemo(() => buildTree(filtered), [filtered])

  const toggle = (dirPath: string) => {
    setCollapsed((prev) => {
      const next = new Set(prev)
      if (next.has(dirPath)) next.delete(dirPath)
      else next.add(dirPath)
      return next
    })
  }

  const dropFile = (dirPath: string) => (e: React.DragEvent) => {
    e.preventDefault()
    e.stopPropagation()
    setDragOverDir(null)
    const file = e.dataTransfer.getData('text/plain')
    if (file) onMoveFile(file, dirPath)
  }

  const rootDrop = (e: React.DragEvent) => {
    e.preventDefault()
    setDragOverDir(null)
    const file = e.dataTransfer.getData('text/plain')
    if (file) onMoveFile(file, '')
  }

  const items: ReactNode[] = []
  const renderNodes = (nodes: TreeNode[], depth: number) => {
    for (const node of nodes) {
      const pad = depth * 14
      if (node.kind === 'dir') {
        const open = !collapsed.has(node.dirPath)
        const highlighted = dragOverDir === node.dirPath
        items.push(
          <li
            key={`dir:${node.dirPath}`}
            className={`knowledge-tree-row drop-target${highlighted ? ' drag-over' : ''}`}
            style={{ paddingLeft: pad }}
            onDragOver={(e) => {
              e.preventDefault()
              e.stopPropagation()
              if (dragOverDir !== node.dirPath) setDragOverDir(node.dirPath)
            }}
            onDragLeave={(e) => {
              if (dragOverDir === node.dirPath && !e.currentTarget.contains(e.relatedTarget as Node)) {
                setDragOverDir(null)
              }
            }}
            onDrop={dropFile(node.dirPath)}
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
            draggable
            onDragStart={(e) => {
              e.dataTransfer.setData('text/plain', node.path)
              e.dataTransfer.effectAllowed = 'move'
            }}
            onDragEnd={() => setDragOverDir(null)}
            onDragOver={(e) => e.stopPropagation()}
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

  const fileMatches = filtered.filter((e) => e.kind === 'file')

  return (
    <aside className="knowledge-explorer">
      <div className="page-header">
        <h1>Files</h1>
      </div>
      <div className="toolbar">
        <SearchBar value={q} onChange={setQ} onSubmit={() => undefined} placeholder="Filter…" />
      </div>
      {entries.length === 0 ? (
        <p className="muted">No files.</p>
      ) : filtering ? (
        <ul className="file-list">
          {fileMatches.map((e) => (
            <li key={e.path}>
              <button className="link-btn" onClick={() => onSelect(e.path)}>
                {e.path}
              </button>
            </li>
          ))}
          {fileMatches.length === 0 && <li className="muted">No matches.</li>}
        </ul>
      ) : (
        <ul className="knowledge-tree" onDragOver={(e) => e.preventDefault()} onDrop={rootDrop}>
          {items}
        </ul>
      )}
    </aside>
  )
}

interface FilePaneProps {
  path: string
  onBack: () => void
  favorites: string[]
  refreshMeta: () => Promise<void>
  onChanged: () => void
  onOpen: (path: string) => void
  onDeleted: () => void
}

function FilePane({ path, onBack, favorites, refreshMeta, onChanged, onOpen, onDeleted }: FilePaneProps) {
  const [content, setContent] = useState('')
  const [draft, setDraft] = useState('')
  const [editing, setEditing] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [saved, setSaved] = useState(false)
  const [isFav, setIsFav] = useState(false)

  useEffect(() => {
    setError(null)
    setEditing(false)
    setSaved(false)
    setIsFav(favorites.includes(path))
    void api
      .getFileContent(path)
      .then((c) => {
        setContent(c.content)
        setDraft(c.content)
      })
      .catch((e) => setError(e instanceof Error ? e.message : String(e)))
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [path])

  const isMarkdown = /\.md$/i.test(path) || /\.markdown$/i.test(path)

  const outline = useMemo(() => extractOutline(content), [content])

  const fav = (enabled: boolean) => {
    void api
      .setFavorite(path, enabled)
      .then(() => {
        setIsFav(enabled)
        void refreshMeta()
      })
      .catch((e) => setError(e instanceof Error ? e.message : String(e)))
  }

  const dirOf = (p: string) => {
    const i = p.lastIndexOf('/')
    return i >= 0 ? p.slice(0, i) : ''
  }

  const baseOf = (p: string) => (p.split('/').pop() ?? p)

  const rename = () => {
    const name = window.prompt('New file name', baseOf(path))
    if (!name || !name.trim() || name.trim() === baseOf(path)) return
    const dir = dirOf(path)
    const newPath = dir ? `${dir}/${name.trim()}` : name.trim()
    void api
      .moveFile(path, newPath)
      .then(() => {
        onChanged()
        onOpen(newPath)
      })
      .catch((e) => setError(e instanceof Error ? e.message : String(e)))
  }

  const duplicate = () => {
    const base = baseOf(path)
    const dot = base.lastIndexOf('.')
    const copyName = dot > 0 ? base.slice(0, dot) + '-copy' + base.slice(dot) : base + '-copy'
    const dir = dirOf(path)
    const defaultPath = dir ? `${dir}/${copyName}` : copyName
    const newPath = window.prompt('New file path', defaultPath)
    if (!newPath || !newPath.trim() || newPath.trim() === path) return
    void api
      .copyFile(path, newPath.trim())
      .then(() => {
        onChanged()
        onOpen(newPath.trim())
      })
      .catch((e) => setError(e instanceof Error ? e.message : String(e)))
  }

  const remove = () => {
    if (!window.confirm(`Delete "${path}"?`)) return
    void api
      .deleteFile(path)
      .then(() => {
        onChanged()
        onDeleted()
      })
      .catch((e) => setError(e instanceof Error ? e.message : String(e)))
  }

  const save = () => {
    setSaved(false)
    void api
      .saveFileContent(path, draft)
      .then(() => {
        setContent(draft)
        setEditing(false)
        setSaved(true)
        setTimeout(() => setSaved(false), 2000)
      })
      .catch((e) => setError(e instanceof Error ? e.message : String(e)))
  }

  const cancel = () => {
    setDraft(content)
    setEditing(false)
  }

  return (
    <div>
      <div className="knowledge-body">
        <div className="knowledge-main">
          <div className="page-header note-toolbar">
            <button className="ghost mobile-back" onClick={onBack}>
              ← Files
            </button>
            <div className="actions">
              <button className={isFav ? 'ghost active' : 'ghost'} onClick={() => fav(!isFav)}>
                {isFav ? '★ Favorite' : '☆ Favorite'}
              </button>
              <button className="ghost" onClick={rename}>
                Rename
              </button>
              <button className="ghost" onClick={duplicate}>
                Duplicate
              </button>
              <button className="ghost" onClick={remove}>
                Delete
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
                <button className="ghost" onClick={cancel}>
                  Cancel
                </button>
              </div>
              <textarea
                className="editor"
                value={draft}
                onChange={(e) => setDraft(e.target.value)}
                aria-label="File editor"
              />
            </div>
          ) : (
            <div className="card knowledge-view">
              <h1>{path.split('/').pop()}</h1>
              <div className="meta-line">
                <span className="muted">{path}</span>
              </div>
              {isMarkdown ? (
                <RichMarkdown text={content} />
              ) : (
                <pre className="file-raw">{content}</pre>
              )}
            </div>
          )}
        </div>
        {!editing && isMarkdown && (
          <aside className="outline">
            <div className="outline-section">
              <div className="outline-title">Content</div>
              {outline.map((h, i) => (
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
          </aside>
        )}
      </div>
    </div>
  )
}

interface DashboardProps {
  refreshMeta: () => Promise<void>
  favorites: string[]
}

export function Dashboard({ refreshMeta, favorites }: DashboardProps) {
  const [files, setFiles] = useState<FileEntry[]>([])
  const [selected, setSelected] = useState<string>('')
  const [loaded, setLoaded] = useState(false)
  const [moveError, setMoveError] = useState<string | null>(null)

  const load = useCallback(() => {
    void api
      .listFiles()
      .then((entries) => {
        setFiles(entries)
        setLoaded(true)
        setSelected((prev) => {
          if (prev) return prev
          const hasReadme = entries.some((e) => e.kind === 'file' && e.path === 'README.md')
          return hasReadme ? 'README.md' : ''
        })
      })
      .catch(() => setFiles([]))
  }, [])

  useEffect(() => {
    load()
  }, [load])

  const handleMoveFile = (filePath: string, dirPath: string) => {
    const name = filePath.split('/').pop() ?? filePath
    const newPath = dirPath ? `${dirPath}/${name}` : name
    if (newPath === filePath) return
    setMoveError(null)
    void api
      .moveFile(filePath, newPath)
      .then(() => {
        load()
        setSelected(newPath)
      })
      .catch((e) => setMoveError(e instanceof Error ? e.message : String(e)))
  }

  return (
    <div className="page">
      {moveError && <div className="error-banner">{moveError}</div>}
      <div className={`knowledge-layout${selected ? ' has-selection' : ''}`}>
        <Explorer
          entries={files}
          selected={selected}
          onSelect={setSelected}
          onMoveFile={handleMoveFile}
        />
        <div className="knowledge-pane">
          {selected ? (
            <FilePane
              path={selected}
              onBack={() => setSelected('')}
              favorites={favorites}
              refreshMeta={refreshMeta}
              onChanged={load}
              onOpen={setSelected}
              onDeleted={() => setSelected('')}
            />
          ) : (
            <div className="card knowledge-empty">
              <h2>Files</h2>
              <p className="muted">Select a file from the explorer to view it here.</p>
              {loaded && files.length === 0 && <p className="muted">No files yet.</p>}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
