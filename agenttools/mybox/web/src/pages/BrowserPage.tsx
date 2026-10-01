import { ReactNode, MouseEvent as ReactMouseEvent, RefObject, forwardRef, useCallback, useEffect, useImperativeHandle, useMemo, useRef, useState } from 'react'
import { useBlocker } from 'react-router-dom'
import { FileEntry, FileExecuteResult, FileSearchResult, GitDetail, GitFile, HerdrOverview, TaskStatus, TaskTriggerRun, api } from '../api/client'
import { SearchBar } from '../components/SearchBar'
import { FileAgentWidget } from '../components/FileAgentWidget'
import { FileTabs } from '../components/FileTabs'
import { RichMarkdown, extractOutline } from '../components/RichMarkdown'
import { TaskProgress } from '../components/TaskProgress'
import { FrontmatterForm, FrontmatterSummary } from '../components/FrontmatterForm'
import { Button } from '../components/ui/button'
import { Card, CardContent } from '../components/ui/card'
import { Separator } from '../components/ui/separator'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '../components/ui/collapsible'
import MonacoEditor from '../components/MonacoEditor'
import { GitViewer } from '../components/GitViewer'
import { TagBadge, StatusBadge } from '../components/badges'
import { Badge } from '../components/ui/badge'
import { Archive, ArrowLeftRight, ChevronDown, Check, Clock, Copy, Eye, EyeOff, FileDiff, FilePlus, FolderHeart, GitBranch, ListPlus, ListTree, Loader2, MoreHorizontal, PanelLeftClose, PanelLeftOpen, PanelRightClose, PanelRightOpen, RefreshCw, Search, Star, Tag, Terminal, Text, Trash2, Upload, X } from 'lucide-react'
import { cn, hasCRLF, normalizeLineEndings } from '@/lib/utils'
import { dispatchNavAction } from '@/lib/nav-actions'
import { useIsMobile } from '@/hooks/use-mobile'
import { MAX_RESIZABLE_WIDTH, MIN_RESIZABLE_WIDTH, useResizableWidth } from '@/hooks/use-resizable-width'
import { Sheet, SheetContent } from '@/components/ui/sheet'
import { useDialogs } from '../components/AppDialogs'
import { useEscapeKey } from '../hooks/use-escape-key'
import { filesUrl, getProject, rawFileUrl } from '../utils/routes'
import { copyToClipboard } from '../utils/clipboard'
import { SyntaxHighlighter } from '../components/SyntaxHighlighter'
import { DiffView } from '../components/DiffView'
import { languageFromPath } from '../utils/prism-langs'
import {
  buildDirListing,
  buildMarkdown,
  extractFrontmatterTags,
  normalizePath,
  parseFrontmatter,
  replaceMarkdownBody,
  extractMarkdownTaskProgress,
  setMarkdownTaskChecked,
  serializeFrontmatter,
  splitFrontmatter,
} from '../utils/markdown'

const OUTLINE_STORAGE_KEY = 'outline_open'
const EXPLORER_STORAGE_KEY = 'explorer_open'
const EXPLORER_WIDTH_STORAGE_KEY = 'mybox_files_explorer_width'
const DETAILS_WIDTH_STORAGE_KEY = 'mybox_files_details_width'
const REFERENCE_WIDTH_STORAGE_KEY = 'mybox_files_reference_width'
const SHOW_HIDDEN_STORAGE_KEY = 'files_show_hidden'
const DEFAULT_EXPLORER_WIDTH = 280
const DEFAULT_DETAILS_WIDTH = 384

function computeViewStartLine(scroller: HTMLElement | null, viewText: string): number {
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
  const target = Array.from(e.currentTarget.querySelectorAll<HTMLElement>('[id]')).find((element) => element.id === id)
  if (!target) return
  e.preventDefault()
  target.scrollIntoView({ behavior: 'smooth', block: 'start' })
}

function openFileInNewTab(path: string) {
  window.open(filesUrl(path), '_blank', 'noopener')
}

function handleAuxClick(e: ReactMouseEvent, path: string) {
  if (e.button !== 1) return
  e.preventDefault()
  openFileInNewTab(path)
}

function taskTriggerIDFromDirectory(path: string): string | null {
  const parts = path.split('/')
  if (parts.length !== 2 || parts[0] !== '_task_triggers' || !parts[1]) return null
  return parts[1]
}

function taskIDFromPath(path: string): string | null {
  const parts = path.split('/')
  if (parts.length !== 3 || parts[0] !== '_tasks' || parts[2] !== 'task.md' || !parts[1]) return null
  return parts[1]
}

const TASK_STATUS_OPTIONS: TaskStatus[] = ['todo', 'doing', 'blocked', 'review', 'done']

export interface BrowserEntry {
  kind: 'file' | 'dir'
  name: string
  path: string
  status?: string
  markdown: boolean
  executable?: boolean
}

export interface FileSearchHit {
  path: string
  line: number
  query: string
}

