import { ReactNode, useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { FileEntry, Knowledge, api } from '../api/client'
import { SearchBar } from '../components/SearchBar'
import { RichMarkdown, extractOutline } from '../components/RichMarkdown'
import { OutlineGraph } from '../components/OutlineGraph'
import { FrontmatterForm, FrontmatterSummary } from '../components/FrontmatterForm'
import { Button } from '../components/ui/button'
import { Card, CardContent } from '../components/ui/card'
import { Textarea } from '../components/ui/textarea'
import { Separator } from '../components/ui/separator'
import { TagBadge, StatusBadge } from '../components/badges'
import { TerminalTabs, TerminalTabData } from '../components/TerminalTabs'
import { TerminalSquare } from 'lucide-react'
import { cn } from '@/lib/utils'
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
  onNewFile?: (dir: string) => void
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

function FileStatusBadge({ status }: { status?: string }) {
  if (!status) return null
  return <StatusBadge status={status} className="file-status ml-auto" />
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
  onNewFile?: (dir: string) => void
  onNewTerminal?: () => void
  onClose?: () => void
  onMoveFile?: (filePath: string, dirPath: string) => void
}

function Explorer({ entries, selected, onSelect, title, mode, onNew, newLabel = 'New', onNewFile, onNewTerminal, onClose, onMoveFile }: ExplorerProps) {
  const [q, setQ] = useState('')
  const [tag, setTag] = useState('')
  const [expanded, setExpanded] = useState<Set<string>>(new Set())
  const [dragOverDir, setDragOverDir] = useState<string | null>(null)

  const allTags = useMemo(
    () => Array.from(new Set(entries.flatMap((e) => e.tags ?? []))).sort(),
    [entries],
  )

  const newFileDir = useMemo(() => {
    if (!selected) return ''
    const entry = entries.find((e) => e.path === selected)
    if (entry?.kind === 'dir') return selected
    const idx = selected.lastIndexOf('/')
    return idx > 0 ? selected.slice(0, idx) : ''
  }, [entries, selected])

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
            className={cn(
              'knowledge-tree-row flex min-h-[26px] items-center gap-1 rounded-md px-1 leading-[1.4] hover:bg-muted',
              draggable && 'drop-target cursor-copy',
              highlighted && 'drag-over bg-primary text-white',
            )}
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
              className={cn(
                'knowledge-caret flex h-6 w-6 shrink-0 cursor-pointer items-center justify-center rounded p-0 text-muted-foreground hover:bg-muted hover:text-primary',
                open && 'open',
              )}
              aria-label={open ? `Collapse ${node.name}` : `Expand ${node.name}`}
              onClick={() => toggle(node.dirPath)}
            >
              <svg
                className={cn('caret-icon h-3 w-3 shrink-0 transition-transform', open && 'rotate-90')}
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
              className={cn(
                'knowledge-dir flex min-w-0 flex-1 cursor-pointer items-center self-stretch bg-transparent p-0 text-left text-sm font-semibold whitespace-nowrap text-foreground overflow-hidden text-ellipsis hover:text-primary',
                node.dirPath === selected && 'active text-primary',
              )}
              onClick={() => {
                onSelect(node.dirPath)
                if (!open) toggle(node.dirPath)
              }}
            >
              {node.name}
            </button>
            <FileStatusBadge status={node.status} />
          </li>,
        )
        if (open) renderNodes(node.children, depth + 1, node)
      } else {
        items.push(
          <li
            key={`file:${node.path}`}
            className="knowledge-tree-row flex min-h-[26px] items-center gap-1 rounded-md px-1 leading-[1.4] hover:bg-muted"
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
              className={cn(
                'knowledge-file flex min-w-0 flex-1 cursor-pointer items-center self-stretch bg-transparent p-0 text-left text-sm whitespace-nowrap text-foreground overflow-hidden text-ellipsis hover:text-primary',
                node.path === selected && 'active text-primary font-semibold',
              )}
              onClick={() => onSelect(node.path)}
            >
              {node.name}
            </button>
            <FileStatusBadge status={parent?.status && node.name === 'task.md' ? undefined : node.status} />
          </li>,
        )
      }
    }
  }
  renderNodes(tree, 0)

  const fileMatches = filtered.filter((e) => e.kind === 'file')

  const selectCls =
    'h-9 rounded-md border border-input bg-card px-3 text-sm text-foreground transition-[color,box-shadow] outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50'

  return (
    <aside
      className={cn(
        'knowledge-explorer sticky top-0 flex h-screen w-[280px] shrink-0 flex-col overflow-y-auto border-r bg-card p-2.5 self-stretch',
        'max-xl:w-[220px]',
        'max-lg:relative max-lg:h-auto max-lg:w-full max-lg:max-h-none max-lg:border-b max-lg:border-r-0',
        selected && 'max-lg:hidden',
      )}
    >
      <div className="page-header mb-1 flex flex-wrap items-center justify-between gap-3">
        {onClose && (
          <Button
            variant="ghost"
            size="sm"
            className="mobile-close hidden max-lg:inline-flex"
            onClick={onClose}
            aria-label="Close explorer"
          >
            ← Back
          </Button>
        )}
        <h1 className="text-lg font-bold">{title}</h1>
        <div className="page-header-actions flex items-center gap-2">
          {onNewTerminal && (
            <Button
              variant="ghost"
              size="icon-sm"
              className="cursor-pointer"
              onClick={onNewTerminal}
              aria-label="Open terminal"
              title="Open terminal"
            >
              <TerminalSquare />
            </Button>
          )}
          {onNewFile && (
            <Button variant="ghost" size="sm" onClick={() => onNewFile(newFileDir)}>
              New file
            </Button>
          )}
          {onNew && (
            <Button size="sm" onClick={onNew}>
              {newLabel}
            </Button>
          )}
        </div>
      </div>
      <div className="toolbar my-3 flex flex-wrap gap-2">
        <SearchBar value={q} onChange={setQ} onSubmit={() => undefined} placeholder="Filter…" />
        {allTags.length > 0 && (
          <select value={tag} onChange={(e) => setTag(e.target.value)} aria-label="Filter by tag" className={selectCls}>
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
        <p className="muted text-sm text-muted-foreground">No files yet.</p>
      ) : filtering ? (
        <ul className="file-list m-0 flex list-none flex-col gap-2 p-0">
          {fileMatches.map((e) => (
            <li key={e.path} className="flex flex-wrap items-center gap-2">
              <Button variant="link" size="xs" onClick={() => onSelect(e.path)}>
                {e.path}
              </Button>
              <FileStatusBadge status={e.status} />
            </li>
          ))}
          {fileMatches.length === 0 && <li className="text-muted-foreground">No matches.</li>}
        </ul>
      ) : mode === 'files' && onMoveFile ? (
        <ul className="knowledge-tree m-0 mt-2 list-none p-0" onDragOver={(e) => e.preventDefault()} onDrop={rootDrop}>
          {items}
        </ul>
      ) : (
        <ul className="knowledge-tree m-0 mt-2 list-none p-0">{items}</ul>
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

  const sectionTitle = 'mb-3 text-xs font-semibold tracking-wider text-muted-foreground uppercase'

  return (
    <div>
      <div className="knowledge-body flex gap-4 max-md:flex-col">
        <div className="knowledge-main min-w-0 flex-1">
          <div className="page-header note-toolbar sticky top-0 z-20 mb-3 flex items-center justify-between gap-3 border-b bg-card/95 py-2 backdrop-blur">
            <Button variant="ghost" size="sm" className="mobile-back hidden max-lg:inline-flex" onClick={onBack}>
              ← Files
            </Button>
            <div className="actions flex flex-wrap gap-2">
              <Button variant="ghost" size="sm" className={isFav ? 'text-primary' : ''} onClick={() => fav(!isFav)}>
                {isFav ? '★ Favorite' : '☆ Favorite'}
              </Button>
              {mode === 'knowledge' && !isDir && (
                <Button variant="ghost" size="sm" onClick={move}>
                  Move
                </Button>
              )}
              {mode === 'files' && (
                <>
                  <Button variant="ghost" size="sm" onClick={move}>
                    Move
                  </Button>
                  <Button variant="ghost" size="sm" onClick={duplicate}>
                    Duplicate
                  </Button>
                  <Button variant="ghost" size="sm" onClick={remove}>
                    Delete
                  </Button>
                </>
              )}
              {!isDir && !editing && !isImage && (
                <Button size="sm" onClick={() => setEditing(true)}>
                  Edit
                </Button>
              )}
            </div>
          </div>
          {error && (
            <div className="error-banner my-2 flex items-center justify-between rounded-md border border-red-200 bg-red-50 px-3.5 py-2.5 text-sm text-red-700">
              {error}
            </div>
          )}
          {saved && (
            <div className="notice my-2 rounded-md border border-green-200 bg-green-50 px-3.5 py-2 text-sm text-green-700">
              Saved.
            </div>
          )}
          {editing ? (
            <Card className="gap-0 p-4">
              <div className="actions mb-4 flex gap-2">
                <Button size="sm" onClick={save}>
                  Save
                </Button>
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => {
                    setDraft(content)
                    const split = splitFrontmatter(content)
                    setDraftBody(split.body)
                    setDraftFm(parseFrontmatter(split.frontmatter).data)
                    setEditing(false)
                  }}
                >
                  Cancel
                </Button>
              </div>
              {useForm ? (
                <>
                  <CardContent className="border-t px-0 pt-4">
                    <h3 className={sectionTitle}>Metadata</h3>
                    <FrontmatterForm value={draftFm} onChange={setDraftFm} />
                  </CardContent>
                  <CardContent className="mt-4 border-t px-0 pt-4">
                    <h3 className={sectionTitle}>Body</h3>
                    <Textarea
                      className="editor min-h-[60vh] w-full font-mono text-sm leading-6"
                      value={draftBody}
                      onChange={(e) => setDraftBody(e.target.value)}
                      aria-label="File editor"
                    />
                  </CardContent>
                </>
              ) : (
                <CardContent className="px-0 py-0">
                  <Textarea
                    className="editor min-h-[60vh] w-full font-mono text-sm leading-6"
                    value={draft}
                    onChange={(e) => setDraft(e.target.value)}
                    aria-label={mode === 'knowledge' ? 'Markdown editor' : 'File editor'}
                  />
                </CardContent>
              )}
            </Card>
          ) : (
            <Card className="gap-0 p-4">
              <CardContent className="px-0 py-0">
                <div className="meta-line my-2 flex flex-wrap items-center gap-2">
                  <span className="muted text-sm text-muted-foreground">{path}</span>
                  {tags.map((t) => (
                    <TagBadge key={t}>{t}</TagBadge>
                  ))}
                </div>
                {viewFileName && (
                  <>
                    <Separator className="my-2" />
                    <div className="view-file-name mb-2 text-sm font-semibold text-muted-foreground">{viewFileName}</div>
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
                  <div className="file-image flex justify-center py-3">
                    <img src={rawFileUrl(path)} alt={baseOf(path)} className="max-w-full rounded-md" />
                  </div>
                ) : (
                  <pre className="file-raw overflow-x-auto rounded-md bg-muted p-3 font-mono text-[13px] leading-6 whitespace-pre-wrap break-words">
                    {viewText}
                  </pre>
                )}
                {mode === 'knowledge' && links.length > 0 && (
                  <div className="mt-4 border-t pt-3">
                    <h3 className={sectionTitle}>Links</h3>
                    <ul className="m-0 flex list-none flex-col gap-1 p-0">
                      {links.map((l) => {
                        const resolved = pathOf(l.target)
                        return (
                          <li key={l.raw}>
                            {resolved ? (
                              <Button
                                variant="link"
                                size="xs"
                                onClick={() => navigate(projectUrl(`/knowledge/${encodePath(resolved)}`))}
                              >
                                {l.alias ?? l.target}
                              </Button>
                            ) : (
                              <span className="broken-link text-muted-foreground italic">{l.alias ?? l.target}</span>
                            )}
                          </li>
                        )
                      })}
                    </ul>
                  </div>
                )}
                {mode === 'knowledge' && backlinks.length > 0 && (
                  <div className="mt-4 border-t pt-3">
                    <h3 className={sectionTitle}>Backlinks</h3>
                    <ul className="m-0 flex list-none flex-col gap-1 p-0">
                      {backlinks.map((b) => (
                        <li key={b.path}>
                          <Button
                            variant="link"
                            size="xs"
                            onClick={() => navigate(projectUrl(`/knowledge/${encodePath(b.path)}`))}
                          >
                            {b.path}
                          </Button>
                        </li>
                      ))}
                    </ul>
                  </div>
                )}
              </CardContent>
            </Card>
          )}
        </div>
        <aside
          className={cn(
            'outline sticky top-0 flex w-80 max-h-[calc(100vh-120px)] shrink-0 flex-col gap-1 self-start overflow-y-auto border-l border-border pl-3',
            'max-md:static max-md:max-h-none max-md:w-full max-md:flex-row max-md:flex-wrap max-md:gap-2 max-md:border-l-0 max-md:border-t max-md:pl-0 max-md:pt-3',
          )}
        >
          <div className="outline-section mb-3 flex flex-col gap-1 border-b pb-3 last:mb-0 last:border-b-0 last:pb-0 max-md:w-full">
            <div className="outline-title text-xs font-semibold tracking-wider text-muted-foreground uppercase">Content</div>
            {extractOutline(viewText).map((h, i) => (
              <a
                key={i}
                href={`#${h.id}`}
                className={cn(
                  'outline-link text-[13px] leading-[1.4] text-muted-foreground no-underline hover:text-primary',
                  `level-${h.level}`,
                  h.level === 1 && 'font-semibold',
                  (h.level === 3 || h.level === 4) && 'pl-3',
                )}
                onClick={(e) => {
                  e.preventDefault()
                  document.getElementById(h.id)?.scrollIntoView({ behavior: 'smooth' })
                }}
              >
                {h.text}
              </a>
            ))}
          </div>
          <div className="outline-section mb-3 flex flex-col gap-1 border-b pb-3 last:mb-0 last:border-b-0 last:pb-0 max-md:w-full">
            <div className="outline-title text-xs font-semibold tracking-wider text-muted-foreground uppercase">Tags</div>
            {tags.length === 0 ? (
              <span className="muted outline-none text-[13px] text-muted-foreground">No tags</span>
            ) : (
              <div className="outline-tags flex flex-wrap gap-1.5">
                {tags.map((t) => (
                  <TagBadge key={t}>{t}</TagBadge>
                ))}
              </div>
            )}
          </div>
          <div className="outline-section mb-3 flex flex-col gap-1 border-b pb-3 last:mb-0 last:border-b-0 last:pb-0 max-md:w-full">
            <div className="outline-title text-xs font-semibold tracking-wider text-muted-foreground uppercase">Graph</div>
            <OutlineGraph
              path={path}
              root={mode === 'knowledge' ? root : ''}
              onNodeClick={
                mode === 'knowledge'
                  ? undefined
                  : (n) => {
                        if (n.type === 'task')
                          navigate(projectUrl(`/dashboard/files/tasks/${taskIdOf(n.id)}/task.md`))
                        else if (n.type === 'dir' || n.type === 'file') onOpen(n.id)
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
  onNewFile,
  onClose,
}: BrowserPageProps) {
  const [entries, setEntries] = useState<BrowserEntry[]>([])
  const [loaded, setLoaded] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [moveError, setMoveError] = useState<string | null>(null)
  const autoDefaulted = useRef(false)

  const [terminals, setTerminals] = useState<TerminalTabData[]>([])
  const [activeTerminal, setActiveTerminal] = useState<number | null>(null)
  const [terminalMaximized, setTerminalMaximized] = useState(false)
  const [terminalCollapsed, setTerminalCollapsed] = useState(false)
  const terminalIdRef = useRef(0)

  const addTerminal = useCallback(() => {
    terminalIdRef.current += 1
    const id = terminalIdRef.current
    setTerminals((prev) => [...prev, { id, title: `Terminal ${id}` }])
    setActiveTerminal(id)
  }, [])

  const closeTerminal = useCallback((id: number) => {
    setTerminals((prev) => {
      const next = prev.filter((t) => t.id !== id)
      if (next.length === 0) {
        setTerminalMaximized(false)
        setTerminalCollapsed(false)
      }
      setActiveTerminal((current) => {
        if (next.length === 0) return null
        if (current === id) return next[next.length - 1].id
        return current
      })
      return next
    })
  }, [])

  const toggleMaximize = useCallback(() => {
    setTerminalCollapsed(false)
    setTerminalMaximized((m) => !m)
  }, [])

  const toggleCollapse = useCallback(() => setTerminalCollapsed((c) => !c), [])

  const activateTerminal = useCallback((id: number) => setActiveTerminal(id), [])

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
      {error && (
        <div className="error-banner my-2 flex items-center justify-between rounded-md border border-red-200 bg-red-50 px-3.5 py-2.5 text-sm text-red-700">
          {error}
        </div>
      )}
      {moveError && (
        <div className="error-banner my-2 flex items-center justify-between rounded-md border border-red-200 bg-red-50 px-3.5 py-2.5 text-sm text-red-700">
          {moveError}
        </div>
      )}
      <div className={cn('knowledge-layout flex items-stretch max-lg:flex-col', selected && 'has-selection')}>
        <Explorer
          entries={entries}
          selected={selected}
          onSelect={onSelect}
          title={title}
          mode={mode}
          onNew={onNew}
          newLabel={newLabel}
          onNewFile={onNewFile}
          onNewTerminal={mode === 'files' ? addTerminal : undefined}
          onClose={onClose}
          onMoveFile={mode === 'files' ? handleMoveFile : undefined}
        />
        <div
          className={cn(
            'knowledge-pane min-w-0 flex-1 bg-card p-4',
            'flex flex-col lg:max-h-screen lg:overflow-hidden',
            !selected && terminals.length === 0 && 'max-lg:hidden',
          )}
        >
          <div
            className={cn(
              'knowledge-files min-w-0 flex-1 lg:min-h-0 lg:overflow-y-auto',
              terminalMaximized && !terminalCollapsed && 'hidden',
            )}
          >
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
              <Card className="knowledge-empty items-center justify-center gap-2 p-12 text-center">
                <h2 className="text-xl font-bold">{title}</h2>
                <p className="text-muted-foreground">
                  {mode === 'knowledge'
                    ? 'Select a note from the explorer to view it here.'
                    : 'Select a file from the explorer to view it here.'}
                </p>
                {loaded && entries.length === 0 && (
                  <p className="text-muted-foreground">
                    {mode === 'knowledge'
                      ? 'No knowledge yet — create your first note.'
                      : 'No files yet.'}
                  </p>
                )}
              </Card>
            )}
          </div>
          {mode === 'files' && terminals.length > 0 && activeTerminal !== null && (
            <div className={terminalMaximized && !terminalCollapsed ? 'min-h-0 flex-1' : 'shrink-0'}>
              <TerminalTabs
                tabs={terminals}
                activeId={activeTerminal}
                maximized={terminalMaximized}
                collapsed={terminalCollapsed}
                onAdd={addTerminal}
                onClose={closeTerminal}
                onActivate={activateTerminal}
                onToggleMaximize={toggleMaximize}
                onToggleCollapse={toggleCollapse}
              />
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
