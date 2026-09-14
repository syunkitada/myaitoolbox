import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react'
import {
  ArrowDownToLine,
  ArrowUpFromLine,
  Check,
  CheckCircle2,
  ChevronDown,
  GitBranch,
  History,
  PanelLeftClose,
  PanelLeftOpen,
  PencilLine,
  RefreshCw,
  RotateCcw,
  SquarePlus,
  Trash2,
} from 'lucide-react'
import { GitBranch as GitBranchInfo, GitDetail, GitFile, GitLogEntry, GitResult, api } from '../api/client'
import { Button } from '../components/ui/button'
import { Textarea } from '../components/ui/textarea'
import { Card, CardContent } from '../components/ui/card'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuLabel, DropdownMenuSeparator, DropdownMenuTrigger } from '../components/ui/dropdown-menu'
import { Sheet, SheetContent } from '../components/ui/sheet'
import { DiffView } from '../components/DiffView'
import { CommitDiffView } from '../components/CommitDiffView'
import { useIsMobile } from '@/hooks/use-mobile'
import { useDialogs } from '../components/AppDialogs'
import { cn } from '@/lib/utils'

const EXPLORER_STORAGE_KEY = 'git_explorer_open'
const LOG_PAGE_SIZE = 30

type GitViewMode = 'working-tree' | 'log'

interface GitWorkspaceProps {
  refreshMeta: () => Promise<void>
  scope?: string
  embedded?: boolean
}

const codeLabel: Record<string, string> = {
  '??': 'untracked',
  M: 'modified',
  A: 'added',
  D: 'deleted',
  R: 'renamed',
  C: 'copied',
  U: 'unmerged',
}

function statusText(f: GitFile): string {
  return codeLabel[f.code] ?? f.code
}

// formatGitDate renders an ISO 8601 date as a short relative or date label.
function formatGitDate(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  const diff = Date.now() - d.getTime()
  const min = 60_000
  const hr = 60 * min
  const day = 24 * hr
  if (diff >= 0) {
    if (diff < min) return 'just now'
    if (diff < hr) return `${Math.floor(diff / min)}m ago`
    if (diff < day) return `${Math.floor(diff / hr)}h ago`
    if (diff < 7 * day) return `${Math.floor(diff / day)}d ago`
  }
  return d.toLocaleDateString()
}

function ModeToggle({ mode, onChange }: { mode: GitViewMode; onChange: (m: GitViewMode) => void }) {
  return (
    <div className="flex shrink-0 rounded-md border border-border p-0.5" data-testid="git-mode-toggle">
      <Button
        variant={mode === 'working-tree' ? 'secondary' : 'ghost'}
        size="sm"
        onClick={() => onChange('working-tree')}
        aria-pressed={mode === 'working-tree'}
        data-testid="git-mode-working-tree"
      >
        Working tree
      </Button>
      <Button
        variant={mode === 'log' ? 'secondary' : 'ghost'}
        size="sm"
        onClick={() => onChange('log')}
        aria-pressed={mode === 'log'}
        data-testid="git-mode-log"
      >
        <History />
        Log
      </Button>
    </div>
  )
}

function GitLogList({
  entries,
  loading,
  hasMore,
  selectedHash,
  onSelect,
  onLoadMore,
}: {
  entries: GitLogEntry[]
  loading: boolean
  hasMore: boolean
  selectedHash: string | null
  onSelect: (e: GitLogEntry) => void
  onLoadMore: () => void
}) {
  return (
    <div className="min-h-0">
      {entries.length === 0 && !loading ? (
        <p className="px-1 text-xs text-muted-foreground">No commits yet.</p>
      ) : (
        <ul className="knowledge-tree m-0 list-none p-0">
          {entries.map((e) => (
            <li
              key={e.hash}
              className="knowledge-tree-row flex min-h-[26px] items-start gap-1 rounded-md px-1 py-0.5 leading-tight hover:bg-muted"
            >
              <button
                type="button"
                className={cn(
                  'knowledge-file flex min-w-0 flex-1 cursor-pointer flex-col items-stretch gap-0.5 self-stretch bg-transparent p-0 text-left',
                  e.hash === selectedHash && 'active text-primary',
                )}
                onClick={() => onSelect(e)}
                title={e.subject}
                data-testid="git-log-entry"
              >
                <span className="flex w-full items-center gap-1.5">
                  <span className="shrink-0 rounded bg-muted px-1.5 font-mono text-[10px] leading-4 text-muted-foreground">
                    {e.short_hash}
                  </span>
                  <span className="truncate text-sm font-medium">{e.subject}</span>
                </span>
                <span className="truncate pl-0.5 font-mono text-[10px] text-muted-foreground">
                  {e.author} · {formatGitDate(e.date)}
                </span>
              </button>
            </li>
          ))}
        </ul>
      )}
      <div className="flex items-center justify-center py-2">
        {loading ? (
          <RefreshCw className="size-4 animate-spin text-muted-foreground" />
        ) : hasMore ? (
          <Button
            variant="ghost"
            size="sm"
            onClick={onLoadMore}
            data-testid="git-log-load-more"
          >
            Load more
          </Button>
        ) : null}
      </div>
    </div>
  )
}

