import { ReactNode, useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { FileEntry, Knowledge, api } from '../api/client'
import { SearchBar } from '../components/SearchBar'
import { RichMarkdown, extractOutline } from '../components/RichMarkdown'
import { OutlineGraph } from '../components/OutlineGraph'
import { FrontmatterForm, FrontmatterSummary } from '../components/FrontmatterForm'
import { encodePath, filesUrl, projectUrl, rawFileUrl, taskIdOf } from '../utils/routes'
import {
  buildDirListing,
  buildMarkdown,
  extractFrontmatterTags,
  extractWikiLinks,
  normalizePath,
  parseFrontmatter,
  serializeFrontmatter,
  splitFrontmatter,
} from '../utils/markdown'

export type BrowserMode = 'files' | 'knowledge'

export interface BrowserEntry {
  kind: 'file' | 'dir'
  name: string
  path: string
  title?: string
  tags?: string[]
  aliases?: string[]
  wikiLinks?: string[]
  status?: string
  markdown: boolean
}

interface BrowserPageProps {
  mode: BrowserMode
  root: string
  title: string
  selected: string
  onSelect: (path: string) => void
  onBack: () => void
  favorites: string[]
  refreshMeta: () => Promise<void>
  defaultSelect?: (entries: BrowserEntry[]) => string | undefined
  onNew?: () => void
  newLabel?: string
  onClose?: () => void
}

interface TreeFile {
  kind: 'file'
  name: string
  path: string
  status?: string
}

interface TreeDir {
  kind: 'dir'
  name: string
  dirPath: string
  children: TreeNode[]
  status?: string
}

type TreeNode = TreeFile | TreeDir

function StatusBadge({ status }: { status?: string }) {
  if (!status) return null
  return <span className={`badge status-${status} file-status`}>{status}</span>
}

function toEntries(mode: BrowserMode, list: (FileEntry | Knowledge)[]): BrowserEntry[] {
  if (mode === 'knowledge') {
    return (list as Knowledge[]).map((k) => ({
      kind: 'file' as const,
      name: k.path.split('/').pop() ?? k.path,
      path: k.path,
      title: k.title,
      tags: k.tags ?? [],
      aliases: k.aliases ?? [],
      wikiLinks: k.wiki_links ?? [],
      markdown: true,
    }))
  }
  return (list as FileEntry[]).map((e) => ({
    kind: e.kind,
    name: e.name,
    path: e.path,
    status: e.status,
    markdown: /\.(md|markdown)$/i.test(e.path),
  }))
}

function applyDirStatus(nodes: TreeNode[]) {
  for (const node of nodes) {
    if (node.kind !== 'dir') continue
    const task = node.children.find(
      (c): c is TreeFile => c.kind === 'file' && c.name === 'task.md',
    )
    if (task?.status) node.status = task.status
    applyDirStatus(node.children)
  }
}

function buildTree(list: BrowserEntry[]): TreeNode[] {
  const root: TreeNode[] = []
  const sorted = [...list].sort((a, b) => {
    if (a.kind !== b.kind) return a.kind === 'dir' ? -1 : 1
    return a.path.localeCompare(b.path)
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
      cur.push({ kind: 'file', name: parts[parts.length - 1], path: e.path, status: e.status })
    }
  }
  applyDirStatus(root)
  return root
}

interface ExplorerProps {
  entries: BrowserEntry[]
  selected: string
  onSelect: (path: string) => void
  title: string
  mode: BrowserMode
  onNew?: () => void
  newLabel?: string
  onClose?: () => void
  onMoveFile?: (filePath: string, dirPath: string) => void
}

function Explorer({ entries, selected, onSelect, title, mode, onNew, newLabel = 'New', onClose, onMoveFile }: ExplorerProps) {
  const [q, setQ] = useState('')
  const [tag, setTag] = useState('')
  const [expanded, setExpanded] = useState<Set<string>>(new Set())
  const [dragOverDir, setDragOverDir] = useState<string | null>(null)

  const allTags = useMemo(
    () => Array.from(new Set(entries.flatMap((e) => e.tags ?? []))).sort(),
    [entries],
  )

  const filtering = q.trim() !== '' || tag !== ''

  const filtered = useMemo(() => {
    const needle = q.trim().toLowerCase()
    return entries.filter(
      (e) =>
        (tag === '' || (e.tags ?? []).includes(tag)) &&
        (!needle ||
          e.path.toLowerCase().includes(needle) ||
          (e.title ?? '').toLowerCase().includes(needle) ||
          (e.tags ?? []).some((t) => t.toLowerCase().includes(needle))),
    )
  }, [entries, q, tag])

  const tree = useMemo(() => buildTree(entries), [entries])

  const toggle = (dirPath: string) => {
    setExpanded((prev) => {
      const next = new Set(prev)
      if (next.has(dirPath)) next.delete(dirPath)
      else next.add(dirPath)
      return next
    })
  }

  useEffect(() => {
    if (!selected) return
    const parts = selected.split('/')
    const dirs: string[] = []
    let prefix = ''
    for (let i = 0; i < parts.length - 1; i++) {
      prefix = prefix ? `${prefix}/${parts[i]}` : parts[i]
      dirs.push(prefix)
    }
    if (dirs.length === 0) return
    setExpanded((prev) => {
      const next = new Set(prev)
      for (const d of dirs) next.add(d)
      return next
    })
  }, [selected])

  const dropFile = (dirPath: string) => (e: React.DragEvent) => {
    e.preventDefault()
    e.stopPropagation()
    setDragOverDir(null)
    const file = e.dataTransfer.getData('text/plain')
    if (file) onMoveFile?.(file, dirPath)
  }

  const rootDrop = (e: React.DragEvent) => {
    e.preventDefault()
    setDragOverDir(null)
    const file = e.dataTransfer.getData('text/plain')
    if (file) onMoveFile?.(file, '')
  }

  const items: ReactNode[] = []
  const renderNodes = (nodes: TreeNode[], depth: number, parent?: TreeDir) => {
    for (const node of nodes) {
      const pad = depth * 14
      if (node.kind === 'dir') {
        const open = expanded.has(node.dirPath)
        const highlighted = dragOverDir === node.dirPath
        const draggable = mode === 'files' && !!onMoveFile
        items.push(
          <li
            key={`dir:${node.dirPath}`}
            className={`knowledge-tree-row${draggable ? ' drop-target' : ''}${highlighted ? ' drag-over' : ''}`}
            style={{ paddingLeft: pad }}
            {...(draggable
              ? {
                  onDragOver: (e) => {
                    e.preventDefault()
                    e.stopPropagation()
                    if (dragOverDir !== node.dirPath) setDragOverDir(node.dirPath)
                  },
                  onDragLeave: (e) => {
                    if (
                      dragOverDir === node.dirPath &&
                      !e.currentTarget.contains(e.relatedTarget as Node)
                    ) {
                      setDragOverDir(null)
                    }
                  },
                  onDrop: dropFile(node.dirPath),
                }
              : {})}
          >
            <button
              className={`knowledge-caret${open ? ' open' : ''}`}
              aria-label={open ? `Collapse ${node.name}` : `Expand ${node.name}`}
              onClick={() => toggle(node.dirPath)}
            >
              <svg
                className="caret-icon"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="2.5"
                strokeLinecap="round"
                strokeLinejoin="round"
                aria-hidden="true"
              >
                <polyline points="9 18 15 12 9 6" />
              </svg>
            </button>
            <button
              className={`knowledge-dir${node.dirPath === selected ? ' active' : ''}`}
              onClick={() => {
                onSelect(node.dirPath)
                if (!open) toggle(node.dirPath)
              }}
            >
              {node.name}
            </button>
            <StatusBadge status={node.status} />
          </li>,
        )
        if (open) renderNodes(node.children, depth + 1, node)
      } else {
        items.push(
          <li
            key={`file:${node.path}`}
            className="knowledge-tree-row"
            style={{ paddingLeft: pad + 20 }}
            {...(mode === 'files'
              ? {
                  draggable: true,
                  onDragStart: (e) => {
                    e.dataTransfer.setData('text/plain', node.path)
                    e.dataTransfer.effectAllowed = 'move'
                  },
                  onDragEnd: () => setDragOverDir(null),
                  onDragOver: (e) => e.stopPropagation(),
                }
              : {})}
          >
            <button
              className={`knowledge-file${node.path === selected ? ' active' : ''}`}
              onClick={() => onSelect(node.path)}
            >
              {node.name}
            </button>
            <StatusBadge status={parent?.status && node.name === 'task.md' ? undefined : node.status} />
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
        {onClose && (
          <button className="ghost mobile-close" onClick={onClose} aria-label="Close explorer">
            ← Back
          </button>
        )}
        <h1>{title}</h1>
        {onNew && (
          <button className="primary" onClick={onNew}>
            {newLabel}
          </button>
        )}
      </div>
      <div className="toolbar">
        <SearchBar value={q} onChange={setQ} onSubmit={() => undefined} placeholder="Filter…" />
        {allTags.length > 0 && (
          <select value={tag} onChange={(e) => setTag(e.target.value)} aria-label="Filter by tag">
            <option value="">all tags</option>
            {allTags.map((t) => (
              <option key={t} value={t}>
                {t}
              </option>
            ))}
          </select>
        )}
      </div>
      {entries.length === 0 ? (
        <p className="muted">No files yet.</p>
      ) : filtering ? (
        <ul className="file-list">
          {fileMatches.map((e) => (
            <li key={e.path}>
              <button className="link-btn" onClick={() => onSelect(e.path)}>
                {e.path}
              </button>
              <StatusBadge status={e.status} />
            </li>
          ))}
          {fileMatches.length === 0 && <li className="muted">No matches.</li>}
        </ul>
      ) : mode === 'files' && onMoveFile ? (
        <ul className="knowledge-tree" onDragOver={(e) => e.preventDefault()} onDrop={rootDrop}>
          {items}
        </ul>
      ) : (
        <ul className="knowledge-tree">{items}</ul>
      )}
    </aside>
  )
}

interface PaneProps {
  mode: BrowserMode
  root: string
  path: string
  entry?: BrowserEntry
  list: BrowserEntry[]
  favorites: string[]
  refreshMeta: () => Promise<void>
  onChanged: () => void
  onOpen: (path: string) => void
  onDeleted: () => void
  onBack: () => void
}

function Pane({ mode, root, path, entry, list, favorites, refreshMeta, onChanged, onOpen, onDeleted, onBack }: PaneProps) {
  const navigate = useNavigate()
  const [content, setContent] = useState('')
  const [draft, setDraft] = useState('')
  const [draftFm, setDraftFm] = useState<Record<string, unknown>>({})
  const [draftBody, setDraftBody] = useState('')
  const [editing, setEditing] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [saved, setSaved] = useState(false)
  const [isFav, setIsFav] = useState(false)

  const byPath = useMemo(() => {
    const m = new Map<string, string>()
    for (const e of list) m.set(normalizePath(e.path), e.path)
    return m
  }, [list])

  const byAlias = useMemo(() => {
    const m = new Map<string, string>()
    for (const e of list) {
      if (e.title) m.set(normalizePath(e.title), e.path)
      for (const a of e.aliases ?? []) m.set(normalizePath(a), e.path)
    }
    return m
  }, [list])

  const byBasename = useMemo(() => {
    const m = new Map<string, string>()
    for (const e of list) {
      const base = normalizePath(e.path.split('/').pop() ?? '')
      if (base && !m.has(base)) m.set(base, e.path)
    }
    return m
  }, [list])

  const pathOf = (target: string) =>
    byPath.get(normalizePath(target)) ?? byAlias.get(normalizePath(target)) ?? byBasename.get(normalizePath(target)) ?? null

  const isDir = entry?.kind === 'dir'

  const isImage =
    mode === 'files' && !isDir && /\.(png|jpe?g|gif|webp|avif|bmp|ico|svg)$/i.test(path)

  const readmePath = useMemo(() => {
    if (!isDir) return null
    for (const name of ['README.md', 'README.markdown', 'task.md']) {
      const p = `${path}/${name}`
      if (byPath.has(normalizePath(p))) return p
    }
    return null
  }, [isDir, path, byPath])

  const listing = useMemo(
    () => (isDir && !readmePath ? buildDirListing(path, list) : null),
    [isDir, readmePath, path, list],
  )

  const backlinks = useMemo(
    () => list.filter((e) => (e.wikiLinks ?? []).some((l) => pathOf(l) === path)),
    [list, path],
  )

  useEffect(() => {
    setError(null)
    setEditing(false)
    setSaved(false)
    setIsFav(favorites.includes(path))
    if (isImage) return
    if (mode === 'files' && isDir) {
      if (readmePath) {
        void api
          .getFileContent(readmePath)
          .then((c) => {
            setContent(c.content)
            setDraft(c.content)
            const split = splitFrontmatter(c.content)
            setDraftBody(split.body)
            setDraftFm(parseFrontmatter(split.frontmatter).data)
          })
          .catch((e) => setError(e instanceof Error ? e.message : String(e)))
      } else {
        const text = listing ?? ''
        setContent(text)
        setDraft(text)
        const split = splitFrontmatter(text)
        setDraftBody(split.body)
        setDraftFm(parseFrontmatter(split.frontmatter).data)
      }
      return
    }
    const p = mode === 'knowledge' ? api.getKnowledgeContent(path) : api.getFileContent(path)
    void p
      .then((c) => {
        setContent(c.content)
        setDraft(c.content)
        const split = splitFrontmatter(c.content)
        setDraftBody(split.body)
        setDraftFm(parseFrontmatter(split.frontmatter).data)
      })
      .catch((e) => setError(e instanceof Error ? e.message : String(e)))
    if (mode === 'knowledge') {
      void api.recordRecent(path).then(() => void refreshMeta()).catch(() => undefined)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [path, isDir, isImage, readmePath, listing])

  const isMarkdown =
    isDir || (entry?.markdown ?? (mode === 'knowledge' || /\.(md|markdown)$/i.test(path)))

  const fmSplit = useMemo(() => splitFrontmatter(content), [content])
  const fmParsed = useMemo(() => parseFrontmatter(fmSplit.frontmatter), [fmSplit])
  const useForm = mode === 'files' && isMarkdown && fmParsed.ok

  const tags = useMemo(() => {
    if (mode === 'knowledge') return entry?.tags ?? []
    return extractFrontmatterTags(content)
  }, [mode, entry, content])

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

  const baseOf = (p: string) => p.split('/').pop() ?? p

  const move = () => {
    const newPath = window.prompt('New path', path)
    if (newPath && newPath.trim() && newPath.trim() !== path) {
      const p =
        mode === 'knowledge'
          ? api.moveKnowledge(path, newPath.trim())
          : api.moveFile(path, newPath.trim())
      void p
        .then(() => {
          onChanged()
          onOpen(newPath.trim())
        })
        .catch((e) => setError(e instanceof Error ? e.message : String(e)))
    }
  }

  const duplicate = () => {
    const base = baseOf(path)
    const dot = base.lastIndexOf('.')
    const copyName = dot > 0 ? base.slice(0, dot) + '-copy' + base.slice(dot) : base + '-copy'
    const dir = dirOf(path)
    const defaultPath = dir ? `${dir}/${copyName}` : copyName
    const newPath = window.prompt(isDir ? 'New directory path' : 'New file path', defaultPath)
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
    const label = isDir ? `directory "${path}" and all its contents` : `"${path}"`
    if (!window.confirm(`Delete ${label}?`)) return
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
    const out = useForm ? buildMarkdown(serializeFrontmatter(draftFm), draftBody) : draft
    const p =
      mode === 'knowledge'
        ? api.saveKnowledgeContent(path, out)
        : api.saveFileContent(path, out)
    void p
      .then(() => {
        setContent(out)
        setDraft(out)
        const split = splitFrontmatter(out)
        setDraftBody(split.body)
        setDraftFm(parseFrontmatter(split.frontmatter).data)
        setEditing(false)
        setSaved(true)
        onChanged()
        setTimeout(() => setSaved(false), 2000)
      })
      .catch((e) => setError(e instanceof Error ? e.message : String(e)))
  }

  const links = useMemo(() => (mode === 'knowledge' ? extractWikiLinks(content) : []), [mode, content])
  const viewText = useForm ? fmSplit.body : content
  const viewRelativeTo = isDir && mode === 'files' ? readmePath ?? `${path}/` : path
  const viewFileName = isDir && readmePath ? baseOf(readmePath) : null

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
              {mode === 'knowledge' && !isDir && (
                <button className="ghost" onClick={move}>
                  Move
                </button>
              )}
              {mode === 'files' && (
                <>
                  <button className="ghost" onClick={move}>
                    Move
                  </button>
                  <button className="ghost" onClick={duplicate}>
                    Duplicate
                  </button>
                  <button className="ghost" onClick={remove}>
                    Delete
                  </button>
                </>
              )}
              {!isDir && !editing && !isImage && (
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
                <button
                  className="ghost"
                  onClick={() => {
                    setDraft(content)
                    const split = splitFrontmatter(content)
                    setDraftBody(split.body)
                    setDraftFm(parseFrontmatter(split.frontmatter).data)
                    setEditing(false)
                  }}
                >
                  Cancel
                </button>
              </div>
              {useForm ? (
                <>
                  <div className="card-section">
                    <h3>Metadata</h3>
                    <FrontmatterForm value={draftFm} onChange={setDraftFm} />
                  </div>
                  <div className="card-section">
                    <h3>Body</h3>
                    <textarea
                      className="editor"
                      value={draftBody}
                      onChange={(e) => setDraftBody(e.target.value)}
                      aria-label="File editor"
                    />
                  </div>
                </>
              ) : (
                <textarea
                  className="editor"
                  value={draft}
                  onChange={(e) => setDraft(e.target.value)}
                  aria-label={mode === 'knowledge' ? 'Markdown editor' : 'File editor'}
                />
              )}
            </div>
          ) : (
            <div className="card knowledge-view">
              <div className="meta-line">
                <span className="muted">{path}</span>
                {tags.map((t) => (
                  <span key={t} className="badge tag">
                    {t}
                  </span>
                ))}
              </div>
              {viewFileName && (
                <>
                  <hr className="view-separator" />
                  <div className="view-file-name">{viewFileName}</div>
                </>
              )}
              {useForm && <FrontmatterSummary data={fmParsed.data} />}
              {isMarkdown ? (
                <RichMarkdown
                  text={viewText}
                  pathOf={mode === 'knowledge' ? pathOf : undefined}
                  relativeTo={viewRelativeTo}
                  linkUrl={mode === 'knowledge' ? undefined : filesUrl}
                  imageUrl={rawFileUrl}
                  preserveExtension={mode === 'files'}
                />
              ) : isImage ? (
                <div className="file-image">
                  <img src={rawFileUrl(path)} alt={baseOf(path)} />
                </div>
              ) : (
                <pre className="file-raw">{viewText}</pre>
              )}
              {mode === 'knowledge' && links.length > 0 && (
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
                              onClick={() => navigate(projectUrl(`/knowledge/${encodePath(resolved)}`))}
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
              {mode === 'knowledge' && backlinks.length > 0 && (
                <div className="card-section">
                  <h3>Backlinks</h3>
                  <ul>
                    {backlinks.map((b) => (
                      <li key={b.path}>
                        <button
                          className="link-btn"
                          onClick={() => navigate(projectUrl(`/knowledge/${encodePath(b.path)}`))}
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
        <aside className="outline">
          <div className="outline-section">
            <div className="outline-title">Content</div>
            {extractOutline(viewText).map((h, i) => (
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
            <OutlineGraph
              path={path}
              root={mode === 'knowledge' ? root : ''}
              onNodeClick={
                mode === 'knowledge'
                  ? undefined
                  : (n) => {
                        if (n.type === 'task')
                          navigate(projectUrl(`/dashboard/files/tasks/${taskIdOf(n.id)}/task.md`))
                        else if (n.type === 'dir') onOpen(n.id)
                      else onOpen(n.id.endsWith('.md') ? n.id : `${n.id}.md`)
                    }
              }
            />
          </div>
        </aside>
      </div>
    </div>
  )
}

export function BrowserPage({
  mode,
  root,
  title,
  selected,
  onSelect,
  onBack,
  favorites,
  refreshMeta,
  defaultSelect,
  onNew,
  newLabel,
  onClose,
}: BrowserPageProps) {
  const [entries, setEntries] = useState<BrowserEntry[]>([])
  const [loaded, setLoaded] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [moveError, setMoveError] = useState<string | null>(null)
  const autoDefaulted = useRef(false)

  const load = useCallback(() => {
    const p = mode === 'knowledge' ? api.listKnowledge() : api.listFiles()
    void p
      .then((list) => {
        setEntries(toEntries(mode, list as (FileEntry | Knowledge)[]))
        setLoaded(true)
        setError(null)
      })
      .catch((e) => {
        setEntries([])
        setError(e instanceof Error ? e.message : String(e))
      })
  }, [mode])

  useEffect(() => {
    load()
  }, [load])

  useEffect(() => {
    if (mode === 'knowledge' && selected) load()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selected])

  const reloadedFor = useRef<string | null>(null)

  useEffect(() => {
    if (mode === 'files' && selected && !entries.some((e) => e.path === selected)) {
      if (reloadedFor.current !== selected) {
        reloadedFor.current = selected
        load()
      }
    }
  }, [selected, entries, mode, load])

  useEffect(() => {
    if (loaded && !selected && !autoDefaulted.current && defaultSelect) {
      const p = defaultSelect(entries)
      if (p) {
        autoDefaulted.current = true
        onSelect(p)
      }
    }
  }, [loaded, selected, entries, defaultSelect, onSelect])

  const selectedEntry = entries.find((e) => e.path === selected)

  const handleMoveFile = (filePath: string, dirPath: string) => {
    const name = filePath.split('/').pop() ?? filePath
    const newPath = dirPath ? `${dirPath}/${name}` : name
    if (newPath === filePath) return
    setMoveError(null)
    void api
      .moveFile(filePath, newPath)
      .then(() => {
        load()
        onSelect(newPath)
      })
      .catch((e) => setMoveError(e instanceof Error ? e.message : String(e)))
  }

  return (
    <div className="page">
      {error && <div className="error-banner">{error}</div>}
      {moveError && <div className="error-banner">{moveError}</div>}
      <div className={`knowledge-layout${selected ? ' has-selection' : ''}`}>
        <Explorer
          entries={entries}
          selected={selected}
          onSelect={onSelect}
          title={title}
          mode={mode}
          onNew={onNew}
          newLabel={newLabel}
          onClose={onClose}
          onMoveFile={mode === 'files' ? handleMoveFile : undefined}
        />
        <div className="knowledge-pane">
          {selected ? (
            <Pane
              mode={mode}
              root={root}
              path={selected}
              entry={selectedEntry}
              list={entries}
              favorites={favorites}
              refreshMeta={refreshMeta}
              onChanged={load}
              onOpen={onSelect}
              onDeleted={onBack}
              onBack={onBack}
            />
          ) : (
            <div className="card knowledge-empty">
              <h2>{title}</h2>
              <p className="muted">
                {mode === 'knowledge'
                  ? 'Select a note from the explorer to view it here.'
                  : 'Select a file from the explorer to view it here.'}
              </p>
              {loaded && entries.length === 0 && (
                <p className="muted">
                  {mode === 'knowledge'
                    ? 'No knowledge yet — create your first note.'
                    : 'No files yet.'}
                </p>
              )}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
