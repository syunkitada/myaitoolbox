import { ReactNode, MouseEvent as ReactMouseEvent, useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useBlocker, useNavigate } from 'react-router-dom'
import { FileEntry, FileExecuteResult, HerdrOverview, Knowledge, api } from '../api/client'
import { SearchBar } from '../components/SearchBar'
import { FileAgentWidget } from '../components/FileAgentWidget'
import { FileTabs } from '../components/FileTabs'
import { RichMarkdown, extractOutline } from '../components/RichMarkdown'
import { FrontmatterForm, FrontmatterSummary } from '../components/FrontmatterForm'
import { Button } from '../components/ui/button'
import { Card, CardContent } from '../components/ui/card'
import { Separator } from '../components/ui/separator'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '../components/ui/collapsible'
import MonacoEditor from '../components/MonacoEditor'
import { GitViewer } from '../components/GitViewer'
import { TagBadge, StatusBadge } from '../components/badges'
import { Badge } from '../components/ui/badge'
import { ChevronDown, Check, Clock, Copy, Eye, EyeOff, FileDiff, FilePlus, FolderHeart, GitBranch, ListPlus, ListTree, Loader2, MoreHorizontal, PanelLeftClose, PanelLeftOpen, PanelRight, RefreshCw, Star, Tag, Terminal, Text, Trash2, X } from 'lucide-react'
import { cn } from '@/lib/utils'
import { dispatchNavAction } from '@/lib/nav-actions'
import { useIsMobile } from '@/hooks/use-mobile'
import { Sheet, SheetContent } from '@/components/ui/sheet'
import { useDialogs } from '../components/AppDialogs'
import { encodePath, filesUrl, getProject, projectUrl, rawFileUrl } from '../utils/routes'
import { SyntaxHighlighter } from '../components/SyntaxHighlighter'
import { languageFromPath } from '../utils/prism-langs'
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

const OUTLINE_STORAGE_KEY = 'outline_open'
const EXPLORER_STORAGE_KEY = 'explorer_open'
const SHOW_HIDDEN_STORAGE_KEY = 'files_show_hidden'

function computeViewStartLine(viewText: string): number {
  const scroller = document.querySelector<HTMLElement>('.knowledge-files')
  const totalLines = viewText.split('\n').length
  if (!scroller || scroller.scrollHeight <= scroller.clientHeight || totalLines <= 1) return 1
  const maxScroll = scroller.scrollHeight - scroller.clientHeight
  const fraction = Math.min(1, Math.max(0, scroller.scrollTop / maxScroll))
  const line = Math.round(fraction * (totalLines - 1)) + 1
  return Math.max(1, line)
}

function handleAnchorClick(e: ReactMouseEvent<HTMLDivElement>) {
  const anchor = (e.target as HTMLElement).closest<HTMLAnchorElement>('a[href^="#"]')
  if (!anchor) return
  const raw = anchor.getAttribute('href')!.slice(1)
  if (!raw) return
  let id: string
  try { id = decodeURIComponent(raw) } catch { id = raw }
  const target = document.getElementById(id)
  if (!target) return
  e.preventDefault()
  target.scrollIntoView({ behavior: 'smooth', block: 'start' })
}

export type BrowserMode = 'files' | 'knowledge'

async function copyToClipboard(text: string): Promise<void> {
  try {
    await navigator.clipboard.writeText(text)
    return
  } catch {
    /* fall through to legacy fallback */
  }
  const ta = document.createElement('textarea')
  ta.value = text
  ta.style.position = 'fixed'
  ta.style.opacity = '0'
  document.body.appendChild(ta)
  ta.select()
  try {
    document.execCommand('copy')
  } finally {
    document.body.removeChild(ta)
  }
}

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
  executable?: boolean
}

interface BrowserPageProps {
  mode: BrowserMode
  title: string
  selected: string
  onSelect: (path: string) => void
  onBack: () => void
  favorites: string[]
  recentFiles: string[]
  refreshMeta: () => Promise<void>
  defaultSelect?: (entries: BrowserEntry[]) => string | undefined
  onClose?: () => void
  herdrOverview?: HerdrOverview | null
  refreshHerdr?: () => void
}

interface TreeFile {
  kind: 'file'
  name: string
  path: string
  status?: string
  gitStatus?: string
  executable?: boolean
}

interface TreeDir {
  kind: 'dir'
  name: string
  dirPath: string
  children: TreeNode[]
  status?: string
  gitStatus?: string
}

type TreeNode = TreeFile | TreeDir

function FileStatusBadge({ status }: { status?: string }) {
  if (!status) return null
  return <StatusBadge status={status} className="file-status ml-auto" />
}

const gitStatusColors: Record<string, string> = {
  staged: 'text-green-500',
  modified: 'text-amber-500',
  untracked: 'text-blue-500',
  deleted: 'text-red-500',
}

function GitFileBadge({ status }: { status?: string }) {
  if (!status) return null
  const color = gitStatusColors[status] ?? 'text-muted-foreground'
  return (
    <span
      className={`git-file-status ml-auto flex shrink-0 items-center ${color}`}
      role="img"
      aria-label={`git: ${status}`}
      title={`git: ${status}`}
    >
      <GitBranch className="size-3.5" />
    </span>
  )
}