interface ChangeNode {
  kind: 'dir' | 'file'
  name: string
  path: string
  file?: GitFile
  children: ChangeNode[]
}

function buildChangeTree(files: GitFile[]): ChangeNode[] {
  const root: ChangeNode[] = []
  for (const f of files) {
    const parts = f.path.split('/')
    let cur = root
    let prefix = ''
    for (let i = 0; i < parts.length - 1; i++) {
      prefix = prefix ? `${prefix}/${parts[i]}` : parts[i]
      let dir = cur.find((n) => n.kind === 'dir' && n.name === parts[i])
      if (!dir) {
        dir = { kind: 'dir', name: parts[i], path: prefix, children: [] }
        cur.push(dir)
      }
      cur = dir.children
    }
    cur.push({ kind: 'file', name: parts[parts.length - 1], path: f.path, file: f, children: [] })
  }
  return root
}

function TreeSection({
  title,
  empty,
  accent,
  files,
  selectedKey,
  onSelect,
}: {
  title: string
  empty: string
  accent: string
  files: GitFile[]
  selectedKey: string | null
  onSelect: (f: GitFile) => void
}) {
  const [collapsed, setCollapsed] = useState<Set<string>>(new Set())
  const tree = useMemo(() => buildChangeTree(files), [files])

  const toggle = (path: string) =>
    setCollapsed((prev) => {
      const next = new Set(prev)
      if (next.has(path)) next.delete(path)
      else next.add(path)
      return next
    })

  const items: ReactNode[] = []
  const render = (nodes: ChangeNode[], depth: number) => {
    const pad = depth * 14
    for (const node of nodes) {
      if (node.kind === 'dir') {
        const open = !collapsed.has(node.path)
        items.push(
          <li
            key={`dir:${node.path}`}
            className="knowledge-tree-row flex min-h-[26px] items-center gap-1 rounded-md px-1 leading-[1.4] hover:bg-muted"
            style={{ paddingLeft: pad }}
          >
            <button
              type="button"
              className={cn(
                'knowledge-caret flex h-6 w-6 shrink-0 cursor-pointer items-center justify-center rounded p-0 text-muted-foreground hover:bg-muted hover:text-primary',
                open && 'open',
              )}
              aria-label={open ? `Collapse ${node.name}` : `Expand ${node.name}`}
              onClick={() => toggle(node.path)}
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
            <span className="knowledge-dir flex min-w-0 flex-1 items-center self-stretch text-left text-sm font-semibold whitespace-nowrap text-foreground overflow-hidden text-ellipsis">
              {node.name}
            </span>
          </li>,
        )
        if (open) render(node.children, depth + 1)
      } else {
        const key = `${node.file!.status}:${node.path}`
        items.push(
          <li
            key={`file:${node.path}`}
            className="knowledge-tree-row flex min-h-[26px] items-center gap-1 rounded-md px-1 leading-[1.4] hover:bg-muted"
            style={{ paddingLeft: pad + 20 }}
          >
            <button
              type="button"
              className={cn(
                'knowledge-file flex min-w-0 flex-1 cursor-pointer items-center self-stretch bg-transparent p-0 text-left text-sm whitespace-nowrap text-foreground overflow-hidden text-ellipsis hover:text-primary',
                key === selectedKey && 'active text-primary font-semibold',
              )}
              onClick={() => onSelect(node.file!)}
            >
              <span className="truncate font-mono text-xs">{node.name}</span>
            </button>
            <span className="shrink-0 rounded bg-muted px-1.5 text-[10px] text-muted-foreground">
              {statusText(node.file!)}
            </span>
          </li>,
        )
      }
    }
  }
  render(tree, 0)

  return (
    <section className="explorer-section mb-4 min-w-0 last:mb-0">
      <h2 className={cn('mb-1 flex items-center gap-1.5 text-xs font-semibold tracking-wider uppercase', accent)}>
        {title}
        <span className="rounded-full bg-muted px-1.5 text-[10px] text-muted-foreground">{files.length}</span>
      </h2>
      {files.length === 0 ? (
        <p className="px-1 text-xs text-muted-foreground">{empty}</p>
      ) : (
        <ul className="knowledge-tree m-0 list-none p-0">{items}</ul>
      )}
    </section>
  )
}