interface BrowserPageProps {
  title: string
  selected: string
  onSelect: (path: string) => void
  onBack: () => void
  favorites: string[]
  recentFiles: string[]
  refreshMeta: () => Promise<void>
  onRecentChanged?: (path: string) => void
  defaultSelect?: (entries: BrowserEntry[]) => string | undefined
  onClose?: () => void
  herdrOverview?: HerdrOverview | null
  refreshHerdr?: () => void
  webuiFocusedPaneId?: string | null
  onWebuiFocusChange?: (paneId: string | null) => void
  revealPath?: string
  onRevealPathHandled?: (path: string) => void
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

const gitDiffStatusLabels: Record<GitFile['status'], string> = {
  staged: 'Staged',
  unstaged: 'Unstaged',
  untracked: 'Untracked',
}

export function gitFilesForPath(detail: GitDetail, path: string): GitFile[] {
  return [...detail.staged, ...detail.unstaged, ...detail.untracked].filter(
    (file) => file.path === path,
  )
}

interface GitDiffPanelProps {
  files: GitFile[]
  loading?: boolean
  error?: string | null
}

export function GitDiffPanel({ files, loading = false, error = null }: GitDiffPanelProps) {
  if (loading) {
    return (
      <div className="flex items-center gap-2 py-8 text-sm text-muted-foreground" data-testid="git-file-diff">
        <Loader2 className="size-4 animate-spin" />
        Loading Git diff…
      </div>
    )
  }

  if (error) {
    return (
      <div
        className="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700"
        role="alert"
        data-testid="git-file-diff"
      >
        {error}
      </div>
    )
  }

  if (files.length === 0) {
    return (
      <p className="py-8 text-sm text-muted-foreground" data-testid="git-file-diff">
        No Git diff available.
      </p>
    )
  }

  return (
    <div className="space-y-4" data-testid="git-file-diff">
      {files.map((file, index) => (
        <section key={`${file.status}:${file.path}:${index}`} className="min-w-0">
          <h2 className="mb-2 flex items-center gap-2 text-xs font-semibold tracking-wider text-muted-foreground uppercase">
            {gitDiffStatusLabels[file.status]}
            <span className="font-normal normal-case">({file.code})</span>
          </h2>
          <DiffView diff={file.diff} />
        </section>
      ))}
    </div>
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
    </Badge>
  )
}

function toEntries(list: FileEntry[]): BrowserEntry[] {
  return list.map((e) => ({
    kind: e.kind,
    name: e.name,
    path: e.path,
    status: e.status,
    markdown: /\.(md|markdown)$/i.test(e.path),
    executable: e.executable,
  }))
}

function groupEntriesByDirectory(list: FileEntry[]): Record<string, BrowserEntry[]> {
  const grouped: Record<string, BrowserEntry[]> = {}
  for (const entry of toEntries(list)) {
    const slash = entry.path.lastIndexOf('/')
    const parent = slash < 0 ? '' : entry.path.slice(0, slash)
    ;(grouped[parent] ??= []).push(entry)
  }
  return grouped
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

function gitStatusRank(status: string): number {
  const rank = gitStatusPriority.indexOf(status)
  return rank === -1 ? gitStatusPriority.length : rank
}

function preferGitStatus(current: string | undefined, candidate: string): string {
  if (!current || gitStatusRank(candidate) < gitStatusRank(current)) return candidate
  return current
}

function buildDirectoryGitStatuses(gitStatus: Record<string, string>): Map<string, string> {
  const directoryStatuses = new Map<string, string>()
  for (const [path, status] of Object.entries(gitStatus)) {
    const parts = path.replace(/\/+$/, '').split('/').filter(Boolean)
    for (let i = 1; i <= parts.length; i++) {
      const dirPath = parts.slice(0, i).join('/')
      directoryStatuses.set(dirPath, preferGitStatus(directoryStatuses.get(dirPath), status))
    }
  }
  return directoryStatuses
}

function applyDirGitStatus(nodes: TreeNode[], directoryStatuses: Map<string, string>) {
  for (const node of nodes) {
    if (node.kind !== 'dir') continue
    node.gitStatus = directoryStatuses.get(node.dirPath)
    applyDirGitStatus(node.children, directoryStatuses)
  }
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
  applyDirGitStatus(root, buildDirectoryGitStatuses(gitStatus))
  return root
}

interface ExplorerProps {
  entries: BrowserEntry[]
  selected: string
  onSelect: (path: string) => void
  title: string
  favorites: string[]
  recentFiles: string[]
  gitStatus?: Record<string, string>
  onClose?: () => void
  onMoveFile?: (filePath: string, dirPath: string) => void
  onChanged?: (revealPath?: string) => void | Promise<void>
  onError?: (message: string) => void
  showHidden?: boolean
  onToggleHidden?: () => void
  onOpenGit?: (path: string) => void
  onOpenReference?: (path: string) => void
  onLoadDir?: (dir: string, force?: boolean) => void | Promise<void>
  onClearSubtree?: (path: string) => void | Promise<void>
  onSearchHit?: (hit: FileSearchHit) => void
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
                onAuxClick={(e) => handleAuxClick(e, p)}
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

export function Explorer({ entries, selected, onSelect, title, favorites, recentFiles, gitStatus, onClose, onMoveFile, onChanged, onError, showHidden, onToggleHidden, onOpenGit, onOpenReference, onLoadDir, onClearSubtree, onSearchHit }: ExplorerProps) {
  const { prompt, confirm, alert, showProgress, hideProgress } = useDialogs()
  const [q, setQ] = useState('')
  const [searchMode, setSearchMode] = useState<'name' | 'text'>('name')
  const [textResults, setTextResults] = useState<FileSearchResult[]>([])
  const [textSearchQuery, setTextSearchQuery] = useState('')
  const [textSearchTotal, setTextSearchTotal] = useState(0)
  const [textSearchTruncated, setTextSearchTruncated] = useState(false)
  const [textSearchLoading, setTextSearchLoading] = useState(false)
  const [textSearchError, setTextSearchError] = useState<string | null>(null)
  const searchRequest = useRef(0)
  const [expanded, setExpanded] = useState<Set<string>>(new Set())
  const [dragOverDir, setDragOverDir] = useState<string | null>(null)
  const [ctxMenu, setCtxMenu] = useState<{ x: number; y: number; path: string; kind: 'file' | 'dir'; executable?: boolean } | null>(null)
  const [execState, setExecState] = useState<{ path: string; running: boolean; output: string; result?: FileExecuteResult; error?: string } | null>(null)
  const [triggerRunState, setTriggerRunState] = useState<{ path: string; running: boolean; result?: TaskTriggerRun; error?: string } | null>(null)
  const [notice, setNotice] = useState<string | null>(null)
  const [uploading, setUploading] = useState(false)
  const ctxRef = useRef<HTMLDivElement | null>(null)
  const uploadInputRef = useRef<HTMLInputElement | null>(null)
  const uploadDirRef = useRef('')
  const noticeTimer = useRef<number | null>(null)
  const executeCancel = useRef<(() => void) | null>(null)

  useEffect(() => () => executeCancel.current?.(), [])

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

  const errorMessage = (e: unknown) => (e instanceof Error ? e.message : String(e))

  const runError = (e: unknown) => {
    onError?.(errorMessage(e))
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

  const openUpload = () => {
    if (!ctxMenu || ctxMenu.kind !== 'dir' || uploading) return
    uploadDirRef.current = ctxMenu.path
    setCtxMenu(null)
    if (uploadInputRef.current) {
      uploadInputRef.current.value = ''
      uploadInputRef.current.click()
    }
  }

  const handleUpload = (e: React.ChangeEvent<HTMLInputElement>) => {
    const files = Array.from(e.target.files ?? [])
    const directory = uploadDirRef.current
    e.target.value = ''
    if (files.length === 0) return

    const count = files.length + ' file' + (files.length === 1 ? '' : 's')
    const destination = directory || 'project root'
    showProgress(`Uploading ${count} to ${destination}…`)
    setUploading(true)
    void (async () => {
      try {
        await api.uploadFiles(directory, files)
      } catch (error) {
        hideProgress()
        setUploading(false)
        await alert(`Upload failed.\n\n${errorMessage(error)}`)
        return
      }

      hideProgress()
      setUploading(false)
      let refreshError: string | null = null
      try {
        await onChanged?.()
        await onLoadDir?.(directory, true)
      } catch (error) {
        refreshError = errorMessage(error)
      }

      const refreshMessage = refreshError
        ? `\n\nThe upload succeeded, but the file list could not be refreshed.\n${refreshError}`
        : ''
      await alert(`Upload succeeded: ${count} uploaded to ${destination}.${refreshMessage}`)
    })()
      .catch((error) => {
        hideProgress()
        setUploading(false)
        void alert(`Upload failed.\n\n${errorMessage(error)}`)
      })
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

  const openReference = () => {
    if (!ctxMenu) return
    const path = ctxMenu.path
    setCtxMenu(null)
    onOpenReference?.(path)
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
    executeCancel.current?.()
    setExecState({ path, running: true, output: '' })
    executeCancel.current = api.executeFileStream(path, {
      onOutput: (chunk) => {
        setExecState((current) =>
          current?.path === path ? { ...current, output: current.output + chunk } : current,
        )
      },
      onComplete: (result) => {
        setExecState((current) =>
          current?.path === path
            ? { ...current, running: false, result: { ...result, output: current.output } }
            : current,
        )
      },
      onError: (error) => {
        setExecState((current) =>
          current?.path === path ? { ...current, running: false, error: error.message } : current,
        )
      },
    })
  }

  const closeExecution = () => {
    executeCancel.current?.()
    executeCancel.current = null
    setExecState(null)
  }

  const runTriggerEntry = () => {
    if (!ctxMenu || ctxMenu.kind !== 'dir') return
    const triggerID = taskTriggerIDFromDirectory(ctxMenu.path)
    if (!triggerID) return
    const path = ctxMenu.path
    setCtxMenu(null)
    setTriggerRunState({ path, running: true })
    void api
      .runTaskTrigger(triggerID)
      .then((result) => {
        setTriggerRunState({ path, running: false, result })
        const taskPath = result.task_id?.trim() ? `_tasks/${result.task_id}/task.md` : undefined
        void Promise.resolve(onChanged?.(taskPath))
          .then(() => {
            if (taskPath) onSelect(taskPath)
          })
          .catch((e) => onError?.(errorMessage(e)))
      })
      .catch((e) => setTriggerRunState({ path, running: false, error: errorMessage(e) }))
  }

  const duplicateEntry = async () => {
    if (!ctxMenu) return
    const path = ctxMenu.path
    const isDir = ctxMenu.kind === 'dir'
    const base = path.split('/').pop() ?? path
    const dot = base.lastIndexOf('.')
    const copyName = dot > 0 ? base.slice(0, dot) + '-copy' + base.slice(dot) : base + '-copy'
    const dir = path.includes('/') ? path.slice(0, path.lastIndexOf('/')) : ''
    const defaultPath = dir ? `${dir}/${copyName}` : copyName
    const newPath = await prompt(isDir ? 'New directory path' : 'New file path', defaultPath)
    if (!newPath || !newPath.trim() || newPath.trim() === path) return
    setCtxMenu(null)
    void api
      .copyFile(path, newPath.trim())
      .then(async () => {
        await onChanged?.()
        if (isDir) await onClearSubtree?.(path)
        const parentDir = path.includes('/') ? path.slice(0, path.lastIndexOf('/')) : ''
        if (parentDir) await onLoadDir?.(parentDir, true)
        onSelect(newPath.trim())
      })
      .catch(runError)
  }

  const filtering = q.trim() !== ''

  const runTextSearch = useCallback((value: string) => {
    const query = value.trim()
    const requestId = ++searchRequest.current
    if (!query) {
      setTextResults([])
      setTextSearchTotal(0)
      setTextSearchTruncated(false)
      setTextSearchLoading(false)
      setTextSearchError(null)
      return
    }
    setTextSearchLoading(true)
    setTextSearchError(null)
    setTextSearchQuery(query)
    void api
      .searchFiles({ q: query, showHidden: showHidden ?? true })
      .then((response) => {
        if (requestId !== searchRequest.current) return
        setTextResults(response.results)
        setTextSearchTotal(response.total)
        setTextSearchTruncated(response.truncated)
      })
      .catch((e) => {
        if (requestId !== searchRequest.current) return
        setTextResults([])
        setTextSearchTotal(0)
        setTextSearchTruncated(false)
        setTextSearchError(e instanceof Error ? e.message : String(e))
      })
      .finally(() => {
        if (requestId === searchRequest.current) setTextSearchLoading(false)
      })
  }, [showHidden])

  useEffect(() => {
    if (searchMode !== 'text') {
      ++searchRequest.current
      setTextSearchLoading(false)
      return
    }
    const query = q.trim()
    if (!query) {
      runTextSearch('')
      return
    }
    setTextSearchLoading(true)
    setTextSearchError(null)
    const timer = window.setTimeout(() => runTextSearch(query), 350)
    return () => window.clearTimeout(timer)
  }, [q, runTextSearch, searchMode])

  const selectSearchMode = (mode: 'name' | 'text') => {
    setSearchMode(mode)
    if (mode === 'name') {
      ++searchRequest.current
      setTextSearchLoading(false)
      setTextSearchError(null)
    }
  }

  const submitSearch = (value: string) => {
    if (searchMode === 'text') runTextSearch(value)
  }

  const filtered = useMemo(() => {
    const needle = q.trim().toLowerCase()
    return entries.filter(
      (e) =>
        !needle || e.path.toLowerCase().includes(needle),
    )
  }, [entries, q])

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
        const draggable = !!onMoveFile
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
              onAuxClick={(e) => handleAuxClick(e, node.dirPath)}
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
            {...{
              draggable: true,
              onDragStart: (e) => {
                e.dataTransfer.setData('text/plain', node.path)
                e.dataTransfer.effectAllowed = 'move'
              },
              onDragEnd: () => setDragOverDir(null),
              onDragOver: (e) => e.stopPropagation(),
            }}
            onContextMenu={(e) => {
              openCtxMenu(e, node.path, 'file', node.executable)
            }}
          >
            <button
              className={cn(
                'knowledge-file flex min-w-0 flex-1 cursor-pointer items-center self-stretch bg-transparent p-0 text-left text-sm whitespace-nowrap text-foreground overflow-hidden text-ellipsis hover:text-primary',
                node.path === selected && 'active text-primary font-semibold',
              )}
              onClick={() => onSelect(node.path)}
              onAuxClick={(e) => handleAuxClick(e, node.path)}
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

  const openTextResult = (result: FileSearchResult) => {
    if (onSearchHit) {
      onSearchHit({ path: result.path, line: result.line, query: textSearchQuery || q.trim() })
    } else {
      onSelect(result.path)
    }
  }

  const highlightedSnippet = (snippet: string, query: string) => {
    if (!query) return snippet
    const parts = snippet.split(new RegExp(`(${query.replace(/[.*+?^${}()|[\\]\\\\]/g, '\\\\$&')})`, 'ig'))
    return parts.map((part, index) =>
      part.toLowerCase() === query.toLowerCase() ? <mark key={index} className="rounded bg-yellow-200 px-0.5 text-inherit dark:bg-yellow-700">{part}</mark> : part,
    )
  }

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
        {showHidden !== undefined && onToggleHidden && (
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
      <div className="toolbar my-3 flex flex-col gap-2">
        <div className="flex gap-1" role="group" aria-label="Search mode">
          <Button
            type="button"
            variant={searchMode === 'name' ? 'secondary' : 'ghost'}
            size="xs"
            className="cursor-pointer"
            aria-pressed={searchMode === 'name'}
            onClick={() => selectSearchMode('name')}
          >
            <Search />
            Name
          </Button>
          <Button
            type="button"
            variant={searchMode === 'text' ? 'secondary' : 'ghost'}
            size="xs"
            className="cursor-pointer"
            aria-pressed={searchMode === 'text'}
            onClick={() => selectSearchMode('text')}
          >
            <Search />
            Text
          </Button>
        </div>
        <SearchBar
          value={q}
          onChange={setQ}
          onSubmit={submitSearch}
          placeholder={searchMode === 'text' ? 'Search file contents…' : 'Search file names…'}
        />
      </div>
      {entries.length === 0 ? (
        <p className="muted text-sm text-muted-foreground">No files yet.</p>
      ) : searchMode === 'text' && filtering ? (
        <div className="flex min-h-0 flex-col gap-2">
          {textSearchLoading && <p className="text-sm text-muted-foreground">Searching…</p>}
          {textSearchError && <p className="text-sm text-destructive">{textSearchError}</p>}
          {!textSearchLoading && !textSearchError && textResults.length > 0 && (
            <>
              <p className="text-xs text-muted-foreground">
                {textSearchTruncated ? `Showing first ${textResults.length} of ${textSearchTotal} files` : `${textSearchTotal} ${textSearchTotal === 1 ? 'file' : 'files'} found`}
              </p>
              <ul className="file-list m-0 flex list-none flex-col gap-2 p-0">
                {textResults.map((result) => (
                  <li key={result.path} className="min-w-0">
                    <button
                      type="button"
                      className="flex min-w-0 w-full cursor-pointer flex-col items-start rounded-md px-1 py-1 text-left hover:bg-muted"
                      onClick={() => openTextResult(result)}
                      title={`Open ${result.path} at line ${result.line}`}
                    >
                      <span className="w-full truncate text-sm font-medium text-foreground hover:text-primary">{result.path}</span>
                      <span className="w-full truncate text-xs text-muted-foreground">
                        line {result.line} · {result.match_count} {result.match_count === 1 ? 'match' : 'matches'} · {highlightedSnippet(result.snippet, q.trim())}
                      </span>
                    </button>
                  </li>
                ))}
              </ul>
            </>
          )}
          {!textSearchLoading && !textSearchError && textResults.length === 0 && (
            <p className="text-sm text-muted-foreground">No matches.</p>
          )}
        </div>
      ) : filtering ? (
        <ul className="file-list m-0 flex list-none flex-col gap-2 p-0">
          {fileMatches.map((e) => (
            <li key={e.path} className="flex flex-wrap items-center gap-2">
              <Button variant="link" size="xs" onClick={() => onSelect(e.path)} onAuxClick={(ev) => handleAuxClick(ev, e.path)}>
                {e.path}
              </Button>
              {e.executable && <ExecutableBadge />}
              <FileStatusBadge status={e.status} />
            </li>
          ))}
          {fileMatches.length === 0 && <li className="text-muted-foreground">No matches.</li>}
        </ul>
      ) : onMoveFile ? (
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
      <input
        ref={uploadInputRef}
        type="file"
        multiple
        className="hidden"
        onChange={handleUpload}
        aria-label="Upload files"
      />
      {ctxMenu && (
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
                onClick={openUpload}
                data-testid="file-upload"
                disabled={uploading}
              >
                <Upload className="size-3.5" />
                Upload
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
              {taskTriggerIDFromDirectory(ctxMenu.path) && (
                <button
                  role="menuitem"
                  className="file-action-item"
                  onClick={runTriggerEntry}
                  data-testid="task-trigger-run"
                >
                  <Terminal className="size-3.5" />
                  Run trigger
                </button>
              )}
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
          {onOpenReference && (
            <button role="menuitem" className="file-action-item" onClick={openReference}>
              <PanelRightOpen className="size-3.5" />
              Open in reference pane
            </button>
          )}
          <button role="menuitem" className="file-action-item" onClick={duplicateEntry}>
            <Copy className="size-3.5" />
            Duplicate
          </button>
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
          output={execState.output}
          result={execState.result}
          error={execState.error}
          onClose={closeExecution}
        />
      )}
      {triggerRunState && (
        <TaskTriggerRunResultModal
          path={triggerRunState.path}
          running={triggerRunState.running}
          result={triggerRunState.result}
          error={triggerRunState.error}
          onClose={() => setTriggerRunState(null)}
        />
      )}
    </div>
  )
}

function TaskTriggerRunResultModal({
  path,
  running,
  result,
  error,
  onClose,
}: {
  path: string
  running: boolean
  result?: TaskTriggerRun
  error?: string
  onClose: () => void
}) {
  useEscapeKey(onClose)

  return (
    <div
      className="fixed inset-0 z-[70] flex items-center justify-center bg-black/40 p-4"
      onClick={onClose}
      role="dialog"
      aria-modal="true"
      aria-label={`Run trigger ${path}`}
    >
      <div
        className="flex w-full max-w-lg flex-col rounded-lg border bg-card shadow-xl"
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
            aria-label="Close trigger result"
          >
            <X className="size-4" />
          </button>
        </div>
        <div className="p-4">
          {running ? (
            <div className="flex items-center gap-2 text-sm text-muted-foreground">
              <Loader2 className="size-4 animate-spin" aria-hidden="true" />
              Running trigger…
            </div>
          ) : error ? (
            <div className="rounded-md border border-red-200 bg-red-50 px-3.5 py-2.5 text-sm text-red-700">
              {error}
            </div>
          ) : result ? (
            <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-2 text-sm">
              <dt className="text-muted-foreground">Status</dt>
              <dd className="font-medium">{result.status}</dd>
              <dt className="text-muted-foreground">Run ID</dt>
              <dd className="break-all font-mono text-xs">{result.id}</dd>
              {result.task_id && (
                <>
                  <dt className="text-muted-foreground">Task ID</dt>
                  <dd className="break-all font-mono text-xs">{result.task_id}</dd>
                </>
              )}
              {result.agent_name && (
                <>
                  <dt className="text-muted-foreground">Agent</dt>
                  <dd>{result.agent_name}</dd>
                </>
              )}
            </dl>
          ) : null}
        </div>
      </div>
    </div>
  )
}

function ExecuteResultModal({
  path,
  running,
  output,
  result,
  error,
  onClose,
}: {
  path: string
  running: boolean
  output: string
  result?: FileExecuteResult
  error?: string
  onClose: () => void
}) {
  const [copied, setCopied] = useState(false)

  useEscapeKey(onClose)

  useEffect(() => {
    if (!copied) return
    const timer = window.setTimeout(() => setCopied(false), 1500)
    return () => window.clearTimeout(timer)
  }, [copied])

  const copyOutput = () => {
    void copyToClipboard(output)
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
          {running && (
            <div className="flex items-center gap-2 text-sm text-muted-foreground">
              <Loader2 className="size-4 animate-spin" aria-hidden="true" />
              Running…
            </div>
          )}
          {error && (
            <div className="rounded-md border border-red-200 bg-red-50 px-3.5 py-2.5 text-sm text-red-700">
              {error}
            </div>
          )}
          {result && (
            <div className="mt-3 flex flex-col gap-3">
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
            </div>
          )}
          {(output || result) && (
            <pre className="exec-result-output mt-3 m-0 max-h-96 overflow-auto rounded-md border border-border bg-muted/40 p-3 text-xs leading-relaxed whitespace-pre-wrap break-words">
              {output || '(no output)'}
            </pre>
          )}
        </div>
      </div>
    </div>
  )
}

export interface PaneHandle {
  hasUnsavedChanges: () => boolean
  save: () => Promise<boolean>
  discardChanges: () => void
}

interface PaneProps {
  path: string
  entry?: BrowserEntry
  list: BrowserEntry[]
  favorites: string[]
  refreshMeta: () => Promise<void>
  onRecentChanged?: (path: string) => void
  onChanged: () => void
  onGitStatusChange: () => void
  onOpen: (path: string) => void
  onDeleted: () => void
  explorerOpen: boolean
  onToggleExplorer: () => void
  onRefresh: () => void
  refreshKey: number
  gitStatus?: string
  onGitDiffOpenChange?: (open: boolean) => void
  herdrOverview?: HerdrOverview | null
  refreshHerdr?: () => void
  webuiFocusedPaneId?: string | null
  onWebuiFocusChange?: (paneId: string | null) => void
  onOpenGit?: (path: string) => void
  searchHit?: FileSearchHit | null
  scrollRef?: RefObject<HTMLDivElement | null>
  paneLabel?: 'Main' | 'Reference'
  onDirtyChange?: (dirty: boolean) => void
  showExplorerToggle?: boolean
}

const Pane = forwardRef<PaneHandle, PaneProps>(function Pane({
  path,
  entry,
  list,
  favorites,
  refreshMeta,
  onRecentChanged,
  onChanged,
  onGitStatusChange,
  onOpen,
  onDeleted,
  explorerOpen,
  onToggleExplorer,
  onRefresh,
  refreshKey,
  gitStatus,
  onGitDiffOpenChange,
  herdrOverview,
  refreshHerdr,
  webuiFocusedPaneId,
  onWebuiFocusChange,
  onOpenGit,
  searchHit,
  scrollRef,
  paneLabel = 'Main',
  onDirtyChange,
  showExplorerToggle = true,
}, ref) {
  const { prompt, confirm } = useDialogs()
  const [content, setContent] = useState('')
  const [draft, setDraft] = useState('')
  const [draftFm, setDraftFm] = useState<Record<string, unknown>>({})
  const [draftBody, setDraftBody] = useState('')
  const [editing, setEditing] = useState(false)
  const [showDiff, setShowDiff] = useState(false)
  const [showGitDiff, setShowGitDiff] = useState(false)
  const [gitDiffFiles, setGitDiffFiles] = useState<GitFile[]>([])
  const [gitDiffLoading, setGitDiffLoading] = useState(false)
  const [gitDiffError, setGitDiffError] = useState<string | null>(null)
  const gitDiffRequest = useRef(0)
  const [error, setError] = useState<string | null>(null)
  const [saved, setSaved] = useState(false)
  const [crlf, setCrlf] = useState(false)
  const [isFav, setIsFav] = useState(false)
  const [menuOpen, setMenuOpen] = useState(false)
  const [taskStatusUpdating, setTaskStatusUpdating] = useState(false)
  const [copyDialogOpen, setCopyDialogOpen] = useState(false)
  const [fileCopied, setFileCopied] = useState(false)
  const menuRef = useRef<HTMLDivElement | null>(null)
  const [execState, setExecState] = useState<{ path: string; running: boolean; output: string; result?: FileExecuteResult; error?: string } | null>(null)
  const executeCancel = useRef<(() => void) | null>(null)
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
  const outlineStorageKey = paneLabel === 'Reference' ? `${OUTLINE_STORAGE_KEY}_reference` : OUTLINE_STORAGE_KEY
  const [outlineOpen, setOutlineOpen] = useState<boolean>(() => {
    const saved = window.localStorage.getItem(outlineStorageKey)
    if (saved !== null) return saved === '1'
    return window.innerWidth >= 768
  })
  const {
    width: outlineWidth,
    resizing: outlineResizing,
    handlePointerDown: handleOutlineResizeStart,
    handleKeyDown: handleOutlineResizeKeyDown,
  } = useResizableWidth({
    storageKey: DETAILS_WIDTH_STORAGE_KEY,
    defaultWidth: DEFAULT_DETAILS_WIDTH,
    handleSide: 'left',
  })

  useEffect(() => {
    window.localStorage.setItem(outlineStorageKey, outlineOpen ? '1' : '0')
  }, [outlineOpen, outlineStorageKey])

  const byPath = useMemo(() => {
    const m = new Map<string, string>()
    for (const e of list) m.set(normalizePath(e.path), e.path)
    return m
  }, [list])

  const isDir = entry?.kind === 'dir'

  const isImage =
    !isDir && /\.(png|jpe?g|gif|webp|avif|bmp|ico|svg)$/i.test(path)

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

  useEffect(() => {
    gitDiffRequest.current += 1
    setError(null)
    setEditing(false)
    setShowDiff(false)
    setShowGitDiff(false)
    setGitDiffFiles([])
    setGitDiffLoading(false)
    setGitDiffError(null)
    setSaved(false)
    setCrlf(false)
    setCopyDialogOpen(false)
    setFileCopied(false)
    editStartLine.current = null
    setIsFav(favorites.includes(path))
    if (isImage) return
    if (isDir) {
      if (readmePath) {
        void api
          .getFileContent(readmePath)
          .then((c) => {
            setContent(c.content)
            setDraft(c.content)
            setCrlf(hasCRLF(c.content))
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
    const p = api.getFileContent(path)
    void p
      .then((c) => {
        setContent(c.content)
        setDraft(c.content)
        setCrlf(hasCRLF(c.content))
        const split = splitFrontmatter(c.content)
        setDraftBody(split.body)
        setDraftFm(parseFrontmatter(split.frontmatter).data)
      })
      .catch((e) => setError(e instanceof Error ? e.message : String(e)))
    void api.recordRecent(path).then(() => onRecentChanged?.(path)).catch(() => undefined)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [path, isDir, isImage, readmePath, listing, refreshKey])

  useEffect(() => {
    if (gitStatus) return
    gitDiffRequest.current += 1
    setShowGitDiff(false)
    setGitDiffFiles([])
  }, [gitStatus])

  useEffect(() => {
    onGitDiffOpenChange?.(showGitDiff)
  }, [onGitDiffOpenChange, showGitDiff])

  const isMarkdown =
    isDir || (entry?.markdown ?? /\.(md|markdown)$/i.test(path))

  const fmSplit = useMemo(() => splitFrontmatter(content), [content])
  const fmParsed = useMemo(() => parseFrontmatter(fmSplit.frontmatter), [fmSplit])
  const useForm = isMarkdown && fmParsed.ok
  const brokenFrontmatter = isMarkdown && fmSplit.has && !fmParsed.ok

  const tags = useMemo(() => extractFrontmatterTags(content), [content])

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

  const taskID = isDir ? null : taskIDFromPath(path)
  const isTaskFile = taskID !== null
  const taskStatus = TASK_STATUS_OPTIONS.includes(fmParsed.data.status as TaskStatus)
    ? (fmParsed.data.status as TaskStatus)
    : 'todo'

  const updateTaskStatus = async (status: TaskStatus) => {
    if (!taskID || taskStatusUpdating || status === fmParsed.data.status) return
    setTaskStatusUpdating(true)
    setError(null)
    try {
      await api.updateTask(taskID, { status })
      onRefresh()
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    } finally {
      setTaskStatusUpdating(false)
    }
  }

  const archiveTask = async () => {
    if (!taskID) return
    let message = `「${String(fmParsed.data.title ?? taskID)}」をアーカイブしますか？`
    try {
      const tmpFiles = await api.listFiles({ path: `_tasks/${taskID}/tmp` })
      if (tmpFiles.length > 0) {
        const names = tmpFiles.slice(0, 5).map((file) => file.name).join(', ')
        const more = tmpFiles.length > 5 ? ` ほか${tmpFiles.length - 5}件` : ''
        message = `「${String(fmParsed.data.title ?? taskID)}」をアーカイブしますか？\nタスク内の tmp ディレクトリ（${tmpFiles.length}件: ${names}${more}）も削除されます。元に戻せません。`
      }
    } catch {
      // tmp の確認に失敗してもアーカイブ確認自体は続行する
    }
    if (!(await confirm(message))) return
    try {
      await api.archiveTask(taskID)
      onRefresh()
      onOpen('')
      await api.deleteRecent(path).catch(() => undefined)
      await refreshMeta()
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    }
  }

  const execute = () => {
    executeCancel.current?.()
    setExecState({ path, running: true, output: '' })
    executeCancel.current = api.executeFileStream(path, {
      onOutput: (chunk) => {
        setExecState((current) =>
          current?.path === path ? { ...current, output: current.output + chunk } : current,
        )
      },
      onComplete: (result) => {
        setExecState((current) =>
          current?.path === path
            ? { ...current, running: false, result: { ...result, output: current.output } }
            : current,
        )
      },
      onError: (error) => {
        setExecState((current) =>
          current?.path === path ? { ...current, running: false, error: error.message } : current,
        )
      },
    })
  }

  useEffect(() => () => executeCancel.current?.(), [])

  const closeExecution = () => {
    executeCancel.current?.()
    executeCancel.current = null
    setExecState(null)
  }

  const copyPath = async () => {
    let abs = path
    const proj = getProject()
    try {
      if (proj) {
        const projects = await api.listProjects()
        const current = projects.find((p) => p.name === proj)
        if (current?.path) abs = `${current.path.replace(/\/+$/, '')}/${path}`
      }
      await copyToClipboard(abs)
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    }
  }

  const copyRelativePath = () => {
    void copyToClipboard(path).catch((e) => setError(e instanceof Error ? e.message : String(e)))
  }

  const copyFileContents = () => {
    void copyToClipboard(content)
      .then(() => {
        setFileCopied(true)
        window.setTimeout(() => setFileCopied(false), 1500)
      })
      .catch((e) => setError(e instanceof Error ? e.message : String(e)))
  }

  const rename = async () => {
    const label = isDir ? 'folder' : 'file'
    const newPath = await prompt(`Move ${label} — enter a path relative to the project root.`, path)
    if (!newPath || !newPath.trim() || newPath.trim() === path) return
    void api
      .moveFile(path, newPath.trim())
      .then(() => {
        onChanged()
        onGitStatusChange()
        onOpen(newPath.trim())
      })
      .catch((e) => setError(e instanceof Error ? e.message : String(e)))
  }

  const newFileInDir = async () => {
    const prefix = path ? `${path}/` : ''
    const name = await prompt('New file path', prefix)
    if (!name || !name.trim()) return
    void api
      .createFile(prefix + name.trim())
      .then(() => {
        onChanged()
        onOpen(prefix + name.trim())
      })
      .catch((e) => setError(e instanceof Error ? e.message : String(e)))
  }

  const newFolderInDir = async () => {
    const prefix = path ? `${path}/` : ''
    const name = await prompt('New folder name', prefix)
    if (!name || !name.trim()) return
    void api
      .createDir(prefix + name.trim())
      .then(() => {
        onChanged()
        onOpen(prefix + name.trim())
      })
      .catch((e) => setError(e instanceof Error ? e.message : String(e)))
  }

  const openGit = () => onOpenGit?.(path)

  const toggleGitDiff = async () => {
    if (showGitDiff) {
      setShowGitDiff(false)
      return
    }
    const requestId = ++gitDiffRequest.current
    setShowGitDiff(true)
    setGitDiffLoading(true)
    setGitDiffError(null)
    try {
      const detail = await api.getGitStatus(undefined, false)
      if (requestId !== gitDiffRequest.current) return
      const files = gitFilesForPath(detail, path)
      const withDiffs = await Promise.all(
        files.map(async (file) => {
          const result = await api.getGitFileDiff(undefined, path, file.status)
          return { ...file, diff: result.diff }
        }),
      )
      if (requestId !== gitDiffRequest.current) return
      setGitDiffFiles(withDiffs)
    } catch (e) {
      if (requestId !== gitDiffRequest.current) return
      setGitDiffError(e instanceof Error ? e.message : String(e))
    } finally {
      if (requestId === gitDiffRequest.current) setGitDiffLoading(false)
    }
  }

  const persist = async (raw: string): Promise<boolean> => {
    setSaved(false)
    const out = normalizeLineEndings(raw)
    try {
      await api.saveFileContent(readmePath ?? path, out)
      setContent(out)
      setDraft(out)
      setCrlf(hasCRLF(out))
      const split = splitFrontmatter(out)
      setDraftBody(split.body)
      setDraftFm(parseFrontmatter(split.frontmatter).data)
      setEditing(false)
      setShowDiff(false)
      setShowGitDiff(false)
      setGitDiffFiles([])
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

  const save = async (): Promise<boolean> => {
    const raw = useForm ? buildMarkdown(serializeFrontmatter(draftFm), draftBody) : draft
    return persist(raw)
  }

  const discardChanges = () => {
    setDraft(content)
    const split = splitFrontmatter(content)
    setDraftBody(split.body)
    setDraftFm(parseFrontmatter(split.frontmatter).data)
    setEditing(false)
    setShowDiff(false)
    setShowGitDiff(false)
    setGitDiffFiles([])
  }

  const viewText = useForm ? fmSplit.body : content
  const taskProgress = isTaskFile ? extractMarkdownTaskProgress(viewText) : null
  const viewSourceLineOffset = useForm
    ? content.slice(0, Math.max(0, content.length - fmSplit.body.length)).split(/\r?\n/).length - 1
    : 0
  const activeSearchHit = searchHit?.path === path ? searchHit : null
  const viewRelativeTo = isDir ? readmePath ?? `${path}/` : path
  const viewFileName = isDir && readmePath ? baseOf(readmePath) : null

  const toggleTask = async (line: number, checked: boolean): Promise<boolean> => {
    const nextBody = setMarkdownTaskChecked(viewText, line, checked)
    if (nextBody === null) return false
    const raw = useForm ? replaceMarkdownBody(content, nextBody) : nextBody
    return persist(raw)
  }

  const diffModified = useMemo(
    () => (useForm ? buildMarkdown(serializeFrontmatter(draftFm), draftBody) : draft),
    [useForm, draftFm, draftBody, draft],
  )
  const diffOriginal = content
  const diffOriginalBody = fmSplit.body
  const hasUnsavedChanges = diffOriginal !== diffModified

  useImperativeHandle(
    ref,
    () => ({
      hasUnsavedChanges: () => hasUnsavedChanges,
      save,
      discardChanges,
    }),
    [hasUnsavedChanges, save],
  )

  useEffect(() => {
    onDirtyChange?.(editing && hasUnsavedChanges)
  }, [editing, hasUnsavedChanges, onDirtyChange])

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
    const scroller = scrollRef?.current ?? null
    editStartLine.current = computeViewStartLine(scroller, viewText)
    viewScroll.current = scroller
      ? { path, top: scroller.scrollTop, maxTop: Math.max(0, scroller.scrollHeight - scroller.clientHeight) }
      : null
    setShowDiff(true)
    setShowGitDiff(false)
    setEditing(true)
  }

  useEffect(() => {
    if (editing) return
    const snap = viewScroll.current
    viewScroll.current = null
    if (!snap || snap.path !== path) return
    // Wait until the view has been re-rendered and laid out before restoring.
    requestAnimationFrame(() => {
    const scroller = scrollRef?.current ?? null
      if (!scroller) return
      const maxTop = Math.max(0, scroller.scrollHeight - scroller.clientHeight)
      scroller.scrollTop = snap.maxTop > 0 && maxTop > 0 ? (snap.top / snap.maxTop) * maxTop : snap.top
    })
  }, [editing, path, scrollRef])

  const outlineItems = useMemo(() => extractOutline(viewText), [viewText])

  useEffect(() => {
    const container = scrollRef?.current
    const els = container
      ? outlineItems.map((h) => Array.from(container.querySelectorAll<HTMLElement>('[id]')).find((element) => element.id === h.id) ?? null)
      : []
    const validEls = els.filter((el): el is HTMLElement => el !== null)
    if (validEls.length === 0) {
      setActiveHeading(null)
      return
    }
    const update = () => {
      let current: string | null = null
      for (const el of validEls) {
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
  }, [outlineItems, scrollRef])

  const sectionTitle = 'mb-3 text-xs font-semibold tracking-wider text-muted-foreground uppercase'

  const outlinePanel = (
    <>
      <div className="outline-header flex items-center border-b border-border px-4 py-3 pr-10">
        <div className="min-w-0">
          <div className="text-sm font-semibold leading-tight">Outline</div>
        </div>
      </div>
      <div className="outline-body min-h-0 flex-1 overflow-y-auto px-3 py-3">
        <div className="outline-section mb-4 flex flex-col gap-1 border-b border-border pb-4 last:mb-0 last:border-b-0 last:pb-0">
          <div className="outline-title mb-1 flex items-center gap-1.5 text-xs font-semibold tracking-wider text-muted-foreground uppercase">
            <ListTree className="size-3.5" />
            Contents
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
                const target = scrollRef?.current
                  ? Array.from(scrollRef.current.querySelectorAll<HTMLElement>('[id]')).find((element) => element.id === h.id)
                  : null
                target?.scrollIntoView({ behavior: 'smooth' })
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
    <div
      className={cn('min-w-0 w-full md:transition-[padding-right] md:duration-200 md:ease-linear', (editing || showGitDiff) && 'flex h-full flex-col')}
      style={{ paddingRight: outlineOpen && !isMobile ? outlineWidth : undefined }}
    >
      {herdrOverview !== undefined && (
        <FileAgentWidget
          path={path}
          overview={herdrOverview}
          onRefresh={refreshHerdr ?? (() => undefined)}
          webuiFocusedPaneId={webuiFocusedPaneId}
          onWebuiFocusChange={onWebuiFocusChange}
        />
      )}
      <div className={cn('knowledge-body flex gap-4 max-md:flex-col', (editing || showGitDiff) && 'min-h-0 flex-1')}>
        <div className={cn('knowledge-main min-w-0 flex-1 md:transition-[margin]', (editing || showGitDiff) && 'flex min-h-0 flex-col')}>
          <div className="page-header note-toolbar sticky top-0 z-20 mb-3 flex shrink-0 items-center justify-between gap-3 border-b bg-card/95 py-2 backdrop-blur">
            <div className="actions flex flex-wrap gap-2">
              <span className="self-center px-1 text-xs font-semibold tracking-wider text-muted-foreground uppercase">
                {paneLabel}
              </span>
              {showExplorerToggle && (
                <Button
                  variant="ghost"
                  size="sm"
                  aria-label="Toggle file explorer"
                  title={explorerOpen ? 'Hide file explorer' : 'Show file explorer'}
                  onClick={onToggleExplorer}
                >
                  {explorerOpen ? <PanelLeftClose /> : <PanelLeftOpen />}
                </Button>
              )}
              <Button
                variant="ghost"
                size="sm"
                aria-label="Toggle details"
                title={outlineOpen ? 'Hide details' : 'Show details'}
                onClick={() => setOutlineOpen((o) => !o)}
              >
                {outlineOpen ? <PanelRightClose /> : <PanelRightOpen />}
              </Button>
              <Button
                variant="ghost"
                size="sm"
                aria-label={isFav ? '★ Favorite' : '☆ Favorite'}
                title={isFav ? '★ Favorite' : '☆ Favorite'}
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
              {taskID && !editing && (
                <label className="flex items-center gap-1.5 text-xs text-muted-foreground">
                  <span>Status</span>
                  <select
                    value={taskStatus}
                    onChange={(e) => void updateTaskStatus(e.target.value as TaskStatus)}
                    disabled={taskStatusUpdating}
                    aria-label="Task status"
                    className="h-8 rounded-md border border-input bg-card px-2 text-sm text-foreground outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 disabled:cursor-not-allowed disabled:opacity-50"
                  >
                    {TASK_STATUS_OPTIONS.map((status) => (
                      <option key={status} value={status}>
                        {status}
                      </option>
                    ))}
                  </select>
                </label>
              )}
              {!editing && !isDir && gitStatus && (
                <Button
                  variant={showGitDiff ? 'default' : 'ghost'}
                  size="sm"
                  aria-label={showGitDiff ? 'Hide Git diff' : 'Show Git diff'}
                  title={showGitDiff ? 'Hide Git diff' : 'Show Git diff'}
                  onClick={() => void toggleGitDiff()}
                  disabled={gitDiffLoading}
                  data-testid="file-git-diff-button"
                >
                  <GitBranch />
                  <span className="hidden sm:inline">Git</span>
                </Button>
              )}
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
                      className="absolute right-0 top-full z-50 mt-1 min-w-44 origin-top rounded-md border bg-popover p-1 text-popover-foreground shadow-md"
                    >
                      {isDir && (
                        <>
                          <button role="menuitem" className="file-action-item" onClick={() => runAction(newFileInDir)}>
                            <FilePlus className="size-3.5" />
                            New File
                          </button>
                          <button role="menuitem" className="file-action-item" onClick={() => runAction(newFolderInDir)}>
                            <FolderHeart className="size-3.5" />
                            New Folder
                          </button>
                          <button role="menuitem" className="file-action-item" onClick={() => runAction(openGit)} data-testid="file-open-git">
                            <GitBranch className="size-3.5" />
                            Open Git
                          </button>
                          <div className="my-1 h-px bg-border" />
                        </>
                      )}
                      {!isDir && entry?.executable && (
                        <>
                          <button
                            role="menuitem"
                            className="file-action-item"
                            onClick={() => runAction(execute)}
                            data-testid="file-execute"
                            title="Run the file and show its output"
                          >
                            <Terminal className="size-3.5" />
                            Execute
                          </button>
                          <div className="my-1 h-px bg-border" />
                        </>
                      )}
                      {taskID && !editing && (
                        <>
                          <button
                            role="menuitem"
                            className="file-action-item"
                            onClick={() => runAction(archiveTask)}
                            data-testid="task-archive"
                          >
                            <Archive className="size-3.5" />
                            Archive task
                          </button>
                          <div className="my-1 h-px bg-border" />
                        </>
                      )}
                      <button role="menuitem" className="file-action-item" onClick={() => runAction(duplicate)}>
                        <Copy className="size-3.5" />
                        Duplicate
                      </button>
                      <button role="menuitem" className="file-action-item" onClick={() => runAction(copyPath)}>
                        <Text className="size-3.5" />
                        Copy Path
                      </button>
                      <button role="menuitem" className="file-action-item" onClick={() => runAction(copyRelativePath)}>
                        <Text className="size-3.5" />
                        Copy Relative Path
                      </button>
                      <div className="my-1 h-px bg-border" />
                      <button role="menuitem" className="file-action-item" onClick={() => runAction(rename)}>
                        <Text className="size-3.5" />
                        Move
                      </button>
                      <button
                        role="menuitem"
                        className="file-action-item text-destructive"
                        onClick={() => runAction(remove)}
                      >
                        <Trash2 className="size-3.5" />
                        Delete
                      </button>
                    </div>
                  )}
                </div>
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
                        discardChanges()
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
            <div className={cn('error-banner my-2 flex items-center justify-between rounded-md border border-red-200 bg-red-50 px-3.5 py-2.5 text-sm text-red-700', editing && 'shrink-0')}>
              {error}
            </div>
          )}
          {brokenFrontmatter && (
            <div className={cn('my-2 flex items-start justify-between gap-2 rounded-md border border-red-200 bg-red-50 px-3.5 py-2.5 text-sm text-red-700', editing && 'shrink-0')} data-testid="frontmatter-error">
              <div className="min-w-0">
                <p className="font-semibold">このファイルのYAML front matterを解析できません</p>
                <p className="mt-0.5 break-all text-muted-foreground">{fmParsed.error}</p>
              </div>
            </div>
          )}
          {crlf && (
            <div className={cn('my-2 flex items-center justify-between gap-2 rounded-md border border-amber-200 bg-amber-50 px-3.5 py-2 text-sm text-amber-700', editing && 'shrink-0')}>
              <span>CRLF改行を検出しました。保存時にLFへ変換されます。</span>
              <button
                type="button"
                aria-label="Dismiss CRLF warning"
                className="shrink-0 text-amber-600 hover:text-amber-800"
                onClick={() => setCrlf(false)}
              >
                <X className="size-4" />
              </button>
            </div>
          )}
          {saved && (
            <div className={cn('notice my-2 rounded-md border border-green-200 bg-green-50 px-3.5 py-2 text-sm text-green-700', editing && 'shrink-0')}>
              Saved.
            </div>
          )}
          {editing ? (
            <Card className="min-h-0 flex-1 gap-0 p-4">
              {useForm ? (
                <>
                  <CardContent className="shrink-0 border-t px-0 pt-4">
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
                  <CardContent className="mt-4 flex min-h-0 flex-1 flex-col border-t px-0 pt-4">
                    <h3 className={sectionTitle}>Body</h3>
                    {showDiff ? (
                      <MonacoEditor
                        className="editor mt-1 min-h-0 flex-1"
                        value={draftBody}
                        onChange={setDraftBody}
                        path={path}
                        language="markdown"
                        ariaLabel="File editor"
                        height="100%"
                        original={diffOriginalBody}
                        initialLine={editStartLine.current ?? undefined}
                        completions={list}
                      />
                    ) : (
                      <MonacoEditor
                        className="editor mt-1 min-h-0 flex-1"
                        value={draftBody}
                        onChange={setDraftBody}
                        path={path}
                        language="markdown"
                        ariaLabel="File editor"
                        height="100%"
                        initialLine={editStartLine.current ?? undefined}
                        completions={list}
                      />
                    )}
                  </CardContent>
                </>
              ) : (
                <CardContent className="flex min-h-0 flex-1 flex-col px-0 py-0">
                  {showDiff ? (
                    <MonacoEditor
                      className="editor min-h-0 flex-1"
                      value={draft}
                      onChange={setDraft}
                      path={path}
                      ariaLabel="File editor"
                      height="100%"
                      original={diffOriginal}
                      initialLine={editStartLine.current ?? undefined}
                      completions={list}
                    />
                  ) : (
                    <MonacoEditor
                      className="editor min-h-0 flex-1"
                      value={draft}
                      onChange={setDraft}
                      path={path}
                      ariaLabel="File editor"
                      height="100%"
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
            <div className={cn('grid min-w-0 gap-4', showGitDiff && 'min-h-0 flex-1 grid-rows-2 lg:grid-cols-2 lg:grid-rows-1')}>
              <Card data-testid={showGitDiff ? 'git-file-content-pane' : undefined} className={cn('min-w-0 gap-0 p-4', showGitDiff && 'min-h-0 overflow-y-auto')}>
                <CardContent className="px-0 py-0">
                  <div className="meta-line my-2 flex items-center gap-2">
                    <div className="flex min-w-0 flex-1 flex-wrap items-center gap-2">
                      <span className="muted text-sm text-muted-foreground">{path}</span>
                      {tags.map((t) => (
                        <TagBadge key={t}>{t}</TagBadge>
                      ))}
                    </div>
                    {(isMarkdown || (!isDir && !isImage)) && (
                      <Button
                        variant="ghost"
                        size="icon-sm"
                        className="ml-auto"
                        onClick={isMarkdown ? () => setCopyDialogOpen(true) : copyFileContents}
                        aria-label="Copy file contents"
                        title="Copy file contents"
                      >
                        {fileCopied ? <Check className="size-4" /> : <Copy className="size-4" />}
                      </Button>
                    )}
                  </div>
                  {viewFileName && (
                    <>
                      <Separator className="my-2" />
                      <div className="view-file-name mb-2 text-sm font-semibold text-muted-foreground">{viewFileName}</div>
                    </>
                  )}
                  {useForm && <FrontmatterSummary data={fmParsed.data} />}
                  {taskProgress && <TaskProgress {...taskProgress} className="my-3" />}
                  {isMarkdown ? (
                    <RichMarkdown
                      text={viewText}
                      relativeTo={viewRelativeTo}
                      linkUrl={filesUrl}
                      imageUrl={rawFileUrl}
                      dataUrl={rawFileUrl}
                      preserveExtension
                      onTaskToggle={!isDir || Boolean(readmePath) ? toggleTask : undefined}
                      copyDialogOpen={copyDialogOpen}
                      onCopyDialogClose={() => setCopyDialogOpen(false)}
                      focusLine={activeSearchHit?.line}
                      searchQuery={activeSearchHit?.query}
                      sourceLineOffset={viewSourceLineOffset}
                      onOpenFile={onOpen}
                    />
                  ) : isImage ? (
                    <div className="file-image flex justify-center py-3">
                      <img src={rawFileUrl(path)} alt={baseOf(path)} className="max-w-full rounded-md" />
                    </div>
                  ) : (
                    <SyntaxHighlighter
                      text={viewText}
                      language={languageFromPath(viewFileName ?? path)}
                      className="file-raw rounded-md bg-muted p-3"
                      focusLine={activeSearchHit?.line}
                      searchQuery={activeSearchHit?.query}
                    />
                  )}
                </CardContent>
              </Card>
              {showGitDiff && (
                <Card data-testid="git-file-diff-pane" className="min-h-0 min-w-0 gap-0 overflow-y-auto p-4">
                  <CardContent className="px-0 py-0">
                    <h2 className="mb-3 text-sm font-semibold">Git diff</h2>
                    <GitDiffPanel files={gitDiffFiles} loading={gitDiffLoading} error={gitDiffError} />
                  </CardContent>
                </Card>
              )}
            </div>
          )}
        </div>
        {!isMobile && (
          <div
            data-outline-open={outlineOpen ? 'true' : 'false'}
            className={cn(
              'outline-pane absolute right-0 top-0 bottom-0 z-40 hidden overflow-hidden md:block',
              !outlineResizing && 'transition-[width] duration-200 ease-linear',
            )}
            style={{ width: outlineOpen ? outlineWidth : 0 }}
          >
            <aside
              aria-hidden={!outlineOpen}
              className={cn(
                'outline absolute top-0 right-0 flex h-full flex-col overflow-hidden border-l border-border bg-card transition-transform duration-200 ease-linear',
                outlineOpen ? 'translate-x-0' : 'translate-x-full',
              )}
              style={{ width: outlineWidth }}
            >
              {outlinePanel}
            </aside>
            {outlineOpen && (
              <div
                data-testid="details-resize-handle"
                role="separator"
                aria-label="Resize details"
                aria-orientation="vertical"
                aria-valuemin={MIN_RESIZABLE_WIDTH}
                aria-valuemax={MAX_RESIZABLE_WIDTH}
                aria-valuenow={outlineWidth}
                tabIndex={0}
                className={cn(
                  'absolute top-0 left-0 z-10 h-full w-2 cursor-col-resize touch-none rounded-sm outline-none hover:bg-primary/20 focus-visible:bg-primary/30',
                  outlineResizing && 'bg-primary/30',
                )}
                onPointerDown={handleOutlineResizeStart}
                onKeyDown={handleOutlineResizeKeyDown}
              />
            )}
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
      {execState && (
        <ExecuteResultModal
          path={execState.path}
          running={execState.running}
          output={execState.output}
          result={execState.result}
          error={execState.error}
          onClose={closeExecution}
        />
      )}
    </div>
  )
})

function ViewerPaneHeader({
  label,
  path,
  onClose,
  onSwap,
}: {
  label: 'Main' | 'Reference'
  path: string
  onClose?: () => void
  onSwap?: () => void
}) {
  const name = path.split('/').pop() ?? path
  return (
    <div className="flex min-h-10 shrink-0 items-center gap-2 border-b border-border bg-card px-3 py-2">
      <span className="shrink-0 text-xs font-semibold tracking-wider text-muted-foreground uppercase">{label}</span>
      <span className="min-w-0 flex-1 truncate text-sm" title={path}>{name}</span>
      {onSwap && (
        <Button
          variant="ghost"
          size="icon-sm"
          aria-label="Swap main and reference panes"
          title="Swap main and reference panes"
          onClick={onSwap}
        >
          <ArrowLeftRight className="size-4" />
        </Button>
      )}
      {onClose && (
        <Button
          variant="ghost"
          size="icon-sm"
          aria-label="Close reference pane"
          title="Close reference pane"
          onClick={onClose}
        >
          <X className="size-4" />
        </Button>
      )}
    </div>
  )
}

function UnsavedRouteGuard({
  mainDirty,
  mainPath,
  mainPaneRef,
  referenceDirty,
  referencePath,
  referencePaneRef,
}: {
  mainDirty: boolean
  mainPath: string
  mainPaneRef: RefObject<PaneHandle>
  referenceDirty: boolean
  referencePath: string | null
  referencePaneRef: RefObject<PaneHandle>
}) {
  const { confirm3 } = useDialogs()
  const blocker = useBlocker(mainDirty || referenceDirty)

  useEffect(() => {
    if (blocker.state !== 'blocked') return
    let active = true

    const confirmLeave = async (paneRef: RefObject<PaneHandle>, dirty: boolean, path: string) => {
      if (!dirty) return true
      const choice = await confirm3(
        `「${path}」には保存されていない変更があります。どうしますか？`,
        { primary: '保存して移動', secondary: '変更を破棄', cancel: 'キャンセル' },
      )
      if (choice === 'primary') return paneRef.current?.save() ?? true
      if (choice === 'secondary') {
        paneRef.current?.discardChanges()
        return true
      }
      return false
    }

    void (async () => {
      if (!(await confirmLeave(mainPaneRef, mainDirty, mainPath))) {
        if (active) blocker.reset()
        return
      }
      if (referencePath && !(await confirmLeave(referencePaneRef, referenceDirty, referencePath))) {
        if (active) blocker.reset()
        return
      }
      if (active) blocker.proceed()
    })()

    return () => {
      active = false
    }
    // The blocker state intentionally starts one confirmation flow per blocked navigation.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [blocker.state])

  return null
}

export function BrowserPage({
  title,
  selected,
  onSelect,
  onBack,
  favorites,
  recentFiles,
  refreshMeta,
  onRecentChanged,
  defaultSelect,
  onClose,
  herdrOverview,
  refreshHerdr,
  webuiFocusedPaneId,
  onWebuiFocusChange,
  revealPath,
  onRevealPathHandled,
}: BrowserPageProps) {
  const { confirm3 } = useDialogs()
  const [childrenByDir, setChildrenByDir] = useState<Record<string, BrowserEntry[]>>({})
  const [gitStatus, setGitStatus] = useState<Record<string, string>>({})
  const [loaded, setLoaded] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [moveError, setMoveError] = useState<string | null>(null)
  const [refreshKey, setRefreshKey] = useState(0)
  const [openGitDir, setOpenGitDir] = useState<string | null>(null)
  const [gitDiffOpen, setGitDiffOpen] = useState(false)
  const [referenceGitDiffOpen, setReferenceGitDiffOpen] = useState(false)
  const [referencePath, setReferencePath] = useState<string | null>(null)
  const [mobileViewer, setMobileViewer] = useState<'main' | 'reference'>('main')
  const [mainDirty, setMainDirty] = useState(false)
  const [referenceDirty, setReferenceDirty] = useState(false)
  const mainScrollRef = useRef<HTMLDivElement>(null)
  const referenceScrollRef = useRef<HTMLDivElement>(null)
  const mainPaneRef = useRef<PaneHandle>(null)
  const referencePaneRef = useRef<PaneHandle>(null)
  const pendingSwapReferenceRef = useRef<string | null>(null)
  const selectedRef = useRef(selected)
  const [searchHit, setSearchHit] = useState<FileSearchHit | null>(null)
  const autoDefaulted = useRef(false)
  const loadedDirsRef = useRef(new Set<string>())
  const loadingDirsRef = useRef(new Set<string>())
  const autoLoadSeq = useRef(0)
  const gitStatusLoaded = useRef(false)
  const fullTreeCacheRef = useRef<{ showHidden: boolean; grouped: Record<string, BrowserEntry[]> } | null>(null)
  const fullTreeRequestRef = useRef<{
    showHidden: boolean
    promise: Promise<Record<string, BrowserEntry[]>>
  } | null>(null)

  const {
    width: referenceWidth,
    resizing: referenceResizing,
    handlePointerDown: handleReferenceResizeStart,
    handleKeyDown: handleReferenceResizeKeyDown,
  } = useResizableWidth({
    storageKey: REFERENCE_WIDTH_STORAGE_KEY,
    defaultWidth: 420,
    minWidth: 280,
    maxWidth: 720,
    handleSide: 'left',
  })

  const entries = useMemo(() => Object.values(childrenByDir).flat(), [childrenByDir])

  const isMobile = useIsMobile()
  const [explorerOpen, setExplorerOpen] = useState<boolean>(() => {
    if (typeof window === 'undefined') return true
    if (window.innerWidth < 768) return false
    const saved = window.localStorage.getItem(EXPLORER_STORAGE_KEY)
    if (saved !== null) return saved === '1'
    return true
  })
  const {
    width: explorerWidth,
    resizing: explorerResizing,
    handlePointerDown: handleExplorerResizeStart,
    handleKeyDown: handleExplorerResizeKeyDown,
  } = useResizableWidth({
    storageKey: EXPLORER_WIDTH_STORAGE_KEY,
    defaultWidth: DEFAULT_EXPLORER_WIDTH,
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
    selectedRef.current = selected
  }, [selected])

  useEffect(() => {
    setOpenGitDir(null)
    setGitDiffOpen(false)
  }, [selected])

  const confirmPaneLeave = useCallback(
    async (paneRef: RefObject<PaneHandle>, dirty: boolean, path: string) => {
      if (!dirty) return true
      const choice = await confirm3(
        `「${path}」には保存されていない変更があります。どうしますか？`,
        { primary: '保存して移動', secondary: '変更を破棄', cancel: 'キャンセル' },
      )
      if (choice === 'primary') return paneRef.current?.save() ?? true
      if (choice === 'secondary') {
        paneRef.current?.discardChanges()
        return true
      }
      return false
    },
    [confirm3],
  )

  const handleSelect = useCallback(
    (p: string, nextSearchHit: FileSearchHit | null = null) => {
      setSearchHit(nextSearchHit)
      if (p === referencePath && p !== selected) {
        void confirmPaneLeave(referencePaneRef, referenceDirty, referencePath).then((ok) => {
          if (!ok) return
          setReferencePath(null)
          setReferenceDirty(false)
          onSelect(p)
          if (isMobile) setExplorerOpen(false)
        })
        return
      }
      onSelect(p)
      if (isMobile) setExplorerOpen(false)
    },
    [confirmPaneLeave, isMobile, onSelect, referenceDirty, referencePath, selected],
  )

  const changeReference = useCallback(
    async (nextPath: string | null) => {
      if (nextPath === referencePath) return true
      if (nextPath && nextPath === selected) return false
      if (referencePath && !(await confirmPaneLeave(referencePaneRef, referenceDirty, referencePath))) return false
      setReferencePath(nextPath)
      setReferenceGitDiffOpen(false)
      setReferenceDirty(false)
      if (nextPath) setMobileViewer('reference')
      else setMobileViewer('main')
      return true
    },
    [confirmPaneLeave, referenceDirty, referencePath, selected],
  )

  const openReference = useCallback(
    (path: string) => {
      if (!selected) {
        handleSelect(path)
        return
      }
      void changeReference(path)
    },
    [changeReference, handleSelect, selected],
  )

  const closeReference = useCallback(() => {
    void changeReference(null)
  }, [changeReference])

  const swapPanes = useCallback(async () => {
    if (!referencePath || !selected) return
    if (!(await confirmPaneLeave(mainPaneRef, mainDirty, selected))) return
    if (!(await confirmPaneLeave(referencePaneRef, referenceDirty, referencePath))) return
    const nextMain = referencePath
    const nextReference = selected
    pendingSwapReferenceRef.current = nextReference
    setMainDirty(false)
    setReferenceDirty(false)
    setReferenceGitDiffOpen(false)
    onSelect(nextMain)
  }, [confirmPaneLeave, mainDirty, onSelect, referenceDirty, referencePath, selected])

  useEffect(() => {
    const pendingReference = pendingSwapReferenceRef.current
    if (pendingReference && selected && referencePath === selected) {
      pendingSwapReferenceRef.current = null
      setReferencePath(pendingReference)
      return
    }
    if (!selected || referencePath === selected) {
      if (referencePath === selected) {
        setReferencePath(null)
        setReferenceDirty(false)
      }
      if (!selected) setMobileViewer('main')
    }
  }, [referencePath, selected])

  const handleSearchHit = useCallback(
    (hit: FileSearchHit) => {
      handleSelect(hit.path, hit)
    },
    [handleSelect],
  )

  const loadDir = useCallback(
    (dir: string, force = false) => {
      if (!force && (loadedDirsRef.current.has(dir) || loadingDirsRef.current.has(dir))) {
        return Promise.resolve()
      }
      if (loadingDirsRef.current.has(dir)) return Promise.resolve()
      loadingDirsRef.current.add(dir)
      return api
        .listFiles({ path: dir, showHidden })
        .then((list) => {
          const children = toEntries(list)
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
    [showHidden],
  )

  const loadFullTree = useCallback(() => {
    // Markdown link completion needs recursive entries; share both the cache
    // and any in-flight request with the page/editor instead of scanning twice.
    const cached = fullTreeCacheRef.current
    if (cached?.showHidden === showHidden) return Promise.resolve(cached.grouped)

    const pending = fullTreeRequestRef.current
    if (pending?.showHidden === showHidden) return pending.promise

    const promise = api
      .listFiles({ showHidden })
      .then((list) => {
        const grouped = groupEntriesByDirectory(list)
        fullTreeCacheRef.current = { showHidden, grouped }
        return grouped
      })
    const request = { showHidden, promise }
    fullTreeRequestRef.current = request
    void promise.then(
      () => {
        if (fullTreeRequestRef.current === request) fullTreeRequestRef.current = null
      },
      () => {
        if (fullTreeRequestRef.current === request) fullTreeRequestRef.current = null
      },
    )
    return promise
  }, [showHidden])

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
    loadedDirsRef.current.clear()
    loadingDirsRef.current.clear()
    setLoaded(false)
    setChildrenByDir({})
    if (selectedRef.current && /\.(md|markdown)$/i.test(selectedRef.current)) {
      void loadFullTree()
        .then((grouped) => {
          for (const parent of Object.keys(grouped)) loadedDirsRef.current.add(parent)
          setChildrenByDir(grouped)
          setLoaded(true)
        })
        .catch((e) => {
          setError(e instanceof Error ? e.message : String(e))
          setLoaded(true)
        })
      return
    }
    void loadDir('').then(() => setLoaded(true))
  }, [loadDir, loadFullTree])

  const refreshGitStatus = useCallback(() => {
    void api
      .getFileGitStatus()
      .then(setGitStatus)
      .catch(() => undefined)
  }, [])

  const handleChanged = useCallback((pathToReveal?: string) => {
    fullTreeCacheRef.current = null
    const directories = new Set<string>([''])
    if (selected) {
      const dir = selected.includes('/') ? selected.slice(0, selected.lastIndexOf('/')) : ''
      if (dir) directories.add(dir)
    }
    if (pathToReveal) {
      const parts = pathToReveal.split('/').filter(Boolean)
      let prefix = ''
      for (let i = 0; i < parts.length - 1; i++) {
        prefix = prefix ? `${prefix}/${parts[i]}` : parts[i]
        directories.add(prefix)
      }
    }
    return Promise.all([...directories].map((dir) => loadDir(dir, true))).then(() => undefined)
  }, [loadDir, selected])

  const revealFile = useCallback(
    async (path: string) => {
      const parts = path.split('/').filter(Boolean)
      if (parts.length === 0) return
      const directories = ['']
      let prefix = ''
      for (let i = 0; i < parts.length - 1; i++) {
        prefix = prefix ? `${prefix}/${parts[i]}` : parts[i]
        directories.push(prefix)
      }
      for (const dir of directories) await loadDir(dir, true)
      onSelect(path)
    },
    [loadDir, onSelect],
  )

  const handleRefresh = useCallback(() => {
    fullTreeCacheRef.current = null
    const dirs = [...loadedDirsRef.current]
    loadingDirsRef.current.clear()
    for (const d of dirs) void loadDir(d, true)
    refreshGitStatus()
    setRefreshKey((k) => k + 1)
  }, [loadDir, refreshGitStatus])

  useEffect(() => {
    load()
  }, [load])

  useEffect(() => {
    if (!selected || !/\.(md|markdown)$/i.test(selected)) return
    if (fullTreeCacheRef.current?.showHidden === showHidden) return
    void loadFullTree()
      .then((grouped) => {
        if (fullTreeCacheRef.current?.showHidden !== showHidden) return
        for (const parent of Object.keys(grouped)) loadedDirsRef.current.add(parent)
        setChildrenByDir((prev) => {
          let changed = false
          for (const [dir, children] of Object.entries(grouped)) {
            if (prev[dir] !== children) {
              changed = true
              break
            }
          }
          return changed ? { ...prev, ...grouped } : prev
        })
      })
      .catch((e) => setError(e instanceof Error ? e.message : String(e)))
  }, [loadFullTree, selected, showHidden])

  useEffect(() => {
    if (!revealPath) return
    let cancelled = false
    void revealFile(revealPath).then(() => {
      if (!cancelled) onRevealPathHandled?.(revealPath)
    })
    return () => {
      cancelled = true
    }
  }, [revealPath, revealFile, onRevealPathHandled])

  // Fetch the ancestor directories of the selected entry one by one so a deep
  // file can be revealed without ever walking the whole tree. When a directory
  // itself is selected, also load its children (used by the directory listing
  // in the pane).
  useEffect(() => {
    if (!selected) return
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
  }, [selected, loadDir, entries])

  useEffect(() => {
    if (!referencePath) return
    const parts = referencePath.split('/').filter(Boolean)
    const targets: string[] = []
    let prefix = ''
    for (let i = 0; i < parts.length - 1; i++) {
      prefix = prefix ? `${prefix}/${parts[i]}` : parts[i]
      targets.push(prefix)
    }
    const seq = ++autoLoadSeq.current
    ;(async () => {
      for (const target of targets) {
        if (autoLoadSeq.current !== seq) return
        if (!loadedDirsRef.current.has(target)) await loadDir(target)
      }
    })()
  }, [referencePath, loadDir])

  useEffect(() => {
    if (gitStatusLoaded.current) return
    gitStatusLoaded.current = true
    void api
      .getFileGitStatus()
      .then(setGitStatus)
      .catch(() => undefined)
  }, [])

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
  const referenceEntry = referencePath ? entries.find((e) => e.path === referencePath) : undefined

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

  const renderPane = (
    slot: 'main' | 'reference',
    path: string,
    entry: BrowserEntry | undefined,
    scrollRef: RefObject<HTMLDivElement | null>,
    paneRef: RefObject<PaneHandle>,
  ) => (
    <Pane
      ref={paneRef}
      path={path}
      entry={entry}
      list={entries}
      favorites={favorites}
      refreshMeta={refreshMeta}
      onRecentChanged={onRecentChanged}
      onChanged={handleChanged}
      onGitStatusChange={refreshGitStatus}
      onOpen={slot === 'main' ? handleSelect : (nextPath) => void changeReference(nextPath)}
      onDeleted={slot === 'main' ? onBack : closeReference}
      explorerOpen={explorerOpen}
      onToggleExplorer={() => setExplorerOpen((o) => !o)}
      onRefresh={handleRefresh}
      refreshKey={refreshKey}
      gitStatus={gitStatus[path]}
      onGitDiffOpenChange={slot === 'main' ? setGitDiffOpen : setReferenceGitDiffOpen}
      herdrOverview={slot === 'main' ? herdrOverview : undefined}
      refreshHerdr={slot === 'main' ? refreshHerdr : undefined}
      webuiFocusedPaneId={webuiFocusedPaneId}
      onWebuiFocusChange={onWebuiFocusChange}
      onOpenGit={setOpenGitDir}
      searchHit={slot === 'main' ? searchHit : null}
      scrollRef={scrollRef}
      paneLabel={slot === 'main' ? 'Main' : 'Reference'}
      onDirtyChange={slot === 'main' ? setMainDirty : setReferenceDirty}
      showExplorerToggle={slot === 'main'}
    />
  )

  return (
    <div className="page relative h-full min-h-0">
      {selected && (mainDirty || referenceDirty) && (
        <UnsavedRouteGuard
          mainDirty={mainDirty}
          mainPath={selected}
          mainPaneRef={mainPaneRef}
          referenceDirty={referenceDirty}
          referencePath={referencePath}
          referencePaneRef={referencePaneRef}
        />
      )}
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
                'explorer-pane relative hidden shrink-0 overflow-hidden border-r border-border bg-card md:block',
                !explorerResizing && 'transition-[width] duration-200 ease-linear',
              )}
              style={{ width: explorerOpen ? explorerWidth : 0 }}
            >
              <div className="flex h-full w-full min-w-0 flex-col overflow-hidden">
                <div className="flex-1 min-h-0 overflow-y-auto">
<Explorer
                    entries={entries}
                    selected={selected}
                    onSelect={handleSelect}
                    title={title}
                    favorites={favorites}
                    recentFiles={recentFiles}
                    gitStatus={gitStatus}
                    onClose={onClose}
                    onMoveFile={handleMoveFile}
                    onChanged={handleChanged}
                    onError={setError}
                    showHidden={showHidden}
                    onToggleHidden={() => setShowHidden((s) => !s)}
                    onOpenGit={setOpenGitDir}
                    onOpenReference={openReference}
                    onLoadDir={loadDir}
                    onClearSubtree={clearSubtree}
                    onSearchHit={handleSearchHit}
                  />
                </div>
              </div>
              {explorerOpen && (
                <div
                  data-testid="explorer-resize-handle"
                  role="separator"
                  aria-label="Resize file explorer"
                  aria-orientation="vertical"
                  aria-valuemin={MIN_RESIZABLE_WIDTH}
                  aria-valuemax={MAX_RESIZABLE_WIDTH}
                  aria-valuenow={explorerWidth}
                  tabIndex={0}
                  className={cn(
                    'absolute top-0 right-0 z-10 h-full w-2 cursor-col-resize touch-none rounded-sm outline-none hover:bg-primary/20 focus-visible:bg-primary/30',
                    explorerResizing && 'bg-primary/30',
                  )}
                  onPointerDown={handleExplorerResizeStart}
                  onKeyDown={handleExplorerResizeKeyDown}
                />
              )}
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
                    favorites={favorites}
                    recentFiles={recentFiles}
                    gitStatus={gitStatus}
                    onClose={onClose}
                    onMoveFile={handleMoveFile}
                    onChanged={handleChanged}
                    onError={setError}
                    showHidden={showHidden}
                    onToggleHidden={() => setShowHidden((s) => !s)}
                    onOpenGit={setOpenGitDir}
                    onOpenReference={openReference}
                    onLoadDir={loadDir}
                    onClearSubtree={clearSubtree}
                    onSearchHit={handleSearchHit}
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
            <FileTabs
              tabs={visibleRecents}
              active={selected}
              onSelect={handleSelect}
              onClose={handleCloseRecent}
            />
            {referencePath && (
              <div className="flex shrink-0 items-center gap-1 border-b border-border py-1 md:hidden" role="tablist" aria-label="File viewer pane">
                <Button
                  variant={mobileViewer === 'main' ? 'secondary' : 'ghost'}
                  size="xs"
                  role="tab"
                  aria-selected={mobileViewer === 'main'}
                  onClick={() => setMobileViewer('main')}
                >
                  Main
                </Button>
                <Button
                  variant={mobileViewer === 'reference' ? 'secondary' : 'ghost'}
                  size="xs"
                  role="tab"
                  aria-selected={mobileViewer === 'reference'}
                  onClick={() => setMobileViewer('reference')}
                >
                  Reference
                </Button>
              </div>
            )}
            {referencePath && !isMobile ? (
              <div className="file-viewer-split flex min-h-0 min-w-0 flex-1">
                <div data-testid="main-file-viewer-pane" className="relative flex min-h-0 min-w-0 flex-1 flex-col">
                  {selected && <ViewerPaneHeader label="Main" path={selected} onSwap={() => void swapPanes()} />}
                  <div
                    data-testid="git-files-scroll-container"
                    data-git-diff-open={gitDiffOpen ? 'true' : 'false'}
                    className={cn('knowledge-files min-h-0 min-w-0 flex-1', gitDiffOpen ? 'overflow-hidden' : 'overflow-y-auto')}
                    onClick={handleAnchorClick}
                  >
                    {selected && loaded ? renderPane('main', selected, selectedEntry, mainScrollRef, mainPaneRef) : (
                      <div className="knowledge-empty flex items-center justify-center p-12 text-center text-muted-foreground">
                        Loading files…
                      </div>
                    )}
                  </div>
                </div>
                <div
                  data-testid="reference-resize-handle"
                  role="separator"
                  aria-label="Resize reference pane"
                  aria-orientation="vertical"
                  aria-valuemin={280}
                  aria-valuemax={720}
                  aria-valuenow={referenceWidth}
                  tabIndex={0}
                  className={cn(
                    'z-10 w-2 shrink-0 cursor-col-resize touch-none rounded-sm outline-none hover:bg-primary/20 focus-visible:bg-primary/30',
                    referenceResizing && 'bg-primary/30',
                  )}
                  onPointerDown={handleReferenceResizeStart}
                  onKeyDown={handleReferenceResizeKeyDown}
                />
                <div data-testid="reference-file-viewer-pane" className="relative flex min-h-0 min-w-0 shrink-0 flex-col border-l border-border" style={{ width: referenceWidth }}>
                  <ViewerPaneHeader label="Reference" path={referencePath} onClose={closeReference} onSwap={() => void swapPanes()} />
                  <div
                    data-git-diff-open={referenceGitDiffOpen ? 'true' : 'false'}
                    className={cn('knowledge-files min-h-0 min-w-0 flex-1', referenceGitDiffOpen ? 'overflow-hidden' : 'overflow-y-auto')}
                    onClick={handleAnchorClick}
                  >
                    {loaded ? renderPane('reference', referencePath, referenceEntry, referenceScrollRef, referencePaneRef) : (
                      <div className="knowledge-empty flex items-center justify-center p-12 text-center text-muted-foreground">
                        Loading files…
                      </div>
                    )}
                  </div>
                </div>
              </div>
            ) : referencePath && isMobile ? (
              <>
                <div
                  aria-hidden={mobileViewer !== 'main'}
                  className={cn('min-h-0 min-w-0 flex-1', mobileViewer !== 'main' && 'hidden')}
                >
                  <div
                    data-testid="git-files-scroll-container"
                    data-git-diff-open={gitDiffOpen ? 'true' : 'false'}
                    className={cn('knowledge-files h-full min-h-0 min-w-0', gitDiffOpen ? 'overflow-hidden' : 'overflow-y-auto')}
                    onClick={handleAnchorClick}
                  >
                    {selected && loaded ? renderPane('main', selected, selectedEntry, mainScrollRef, mainPaneRef) : null}
                  </div>
                </div>
                <div
                  aria-hidden={mobileViewer !== 'reference'}
                  className={cn('min-h-0 min-w-0 flex-1', mobileViewer !== 'reference' && 'hidden')}
                >
                  <div className="flex h-full min-h-0 flex-col">
                    <ViewerPaneHeader label="Reference" path={referencePath} onClose={closeReference} />
                    <div
                      data-git-diff-open={referenceGitDiffOpen ? 'true' : 'false'}
                      className={cn('knowledge-files min-h-0 min-w-0 flex-1', referenceGitDiffOpen ? 'overflow-hidden' : 'overflow-y-auto')}
                      onClick={handleAnchorClick}
                    >
                      {loaded ? renderPane('reference', referencePath, referenceEntry, referenceScrollRef, referencePaneRef) : null}
                    </div>
                  </div>
                </div>
              </>
            ) : (
              <div
                data-testid="git-files-scroll-container"
                data-git-diff-open={gitDiffOpen ? 'true' : 'false'}
                className={cn('knowledge-files min-h-0 min-w-0 flex-1', gitDiffOpen ? 'overflow-hidden' : 'overflow-y-auto')}
                onClick={handleAnchorClick}
              >
                {selected && loaded ? (
                  renderPane('main', selected, selectedEntry, mainScrollRef, mainPaneRef)
                ) : selected ? (
                  <div className="knowledge-empty flex items-center justify-center p-12 text-center text-muted-foreground">
                    Loading files…
                  </div>
                ) : (
                  <Card className="knowledge-empty items-center justify-center gap-2 p-12 text-center">
                    <h2 className="text-xl font-bold">{title}</h2>
                    <p className="text-muted-foreground">
                      Select a file from the explorer to view it here.
                    </p>
                    {loaded && entries.length === 0 && (
                      <p className="text-muted-foreground">
                        No files yet.
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
            )}
          </div>
        </div>
      </div>
      {openGitDir && (
        <GitViewer path={openGitDir} onClose={() => setOpenGitDir(null)} refreshMeta={refreshMeta} />
      )}
    </div>
  )
}