function ExecutableBadge() {
  return (
    <Badge
      variant="outline"
      className="exec-file-badge shrink-0 border-transparent bg-slate-100 px-1.5 py-px font-normal text-slate-600 select-none"
      title="Executable"
      aria-label="Executable"
    >
      <Terminal className="size-2.5" aria-hidden="true" />
      exec
    </Badge>
  )
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
    executable: e.executable,
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

const gitStatusPriority = ['staged', 'modified', 'untracked', 'deleted']

function applyDirGitStatus(nodes: TreeNode[]): boolean {
  let hasDirty = false
  for (const node of nodes) {
    if (node.kind === 'file') {
      if (node.gitStatus) {
        hasDirty = true
      }
      continue
    }
    const childDirty = applyDirGitStatus(node.children)
    if (childDirty) {
      hasDirty = true
      const best = node.children.reduce((acc, c) => {
        if (c.kind !== 'dir' || !c.gitStatus) return acc
        return gitStatusPriority.indexOf(c.gitStatus) < gitStatusPriority.indexOf(acc) ? c.gitStatus : acc
      }, node.children.some((c) => c.kind === 'file' && c.gitStatus) ? 'modified' : 'modified')
      node.gitStatus = best
    }
  }
  return hasDirty
}

function buildTree(list: BrowserEntry[], gitStatus: Record<string, string>): TreeNode[] {
  const root: TreeNode[] = []
  const sorted = [...list].sort((a, b) => {
    if (a.kind !== b.kind) return a.kind === 'dir' ? -1 : 1
    return a.path.localeCompare(b.path)
  })
  const dirCache = new Map<string, TreeDir>()
  const getDir = (dirPath: string): TreeDir => {
    let dir = dirCache.get(dirPath)
    if (dir) return dir
    const name = dirPath.split('/').pop() ?? dirPath
    dir = { kind: 'dir', name, dirPath, children: [] }
    dirCache.set(dirPath, dir)
    const slash = dirPath.lastIndexOf('/')
    if (slash < 0) root.push(dir)
    else getDir(dirPath.slice(0, slash)).children.push(dir)
    return dir
  }
  for (const e of sorted) {
    if (e.kind === 'dir') {
      getDir(e.path).status = e.status
    } else {
      const slash = e.path.lastIndexOf('/')
      const file: TreeFile = {
        kind: 'file',
        name: e.name,
        path: e.path,
        status: e.status,
        gitStatus: gitStatus[e.path],
        executable: e.executable,
      }
      if (slash < 0) root.push(file)
      else getDir(e.path.slice(0, slash)).children.push(file)
    }
  }
  applyDirStatus(root)
  applyDirGitStatus(root)
  return root
}

interface ExplorerProps {
  entries: BrowserEntry[]
  selected: string
  onSelect: (path: string) => void
  title: string
  mode: BrowserMode
  favorites: string[]
  recentFiles: string[]
  gitStatus?: Record<string, string>
  onClose?: () => void
  onMoveFile?: (filePath: string, dirPath: string) => void
  onChanged?: () => void | Promise<void>
  onError?: (message: string) => void
  showHidden?: boolean
  onToggleHidden?: () => void
  onOpenGit?: (path: string) => void
  onLoadDir?: (dir: string, force?: boolean) => void | Promise<void>
  onClearSubtree?: (path: string) => void | Promise<void>
}

interface ExplorerSectionProps {
  label: string
  icon: ReactNode
  items: string[]
  emptyText: string
  onSelect: (path: string) => void
}

function ExplorerSection({ label, icon, items, emptyText, onSelect }: ExplorerSectionProps) {
  return (
    <div className="explorer-section mb-4 last:mb-0">
      <h2 className="mb-1 text-xs font-semibold tracking-wider text-muted-foreground uppercase">{label}</h2>
      {items.length === 0 ? (
        <p className="px-1 text-xs text-muted-foreground">{emptyText}</p>
      ) : (
        <ul className="m-0 list-none p-0">
          {items.map((p) => (
            <li key={p} className="knowledge-tree-row flex min-h-[26px] items-center gap-1 rounded-md px-1 leading-[1.4] hover:bg-muted">
              <button
                className="flex min-w-0 flex-1 cursor-pointer items-center gap-1.5 self-stretch bg-transparent p-0 text-left text-sm whitespace-nowrap text-foreground overflow-hidden text-ellipsis hover:text-primary"
                title={p}
                onClick={() => onSelect(p)}
              >
                <span className="flex size-3.5 shrink-0 items-center justify-center text-muted-foreground [&_svg]:size-3.5">
                  {icon}
                </span>
                <span className="truncate">{p}</span>
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}

function Explorer({ entries, selected, onSelect, title, mode, favorites, recentFiles, gitStatus, onClose, onMoveFile, onChanged, onError, showHidden, onToggleHidden, onOpenGit, onLoadDir, onClearSubtree }: ExplorerProps) {
  const { prompt, confirm } = useDialogs()
  const [q, setQ] = useState('')
  const [tag, setTag] = useState('')
  const [expanded, setExpanded] = useState<Set<string>>(new Set())
  const [dragOverDir, setDragOverDir] = useState<string | null>(null)
  const [ctxMenu, setCtxMenu] = useState<{ x: number; y: number; path: string; kind: 'file' | 'dir'; executable?: boolean } | null>(null)
  const [execState, setExecState] = useState<{ path: string; running: boolean; result?: FileExecuteResult; error?: string } | null>(null)
  const [notice, setNotice] = useState<string | null>(null)
  const ctxRef = useRef<HTMLDivElement | null>(null)
  const noticeTimer = useRef<number | null>(null)

  const showNotice = (text: string) => {
    setNotice(text)
    if (noticeTimer.current) window.clearTimeout(noticeTimer.current)
    noticeTimer.current = window.setTimeout(() => setNotice(null), 2000)
  }

  useEffect(() => {
    if (!notice) return
    return () => {
      if (noticeTimer.current) window.clearTimeout(noticeTimer.current)
    }
  }, [notice])

  useEffect(() => {
    if (!ctxMenu) return
    const onDocMouseDown = (e: MouseEvent) => {
      if (!ctxRef.current?.contains(e.target as Node)) setCtxMenu(null)
    }
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setCtxMenu(null)
    }
    const onClose = () => setCtxMenu(null)
    document.addEventListener('mousedown', onDocMouseDown)
    document.addEventListener('keydown', onKeyDown)
    window.addEventListener('blur', onClose)
    window.addEventListener('resize', onClose)
    window.addEventListener('scroll', onClose, true)
    return () => {
      document.removeEventListener('mousedown', onDocMouseDown)
      document.removeEventListener('keydown', onKeyDown)
      window.removeEventListener('blur', onClose)
      window.removeEventListener('resize', onClose)
      window.removeEventListener('scroll', onClose, true)
    }
  }, [ctxMenu])

  const runError = (e: unknown) => {
    onError?.(e instanceof Error ? e.message : String(e))
  }

  const newFileInDir = async () => {
    if (!ctxMenu) return
    const name = await prompt(`New file in "${ctxMenu.path}"`)
    if (!name || !name.trim()) return
    const target = ctxMenu.path ? `${ctxMenu.path}/${name.trim()}` : name.trim()
    setCtxMenu(null)
    void api
      .createFile(target)
      .then(async () => {
        await onChanged?.()
        const parentDir = target.includes('/') ? target.slice(0, target.lastIndexOf('/')) : ''
        if (parentDir) await onLoadDir?.(parentDir, true)
        onSelect(target)
      })
      .catch(runError)
  }

  const newFolderInDir = async () => {
    if (!ctxMenu) return
    const name = await prompt(`New folder in "${ctxMenu.path}"`)
    if (!name || !name.trim()) return
    const target = ctxMenu.path ? `${ctxMenu.path}/${name.trim()}` : name.trim()
    setCtxMenu(null)
    void api
      .createDir(target)
      .then(async () => {
        await onChanged?.()
        const parentDir = target.includes('/') ? target.slice(0, target.lastIndexOf('/')) : ''
        if (parentDir) await onLoadDir?.(parentDir, true)
        onSelect(target)
      })
      .catch(runError)
  }

  const copyRelativePath = () => {
    if (!ctxMenu) return
    const text = ctxMenu.path
    setCtxMenu(null)
    void copyToClipboard(text)
      .then(() => showNotice(`Copied "${text}"`))
      .catch(runError)
  }

  const openGit = () => {
    if (!ctxMenu) return
    onOpenGit?.(ctxMenu.path)
    setCtxMenu(null)
  }

  const copyPath = () => {
    if (!ctxMenu) return
    const rel = ctxMenu.path
    const proj = getProject()
    setCtxMenu(null)
    void (proj
      ? (async () => {
          let abs = rel
          try {
            const projects = await api.listProjects()
            const current = projects.find((p) => p.name === proj)
            if (current?.path) abs = `${current.path.replace(/\/+$/, '')}/${rel}`
          } catch {
            /* fall back to relative path */
          }
          await copyToClipboard(abs)
          showNotice(`Copied "${abs}"`)
        })()
      : copyToClipboard(rel).then(() => showNotice(`Copied "${rel}"`))
    ).catch(runError)
  }

  const renameEntry = async () => {
    if (!ctxMenu) return
    const label = ctxMenu.kind === 'dir' ? 'folder' : 'file'
    const oldPath = ctxMenu.path
    const newPath = await prompt(
      `Rename ${label} — enter a path relative to the project root.`,
      oldPath,
    )
    if (!newPath || !newPath.trim() || newPath.trim() === oldPath) return
    setCtxMenu(null)
    void api
      .moveFile(oldPath, newPath.trim())
      .then(async () => {
        await onChanged?.()
        if (oldPath !== newPath.trim()) await onClearSubtree?.(oldPath)
        const oldDir = oldPath.includes('/') ? oldPath.slice(0, oldPath.lastIndexOf('/')) : ''
        const newDir = newPath.trim().includes('/') ? newPath.trim().slice(0, newPath.trim().lastIndexOf('/')) : ''
        for (const dir of new Set([oldDir, newDir])) if (dir) await onLoadDir?.(dir, true)
        onSelect(newPath.trim())
      })
      .catch(runError)
  }

  const removeEntry = async () => {
    if (!ctxMenu) return
    const path = ctxMenu.path
    const isDir = ctxMenu.kind === 'dir'
    setCtxMenu(null)
    const label = isDir ? `Directory "${path}" and all its contents` : `"${path}"`
    if (!(await confirm(`Delete ${label}?`))) return
    void api
      .deleteFile(path)
      .then(async () => {
        await onChanged?.()
        if (isDir) await onClearSubtree?.(path)
        const parentDir = path.includes('/') ? path.slice(0, path.lastIndexOf('/')) : ''
        if (parentDir) await onLoadDir?.(parentDir, true)
        if (selected === path) onSelect('')
      })
      .catch(runError)
  }

  const openCtxMenu = (e: React.MouseEvent, path: string, kind: 'file' | 'dir', executable?: boolean) => {
    e.preventDefault()
    e.stopPropagation()
    setCtxMenu({ x: e.clientX, y: e.clientY, path, kind, executable })
  }

  const executeEntry = () => {
    if (!ctxMenu) return
    const path = ctxMenu.path
    setCtxMenu(null)
    setExecState({ path, running: true })
    void api
      .executeFile(path)
      .then((result) => setExecState({ path, running: false, result }))
      .catch((e) => setExecState({ path, running: false, error: e instanceof Error ? e.message : String(e) }))
  }

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

  const tree = useMemo(() => buildTree(entries, gitStatus ?? {}), [entries, gitStatus])

  const knownPaths = useMemo(() => {
    const set = new Set<string>()
    for (const e of entries) {
      set.add(e.path)
      const parts = e.path.split('/')
      let prefix = ''
      for (let i = 0; i < parts.length - 1; i++) {
        prefix = prefix ? `${prefix}/${parts[i]}` : parts[i]
        set.add(prefix)
      }
    }
    return set
  }, [entries])

  const visibleFavorites = useMemo(() => favorites.filter((p) => knownPaths.has(p)), [favorites, knownPaths])
  const visibleRecents = useMemo(() => recentFiles.filter((p) => knownPaths.has(p)), [recentFiles, knownPaths])

  const toggle = (dirPath: string) => {
    const open = expanded.has(dirPath)
    if (!open) onLoadDir?.(dirPath)
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
            onContextMenu={(e) => {
              if (mode !== 'files') return
              openCtxMenu(e, node.dirPath, 'dir')
            }}
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
            <GitFileBadge status={node.gitStatus} />
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
            onContextMenu={(e) => {
              if (mode !== 'files') return
              openCtxMenu(e, node.path, 'file', node.executable)
            }}
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
            {node.executable && <ExecutableBadge />}
            <GitFileBadge status={gitStatus?.[node.path]} />
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
    <div className="knowledge-explorer flex h-full min-h-0 w-full flex-col overflow-y-auto bg-card p-2.5">
      <div className="page-header mb-1 flex flex-wrap items-center gap-3">
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
        {mode === 'files' && (
          <div className="flex items-center gap-1">
            <Button
              variant="ghost"
              size="sm"
              className="cursor-pointer"
              onClick={() => dispatchNavAction('new-file')}
              aria-label="New file"
              title="New file"
            >
              <FilePlus />
            </Button>
            <Button
              variant="ghost"
              size="sm"
              className="cursor-pointer"
              onClick={() => dispatchNavAction('new-task')}
              aria-label="New task"
              title="New task"
            >
              <ListPlus />
            </Button>
          </div>
        )}
        {mode === 'files' && showHidden !== undefined && onToggleHidden && (
          <Button
            variant="ghost"
            size="sm"
            className={cn('cursor-pointer', showHidden && 'text-primary')}
            aria-pressed={showHidden}
            aria-label={showHidden ? 'Hide hidden files' : 'Show hidden files'}
            title={
              showHidden
                ? 'Hidden files are shown — click to hide them'
                : 'Hidden files are hidden — click to show them'
            }
            onClick={onToggleHidden}
          >
            {showHidden ? <Eye /> : <EyeOff />}
          </Button>
        )}
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
              {e.executable && <ExecutableBadge />}
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
      <div className="explorer-meta mt-3 border-t border-border pt-3">
        <ExplorerSection
          label="Favorites"
          icon={<FolderHeart />}
          items={visibleFavorites}
          emptyText="No favorites yet."
          onSelect={onSelect}
        />
        <ExplorerSection
          label="Recent"
          icon={<Clock />}
          items={visibleRecents}
          emptyText="No recent files."
          onSelect={onSelect}
        />
      </div>
      {ctxMenu && mode === 'files' && (
        <div
          ref={ctxRef}
          role="menu"
          aria-label="Explorer actions"
          className="fixed z-50 min-w-44 origin-top rounded-md border bg-popover p-1 text-popover-foreground shadow-md"
          style={{
            left: Math.min(ctxMenu.x, window.innerWidth - 200),
            top: Math.min(ctxMenu.y, window.innerHeight - 280),
          }}
        >
          <div className="px-2 pt-1 pb-1 text-xs text-muted-foreground select-none">{ctxMenu.path}</div>
          {ctxMenu.kind === 'dir' && (
            <>
              <button role="menuitem" className="file-action-item" onClick={newFileInDir}>
                <FilePlus className="size-3.5" />
                New File
              </button>
              <button role="menuitem" className="file-action-item" onClick={newFolderInDir}>
                <FolderHeart className="size-3.5" />
                New Folder
              </button>
              <button
                role="menuitem"
                className="file-action-item"
                onClick={openGit}
                data-testid="file-open-git"
              >
                <GitBranch className="size-3.5" />
                Open Git
              </button>
              <div className="my-1 h-px bg-border" />
            </>
          )}
          {ctxMenu.kind === 'file' && ctxMenu.executable && (
            <>
              <button
                role="menuitem"
                className="file-action-item"
                onClick={executeEntry}
                data-testid="file-execute"
                title="Run the file and show its output"
              >
                <Terminal className="size-3.5" />
                Execute
              </button>
              <div className="my-1 h-px bg-border" />
            </>
          )}
          <button role="menuitem" className="file-action-item" onClick={copyPath}>
            <Text className="size-3.5" />
            Copy Path
          </button>
          <button role="menuitem" className="file-action-item" onClick={copyRelativePath}>
            <Text className="size-3.5" />
            Copy Relative Path
          </button>
          <div className="my-1 h-px bg-border" />
          <button role="menuitem" className="file-action-item" onClick={renameEntry}>
            <Text className="size-3.5" />
            Rename
          </button>
          <button
            role="menuitem"
            className="file-action-item text-destructive"
            onClick={removeEntry}
          >
            <Trash2 className="size-3.5" />
            Delete
          </button>
        </div>
      )}
      {notice && (
        <div className="pointer-events-none fixed right-3 bottom-3 z-50 rounded-md border bg-popover px-3 py-2 text-sm text-popover-foreground shadow-md">
          {notice}
        </div>
      )}
      {execState && (
        <ExecuteResultModal
          path={execState.path}
          running={execState.running}
          result={execState.result}
          error={execState.error}
          onClose={() => setExecState(null)}
        />
      )}
    </div>
  )
}

function ExecuteResultModal({
  path,
  running,
  result,
  error,
  onClose,
}: {
  path: string
  running: boolean
  result?: FileExecuteResult
  error?: string
  onClose: () => void
}) {
  const [copied, setCopied] = useState(false)

  useEffect(() => {
    if (!copied) return
    const timer = window.setTimeout(() => setCopied(false), 1500)
    return () => window.clearTimeout(timer)
  }, [copied])

  const copyOutput = () => {
    if (!result) return
    void copyToClipboard(result.output)
      .then(() => setCopied(true))
      .catch(() => undefined)
  }

  const exitedCleanly = result && result.exit_code === 0 && !result.timed_out

  return (
    <div
      className="fixed inset-0 z-[70] flex items-center justify-center bg-black/40 p-4"
      onClick={onClose}
      role="dialog"
      aria-modal="true"
      aria-label={`Execute ${path}`}
    >
      <div
        className="flex max-h-[80vh] w-full max-w-2xl flex-col rounded-lg border bg-card shadow-xl"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-center justify-between gap-2 border-b border-border px-4 py-3">
          <div className="flex min-w-0 items-center gap-2">
            <Terminal className="size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
            <span className="truncate text-sm font-semibold">{path}</span>
          </div>
          <button
            className="flex size-7 shrink-0 cursor-pointer items-center justify-center rounded-md text-muted-foreground hover:bg-muted hover:text-foreground"
            onClick={onClose}
            aria-label="Close execution result"
          >
            <X className="size-4" />
          </button>
        </div>
        <div className="min-h-0 flex-1 overflow-y-auto p-4">
          {running ? (
            <div className="flex items-center gap-2 text-sm text-muted-foreground">
              <Loader2 className="size-4 animate-spin" aria-hidden="true" />
              Running…
            </div>
          ) : error ? (
            <div className="rounded-md border border-red-200 bg-red-50 px-3.5 py-2.5 text-sm text-red-700">
              {error}
            </div>
          ) : result ? (
            <div className="flex flex-col gap-3">
              <div className="flex flex-wrap items-center gap-2">
                <span
                  className={cn(
                    'inline-flex items-center gap-1 rounded-full px-2.5 py-0.5 text-xs font-semibold',
                    result.timed_out
                      ? 'bg-amber-100 text-amber-700'
                      : exitedCleanly
                        ? 'bg-green-100 text-green-700'
                        : 'bg-red-100 text-red-700',
                  )}
                >
                  <span className="size-1.5 rounded-full bg-current" aria-hidden="true" />
                  {result.timed_out ? `Timed out (exit ${result.exit_code})` : `Exit code ${result.exit_code}`}
                </span>
                <Button variant="ghost" size="sm" className="ml-auto cursor-pointer" onClick={copyOutput}>
                  {copied ? <Check className="size-3.5" /> : <Copy className="size-3.5" />}
                  {copied ? 'Copied' : 'Copy output'}
                </Button>
              </div>
              <pre className="exec-result-output m-0 max-h-96 overflow-auto rounded-md border border-border bg-muted/40 p-3 text-xs leading-relaxed whitespace-pre-wrap break-words">
                {result.output || '(no output)'}
              </pre>
            </div>
          ) : null}
        </div>
      </div>
    </div>
  )
}

interface PaneProps {
  mode: BrowserMode
  path: string
  entry?: BrowserEntry
  list: BrowserEntry[]
  favorites: string[]
  refreshMeta: () => Promise<void>
  onChanged: () => void
  onGitStatusChange: () => void
  onOpen: (path: string) => void
  onDeleted: () => void
  explorerOpen: boolean
  onToggleExplorer: () => void
  onRefresh: () => void
  refreshKey: number
  herdrOverview?: HerdrOverview | null
  refreshHerdr?: () => void
}

function Pane({ mode, path, entry, list, favorites, refreshMeta, onChanged, onGitStatusChange, onOpen, onDeleted, explorerOpen, onToggleExplorer, onRefresh, refreshKey, herdrOverview, refreshHerdr }: PaneProps) {
  const navigate = useNavigate()
  const { prompt, confirm, confirm3 } = useDialogs()
  const [content, setContent] = useState('')
  const [draft, setDraft] = useState('')
  const [draftFm, setDraftFm] = useState<Record<string, unknown>>({})
  const [draftBody, setDraftBody] = useState('')
  const [editing, setEditing] = useState(false)
  const [showDiff, setShowDiff] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [saved, setSaved] = useState(false)
  const [isFav, setIsFav] = useState(false)
  const [menuOpen, setMenuOpen] = useState(false)
  const menuRef = useRef<HTMLDivElement | null>(null)
  const editStartLine = useRef<number | null>(null)
  const viewScroll = useRef<{ path: string; top: number; maxTop: number } | null>(null)

  useEffect(() => {
    const onDocClick = (e: MouseEvent) => {
      if (menuOpen && !menuRef.current?.contains(e.target as Node)) setMenuOpen(false)
    }
    document.addEventListener('mousedown', onDocClick)
    return () => document.removeEventListener('mousedown', onDocClick)
  }, [menuOpen])

  const runAction = (fn: () => void) => {
    setMenuOpen(false)
    fn()
  }
  const [activeHeading, setActiveHeading] = useState<string | null>(null)
  const isMobile = useIsMobile()
  const [outlineOpen, setOutlineOpen] = useState<boolean>(() => {
    const saved = window.localStorage.getItem(OUTLINE_STORAGE_KEY)
    if (saved !== null) return saved === '1'
    return window.innerWidth >= 768
  })

  useEffect(() => {
    window.localStorage.setItem(OUTLINE_STORAGE_KEY, outlineOpen ? '1' : '0')
  }, [outlineOpen])

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
    setShowDiff(false)
    setSaved(false)
    editStartLine.current = null
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
    void api.recordRecent(path).then(() => void refreshMeta()).catch(() => undefined)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [path, isDir, isImage, readmePath, listing, refreshKey])

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

  const move = async () => {
    const newPath = await prompt('New path', path)
    if (newPath && newPath.trim() && newPath.trim() !== path) {
      const p =
        mode === 'knowledge'
          ? api.moveKnowledge(path, newPath.trim())
          : api.moveFile(path, newPath.trim())
      void p
        .then(() => {
          onChanged()
          onGitStatusChange()
          onOpen(newPath.trim())
        })
        .catch((e) => setError(e instanceof Error ? e.message : String(e)))
    }
  }

  const duplicate = async () => {
    const base = baseOf(path)
    const dot = base.lastIndexOf('.')
    const copyName = dot > 0 ? base.slice(0, dot) + '-copy' + base.slice(dot) : base + '-copy'
    const dir = dirOf(path)
    const defaultPath = dir ? `${dir}/${copyName}` : copyName
    const newPath = await prompt(isDir ? 'New directory path' : 'New file path', defaultPath)
    if (!newPath || !newPath.trim() || newPath.trim() === path) return
    void api
      .copyFile(path, newPath.trim())
      .then(() => {
        onChanged()
        onGitStatusChange()
        onOpen(newPath.trim())
      })
      .catch((e) => setError(e instanceof Error ? e.message : String(e)))
  }

  const remove = async () => {
    const label = isDir ? `directory "${path}" and all its contents` : `"${path}"`
    if (!(await confirm(`Delete ${label}?`))) return
    void api
      .deleteFile(path)
      .then(() => {
        onChanged()
        onGitStatusChange()
        onDeleted()
      })
      .catch((e) => setError(e instanceof Error ? e.message : String(e)))
  }

  const save = async (): Promise<boolean> => {
    setSaved(false)
    const out = useForm ? buildMarkdown(serializeFrontmatter(draftFm), draftBody) : draft
    const p =
      mode === 'knowledge'
        ? api.saveKnowledgeContent(path, out)
        : api.saveFileContent(path, out)
    try {
      await p
      setContent(out)
      setDraft(out)
      const split = splitFrontmatter(out)
      setDraftBody(split.body)
      setDraftFm(parseFrontmatter(split.frontmatter).data)
      setEditing(false)
      setShowDiff(false)
      setSaved(true)
      onChanged()
      onGitStatusChange()
      setTimeout(() => setSaved(false), 2000)
      return true
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
      return false
    }
  }

  const links = useMemo(() => (mode === 'knowledge' ? extractWikiLinks(content) : []), [mode, content])
  const viewText = useForm ? fmSplit.body : content
  const viewRelativeTo = isDir && mode === 'files' ? readmePath ?? `${path}/` : path
  const viewFileName = isDir && readmePath ? baseOf(readmePath) : null

  const diffModified = useMemo(
    () => (useForm ? buildMarkdown(serializeFrontmatter(draftFm), draftBody) : draft),
    [useForm, draftFm, draftBody, draft],
  )
  const diffOriginal = content
  const diffOriginalBody = fmSplit.body
  const hasUnsavedChanges = diffOriginal !== diffModified

  // Block navigation when leaving with unsaved edits and ask how to proceed.
  const blocker = useBlocker(editing && hasUnsavedChanges)

  useEffect(() => {
    if (blocker.state !== 'blocked') return
    void confirm3(
      `「${path}」には保存されていない変更があります。どうしますか？`,
      { primary: '保存して移動', secondary: '変更を破棄', cancel: 'キャンセル' },
    ).then((choice) => {
      if (choice === 'primary') {
        void save().then((ok) => {
          if (ok) blocker.proceed()
        })
      } else if (choice === 'secondary') {
        blocker.proceed()
      } else {
        blocker.reset()
      }
    })
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [blocker.state])

  useEffect(() => {
    if (!(editing && hasUnsavedChanges)) return
    const onBeforeUnload = (e: BeforeUnloadEvent) => {
      e.preventDefault()
      e.returnValue = ''
    }
    window.addEventListener('beforeunload', onBeforeUnload)
    return () => window.removeEventListener('beforeunload', onBeforeUnload)
  }, [editing, hasUnsavedChanges])

  const startEdit = () => {
    editStartLine.current = computeViewStartLine(viewText)
    const scroller = document.querySelector<HTMLElement>('.knowledge-files')
    viewScroll.current = scroller
      ? { path, top: scroller.scrollTop, maxTop: Math.max(0, scroller.scrollHeight - scroller.clientHeight) }
      : null
    setShowDiff(true)
    setEditing(true)
  }

  useEffect(() => {
    if (editing) return
    const snap = viewScroll.current
    viewScroll.current = null
    if (!snap || snap.path !== path) return
    // Wait until the view has been re-rendered and laid out before restoring.
    requestAnimationFrame(() => {
      const scroller = document.querySelector<HTMLElement>('.knowledge-files')
      if (!scroller) return
      const maxTop = Math.max(0, scroller.scrollHeight - scroller.clientHeight)
      scroller.scrollTop = snap.maxTop > 0 && maxTop > 0 ? (snap.top / snap.maxTop) * maxTop : snap.top
    })
  }, [editing, path])

  const outlineItems = useMemo(() => extractOutline(viewText), [viewText])

  useEffect(() => {
    const els = outlineItems
      .map((h) => document.getElementById(h.id))
      .filter((el): el is HTMLElement => el !== null)
    if (els.length === 0) {
      setActiveHeading(null)
      return
    }
    const update = () => {
      let current: string | null = null
      for (const el of els) {
        if (el.getBoundingClientRect().top <= 150) current = el.id
      }
      setActiveHeading(current)
    }
    update()
    document.addEventListener('scroll', update, true)
    window.addEventListener('resize', update)
    return () => {
      document.removeEventListener('scroll', update, true)
      window.removeEventListener('resize', update)
    }
  }, [outlineItems])

  const sectionTitle = 'mb-3 text-xs font-semibold tracking-wider text-muted-foreground uppercase'

  const outlinePanel = (
    <>
      <div className="outline-header flex items-center gap-2.5 border-b border-border px-4 py-3 pr-10">
        <div className="flex size-8 shrink-0 items-center justify-center rounded-lg bg-sidebar-primary text-sidebar-primary-foreground">
          <PanelRight className="size-4" />
        </div>
        <div className="min-w-0">
          <div className="text-sm font-semibold leading-tight">Details</div>
          <div className="text-xs leading-tight text-muted-foreground">On this page</div>
        </div>
      </div>
      <div className="outline-body min-h-0 flex-1 overflow-y-auto px-3 py-3">
        <div className="outline-section mb-4 flex flex-col gap-1 border-b border-border pb-4 last:mb-0 last:border-b-0 last:pb-0">
          <div className="outline-title mb-1 flex items-center gap-1.5 text-xs font-semibold tracking-wider text-muted-foreground uppercase">
            <ListTree className="size-3.5" />
            Content
          </div>
          {outlineItems.map((h, i) => (
            <a
              key={i}
              href={`#${h.id}`}
              className={cn(
                'outline-link rounded-sm px-1.5 py-0.5 text-[13px] leading-[1.4] text-muted-foreground no-underline transition-colors hover:bg-muted/60 hover:text-primary',
                `level-${h.level}`,
                h.level === 1 && 'font-semibold',
                (h.level === 3 || h.level === 4) && 'pl-3',
                activeHeading === h.id && 'bg-muted/70 text-primary',
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
        <div className="outline-section mb-4 flex flex-col gap-1 border-b border-border pb-4 last:mb-0 last:border-b-0 last:pb-0">
          <div className="outline-title mb-1 flex items-center gap-1.5 text-xs font-semibold tracking-wider text-muted-foreground uppercase">
            <Tag className="size-3.5" />
            Tags
          </div>
          {tags.length === 0 ? (
            <span className="muted outline-none px-1.5 text-[13px] text-muted-foreground">No tags</span>
          ) : (
            <div className="outline-tags flex flex-wrap gap-1.5 px-1">
              {tags.map((t) => (
                <TagBadge key={t}>{t}</TagBadge>
              ))}
            </div>
          )}
        </div>
      </div>
    </>
  )

  return (
    <div className={cn('min-w-0 w-full', outlineOpen && 'md:pr-96')}>
      {mode === 'files' && herdrOverview !== undefined && (
        <FileAgentWidget
          path={path}
          overview={herdrOverview}
          onRefresh={refreshHerdr ?? (() => undefined)}
        />
      )}
      <div className="knowledge-body flex gap-4 max-md:flex-col">
        <div className={cn('knowledge-main min-w-0 flex-1 md:transition-[margin]')}>
          <div className="page-header note-toolbar sticky top-0 z-20 mb-3 flex items-center justify-between gap-3 border-b bg-card/95 py-2 backdrop-blur">
            <div className="actions flex flex-wrap gap-2">
              <Button
                variant="ghost"
                size="sm"
                aria-label="Toggle file explorer"
                title={explorerOpen ? 'Hide file explorer' : 'Show file explorer'}
                onClick={onToggleExplorer}
                className={cn(explorerOpen && 'text-primary')}
              >
                {explorerOpen ? <PanelLeftClose /> : <PanelLeftOpen />}
              </Button>
              <Button
                variant="ghost"
                size="sm"
                aria-label="Toggle details"
                title={outlineOpen ? 'Hide details' : 'Show details'}
                onClick={() => setOutlineOpen((o) => !o)}
                className={cn(outlineOpen && 'text-primary')}
              >
                <PanelRight />
              </Button>
              <Button
                variant="ghost"
                size="sm"
                aria-label={isFav ? 'Remove from favorites' : 'Add to favorites'}
                title={isFav ? 'Remove from favorites' : 'Add to favorites'}
                className={isFav ? 'text-primary' : ''}
                onClick={() => fav(!isFav)}
              >
                {isFav ? <Star fill="currentColor" /> : <Star />}
              </Button>
              <Button
                variant="ghost"
                size="sm"
                aria-label="Refresh explorer and file"
                title="Refresh explorer and file"
                onClick={onRefresh}
              >
                <RefreshCw />
              </Button>
              {mode === 'knowledge' && !isDir && (
                <Button variant="ghost" size="sm" onClick={move}>
                  Move
                </Button>
              )}
              {mode === 'files' && (
                <div className="file-actions relative" ref={menuRef}>
                  <Button
                    variant="ghost"
                    size="sm"
                    aria-label="File actions"
                    aria-expanded={menuOpen}
                    title="File actions"
                    onClick={() => setMenuOpen((o) => !o)}
                  >
                    <MoreHorizontal className="size-4" />
                  </Button>
                  {menuOpen && (
                    <div
                      role="menu"
                      aria-label="File actions"
                      className="absolute right-0 top-full z-50 mt-1 min-w-40 origin-top rounded-md border bg-popover p-1 text-popover-foreground shadow-md"
                    >
                      <button role="menuitem" className="file-action-item" onClick={() => runAction(move)}>
                        Move
                      </button>
                      <button role="menuitem" className="file-action-item" onClick={() => runAction(duplicate)}>
                        Duplicate
                      </button>
                      <div className="my-1 h-px bg-border" />
                      <button
                        role="menuitem"
                        className="file-action-item text-destructive"
                        onClick={() => runAction(remove)}
                      >
                        Delete
                      </button>
                    </div>
                  )}
                </div>
              )}
              {!isDir && !isImage && (
                editing ? (
                  <>
                    <Button
                      variant={showDiff ? 'default' : 'ghost'}
                      size="sm"
                      aria-label={showDiff ? 'Switch to editor mode' : 'Switch to diff mode'}
                      title={showDiff ? 'Show editor' : 'Show unsaved diff'}
                      onClick={() => setShowDiff((d) => !d)}
                    >
                      {showDiff ? (
                        <Text className="size-4" />
                      ) : (
                        <FileDiff className="size-4" />
                      )}
                      <span className="sr-only">{showDiff ? 'Editor' : 'Diff'}</span>
                    </Button>
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
                        setShowDiff(false)
                      }}
                    >
                      Cancel
                    </Button>
                  </>
                ) : (
                  <Button size="sm" onClick={startEdit}>
                    Edit
                  </Button>
                )
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
              {useForm ? (
                <>
                  <CardContent className="border-t px-0 pt-4">
                    <Collapsible defaultOpen={false}>
                      <CollapsibleTrigger asChild>
                        <button
                          type="button"
                          className="group flex w-full items-center gap-1.5 text-left"
                        >
                          <ChevronDown className="size-4 shrink-0 text-muted-foreground transition-transform duration-200 group-data-[state=open]:rotate-180" />
                          <h3 className="text-xs font-semibold tracking-wider text-muted-foreground uppercase">
                            Metadata
                          </h3>
                          {Object.keys(draftFm).length > 0 && (
                            <span className="text-xs text-muted-foreground">
                              {Object.keys(draftFm).length}
                            </span>
                          )}
                        </button>
                      </CollapsibleTrigger>
                      <CollapsibleContent>
                        <div className="mt-3">
                          <FrontmatterForm value={draftFm} onChange={setDraftFm} />
                        </div>
                      </CollapsibleContent>
                    </Collapsible>
                  </CardContent>
                  <CardContent className="mt-4 border-t px-0 pt-4">
                    <h3 className={sectionTitle}>Body</h3>
                    {showDiff ? (
                      <MonacoEditor
                        className="editor mt-1"
                        value={draftBody}
                        onChange={setDraftBody}
                        path={path}
                        language="markdown"
                        ariaLabel="File editor"
                        original={diffOriginalBody}
                        initialLine={editStartLine.current ?? undefined}
                        completions={list}
                      />
                    ) : (
                      <MonacoEditor
                        className="editor mt-1"
                        value={draftBody}
                        onChange={setDraftBody}
                        path={path}
                        language="markdown"
                        ariaLabel="File editor"
                        initialLine={editStartLine.current ?? undefined}
                        completions={list}
                      />
                    )}
                  </CardContent>
                </>
              ) : (
                <CardContent className="px-0 py-0">
                  {showDiff ? (
                    <MonacoEditor
                      className="editor"
                      value={draft}
                      onChange={setDraft}
                      path={path}
                      ariaLabel={mode === 'knowledge' ? 'Markdown editor' : 'File editor'}
                      original={diffOriginal}
                      initialLine={editStartLine.current ?? undefined}
                      completions={list}
                    />
                  ) : (
                    <MonacoEditor
                      className="editor"
                      value={draft}
                      onChange={setDraft}
                      path={path}
                      ariaLabel={mode === 'knowledge' ? 'Markdown editor' : 'File editor'}
                      initialLine={editStartLine.current ?? undefined}
                      completions={list}
                    />
                  )}
                </CardContent>
              )}
              {showDiff && !hasUnsavedChanges && (
                <CardContent className="px-0 py-0">
                  <p className="text-sm text-muted-foreground">No unsaved changes.</p>
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
                  <SyntaxHighlighter
                    text={viewText}
                    language={languageFromPath(viewFileName ?? undefined)}
                    className="file-raw rounded-md bg-muted p-3"
                  />
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
        {!isMobile && (
          <div
            data-outline-open={outlineOpen ? 'true' : 'false'}
            className={cn(
              'outline-pane absolute right-0 top-0 bottom-0 z-40 hidden overflow-hidden transition-[width] duration-200 ease-linear md:block',
              outlineOpen ? 'w-96' : 'w-0',
            )}
          >
            <aside
              aria-hidden={!outlineOpen}
              className={cn(
                'outline absolute top-0 right-0 flex h-full w-96 flex-col overflow-hidden border-l border-border bg-card transition-transform duration-200 ease-linear',
                outlineOpen ? 'translate-x-0' : 'translate-x-full',
              )}
            >
              {outlinePanel}
            </aside>
          </div>
        )}
      </div>
      {isMobile && (
        <Sheet open={outlineOpen} onOpenChange={setOutlineOpen}>
          <SheetContent
            side="right"
            className="outline w-[85vw] max-w-sm gap-0 border-l bg-card p-0 text-sidebar-foreground"
          >
            {outlinePanel}
          </SheetContent>
        </Sheet>
      )}
    </div>
  )
}

export function BrowserPage({
  mode,
  title,
  selected,
  onSelect,
  onBack,
  favorites,
  recentFiles,
  refreshMeta,
  defaultSelect,
  onClose,
  herdrOverview,
  refreshHerdr,
}: BrowserPageProps) {
  const [childrenByDir, setChildrenByDir] = useState<Record<string, BrowserEntry[]>>({})
  const [gitStatus, setGitStatus] = useState<Record<string, string>>({})
  const [loaded, setLoaded] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [moveError, setMoveError] = useState<string | null>(null)
  const [refreshKey, setRefreshKey] = useState(0)
  const [openGitDir, setOpenGitDir] = useState<string | null>(null)
  const autoDefaulted = useRef(false)
  const loadedDirsRef = useRef(new Set<string>())
  const loadingDirsRef = useRef(new Set<string>())
  const autoLoadSeq = useRef(0)
  const gitStatusLoaded = useRef(false)

  const entries = useMemo(() => {
    if (mode === 'knowledge') return childrenByDir[''] ?? []
    return Object.values(childrenByDir).flat()
  }, [mode, childrenByDir])

  const isMobile = useIsMobile()
  const [explorerOpen, setExplorerOpen] = useState<boolean>(() => {
    if (typeof window === 'undefined') return true
    if (window.innerWidth < 768) return false
    const saved = window.localStorage.getItem(EXPLORER_STORAGE_KEY)
    if (saved !== null) return saved === '1'
    return true
  })

  const [showHidden, setShowHidden] = useState<boolean>(() => {
    if (typeof window === 'undefined') return true
    const saved = window.localStorage.getItem(SHOW_HIDDEN_STORAGE_KEY)
    return saved === null ? true : saved === '1'
  })

  useEffect(() => {
    if (!isMobile) window.localStorage.setItem(EXPLORER_STORAGE_KEY, explorerOpen ? '1' : '0')
  }, [explorerOpen, isMobile])

  useEffect(() => {
    window.localStorage.setItem(SHOW_HIDDEN_STORAGE_KEY, showHidden ? '1' : '0')
  }, [showHidden])

  useEffect(() => {
    setOpenGitDir(null)
  }, [selected])

  const handleSelect = useCallback(
    (p: string) => {
      onSelect(p)
      if (isMobile) setExplorerOpen(false)
    },
    [isMobile, onSelect],
  )

  const loadDir = useCallback(
    (dir: string, force = false) => {
      if (mode !== 'files') return Promise.resolve()
      if (!force && (loadedDirsRef.current.has(dir) || loadingDirsRef.current.has(dir))) {
        return Promise.resolve()
      }
      if (loadingDirsRef.current.has(dir)) return Promise.resolve()
      loadingDirsRef.current.add(dir)
      return api
        .listFiles({ path: dir, showHidden })
        .then((list) => {
          const children = toEntries(mode, list)
          loadedDirsRef.current.add(dir)
          setChildrenByDir((prev) => ({ ...prev, [dir]: children }))
          setError(null)
        })
        .catch((e) => {
          loadedDirsRef.current.delete(dir)
          setError(e instanceof Error ? e.message : String(e))
        })
        .finally(() => {
          loadingDirsRef.current.delete(dir)
        })
    },
    [mode, showHidden],
  )

  const clearSubtree = useCallback(
    (path: string) => {
      loadedDirsRef.current.delete(path)
      setChildrenByDir((prev) => {
        const next: Record<string, BrowserEntry[]> = {}
        for (const [k, v] of Object.entries(prev)) {
          if (k === path || k.startsWith(`${path}/`)) {
            loadedDirsRef.current.delete(k)
            continue
          }
          next[k] = v
        }
        return next
      })
    },
    [],
  )

  const load = useCallback(() => {
    if (mode === 'knowledge') {
      void api
        .listKnowledge()
        .then((list) => {
          setChildrenByDir({ '': toEntries(mode, list) })
          setLoaded(true)
          setError(null)
        })
        .catch((e) => {
          setChildrenByDir({})
          setLoaded(true)
          setError(e instanceof Error ? e.message : String(e))
        })
      return
    }
    loadedDirsRef.current.clear()
    loadingDirsRef.current.clear()
    setChildrenByDir({})
    void loadDir('').then(() => setLoaded(true))
  }, [mode, loadDir])

  const refreshGitStatus = useCallback(() => {
    if (mode !== 'files') return
    void api
      .getFileGitStatus()
      .then(setGitStatus)
      .catch(() => undefined)
  }, [mode])

  const handleChanged = useCallback(() => {
    if (mode === 'knowledge') {
      load()
      return
    }
    const promises: Promise<void>[] = [loadDir('', true)]
    if (selected) {
      const dir = selected.includes('/') ? selected.slice(0, selected.lastIndexOf('/')) : ''
      if (dir) promises.push(loadDir(dir, true))
    }
    return Promise.all(promises).then(() => undefined)
  }, [mode, load, loadDir, selected])

  const handleRefresh = useCallback(() => {
    if (mode === 'files') {
      const dirs = [...loadedDirsRef.current]
      loadingDirsRef.current.clear()
      for (const d of dirs) void loadDir(d, true)
    } else {
      load()
    }
    refreshGitStatus()
    setRefreshKey((k) => k + 1)
  }, [mode, load, loadDir, refreshGitStatus])

  useEffect(() => {
    load()
  }, [load])

  useEffect(() => {
    if (mode === 'knowledge' && selected) load()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selected])

  // Fetch the ancestor directories of the selected entry one by one so a deep
  // file can be revealed without ever walking the whole tree. When a directory
  // itself is selected, also load its children (used by the directory listing
  // in the pane).
  useEffect(() => {
    if (mode !== 'files' || !selected) return
    const parts = selected.split('/')
    const targets: string[] = []
    let prefix = ''
    for (let i = 0; i < parts.length - 1; i++) {
      prefix = prefix ? `${prefix}/${parts[i]}` : parts[i]
      targets.push(prefix)
    }
    const sel = entries.find((e) => e.path === selected)
    if (sel?.kind === 'dir') targets.push(selected)
    const seq = ++autoLoadSeq.current
    ;(async () => {
      for (const t of targets) {
        if (autoLoadSeq.current !== seq) return
        if (!loadedDirsRef.current.has(t)) await loadDir(t)
      }
    })()
  }, [mode, selected, loadDir, entries])

  useEffect(() => {
    if (mode !== 'files' || gitStatusLoaded.current) return
    gitStatusLoaded.current = true
    void api
      .getFileGitStatus()
      .then(setGitStatus)
      .catch(() => undefined)
  }, [mode])

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

  const knownPaths = useMemo(() => {
    const set = new Set<string>()
    for (const e of entries) {
      set.add(e.path)
      const parts = e.path.split('/')
      let prefix = ''
      for (let i = 0; i < parts.length - 1; i++) {
        prefix = prefix ? `${prefix}/${parts[i]}` : parts[i]
        set.add(prefix)
      }
    }
    return set
  }, [entries])

  const visibleRecents = useMemo(
    () => recentFiles.filter((p) => knownPaths.has(p)).slice(0, 10),
    [recentFiles, knownPaths],
  )

  const handleCloseRecent = useCallback(
    (p: string) => {
      void api
        .deleteRecent(p)
        .then(() => {
          void refreshMeta()
          if (p === selected) {
            const next = visibleRecents.find((q) => q !== p)
            onSelect(next ?? '')
          }
        })
        .catch(() => undefined)
    },
    [selected, visibleRecents, onSelect, refreshMeta],
  )

  const handleMoveFile = (filePath: string, dirPath: string) => {
    const name = filePath.split('/').pop() ?? filePath
    const newPath = dirPath ? `${dirPath}/${name}` : name
    if (newPath === filePath) return
    setMoveError(null)
    void api
      .moveFile(filePath, newPath)
      .then(() => {
        clearSubtree(filePath)
        loadDir('', true)
        if (dirPath) loadDir(dirPath, true)
        const srcDir = filePath.includes('/') ? filePath.slice(0, filePath.lastIndexOf('/')) : ''
        if (srcDir && srcDir !== dirPath) loadDir(srcDir, true)
        onSelect(newPath)
      })
      .catch((e) => setMoveError(e instanceof Error ? e.message : String(e)))
  }

  return (
    <div className="page relative h-full min-h-0">
      {error && (
        <div className="error-banner my-2 flex items-center justify-between rounded-md border border-red-200 bg-red-50 px-3.5 py-2.5 text-sm text-red-700">
          {error}
          <button type="button" onClick={() => setError(null)} className="text-red-500 hover:text-red-800" aria-label="Close error">
            <X className="size-4" />
          </button>
        </div>
      )}
      {moveError && (
        <div className="error-banner my-2 flex items-center justify-between rounded-md border border-red-200 bg-red-50 px-3.5 py-2.5 text-sm text-red-700">
          {moveError}
          <button type="button" onClick={() => setMoveError(null)} className="text-red-500 hover:text-red-800" aria-label="Close error">
            <X className="size-4" />
          </button>
        </div>
      )}
      <div className={cn('knowledge-layout flex h-full min-h-0 flex-col items-stretch overflow-hidden max-md:flex-col', selected && 'has-selection')}>
        <div className="flex min-h-0 flex-1 max-md:flex-col">
          {!isMobile && (
            <div
              data-explorer-open={explorerOpen ? 'true' : 'false'}
              aria-hidden={!explorerOpen}
              className={cn(
                'explorer-pane hidden shrink-0 overflow-hidden border-r border-border bg-card transition-[width] duration-200 ease-linear md:block',
                explorerOpen ? 'w-[280px] max-xl:w-[220px]' : 'w-0',
              )}
            >
              <div className="flex h-full w-[280px] max-w-[280px] flex-col overflow-hidden max-xl:w-[220px] max-xl:max-w-[220px]">
                <div className="flex-1 min-h-0 overflow-y-auto">
                  <Explorer
                    entries={entries}
                    selected={selected}
                    onSelect={handleSelect}
                    title={title}
                    mode={mode}
                    favorites={favorites}
                    recentFiles={recentFiles}
                    gitStatus={gitStatus}
                    onClose={onClose}
                    onMoveFile={mode === 'files' ? handleMoveFile : undefined}
                    onChanged={handleChanged}
                    onError={(msg) => setMoveError(msg)}
                    showHidden={mode === 'files' ? showHidden : undefined}
                    onToggleHidden={mode === 'files' ? () => setShowHidden((s) => !s) : undefined}
                    onOpenGit={setOpenGitDir}
                    onLoadDir={mode === 'files' ? loadDir : undefined}
                    onClearSubtree={mode === 'files' ? clearSubtree : undefined}
                  />
                </div>
              </div>
            </div>
          )}
          {isMobile && (
            <Sheet open={explorerOpen} onOpenChange={setExplorerOpen}>
              <SheetContent
                side="left"
                className="explorer w-[85vw] max-w-sm gap-0 border-r bg-card p-0 text-sidebar-foreground"
              >
                <Explorer
                  entries={entries}
                  selected={selected}
                  onSelect={handleSelect}
                  title={title}
                  mode={mode}
                  favorites={favorites}
                  recentFiles={recentFiles}
                  gitStatus={gitStatus}
                  onClose={onClose}
                  onMoveFile={mode === 'files' ? handleMoveFile : undefined}
                  onChanged={handleChanged}
                  onError={(msg) => setMoveError(msg)}
                  showHidden={mode === 'files' ? showHidden : undefined}
                  onToggleHidden={mode === 'files' ? () => setShowHidden((s) => !s) : undefined}
                  onLoadDir={mode === 'files' ? loadDir : undefined}
                  onClearSubtree={mode === 'files' ? clearSubtree : undefined}
                />
              </SheetContent>
            </Sheet>
          )}
          <div
            className={cn(
              'knowledge-pane relative min-w-0 min-h-0 flex-1 bg-card p-4',
              'flex flex-col lg:overflow-hidden',
            )}
          >
            {mode === 'files' && (
              <FileTabs
                tabs={visibleRecents}
                active={selected}
                onSelect={handleSelect}
                onClose={handleCloseRecent}
              />
            )}
            <div
              className="knowledge-files min-w-0 flex-1 overflow-y-auto"
              onClick={handleAnchorClick}
            >
              {selected ? (
                <Pane
                  mode={mode}
                  path={selected}
                  entry={selectedEntry}
                  list={entries}
                  favorites={favorites}
                  refreshMeta={refreshMeta}
                  onChanged={handleChanged}
                  onGitStatusChange={refreshGitStatus}
                  onOpen={onSelect}
                  onDeleted={onBack}
                  explorerOpen={explorerOpen}
                  onToggleExplorer={() => setExplorerOpen((o) => !o)}
                  onRefresh={handleRefresh}
                  refreshKey={refreshKey}
                  herdrOverview={herdrOverview}
                  refreshHerdr={refreshHerdr}
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
                  <Button
                    variant="outline"
                    size="sm"
                    className="cursor-pointer"
                    onClick={() => setExplorerOpen(true)}
                  >
                    <PanelLeftOpen />
                    Open file explorer
                  </Button>
                </Card>
              )}
            </div>
          </div>
        </div>
      </div>
      {openGitDir && (
        <GitViewer path={openGitDir} onClose={() => setOpenGitDir(null)} refreshMeta={refreshMeta} />
      )}
    </div>
  )
}