function SimpleTreeList({
  detail,
  hasChanges,
  selectedKey,
  onSelect,
  onClose,
  onStageAll,
  onUnstageAll,
  scope,
  mode,
  onModeChange,
  logEntries,
  logLoading,
  logHasMore,
  selectedCommitHash,
  onSelectCommit,
  onLoadMore,
}: {
  detail: GitDetail
  hasChanges: boolean
  selectedKey: string | null
  onSelect: (f: GitFile) => void
  onClose?: () => void
  onStageAll: () => void
  onUnstageAll: () => void
  scope?: string
  mode: GitViewMode
  onModeChange: (m: GitViewMode) => void
  logEntries: GitLogEntry[]
  logLoading: boolean
  logHasMore: boolean
  selectedCommitHash: string | null
  onSelectCommit: (e: GitLogEntry) => void
  onLoadMore: () => void
}) {
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
        <h1 className="flex items-center gap-2 text-lg font-bold">
          <GitBranch className="size-4" />
          Git
        </h1>
        <span className="rounded-md border px-2 py-0.5 font-mono text-xs">{detail.branch || 'HEAD'}</span>
        {scope && (
          <span className="truncate rounded-md border px-2 py-0.5 font-mono text-xs text-muted-foreground" title={scope}>
            {scope}
          </span>
        )}
        {detail.remote && (
          <span className="truncate rounded-md border px-2 py-0.5 font-mono text-xs text-muted-foreground">
            {detail.remote}
          </span>
        )}
        {(detail.ahead > 0 || detail.behind > 0) && (
          <span className="flex items-center gap-1.5 text-xs text-muted-foreground">
            {detail.ahead > 0 && (
              <span className="rounded-md bg-blue-500/10 px-1.5 py-0.5 text-blue-700 dark:text-blue-300">
                {detail.ahead} ahead
              </span>
            )}
            {detail.behind > 0 && (
              <span className="rounded-md bg-amber-500/10 px-1.5 py-0.5 text-amber-700 dark:text-amber-300">
                {detail.behind} behind
              </span>
            )}
          </span>
        )}
      </div>
      <div className="mb-2 w-full">
        <ModeToggle mode={mode} onChange={onModeChange} />
      </div>
      {mode === 'log' ? (
        <GitLogList
          entries={logEntries}
          loading={logLoading}
          hasMore={logHasMore}
          selectedHash={selectedCommitHash}
          onSelect={onSelectCommit}
          onLoadMore={onLoadMore}
        />
      ) : !hasChanges ? (
        <div className="flex flex-col items-start gap-2 p-2 text-sm text-muted-foreground">
          <CheckCircle2 className="size-5 text-green-600" />
          Working tree is clean.
        </div>
      ) : (
        <div>
          <div className="mb-2 flex flex-wrap items-center gap-1.5">
            {(detail.unstaged.length > 0 || detail.untracked.length > 0) && (
              <Button
                variant="outline"
                size="sm"
                onClick={onStageAll}
                aria-label="Stage all changes"
                data-testid="git-stage-all"
              >
                <SquarePlus />
                Stage all
              </Button>
            )}
            {detail.staged.length > 0 && (
              <Button
                variant="outline"
                size="sm"
                onClick={onUnstageAll}
                aria-label="Unstage all changes"
                data-testid="git-unstage-all"
              >
                <RotateCcw />
                Unstage all
              </Button>
            )}
          </div>
          <TreeSection
            title="Staged"
            files={detail.staged}
            empty="Nothing staged."
            accent="text-green-700 dark:text-green-300"
            selectedKey={selectedKey}
            onSelect={onSelect}
          />
          <TreeSection
            title="Changes"
            files={detail.unstaged}
            empty="No unstaged changes."
            accent="text-amber-700 dark:text-amber-300"
            selectedKey={selectedKey}
            onSelect={onSelect}
          />
          <TreeSection
            title="Untracked"
            files={detail.untracked}
            empty="No untracked files."
            accent="text-blue-700 dark:text-blue-300"
            selectedKey={selectedKey}
            onSelect={onSelect}
          />
        </div>
      )}
    </div>
  )
}

function BranchSwitcher({
  branches,
  loading,
  currentBranch,
  onCheckout,
  onNewBranch,
}: {
  branches: GitBranchInfo[]
  loading: boolean
  currentBranch: string
  onCheckout: (branch: string) => void
  onNewBranch: () => void
}) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="sm" data-testid="git-branch-switcher">
          <GitBranch className="size-3.5" />
          <span className="max-w-[120px] truncate font-mono text-xs">{currentBranch || 'HEAD'}</span>
          <ChevronDown className="size-3" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" className="max-h-[300px] w-56">
        <DropdownMenuLabel>Branches</DropdownMenuLabel>
        <DropdownMenuSeparator />
        {branches.map((b) => (
          <DropdownMenuItem
            key={b.name}
            onSelect={() => { if (!b.current) onCheckout(b.name) }}
            className={cn('gap-1', b.current && 'font-semibold')}
            data-testid={b.current ? 'git-branch-current' : undefined}
          >
            {b.current && <Check className="size-3.5 shrink-0 text-green-600" />}
            <span className="truncate">{b.name}</span>
          </DropdownMenuItem>
        ))}
        {branches.length === 0 && (
          <p className="px-2 py-1.5 text-xs text-muted-foreground">
            {loading ? 'Loading…' : 'No branches found.'}
          </p>
        )}
        <DropdownMenuSeparator />
        <DropdownMenuItem onSelect={onNewBranch} data-testid="git-new-branch">
          <SquarePlus className="size-3.5" />
          New branch…
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

export function GitWorkspace({ refreshMeta, scope, embedded }: GitWorkspaceProps) {
  const { confirm, prompt } = useDialogs()
  const [detail, setDetail] = useState<GitDetail | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [selectedKey, setSelectedKey] = useState<string | null>(null)
  const [message, setMessage] = useState('')
  const [stageAll, setStageAll] = useState(true)
  const [busy, setBusy] = useState(false)
  const [output, setOutput] = useState<string | null>(null)
  const [outputError, setOutputError] = useState(false)
  const rawIsMobile = useIsMobile()
  const isMobile = embedded ? false : rawIsMobile
  const [explorerOpen, setExplorerOpen] = useState<boolean>(() => {
    if (typeof window === 'undefined') return true
    if (window.innerWidth < 768) return false
    const saved = window.localStorage.getItem(EXPLORER_STORAGE_KEY)
    if (saved !== null) return saved === '1'
    return true
  })

  useEffect(() => {
    if (!isMobile) window.localStorage.setItem(EXPLORER_STORAGE_KEY, explorerOpen ? '1' : '0')
  }, [explorerOpen, isMobile])

  const [mode, setMode] = useState<GitViewMode>('working-tree')
  const [logEntries, setLogEntries] = useState<GitLogEntry[]>([])
  const [logLoading, setLogLoading] = useState(false)
  const [logHasMore, setLogHasMore] = useState(true)
  const [selectedCommitHash, setSelectedCommitHash] = useState<string | null>(null)
  const [commitDiff, setCommitDiff] = useState<string | null>(null)
  const [commitDiffLoading, setCommitDiffLoading] = useState(false)

  const loadLogPage = useCallback(
    async (offset: number, reset: boolean) => {
      if (logLoading) return
      setLogLoading(true)
      try {
        const res = await api.getGitLog(scope, offset, LOG_PAGE_SIZE)
        const commits = res.commits ?? []
        setLogEntries((prev) => (reset ? commits : [...prev, ...commits]))
        setLogOffset(offset + commits.length)
        setLogHasMore(commits.length === LOG_PAGE_SIZE)
        if (reset && commits.length > 0) {
          setSelectedCommitHash((cur) => cur ?? commits[0].hash)
        }
      } catch (e) {
        setError(e instanceof Error ? e.message : String(e))
      } finally {
        setLogLoading(false)
      }
    },
    [logLoading, scope],
  )

  const [logOffset, setLogOffset] = useState(0)
  const [branches, setBranches] = useState<GitBranchInfo[]>([])
  const [branchesLoading, setBranchesLoading] = useState(false)

  useEffect(() => {
    if (mode === 'log' && detail?.is_repo && logEntries.length === 0) {
      void loadLogPage(0, true)
    }
  }, [mode, detail?.is_repo, logEntries.length])

  useEffect(() => {
    if (!selectedCommitHash) {
      setCommitDiff(null)
      return
    }
    let cancelled = false
    setCommitDiffLoading(true)
    api.getGitCommitDiff(scope, selectedCommitHash)
      .then((res) => {
        if (!cancelled) setCommitDiff(res.diff)
      })
      .catch((e) => {
        if (!cancelled) setError(e instanceof Error ? e.message : String(e))
      })
      .finally(() => {
        if (!cancelled) setCommitDiffLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [selectedCommitHash, scope])

  const handleModeChange = useCallback(
    (m: GitViewMode) => {
      setMode(m)
      if (m === 'log') {
        setLogEntries([])
        setLogHasMore(true)
        setLogOffset(0)
        setSelectedCommitHash(null)
      } else {
        setSelectedCommitHash(null)
        setCommitDiff(null)
      }
    },
    [],
  )

  const handleSelectCommit = useCallback((e: GitLogEntry) => {
    setSelectedCommitHash((cur) => (cur === e.hash ? null : e.hash))
  }, [])

  const refreshBranches = useCallback(async () => {
    if (!detail?.is_repo) return
    setBranchesLoading(true)
    try {
      const res = await api.getGitBranches(scope)
      setBranches(res.branches ?? [])
    } catch { /* non-critical */ }
    finally { setBranchesLoading(false) }
  }, [detail?.is_repo, scope])

  useEffect(() => {
    void refreshBranches()
  }, [refreshBranches])

  const refresh = useCallback(async () => {
    try {
      const d = await api.getGitStatus(scope)
      setDetail(d)
      setError(null)
      setSelectedKey((key) => {
        if (!key) return null
        const all = [...d.staged, ...d.unstaged, ...d.untracked]
        return all.some((f) => f.status + ':' + f.path === key) ? key : null
      })
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    }
  }, [scope])

  useEffect(() => {
    void refresh()
  }, [refresh])

  const run = useCallback(
    async (fn: () => Promise<void>, after?: () => Promise<void>) => {
      setBusy(true)
      setOutputError(false)
      try {
        await fn()
      } catch (e) {
        setOutputError(true)
        setOutput(e instanceof Error ? e.message : String(e))
      } finally {
        setBusy(false)
        await after?.()
        await refresh()
        void refreshMeta()
      }
    },
    [refresh, refreshMeta],
  )

  const doGitResult = useCallback(async (fn: () => Promise<GitResult>) => {
    const res = await fn()
    setOutputError(!res.ok)
    setOutput(res.output ?? (res.ok ? null : 'operation failed'))
  }, [])

  const allFiles = useMemo(
    () => [...(detail?.staged ?? []), ...(detail?.unstaged ?? []), ...(detail?.untracked ?? [])],
    [detail],
  )
  const selected = allFiles.find((f) => f.status + ':' + f.path === selectedKey) ?? null
  const hasChanges = allFiles.length > 0

  if (error)
    return (
      <div className="error-banner m-4 rounded-md border border-red-200 bg-red-50 px-3.5 py-2.5 text-sm text-red-700">
        {error}
        <button className="ml-2 cursor-pointer underline" onClick={() => void refresh()}>
          retry
        </button>
      </div>
    )

  if (!detail)
    return <div className="page p-4 text-muted-foreground md:p-6">Loading…</div>

  if (!detail.is_repo)
    return (
      <div className="page p-4 md:p-6">
        <div className="flex max-w-md flex-col items-start gap-3 rounded-lg border bg-card p-6">
          <h1 className="text-lg font-semibold">No git repository</h1>
          <p className="text-sm text-muted-foreground">
            This workspace directory is not a git repository yet. Initialize one to track changes, commit, pull and push.
          </p>
          <Button
            variant="default"
            disabled={busy}
            onClick={() =>
              void run(async () => {
                await doGitResult(() => api.gitInit(scope))
              })
            }
          >
            <GitBranch />
            Initialize repository
          </Button>
        </div>
        {output && <OutputBox error={outputError} output={output} />}
      </div>
    )

  const handleSelect = (f: GitFile) => {
    setSelectedKey(f.status + ':' + f.path)
    if (isMobile) setExplorerOpen(false)
  }

  const handleStage = (f: GitFile) =>
    void run(
      () =>
        doGitResult(() => {
          const paths = [f.path]
          return f.status === 'staged' ? api.gitUnstage(scope, paths) : api.gitStage(scope, paths)
        }),
      () => refresh(),
    )

  const handleStageAll = () => {
    const changes = [...(detail?.unstaged ?? []), ...(detail?.untracked ?? [])]
    if (changes.length === 0) return
    void run(
      () =>
        doGitResult(() =>
          api.gitStage(scope, changes.map((f) => f.path)),
        ),
      () => refresh(),
    )
  }

  const handleUnstageAll = () => {
    const staged = detail?.staged ?? []
    if (staged.length === 0) return
    void run(
      () =>
        doGitResult(() =>
          api.gitUnstage(scope, staged.map((f) => f.path)),
        ),
      () => refresh(),
    )
  }

  const handleDiscard = async (f: GitFile) => {
    if (!(await confirm(`Discard changes to ${f.path}?`))) return
    void run(() => doGitResult(() => api.gitDiscard(scope, [f.path])))
  }

  const handleCommit = () => {
    const msg = message.trim()
    if (!msg) return
    void run(
      () => doGitResult(() => api.gitCommit(scope, msg, !stageAll)),
      async () => {
        setMessage('')
        setSelectedKey(null)
        await refresh()
      },
    )
  }

  const handleAmend = async () => {
    if (!detail?.last_commit_message) return
    if (!(await confirm('Rewrite the most recent commit? This changes history.'))) return
    const msg = message.trim()
    void run(
      () => doGitResult(() => api.gitCommit(scope, msg, !stageAll, true)),
      async () => {
        setMessage('')
        setSelectedKey(null)
        await refresh()
      },
    )
  }

  const handlePull = () =>
    void run(() => doGitResult(() => api.gitPull(scope)))
  const handlePush = () =>
    void run(() => doGitResult(() => api.gitPush(scope)))

  const handleCheckout = (branch: string) =>
    void run(
      () => doGitResult(() => api.gitCheckout(scope, branch)),
      async () => {
        await refreshBranches()
        if (mode === 'log') {
          setLogEntries([])
          setLogHasMore(true)
          setLogOffset(0)
          setSelectedCommitHash(null)
        }
      },
    )

  const handleNewBranch = async () => {
    const name = await prompt('New branch name:')
    if (!name?.trim()) return
    const startPoint = await prompt('Start point (optional, defaults to HEAD):', '')
    void run(
      () => doGitResult(() => api.gitCheckout(scope, name.trim(), true, startPoint?.trim() || undefined)),
      async () => {
        await refreshBranches()
        if (mode === 'log') {
          setLogEntries([])
          setLogHasMore(true)
          setLogOffset(0)
          setSelectedCommitHash(null)
        }
      },
    )
  }

  const currentBranch = detail.branch || 'HEAD'

  const workingTree = (
    <SimpleTreeList
      detail={detail}
      hasChanges={hasChanges}
      selectedKey={selectedKey}
      onSelect={handleSelect}
      onClose={() => setExplorerOpen(false)}
      onStageAll={handleStageAll}
      onUnstageAll={handleUnstageAll}
      scope={scope}
      mode={mode}
      onModeChange={handleModeChange}
      logEntries={logEntries}
      logLoading={logLoading}
      logHasMore={logHasMore}
      selectedCommitHash={selectedCommitHash}
      onSelectCommit={handleSelectCommit}
      onLoadMore={() => void loadLogPage(logOffset, false)}
    />
  )

  return (
    <div className="page relative flex h-full min-h-0 flex-col">
      <div className="relative min-h-0 flex-1">
        <div
          className={cn(
            'knowledge-layout flex h-full min-h-0 flex-col items-stretch overflow-hidden max-md:flex-col',
            (selectedKey || (mode === 'log' && selectedCommitHash)) && 'has-selection',
          )}
        >
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
                  <div className="flex-1 min-h-0 overflow-y-auto overflow-x-hidden">{workingTree}</div>
                </div>
              </div>
            )}
            {isMobile && (
              <Sheet open={explorerOpen} onOpenChange={setExplorerOpen}>
                <SheetContent
                  side="left"
                  className="explorer w-[85vw] max-w-sm gap-0 border-r bg-card p-0 text-sidebar-foreground"
                >
                  {workingTree}
                </SheetContent>
              </Sheet>
            )}
            <div className="knowledge-pane min-w-0 min-h-0 flex-1 bg-card p-4 flex-col lg:overflow-hidden flex">
              <div className="knowledge-files min-w-0 flex-1 overflow-y-auto">
                <div className="page-header note-toolbar sticky top-0 z-20 mb-3 flex flex-wrap items-center justify-between gap-3 border-b bg-card/95 py-2 backdrop-blur">
                  <div className="actions flex flex-wrap items-center gap-1.5">
                    <Button
                      variant="ghost"
                      size="sm"
                      aria-label="Toggle file explorer"
                      title={explorerOpen ? 'Hide file explorer' : 'Show file explorer'}
                      onClick={() => setExplorerOpen((o) => !o)}
                      className={cn(explorerOpen && 'text-primary')}
                    >
                      {explorerOpen ? <PanelLeftClose /> : <PanelLeftOpen />}
                    </Button>
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => {
                        void refresh()
                        if (mode === 'log' && detail?.is_repo) {
                          setLogEntries([])
                          setLogHasMore(true)
                          setLogOffset(0)
                          setSelectedCommitHash(() => null)
                        }
                      }}
                      disabled={busy}
                      aria-label="Refresh"
                      title="Refresh"
                    >
                      <RefreshCw className={cn((busy || logLoading) && 'animate-spin')} />
                      <span className="hidden sm:inline">Refresh</span>
                    </Button>
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={handlePull}
                      disabled={busy}
                      aria-label="Pull"
                      title="Pull"
                    >
                      <ArrowDownToLine />
                      <span className="hidden sm:inline">Pull</span>
                    </Button>
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={handlePush}
                      disabled={busy}
                      aria-label="Push"
                      title="Push"
                    >
                      <ArrowUpFromLine />
                      <span className="hidden sm:inline">Push</span>
                    </Button>
                    <BranchSwitcher
                      branches={branches}
                      loading={branchesLoading}
                      currentBranch={currentBranch}
                      onCheckout={handleCheckout}
                      onNewBranch={handleNewBranch}
                    />
                    <span
                      className="min-w-0 truncate font-mono text-sm"
                      data-testid="git-selected-file"
                      title={mode === 'log' && selectedCommitHash
                        ? logEntries.find((e) => e.hash === selectedCommitHash)?.subject ?? selectedCommitHash
                        : selected?.path ?? ''}
                    >
                      {mode === 'log' && selectedCommitHash
                        ? logEntries.find((e) => e.hash === selectedCommitHash)?.subject ?? selectedCommitHash.slice(0, 7)
                        : selected ? selected.path : 'Select a file to view its diff'}
                    </span>
                  </div>
                  {selected && mode !== 'log' && (
                    <div className="ml-auto flex shrink-0 items-center gap-1.5">
                      <Button
                        variant="outline"
                        size="sm"
                        disabled={busy}
                        onClick={() => handleStage(selected)}
                        title={selected.status === 'staged' ? 'Unstage' : 'Stage'}
                        aria-label={selected.status === 'staged' ? 'Unstage' : 'Stage'}
                      >
                        {selected.status === 'staged' ? <RotateCcw /> : <SquarePlus />}
                        <span className="hidden sm:inline">
                          {selected.status === 'staged' ? 'Unstage' : 'Stage'}
                        </span>
                      </Button>
                      <Button
                        variant="outline"
                        size="sm"
                        disabled={busy}
                        onClick={() => handleDiscard(selected)}
                        className="text-red-600 hover:text-red-700"
                        title={selected.status === 'untracked' ? 'Delete file' : 'Discard changes'}
                        aria-label={selected.status === 'untracked' ? 'Delete file' : 'Discard changes'}
                      >
                        <Trash2 />
                        <span className="hidden sm:inline">Discard</span>
                      </Button>
                    </div>
                  )}
                </div>
                <Card className="gap-0 p-4">
                  <CardContent className="px-0 py-0">
                    {mode === 'log' ? (
                      commitDiffLoading ? (
                        <div className="flex items-center justify-center gap-2 py-12 text-sm text-muted-foreground">
                          <RefreshCw className="size-4 animate-spin" />
                          Loading diff…
                        </div>
                      ) : selectedCommitHash === null ? (
                        <div className="flex flex-col items-center justify-center gap-2 py-12 text-center">
                          <p className="text-sm text-muted-foreground">
                            Select a commit from the history to view its diff.
                          </p>
                        </div>
                      ) : (
                        <CommitDiffView diff={commitDiff ?? ''} />
                      )
                    ) : selected ? (
                      <DiffView diff={selected.diff ?? ''} />
                    ) : (
                      <div className="flex flex-col items-center justify-center gap-2 py-12 text-center">
                        <p className="text-sm text-muted-foreground">
                          Select a file from the working tree to view its diff.
                        </p>
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={() => setExplorerOpen(true)}
                        >
                          <PanelLeftOpen />
                          Open file explorer
                        </Button>
                      </div>
                    )}
                  </CardContent>
                </Card>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div className="shrink-0 border-t bg-card p-3">
        <div className="flex flex-wrap items-end gap-2">
          <Textarea
            value={message}
            onChange={(e) => setMessage(e.target.value)}
            placeholder="Commit message"
            className="min-h-16 flex-1 basis-64"
            aria-label="Commit message"
            data-testid="git-commit-message"
          />
          <div className="flex flex-col items-stretch gap-2">
            <label className="flex cursor-pointer items-center gap-1.5 text-xs text-muted-foreground">
              <input
                type="checkbox"
                className="size-3.5 cursor-pointer accent-[var(--primary)]"
                checked={stageAll}
                onChange={(e) => setStageAll(e.target.checked)}
              />
              <span>Stage all before commit</span>
            </label>
            <div className="flex items-center gap-2">
              <Button onClick={handleCommit} disabled={busy || message.trim() === ''}>
                Commit
              </Button>
              <Button
                variant="outline"
                onClick={handleAmend}
                disabled={busy || !detail?.last_commit_message}
                aria-label="Amend last commit"
                title={
                  detail?.last_commit_message
                    ? `Rewrite the most recent commit${message.trim() ? '' : ' (keeps its message)'}`
                    : 'No commit to amend'
                }
              >
                <PencilLine />
                <span className="max-md:hidden">Amend</span>
              </Button>
            </div>
          </div>
        </div>
      </div>

      {output && (
        <div className="shrink-0 border-t bg-card px-3 py-2">
          <OutputBox error={outputError} output={output} />
        </div>
      )}
    </div>
  )
}

export function GitPage({ refreshMeta }: { refreshMeta: () => Promise<void> }) {
  return <GitWorkspace refreshMeta={refreshMeta} />
}

function OutputBox({ error, output }: { error: boolean; output: string }) {
  return (
    <div
      className={cn(
        'overflow-auto rounded-md border p-2 font-mono text-xs whitespace-pre-wrap',
        error ? 'border-red-200 bg-red-50 text-red-700' : 'border-green-200 bg-green-50 text-green-800',
      )}
      data-testid="git-output"
    >
      {output}
    </div>
  )
}