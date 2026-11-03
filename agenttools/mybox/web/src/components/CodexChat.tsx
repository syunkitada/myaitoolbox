import { useCallback, useEffect, useMemo, useRef, useState, type KeyboardEvent, type MouseEvent } from 'react'
import { ArrowLeft, Check, ChevronDown, Keyboard, Loader2, MessageSquarePlus, Pencil, RefreshCw, Send, Square, Trash2, X } from 'lucide-react'
import { api, type CodexThread } from '../api/client'
import { codexThreadWsUrl, encodePath, getBasePath, projectUrlFor } from '../utils/routes'
import { Button } from './ui/button'
import { Markdown } from './Markdown'
import { DiffView } from './DiffView'
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from './ui/sheet'
import { cn } from '@/lib/utils'
import { appendReviewContextToDraft } from '../utils/file-review-comments'
import { useDialogs } from './AppDialogs'

type JsonObject = Record<string, unknown>
type CodexTurn = JsonObject & { id: string; status?: string; error?: unknown; items?: CodexItem[] }
type CodexItem = JsonObject & { id?: string; type?: string; text?: string }
type ApprovalRequest = { request_id: string; method: string; params: JsonObject }
type ReviewContextAppend = { id: number; threadId: string; context: string }

const CODEX_COMMANDS = [
  { id: 'compact', label: '/compact', shortLabel: '/compact', kind: 'compact' as const },
  { id: 'continue', label: '進めて', shortLabel: '進めて', kind: 'prompt' as const, prompt: '進めて' },
  { id: 'next-step', label: '次は何をするとよいですか？', shortLabel: '次を聞く', kind: 'prompt' as const, prompt: '次は何をするとよいですか？' },
  { id: 'plan', label: '計画を立てて', shortLabel: '計画', kind: 'prompt' as const, prompt: 'まず作業計画を立ててください。実装を始める前に、手順と確認方法を示してください。' },
  { id: 'status', label: '状況を教えて', shortLabel: '状況', kind: 'prompt' as const, prompt: '現在の作業状況と次に行うことを簡潔に教えてください。' },
  { id: 'self-review', label: 'セルフレビューして', shortLabel: 'レビュー', kind: 'prompt' as const, prompt: 'セルフレビューして' },
  { id: 'retrospective', label: '振り返りをして', shortLabel: '振り返り', kind: 'prompt' as const, prompt: '振り返りをして' },
  { id: 'commit', label: 'Commitして', shortLabel: 'Commit', kind: 'prompt' as const, prompt: 'Commitして' },
]

const CODEX_RECENT_COMMAND_LIMIT = 5
const CODEX_RECONNECT_DELAYS_MS = [1_000, 2_000, 4_000, 8_000, 16_000]

function codexRecentCommandsStorageKey(project: string): string {
  return `mybox.codex.recent-commands.v1:${encodeURIComponent(project)}`
}

function readCodexRecentCommandIds(project: string): string[] {
  if (typeof window === 'undefined') return []
  try {
    const stored: unknown = JSON.parse(window.localStorage.getItem(codexRecentCommandsStorageKey(project)) ?? '[]')
    if (!Array.isArray(stored)) return []
    const validIds = new Set<string>(CODEX_COMMANDS.map((command) => command.id))
    return [...new Set(stored.filter((id): id is string => typeof id === 'string' && validIds.has(id)))].slice(0, CODEX_RECENT_COMMAND_LIMIT)
  } catch {
    return []
  }
}

function record(value: unknown): JsonObject {
  return value !== null && typeof value === 'object' && !Array.isArray(value) ? value as JsonObject : {}
}

function string(value: unknown): string {
  return typeof value === 'string' ? value : ''
}

function stringList(value: unknown): string[] {
  return Array.isArray(value) ? value.filter((item): item is string => typeof item === 'string') : []
}

function activeStatus(value: unknown): boolean {
  return value === 'inProgress' || value === 'in_progress' || value === 'running'
}

function turnFailureMessage(turn: CodexTurn): string {
  const error = record(turn.error)
  return string(error.message) || string(error.detail) || string(turn.error)
}

function turnHasAgentMessage(turn: CodexTurn): boolean {
  return (turn.items ?? []).some((item) => item.type === 'agentMessage' && Boolean(string(item.text) || itemText(item)))
}

function turnIsActive(turns: CodexTurn[]): boolean {
  return turns.some((turn) => activeStatus(turn.status))
}

function itemText(item: CodexItem): string {
  const content = item.content
  if (typeof content === 'string') return content
  if (!Array.isArray(content)) return ''
  return content.map((part) => {
    const value = record(part)
    if (typeof value.text === 'string') return value.text
    if (value.type === 'image') return '[Image attached]'
    return ''
  }).filter(Boolean).join('\n')
}

function itemSummary(item: CodexItem): string {
  const type = string(item.type)
  if (type === 'commandExecution') return string(item.command) || 'Command execution'
  if (type === 'fileChange') {
    const changes = Array.isArray(item.changes) ? item.changes : []
    return changes.map((change) => string(record(change).path)).filter(Boolean).join(', ') || 'File changes'
  }
  if (type === 'webSearch') return string(item.query) || 'Web search'
  if (type === 'mcpToolCall') return [string(item.server), string(item.tool)].filter(Boolean).join(' · ') || 'MCP tool'
  return string(item.title) || type.replace(/([A-Z])/g, ' $1').trim() || 'Activity'
}

function decodePathSegments(path: string): string | null {
  let decoded: string
  try {
    decoded = decodeURIComponent(path)
  } catch {
    return null
  }
  return normalizePathSegments(decoded)
}

function normalizePathSegments(path: string): string | null {
  const segments: string[] = []
  for (const segment of path.replace(/\\/g, '/').split('/')) {
    if (!segment || segment === '.') continue
    if (segment === '..') {
      if (segments.length === 0) return null
      segments.pop()
      continue
    }
    segments.push(segment)
  }
  return segments.join('/')
}

function normalizeAbsolutePath(path: string): string | null {
  if (!path.startsWith('/')) return null
  const relative = normalizePathSegments(path)
  if (relative === null) return null
  return relative ? `/${relative}` : '/'
}

function codexFileLinkLine(href: string): number | undefined {
  const fragment = href.match(/#(?:L|line=)(\d+)(?::\d+)?$/i)
  const query = href.match(/[?&]line=(\d+)(?:&|$)/i)
  const suffix = href.split(/[?#]/, 1)[0].match(/(?::|%3A)(\d+)(?:(?::|%3A)\d+)?$/i)
  const line = Number(fragment?.[1] ?? query?.[1] ?? suffix?.[1])
  return Number.isSafeInteger(line) && line > 0 ? line : undefined
}

function stripCodexFileLineSuffix(path: string): string {
  return path.replace(/(?::|%3A)\d+(?:(?::|%3A)\d+)?$/i, '')
}

function codexFileLinkPath(href: string, project: string, projectRoot: string): string | null {
  const trimmed = href.trim()
  if (!trimmed || trimmed.startsWith('#') || trimmed.startsWith('//')) return null

  let rawPath = stripCodexFileLineSuffix(trimmed.split(/[?#]/, 1)[0])
  let absolute = false
  if (/^(?:file|vscode|vscode-insiders|https?):/i.test(trimmed)) {
    let url: URL
    try {
      url = new URL(trimmed)
    } catch {
      return null
    }
    const isFileURL = url.protocol === 'file:' && (url.hostname === '' || url.hostname === 'localhost')
    const isEditorURL = (url.protocol === 'vscode:' || url.protocol === 'vscode-insiders:') && url.hostname === 'file'
    const isLocalAppURL = (url.protocol === 'http:' || url.protocol === 'https:') && url.origin === window.location.origin
    if (!isFileURL && !isEditorURL && !isLocalAppURL) return null
    rawPath = stripCodexFileLineSuffix(url.pathname)
    absolute = !isLocalAppURL
    if (isLocalAppURL) {
      const base = getBasePath()
      const pathname = base && rawPath.startsWith(base) ? rawPath.slice(base.length) : rawPath
      const routePrefix = `/projects/${encodeURIComponent(project)}/dashboard/files/`
      if (pathname.startsWith(routePrefix)) return decodePathSegments(stripCodexFileLineSuffix(pathname.slice(routePrefix.length))) || null
      rawPath = stripCodexFileLineSuffix(pathname)
      absolute = true
    }
  } else if (/^https?:/i.test(trimmed) || /^[a-z][a-z\d+.-]*:/i.test(trimmed)) {
    return null
  } else if (trimmed.startsWith('/')) {
    let url: URL
    try {
      url = new URL(trimmed, window.location.origin)
    } catch {
      return null
    }
    const base = getBasePath()
    const pathname = base && url.pathname.startsWith(base) ? url.pathname.slice(base.length) : url.pathname
    const routePrefix = `/projects/${encodeURIComponent(project)}/dashboard/files/`
    if (pathname.startsWith(routePrefix)) return decodePathSegments(stripCodexFileLineSuffix(pathname.slice(routePrefix.length))) || null
    rawPath = stripCodexFileLineSuffix(pathname)
    absolute = true
  }

  if (!absolute) return decodePathSegments(rawPath) || null
  const decodedTarget = decodePathSegments(rawPath)
  const target = decodedTarget === null ? null : normalizeAbsolutePath(`/${decodedTarget}`)
  const root = normalizeAbsolutePath(projectRoot)
  if (!target || !root || target === root || !target.startsWith(`${root}/`)) return null
  return target.slice(root.length + 1)
}

function updateTurn(turns: CodexTurn[], incoming: JsonObject): CodexTurn[] {
  const id = string(incoming.id)
  if (!id) return turns
  const index = turns.findIndex((turn) => turn.id === id)
  if (index < 0) return [...turns, incoming as CodexTurn].sort((a, b) => Number(a.startedAt ?? 0) - Number(b.startedAt ?? 0))
  return turns.map((turn, turnIndex) => turnIndex === index ? { ...turn, ...incoming } as CodexTurn : turn)
}

function updateItem(turns: CodexTurn[], turnId: string, incoming: CodexItem): CodexTurn[] {
  if (!turnId) return turns
  const current = turns.find((turn) => turn.id === turnId) ?? { id: turnId, status: 'inProgress', items: [] }
  const items = current.items ?? []
  const itemId = string(incoming.id)
  const index = itemId ? items.findIndex((item) => item.id === itemId) : -1
  const nextItems = index < 0
    ? [...items, incoming]
    : items.map((item, itemIndex) => itemIndex === index ? { ...item, ...incoming } : item)
  return updateTurn(turns, { ...current, items: nextItems })
}

function applyNotification(turns: CodexTurn[], method: string, params: JsonObject): CodexTurn[] {
  const turnId = string(params.turnId)
  if (method === 'turn/started' || method === 'turn/completed') {
    const turn = record(params.turn)
    if (Object.keys(turn).length > 0) return updateTurn(turns, turn)
    if (!turnId) return turns
    const current = turns.find((candidate) => candidate.id === turnId) ?? { id: turnId, items: [] }
    return updateTurn(turns, { ...current, status: method === 'turn/started' ? 'inProgress' : string(params.status) || 'completed' })
  }
  const item = record(params.item) as CodexItem
  if (method === 'item/started' || method === 'item/completed') {
    return updateItem(turns, turnId, item)
  }
  if (method === 'item/agentMessage/delta' || method === 'item/plan/delta') {
    const itemId = string(params.itemId)
    const delta = string(params.delta)
    if (!delta || !turnId) return turns
    const currentTurn = turns.find((candidate) => candidate.id === turnId) ?? { id: turnId, status: 'inProgress', items: [] }
    const currentItem = (currentTurn.items ?? []).find((candidate) => candidate.id === itemId) ?? { id: itemId, type: method === 'item/plan/delta' ? 'plan' : 'agentMessage', text: '' }
    return updateItem(turns, turnId, { ...currentItem, text: `${string(currentItem.text)}${delta}` })
  }
  if (method === 'item/commandExecution/outputDelta') {
    const itemId = string(params.itemId)
    const delta = string(params.delta)
    if (!delta || !turnId) return turns
    const currentTurn = turns.find((candidate) => candidate.id === turnId) ?? { id: turnId, status: 'inProgress', items: [] }
    const currentItem = (currentTurn.items ?? []).find((candidate) => candidate.id === itemId) ?? { id: itemId, type: 'commandExecution', aggregatedOutput: '' }
    return updateItem(turns, turnId, { ...currentItem, aggregatedOutput: `${string(currentItem.aggregatedOutput)}${delta}` })
  }
  return turns
}

function ApprovalCard({
  request,
  busy,
  onRespond,
}: {
  request: ApprovalRequest
  busy: boolean
  onRespond: (request: ApprovalRequest, decision: string) => void
}) {
  const commandApproval = request.method === 'item/commandExecution/requestApproval'
  const fileApproval = request.method === 'item/fileChange/requestApproval'
  const permissionsApproval = request.method === 'item/permissions/requestApproval'
  const command = string(request.params.command)
  const cwd = string(request.params.cwd)
  const reason = string(request.params.reason)
  const grantRoot = string(request.params.grantRoot)
  const decisions = stringList(request.params.availableDecisions)
  const allowDecision = decisions.length === 0 || decisions.includes('accept')
  const declineDecision = decisions.length === 0 || decisions.includes('decline')
  const cancelDecision = decisions.includes('cancel')

  return (
    <section className="rounded-lg border border-amber-300 bg-amber-50 p-3 text-amber-950 dark:border-amber-700 dark:bg-amber-950/30 dark:text-amber-100" aria-label="Codex approval request">
      <div className="text-xs font-semibold">{commandApproval ? 'コマンドの実行許可' : fileApproval ? 'ファイル変更の許可' : permissionsApproval ? '追加権限の許可' : 'Codexからの確認'}</div>
      {reason && <p className="mt-1 whitespace-pre-wrap text-xs">{reason}</p>}
      {command && <pre className="mt-2 max-h-36 overflow-auto rounded bg-black/5 p-2 font-mono text-xs dark:bg-black/20">{command}</pre>}
      {cwd && <p className="mt-1 truncate font-mono text-[11px]" title={cwd}>cwd: {cwd}</p>}
      {grantRoot && <p className="mt-1 truncate font-mono text-[11px]" title={grantRoot}>対象: {grantRoot}</p>}
      {permissionsApproval && (
        <details open className="mt-2 rounded border border-amber-300/70 p-2 text-[11px] dark:border-amber-700/70">
          <summary className="cursor-pointer font-medium">要求された権限を確認</summary>
          <pre className="mt-2 max-h-32 overflow-auto whitespace-pre-wrap break-words font-mono">{JSON.stringify(request.params.permissions ?? {}, null, 2)}</pre>
          <p className="mt-1 text-muted-foreground">許可はこのターン内だけ有効です。</p>
        </details>
      )}
      {!commandApproval && !fileApproval && !permissionsApproval && <p className="mt-1 text-xs">この種類の確認にはまだ回答できません。</p>}
      <div className="mt-3 flex flex-wrap justify-end gap-2">
        {declineDecision && (
          <Button size="xs" variant="outline" className="cursor-pointer" disabled={busy} onClick={() => onRespond(request, 'decline')}>
            <X className="mr-1 size-3" />拒否
          </Button>
        )}
        {cancelDecision && (
          <Button size="xs" variant="outline" className="cursor-pointer" disabled={busy} onClick={() => onRespond(request, 'cancel')}>
            <Square className="mr-1 size-3" />拒否して停止
          </Button>
        )}
        {allowDecision && (commandApproval || fileApproval || permissionsApproval) && (
          <Button size="xs" className="cursor-pointer" disabled={busy} onClick={() => onRespond(request, 'accept')}>
            {busy ? <Loader2 className="mr-1 size-3 animate-spin" /> : <Check className="mr-1 size-3" />}
            {permissionsApproval ? 'このターンで許可' : '今回許可'}
          </Button>
        )}
      </div>
    </section>
  )
}

function UserInputCard({
  request,
  busy,
  onSubmit,
}: {
  request: ApprovalRequest
  busy: boolean
  onSubmit: (request: ApprovalRequest, answers: JsonObject) => void
}) {
  const questions = Array.isArray(request.params.questions) ? request.params.questions.map(record) : []
  const [answers, setAnswers] = useState<Record<string, string>>({})
  return (
    <form
      className="rounded-lg border bg-card p-3"
      onSubmit={(event) => {
        event.preventDefault()
        const response: JsonObject = {}
        for (const question of questions) {
          const id = string(question.id)
          if (id) response[id] = { answers: [answers[id] ?? ''] }
        }
        onSubmit(request, response)
      }}
    >
      <div className="text-xs font-semibold">Codexからの質問</div>
      <div className="mt-2 flex flex-col gap-3">
        {questions.map((question, index) => {
          const id = string(question.id) || `question-${index}`
          const options = Array.isArray(question.options) ? question.options.map(record) : []
          return (
            <label key={id} className="flex flex-col gap-1 text-xs">
              <span className="font-medium">{string(question.question) || string(question.header)}</span>
              {options.length > 0 ? (
                <select className="h-8 rounded-md border bg-background px-2 text-xs" value={answers[id] ?? ''} onChange={(event) => setAnswers((current) => ({ ...current, [id]: event.target.value }))}>
                  <option value="">選択してください</option>
                  {options.map((option, optionIndex) => {
                    const label = string(option.label)
                    return <option key={`${label}-${optionIndex}`} value={label}>{label}</option>
                  })}
                </select>
              ) : question.isSecret === true ? (
                <input
                  type="password"
                  className="h-8 rounded-md border bg-background px-2 text-xs"
                  value={answers[id] ?? ''}
                  onChange={(event) => setAnswers((current) => ({ ...current, [id]: event.target.value }))}
                  autoComplete="off"
                  aria-label={string(question.question) || string(question.header)}
                />
              ) : (
                <textarea
                  className="min-h-16 resize-y rounded-md border bg-background p-2 text-xs"
                  value={answers[id] ?? ''}
                  onChange={(event) => setAnswers((current) => ({ ...current, [id]: event.target.value }))}
                  aria-label={string(question.question) || string(question.header)}
                />
              )}
            </label>
          )
        })}
      </div>
      <div className="mt-3 flex justify-end">
        <Button size="xs" className="cursor-pointer" disabled={busy || questions.length === 0 || questions.some((question, index) => !(answers[string(question.id) || `question-${index}`] ?? '').trim())}>
          {busy ? <Loader2 className="mr-1 size-3 animate-spin" /> : <Send className="mr-1 size-3" />}回答する
        </Button>
      </div>
    </form>
  )
}

function ApprovalRequestView({
  request,
  busy,
  onRespond,
}: {
  request: ApprovalRequest
  busy: boolean
  onRespond: (request: ApprovalRequest, decision: string, answers?: JsonObject) => void
}) {
  if (request.method === 'item/tool/requestUserInput') {
    return <UserInputCard request={request} busy={busy} onSubmit={(current, answers) => onRespond(current, 'respond', answers)} />
  }
  return <ApprovalCard request={request} busy={busy} onRespond={onRespond} />
}

function TurnItem({ item, transformHref }: { item: CodexItem; transformHref?: (href: string) => string }) {
  const type = string(item.type)
  if (type === 'userMessage') {
    const text = itemText(item)
    return text ? <div className="max-w-[90%] self-end rounded-lg bg-primary/10 px-3 py-2 text-sm break-words whitespace-pre-wrap">{text}</div> : null
  }
  if (type === 'agentMessage') {
    const text = string(item.text)
    return text ? <Markdown text={text} className="markdown-body min-w-0 text-sm" transformHref={transformHref} /> : null
  }
  if (type === 'reasoning' || type === 'reasoningSummary') return null

  if (type === 'fileChange') {
    const changes = Array.isArray(item.changes) ? item.changes.map(record) : []
    return (
      <details className="rounded-lg border bg-card text-xs">
        <summary className="flex cursor-pointer list-none items-center gap-2 px-3 py-2 font-medium">
          <ChevronDown className="size-3 shrink-0" />
          <span className="min-w-0 flex-1 truncate">変更ファイル: {itemSummary(item)}</span>
          {string(item.status) && <span className="text-muted-foreground">{string(item.status)}</span>}
        </summary>
        <div className="flex flex-col gap-2 border-t p-2">
          {changes.map((change, index) => (
            <div key={`${string(change.path)}-${index}`} className="min-w-0">
              <div className="mb-1 truncate font-mono text-[11px]">{string(change.path) || '変更'}</div>
              <DiffView diff={string(change.diff)} className="max-h-64" />
            </div>
          ))}
          {changes.length === 0 && <pre className="overflow-auto whitespace-pre-wrap text-[11px]">{string(item.output)}</pre>}
        </div>
      </details>
    )
  }

  if (type === 'commandExecution') {
    const output = string(item.aggregatedOutput)
    return (
      <details className="rounded-lg border bg-muted/30 text-xs">
        <summary className="flex cursor-pointer list-none items-start gap-2 px-3 py-2">
          <ChevronDown className="mt-0.5 size-3 shrink-0" />
          <span className="min-w-0 flex-1 font-mono whitespace-pre-wrap break-all">{string(item.command) || 'Command execution'}</span>
          <span className="shrink-0 text-muted-foreground">{string(item.status)}</span>
        </summary>
        <div className="border-t p-2">
          {string(item.cwd) && <div className="mb-1 truncate font-mono text-[10px] text-muted-foreground">{string(item.cwd)}</div>}
          {output && <pre className="max-h-60 overflow-auto whitespace-pre-wrap break-words font-mono text-[11px]">{output}</pre>}
        </div>
      </details>
    )
  }

  const summary = itemSummary(item)
  const details = string(item.text) || string(item.output) || string(item.content)
  return (
    <details className="rounded-lg border bg-muted/20 text-xs">
      <summary className="flex cursor-pointer list-none items-center gap-2 px-3 py-2">
        <ChevronDown className="size-3 shrink-0" />
        <span className="min-w-0 flex-1 truncate">{summary}</span>
        <span className="text-muted-foreground">{type || 'activity'}</span>
      </summary>
      {details && <pre className="max-h-60 overflow-auto whitespace-pre-wrap break-words border-t p-2 font-mono text-[11px]">{details}</pre>}
    </details>
  )
}

function formatUpdatedAt(value: number): string {
  if (!value) return ''
  const date = new Date(value * 1000)
  if (Number.isNaN(date.getTime())) return ''
  return new Intl.DateTimeFormat(undefined, { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' }).format(date)
}

type CodexThreadStatusKind = 'working' | 'blocked' | 'idle' | 'done' | 'unknown'

function codexThreadStatus(status: string): { kind: CodexThreadStatusKind; label: string } {
  const normalized = status.trim().toLowerCase().replace(/[\s_-]/g, '')
  switch (normalized) {
    case 'active':
    case 'inprogress':
    case 'running':
      return { kind: 'working', label: '作業中' }
    case 'blocked':
    case 'waitingonapproval':
    case 'needsapproval':
    case 'approvalrequired':
      return { kind: 'blocked', label: '確認待ち' }
    case 'idle':
      return { kind: 'idle', label: '待機中' }
    case 'completed':
    case 'complete':
    case 'done':
      return { kind: 'done', label: '完了' }
    case 'systemerror':
    case 'failed':
    case 'error':
      return { kind: 'blocked', label: 'エラー' }
    case 'interrupted':
    case 'cancelled':
    case 'canceled':
      return { kind: 'unknown', label: '中断' }
    default:
      return { kind: 'unknown', label: status.trim() || '状態不明' }
  }
}

export function CodexThreadStatus({ status, compact = false }: { status: string; compact?: boolean }) {
  const display = codexThreadStatus(status)
  const dotClass = {
    working: 'bg-blue-500',
    blocked: 'bg-red-500',
    idle: 'bg-emerald-500',
    done: 'bg-violet-500',
    unknown: 'bg-zinc-400',
  }[display.kind]
  return (
    <span
      role="img"
      aria-label={`セッション状態: ${display.label}`}
      title={`セッション状態: ${display.label}`}
      className={cn('inline-flex shrink-0 items-center gap-1.5 text-[10px] text-muted-foreground', compact && 'size-8 justify-center')}
    >
      <span aria-hidden="true" className={cn('rounded-full', compact ? 'size-3' : 'size-2', dotClass, display.kind === 'working' && 'animate-pulse')} />
      {!compact && display.label}
    </span>
  )
}

export function CodexChat({
  project,
  requestedThreadId,
  onThreadSelected,
  onActiveThreadChange,
  pendingReviewContextAppend,
  onReviewContextAppendConsumed,
  onFilePathClick,
  onSessionStatusChange,
}: {
  project: string
  requestedThreadId?: string | null
  onThreadSelected?: (threadId: string | null) => void
  onActiveThreadChange?: (threadId: string | null) => void
  pendingReviewContextAppend?: ReviewContextAppend | null
  onReviewContextAppendConsumed?: (id: number) => void
  onFilePathClick?: (path: string, line?: number) => void
  onSessionStatusChange?: (status: string | null) => void
}) {
  const { confirm, prompt } = useDialogs()
  const [threads, setThreads] = useState<CodexThread[]>([])
  const [selectedThread, setSelectedThread] = useState<CodexThread | null>(null)
  const [turns, setTurns] = useState<CodexTurn[]>([])
  const [pendingRequests, setPendingRequests] = useState<ApprovalRequest[]>([])
  const [busyRequests, setBusyRequests] = useState<Set<string>>(() => new Set())
  const [draft, setDraft] = useState('')
  const [listLoading, setListLoading] = useState(true)
  const [historyLoading, setHistoryLoading] = useState(false)
  const [refreshingHistory, setRefreshingHistory] = useState(false)
  const [creating, setCreating] = useState(false)
  const [renamingThread, setRenamingThread] = useState(false)
  const [deletingThreadId, setDeletingThreadId] = useState<string | null>(null)
  const [replaceEmptyThread, setReplaceEmptyThread] = useState(false)
  const [sending, setSending] = useState(false)
  const [socketConnecting, setSocketConnecting] = useState(false)
  const [reconnectAttempt, setReconnectAttempt] = useState(0)
  const [commandPaletteOpen, setCommandPaletteOpen] = useState(false)
  const [recentCommandState, setRecentCommandState] = useState(() => ({ project, ids: readCodexRecentCommandIds(project) }))
  const [goalEditorOpen, setGoalEditorOpen] = useState(false)
  const [goalDraft, setGoalDraft] = useState('')
  const [goal, setGoal] = useState<JsonObject | null>(null)
  const [collaborationMode, setCollaborationMode] = useState<string | null>(null)
  const [controlBusy, setControlBusy] = useState(false)
  const [connected, setConnected] = useState(false)
  const [writerConflict, setWriterConflict] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [projectRoot, setProjectRoot] = useState('')
  const [listTruncated, setListTruncated] = useState(false)
  const [nextCursor, setNextCursor] = useState<string | null>(null)
  const socketRef = useRef<WebSocket | null>(null)
  const reconnectTimerRef = useRef<number | null>(null)
  const reconnectAttemptRef = useRef(0)
  const openSequenceRef = useRef(0)
  const threadListRequestRef = useRef(0)
  const handledRequestedThreadRef = useRef<string | null>(null)
  const processedReviewContextAppendRef = useRef(0)
  const controlRequestSequenceRef = useRef(0)
  const activeThreadIdRef = useRef<string | null>(null)
  const threadHasTurnsRef = useRef(false)
  const scrollRef = useRef<HTMLDivElement | null>(null)
  const scrollToBottomRef = useRef(true)
  const scrollAnchorRef = useRef<{ height: number; top: number } | null>(null)
  const composerRef = useRef<HTMLTextAreaElement | null>(null)
  const recentCommandIds = recentCommandState.project === project ? recentCommandState.ids : []
  const recentCommands = useMemo(
    () => recentCommandIds
      .map((id) => CODEX_COMMANDS.find((command) => command.id === id))
      .filter((command): command is (typeof CODEX_COMMANDS)[number] => command !== undefined),
    [recentCommandIds],
  )
  const active = useMemo(() => turnIsActive(turns), [turns])
  const sessionStatus = useMemo(() => {
    if (!selectedThread) return null
    if (pendingRequests.length > 0) return 'blocked'
    if (active) return 'active'
    const latestTurn = turns[turns.length - 1]
    return string(latestTurn?.status) || selectedThread.status
  }, [active, pendingRequests.length, selectedThread, turns])
  const representativeStatus = useMemo(() => {
    const statuses = new Map(threads.map((thread) => [thread.thread_id, thread.status] as const))
    if (selectedThread) statuses.set(selectedThread.thread_id, sessionStatus || selectedThread.status)
    const values = [...statuses.values()]
    for (const kind of ['blocked', 'done', 'working', 'idle', 'unknown'] as const) {
      const representative = values.find((status) => codexThreadStatus(status).kind === kind)
      if (representative) return representative
    }
    return null
  }, [selectedThread, sessionStatus, threads])

  useEffect(() => {
    onSessionStatusChange?.(representativeStatus)
  }, [onSessionStatusChange, representativeStatus])

  useEffect(() => {
    setRecentCommandState({ project, ids: readCodexRecentCommandIds(project) })
  }, [project])

  useEffect(() => {
    if (typeof window === 'undefined' || recentCommandState.project !== project) return
    try {
      window.localStorage.setItem(codexRecentCommandsStorageKey(project), JSON.stringify(recentCommandState.ids))
    } catch {
      // Recent commands remain available for the current component lifetime.
    }
  }, [project, recentCommandState])

  const loadThreads = useCallback(async (silent = false) => {
    const requestId = ++threadListRequestRef.current
    if (!silent) {
      setListLoading(true)
      setError(null)
    }
    try {
      const result = await api.listCodexThreads(project)
      if (requestId !== threadListRequestRef.current) return
      setThreads(result.threads)
      setListTruncated(result.truncated)
    } catch (loadError) {
      if (!silent && requestId === threadListRequestRef.current) {
        setError(loadError instanceof Error ? loadError.message : String(loadError))
      }
    } finally {
      if (!silent && requestId === threadListRequestRef.current) setListLoading(false)
    }
  }, [project])

  const removeDeletedThread = useCallback((threadId: string) => {
    setThreads((current) => current.filter((thread) => thread.thread_id !== threadId))
    if (activeThreadIdRef.current !== threadId) {
      void loadThreads(true)
      return
    }
    openSequenceRef.current += 1
    if (reconnectTimerRef.current !== null) window.clearTimeout(reconnectTimerRef.current)
    reconnectTimerRef.current = null
    reconnectAttemptRef.current = 0
    setReconnectAttempt(0)
    socketRef.current?.close()
    socketRef.current = null
    activeThreadIdRef.current = null
    threadHasTurnsRef.current = false
    setSelectedThread(null)
    setTurns([])
    setPendingRequests([])
    onActiveThreadChange?.(null)
    onThreadSelected?.(null)
    void loadThreads(true)
  }, [loadThreads, onActiveThreadChange, onThreadSelected])

  useEffect(() => {
    void loadThreads()
  }, [loadThreads])

  useEffect(() => {
    if (!selectedThread) return
    let refreshing = false
    const refresh = () => {
      if (document.hidden || refreshing) return
      refreshing = true
      void loadThreads(true).finally(() => { refreshing = false })
    }
    const interval = window.setInterval(refresh, 15000)
    document.addEventListener('visibilitychange', refresh)
    return () => {
      window.clearInterval(interval)
      document.removeEventListener('visibilitychange', refresh)
    }
  }, [loadThreads, selectedThread])

  useEffect(() => {
    let cancelled = false
    void api.listProjects().then((projects) => {
      if (!cancelled) setProjectRoot(projects.find((entry) => entry.name === project)?.path ?? '')
    }).catch(() => {
      if (!cancelled) setProjectRoot('')
    })
    return () => { cancelled = true }
  }, [project])

  const rewriteCodexLink = useCallback((href: string) => {
    if (href.startsWith('#') || href.startsWith('//')) return href
    const path = codexFileLinkPath(href, project, projectRoot)
    if (path) {
      const line = codexFileLinkLine(href)
      const query = line ? `?line=${line}` : ''
      return projectUrlFor(project, `/dashboard/files/${encodePath(path)}${query}`)
    }
    if (!/^[a-z][a-z\d+.-]*:/i.test(href) || /^(?:file|vscode|vscode-insiders):/i.test(href)) {
      return `#codex-file:${encodeURIComponent(href)}`
    }
    return href
  }, [project, projectRoot])

  useEffect(() => {
    const element = scrollRef.current
    if (!element) return
    const anchor = scrollAnchorRef.current
    if (anchor) {
      element.scrollTop = anchor.top + element.scrollHeight - anchor.height
      scrollAnchorRef.current = null
    } else if (scrollToBottomRef.current) {
      element.scrollTop = element.scrollHeight
    }
  }, [turns, pendingRequests.length])

  useEffect(() => {
    const append = pendingReviewContextAppend
    if (!append || !selectedThread || append.id <= processedReviewContextAppendRef.current) return

    processedReviewContextAppendRef.current = append.id
    if (append.threadId === selectedThread.thread_id) {
      setDraft((current) => appendReviewContextToDraft(current, append.context))
      requestAnimationFrame(() => composerRef.current?.focus())
    }
    onReviewContextAppendConsumed?.(append.id)
  }, [onReviewContextAppendConsumed, pendingReviewContextAppend, selectedThread])

  const openThread = useCallback(async (thread: CodexThread, automaticReconnect = false) => {
    if (reconnectTimerRef.current !== null) {
      window.clearTimeout(reconnectTimerRef.current)
      reconnectTimerRef.current = null
    }
    if (!automaticReconnect) {
      reconnectAttemptRef.current = 0
      setReconnectAttempt(0)
    }
    const openSequence = ++openSequenceRef.current
    handledRequestedThreadRef.current = thread.thread_id
    activeThreadIdRef.current = thread.thread_id
    onActiveThreadChange?.(thread.thread_id)
    onThreadSelected?.(thread.thread_id)
    socketRef.current?.close()
    socketRef.current = null
    setCommandPaletteOpen(false)
    setSelectedThread(thread)
    if (!automaticReconnect) {
      setTurns([])
      setPendingRequests([])
    }
    setNextCursor(null)
    scrollToBottomRef.current = true
    scrollAnchorRef.current = null
    setConnected(false)
    setCollaborationMode(null)
    setGoal(null)
    setGoalDraft('')
    setGoalEditorOpen(false)
    setControlBusy(false)
    setSocketConnecting(false)
    setWriterConflict(false)
    setHistoryLoading(true)
    setRefreshingHistory(false)
    if (!automaticReconnect) setError(null)
    if (!automaticReconnect) threadHasTurnsRef.current = false
    const scheduleReconnect = () => {
      if (reconnectAttemptRef.current >= CODEX_RECONNECT_DELAYS_MS.length) {
        setError('Codexとの接続を自動で復旧できませんでした。接続先を確認して手動で再接続してください。')
        setSocketConnecting(false)
        return
      }
      const attempt = reconnectAttemptRef.current + 1
      reconnectAttemptRef.current = attempt
      setReconnectAttempt(attempt)
      setSocketConnecting(true)
      setError(`Codexとの接続が切れました。自動再接続しています（${attempt}/${CODEX_RECONNECT_DELAYS_MS.length}）。`)
      reconnectTimerRef.current = window.setTimeout(() => {
        reconnectTimerRef.current = null
        void openThread(thread, true)
      }, CODEX_RECONNECT_DELAYS_MS[attempt - 1])
    }
    try {
      const history = await api.getCodexThreadHistory(thread.thread_id, { limit: 100 }, project)
      if (openSequenceRef.current !== openSequence) return
      const loadedTurns = history.turns.map((turn) => record(turn) as CodexTurn)
      if (!automaticReconnect || loadedTurns.length > 0) threadHasTurnsRef.current = loadedTurns.length > 0
      if (!automaticReconnect || loadedTurns.length > 0) onThreadSelected?.(thread.thread_id)
      setSelectedThread(history.thread)
      if (!automaticReconnect || loadedTurns.length > 0) setTurns(loadedTurns)
      setNextCursor(history.next_cursor ?? null)
    } catch (loadError) {
      if (openSequenceRef.current !== openSequence) return
      const message = loadError instanceof Error ? loadError.message : String(loadError)
      if (/codex conversation not found/i.test(message)) {
        setSelectedThread(null)
        activeThreadIdRef.current = null
        onActiveThreadChange?.(null)
        onThreadSelected?.(null)
        setError(null)
        setHistoryLoading(false)
        void loadThreads(true)
        setReplaceEmptyThread(true)
        return
      }
      setError(message)
      setHistoryLoading(false)
      if (automaticReconnect) scheduleReconnect()
      return
    }
    if (openSequenceRef.current !== openSequence) return
    setHistoryLoading(false)
    setSocketConnecting(true)
    const socket = new WebSocket(codexThreadWsUrl(thread.thread_id, project))
    socketRef.current = socket
    let connectionReady = false
    let connectionRejected = false
    socket.onopen = () => {
      if (openSequenceRef.current === openSequence) setSocketConnecting(true)
    }
    socket.onclose = () => {
      if (openSequenceRef.current !== openSequence) return
      setConnected(false)
      setSocketConnecting(false)
      setControlBusy(false)
      if (!connectionRejected) {
        scheduleReconnect()
      }
    }
    socket.onerror = () => {
      if (openSequenceRef.current !== openSequence) return
      setConnected(false)
      setSocketConnecting(false)
      setControlBusy(false)
      if (!connectionRejected) setError('Codexとの接続に失敗しました。myboxサーバーとapp-serverを確認して再接続してください。')
    }
    socket.onmessage = (event) => {
      if (openSequenceRef.current !== openSequence) return
      let message: JsonObject
      try {
        message = JSON.parse(String(event.data)) as JsonObject
      } catch {
        return
      }
      if (message.type === 'writer_conflict') {
        connectionRejected = true
        setWriterConflict(true)
        setConnected(false)
        setSocketConnecting(false)
      } else if (message.type === 'empty_thread_unavailable') {
        connectionRejected = true
        setConnected(false)
        setSocketConnecting(false)
        if (threadHasTurnsRef.current) {
          setError('Codexの会話を再開できませんでした。履歴を更新して再接続してください。')
        } else {
          setReplaceEmptyThread(true)
        }
      } else if (message.type === 'error') {
        const requestId = string(message.request_id)
        const messageError = string(message.error) || 'Codexとの接続でエラーが発生しました。'
        setError(messageError)
        if (requestId) setControlBusy(false)
        if (!requestId && !connectionReady) {
          connectionRejected = true
          setSocketConnecting(false)
        }
        if (requestId) setBusyRequests((current) => {
          const next = new Set(current)
          next.delete(requestId)
          return next
        })
      } else if (message.type === 'ready') {
        connectionReady = true
        reconnectAttemptRef.current = 0
        setReconnectAttempt(0)
        setSocketConnecting(false)
        setWriterConflict(false)
        setError(null)
        setConnected(true)
        const mode = string(record(message.collaboration_mode).mode)
        setCollaborationMode(mode || 'default')
        socket.send(JSON.stringify({ type: 'goal_get', request_id: `goal-get-${thread.thread_id}` }))
      } else if (message.type === 'prompt_started') {
        if (!threadHasTurnsRef.current) {
          threadHasTurnsRef.current = true
          handledRequestedThreadRef.current = thread.thread_id
          onThreadSelected?.(thread.thread_id)
        }
      } else if (message.type === 'response') {
        const requestId = string(message.request_id)
        setPendingRequests((current) => current.filter((entry) => entry.request_id !== requestId))
        setBusyRequests((current) => {
          const next = new Set(current)
          next.delete(requestId)
          return next
        })
      } else if (message.type === 'control_response') {
        const control = string(message.control)
        const result = record(message.result)
        setControlBusy(false)
        if (control === 'set_mode') {
          setCollaborationMode(string(result.mode) || string(record(result.collaboration_mode).mode) || 'default')
        } else if (control === 'goal_clear') {
          setGoal(null)
          setGoalDraft('')
        } else if (control.startsWith('goal_') && Object.prototype.hasOwnProperty.call(result, 'goal')) {
          const nextGoal = record(result.goal)
          setGoal(Object.keys(nextGoal).length > 0 ? nextGoal : null)
          setGoalDraft(string(nextGoal.objective))
        }
      } else if (message.type === 'server_request') {
        const request: ApprovalRequest = {
          request_id: string(message.request_id),
          method: string(message.method),
          params: record(message.params),
        }
        setPendingRequests((current) => current.some((entry) => entry.request_id === request.request_id) ? current : [...current, request])
      } else if (message.type === 'notification') {
        const method = string(message.method)
        const params = record(message.params)
        if (method === 'thread/deleted') {
          const deletedThreadId = string(params.threadId)
          if (deletedThreadId) removeDeletedThread(deletedThreadId)
        } else if (method === 'thread/name/updated' && string(params.threadId) === thread.thread_id) {
          const title = string(params.threadName).trim()
          if (title) {
            setThreads((current) => current.map((entry) => entry.thread_id === thread.thread_id ? { ...entry, title } : entry))
            setSelectedThread((current) => current?.thread_id === thread.thread_id ? { ...current, title } : current)
          }
        } else if (method === 'thread/goal/updated' && string(params.threadId) === thread.thread_id) {
          const updatedGoal = record(params.goal)
          setGoal(Object.keys(updatedGoal).length > 0 ? updatedGoal : null)
          setGoalDraft(string(updatedGoal.objective))
        } else if (method === 'thread/goal/cleared' && string(params.threadId) === thread.thread_id) {
          setGoal(null)
          setGoalDraft('')
        } else if (method === 'thread/settings/updated' && string(params.threadId) === thread.thread_id) {
          const mode = string(record(record(params.threadSettings).collaborationMode).mode)
          if (mode) setCollaborationMode(mode)
        }
        setTurns((current) => applyNotification(current, method, params))
      }
    }
  }, [loadThreads, onActiveThreadChange, onThreadSelected, project, removeDeletedThread])

  useEffect(() => {
    if (!requestedThreadId) {
      handledRequestedThreadRef.current = null
      return
    }
    if (listLoading || handledRequestedThreadRef.current === requestedThreadId) return

    handledRequestedThreadRef.current = requestedThreadId
    const thread = threads.find((entry) => entry.thread_id === requestedThreadId) ?? {
      thread_id: requestedThreadId,
      title: 'Codex conversation',
      preview: '',
      created_at: 0,
      updated_at: 0,
      status: 'idle',
      model: '',
    }
    void openThread(thread)
  }, [listLoading, openThread, requestedThreadId, threads])

  useEffect(() => {
    if (activeThreadIdRef.current) onActiveThreadChange?.(activeThreadIdRef.current)
    return () => {
      openSequenceRef.current += 1
      if (reconnectTimerRef.current !== null) window.clearTimeout(reconnectTimerRef.current)
      reconnectTimerRef.current = null
      socketRef.current?.close()
      socketRef.current = null
      onActiveThreadChange?.(null)
    }
  }, [onActiveThreadChange])

  const createThread = useCallback(async () => {
    if (creating) return
    setCreating(true)
    setError(null)
    try {
      const thread = await api.createCodexThread(project)
      setThreads((current) => [thread, ...current.filter((entry) => entry.thread_id !== thread.thread_id)])
      await openThread(thread)
    } catch (createError) {
      setError(createError instanceof Error ? createError.message : String(createError))
    } finally {
      setCreating(false)
    }
  }, [creating, openThread, project])

  useEffect(() => {
    if (!replaceEmptyThread) return
    setReplaceEmptyThread(false)
    void createThread()
  }, [createThread, replaceEmptyThread])

  const rememberRecentCommand = useCallback((commandId: string) => {
    if (!CODEX_COMMANDS.some((command) => command.id === commandId)) return
    setRecentCommandState((current) => {
      const currentIds = current.project === project ? current.ids : []
      return {
        project,
        ids: [commandId, ...currentIds.filter((id) => id !== commandId)].slice(0, CODEX_RECENT_COMMAND_LIMIT),
      }
    })
  }, [project])

  const sendText = useCallback((text: string, clearDraft: boolean): boolean => {
    const message = text.trim()
    const socket = socketRef.current
    if (!message || !socket || socket.readyState !== WebSocket.OPEN || sending) return false
    setSending(true)
    setError(null)
    socket.send(JSON.stringify({ type: 'prompt', text: message }))
    if (clearDraft) {
      setDraft('')
      requestAnimationFrame(() => composerRef.current?.focus())
    }
    window.setTimeout(() => setSending(false), 500)
    return true
  }, [sending])

  const sendControl = useCallback((type: string, payload: JsonObject = {}) => {
    const socket = socketRef.current
    if (!socket || socket.readyState !== WebSocket.OPEN) {
      setError('Codexとの接続がありません。再接続してから操作してください。')
      return false
    }
    controlRequestSequenceRef.current += 1
    const requestId = `control-${Date.now()}-${controlRequestSequenceRef.current}`
    setControlBusy(true)
    setError(null)
    socket.send(JSON.stringify({ type, request_id: requestId, ...payload }))
    return true
  }, [])

  const changeMode = useCallback((mode: 'plan' | 'default') => {
    if (sendControl('set_mode', { mode })) setCommandPaletteOpen(false)
  }, [sendControl])

  const openGoalEditor = useCallback(() => {
    setGoalDraft(string(goal?.objective))
    setGoalEditorOpen(true)
    setCommandPaletteOpen(true)
    sendControl('goal_get')
  }, [goal?.objective, sendControl])

  const submitGoal = useCallback(() => {
    if (sendControl('goal_set', { objective: goalDraft })) setCommandPaletteOpen(false)
  }, [goalDraft, sendControl])

  const runSlashCommand = useCallback((text: string): boolean => {
    const command = text.trim()
    if (command === '/plan' || command === '/default') {
      setDraft('')
      changeMode(command.slice(1) as 'plan' | 'default')
      return true
    }
    const goalCommand = command.match(/^\/goal(?:\s+([\s\S]*))?$/)
    if (!goalCommand) return false
    const argument = (goalCommand[1] ?? '').trim()
    setDraft('')
    if (!argument) {
      openGoalEditor()
    } else if (argument === 'pause' || argument === 'resume') {
      sendControl('goal_status', { status: argument === 'pause' ? 'paused' : 'active' })
    } else if (argument === 'clear') {
      sendControl('goal_clear')
    } else {
      sendControl('goal_set', { objective: argument })
    }
    return true
  }, [changeMode, openGoalEditor, sendControl])

  const sendComposer = useCallback(() => {
    if (runSlashCommand(draft)) return
    const sent = sendText(draft, true)
    if (sent) {
      const matchingCommand = CODEX_COMMANDS.find((command) => command.kind === 'prompt' && command.prompt === draft.trim())
      if (matchingCommand) rememberRecentCommand(matchingCommand.id)
    }
  }, [draft, rememberRecentCommand, runSlashCommand, sendText])

  const sendQuickPrompt = useCallback((text: string, commandId: string) => {
    setCommandPaletteOpen(false)
    if (sendText(text, false)) rememberRecentCommand(commandId)
  }, [rememberRecentCommand, sendText])

  const renameSelectedThread = useCallback(async () => {
    if (!selectedThread || renamingThread) return
    const name = await prompt('Codex会話の名前', selectedThread.title)
    if (name === null) return
    const trimmedName = name.trim()
    if (!trimmedName || trimmedName === selectedThread.title) return
    setRenamingThread(true)
    setError(null)
    try {
      await api.renameCodexThread(selectedThread.thread_id, trimmedName, project)
      setThreads((current) => current.map((thread) => thread.thread_id === selectedThread.thread_id ? { ...thread, title: trimmedName } : thread))
      setSelectedThread((current) => current?.thread_id === selectedThread.thread_id ? { ...current, title: trimmedName } : current)
    } catch (renameError) {
      setError(renameError instanceof Error ? renameError.message : String(renameError))
    } finally {
      setRenamingThread(false)
    }
  }, [project, prompt, renamingThread, selectedThread])

  const deleteThread = useCallback(async (thread: CodexThread) => {
    if (deletingThreadId) return
    const accepted = await confirm(
      `「${thread.title || 'New Codex conversation'}」を完全に削除しますか？\nこの操作は取り消せません。派生したCodex会話も削除されます。`,
    )
    if (!accepted) return
    setDeletingThreadId(thread.thread_id)
    setError(null)
    try {
      await api.deleteCodexThread(thread.thread_id, project)
      removeDeletedThread(thread.thread_id)
    } catch (deleteError) {
      setError(deleteError instanceof Error ? deleteError.message : String(deleteError))
    } finally {
      setDeletingThreadId(null)
    }
  }, [confirm, deletingThreadId, project, removeDeletedThread])

  const compactThread = useCallback(() => {
    const socket = socketRef.current
    if (!socket || socket.readyState !== WebSocket.OPEN || sending) return
    setCommandPaletteOpen(false)
    setSending(true)
    setError(null)
    socket.send(JSON.stringify({ type: 'compact' }))
    rememberRecentCommand('compact')
    window.setTimeout(() => setSending(false), 500)
  }, [rememberRecentCommand, sending])

  const handleConversationClick = useCallback((event: MouseEvent<HTMLDivElement>) => {
    if (!onFilePathClick || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return
    const anchor = (event.target as HTMLElement).closest<HTMLAnchorElement>('a[href]')
    if (!anchor || !event.currentTarget.contains(anchor)) return
    const href = anchor.getAttribute('href') ?? ''
    if (href.startsWith('#codex-file:')) {
      event.preventDefault()
      let target: string
      try {
        target = decodeURIComponent(href.slice('#codex-file:'.length))
      } catch {
        setError('このファイルリンクを読み取れませんでした。')
        return
      }
      const openFile = (root: string) => {
        const path = codexFileLinkPath(target, project, root)
        if (path) {
          onFilePathClick(path, codexFileLinkLine(target))
        } else {
          setError('プロジェクト内のファイルだけmyboxで開けます。')
        }
      }
      if (projectRoot) {
        openFile(projectRoot)
      } else {
        void api.listProjects().then((projects) => {
          const root = projects.find((entry) => entry.name === project)?.path ?? ''
          setProjectRoot(root)
          openFile(root)
        }).catch((loadError) => setError(loadError instanceof Error ? loadError.message : String(loadError)))
      }
      return
    }
    const path = codexFileLinkPath(href, project, projectRoot)
    if (!path) return
    event.preventDefault()
    onFilePathClick(path, codexFileLinkLine(href))
  }, [onFilePathClick, project, projectRoot])

  const interrupt = useCallback(() => {
    const socket = socketRef.current
    if (socket?.readyState === WebSocket.OPEN) socket.send(JSON.stringify({ type: 'interrupt' }))
  }, [])

  const respond = useCallback((request: ApprovalRequest, decision: string, answers?: JsonObject) => {
    const socket = socketRef.current
    if (!socket || socket.readyState !== WebSocket.OPEN) {
      setError('Codexとの接続がありません。再接続してから回答してください。')
      return
    }
    setBusyRequests((current) => new Set(current).add(request.request_id))
    socket.send(JSON.stringify({ type: 'respond', request_id: request.request_id, decision, answers }))
  }, [])

  const loadOlder = useCallback(async () => {
    if (!selectedThread || !nextCursor) return
    const openSequence = openSequenceRef.current
    try {
      const history = await api.getCodexThreadHistory(selectedThread.thread_id, { cursor: nextCursor, limit: 100 }, project)
      if (openSequenceRef.current !== openSequence) return
      const element = scrollRef.current
      if (element) scrollAnchorRef.current = { height: element.scrollHeight, top: element.scrollTop }
      const older = history.turns.map((turn) => record(turn) as CodexTurn)
      setTurns((current) => [...older.filter((turn) => !current.some((existing) => existing.id === turn.id)), ...current])
      setNextCursor(history.next_cursor ?? null)
    } catch (loadError) {
      if (openSequenceRef.current !== openSequence) return
      scrollAnchorRef.current = null
      setError(loadError instanceof Error ? loadError.message : String(loadError))
    }
  }, [nextCursor, project, selectedThread])

  const refreshHistory = useCallback(async () => {
    if (!selectedThread || refreshingHistory) return
    const openSequence = openSequenceRef.current
    setRefreshingHistory(true)
    setError(null)
    try {
      const history = await api.getCodexThreadHistory(selectedThread.thread_id, { limit: 100 }, project)
      if (openSequenceRef.current !== openSequence) return
      setTurns(history.turns.map((turn) => record(turn) as CodexTurn))
      setNextCursor(history.next_cursor ?? null)
    } catch (loadError) {
      if (openSequenceRef.current !== openSequence) return
      setError(loadError instanceof Error ? loadError.message : String(loadError))
    } finally {
      if (openSequenceRef.current === openSequence) setRefreshingHistory(false)
    }
  }, [project, refreshingHistory, selectedThread])

  const onComposerKeyDown = (event: KeyboardEvent<HTMLTextAreaElement>) => {
    if (event.key === 'Enter' && !event.shiftKey) {
      event.preventDefault()
      sendComposer()
    }
  }

  if (!selectedThread) {
    return (
      <div className="flex min-h-0 flex-1 flex-col" data-testid="codex-chat-history">
        <div className="flex shrink-0 items-center gap-2 border-b px-3 py-2">
          <div className="min-w-0 flex-1">
            <div className="text-xs font-semibold">Codex の会話</div>
            <div className="truncate text-[10px] text-muted-foreground">このプロジェクトのCodex CLI / IDE会話を表示</div>
          </div>
          <Button size="xs" className="shrink-0 cursor-pointer" onClick={() => void createThread()} disabled={creating} aria-label="新しいCodex会話">
            {creating ? <Loader2 className="mr-1 size-3 animate-spin" /> : <MessageSquarePlus className="mr-1 size-3" />}
            新規
          </Button>
          <Button size="icon-xs" variant="ghost" className="shrink-0 cursor-pointer" onClick={() => void loadThreads()} disabled={listLoading} aria-label="会話履歴を更新">
            <Loader2 className={cn('size-3.5', listLoading && 'animate-spin')} />
          </Button>
        </div>
        {error && <div role="alert" className="m-3 rounded-md border border-red-200 bg-red-50 p-2 text-xs text-red-700">{error}</div>}
        <div className="min-h-0 flex-1 overflow-y-auto p-2">
          {listLoading && threads.length === 0 ? (
            <div className="flex items-center justify-center gap-2 py-8 text-xs text-muted-foreground"><Loader2 className="size-4 animate-spin" />会話を読み込み中…</div>
          ) : threads.length === 0 ? (
            <div className="rounded-lg border border-dashed p-5 text-center text-xs text-muted-foreground">Codexの会話履歴はありません。新しい会話を始められます。</div>
          ) : (
            <div className="flex flex-col gap-1">
              {threads.map((thread) => (
                <div key={thread.thread_id} className="group flex min-w-0 items-stretch gap-1 rounded-lg border border-transparent hover:border-border hover:bg-accent/60">
                  <button type="button" className="flex min-w-0 flex-1 cursor-pointer flex-col gap-1 px-3 py-2 text-left" onClick={() => void openThread(thread)}>
                    <span className="flex min-w-0 items-center gap-2">
                      <span className="min-w-0 flex-1 truncate text-xs font-medium">{thread.title || 'New Codex conversation'}</span>
                      <CodexThreadStatus status={thread.status} />
                    </span>
                    <span className="line-clamp-2 w-full break-words text-[11px] text-muted-foreground">{thread.preview || 'メッセージなし'}</span>
                    <span className="flex w-full items-center justify-between gap-2 text-[10px] text-muted-foreground">
                      <span>{formatUpdatedAt(thread.updated_at)}</span>
                      <span className="truncate">{thread.model}</span>
                    </span>
                  </button>
                  <Button
                    size="icon-xs"
                    variant="ghost"
                    className="my-1 mr-1 shrink-0 cursor-pointer self-start text-muted-foreground opacity-70 hover:text-destructive sm:opacity-0 sm:group-hover:opacity-100 sm:focus-visible:opacity-100"
                    onClick={() => void deleteThread(thread)}
                    disabled={deletingThreadId !== null}
                    aria-label={`会話を削除: ${thread.title || 'New Codex conversation'}`}
                    title="Codex会話を完全に削除"
                  >
                    {deletingThreadId === thread.thread_id ? <Loader2 className="animate-spin" /> : <Trash2 />}
                  </Button>
                </div>
              ))}
            </div>
          )}
          {listTruncated && <p className="px-2 py-3 text-center text-[10px] text-muted-foreground">表示件数の上限に達しました。最近の会話を表示しています。</p>}
        </div>
      </div>
    )
  }

  const sessionThreads = threads.some((thread) => thread.thread_id === selectedThread.thread_id)
    ? threads
    : [selectedThread, ...threads]

  return (
    <div className="codex-chat-conversation flex min-h-0 min-w-0 flex-1 overflow-hidden" data-testid="codex-chat-conversation">
      <aside className="flex w-12 shrink-0 flex-col border-r" aria-label="Codex sessions" data-testid="codex-session-list">
        <div className="flex shrink-0 items-center justify-center gap-0.5 border-b px-1 py-2">
          <div className="flex shrink-0 flex-col items-center gap-0.5">
            <Button size="icon-xs" variant="ghost" className="cursor-pointer" onClick={() => void loadThreads()} disabled={listLoading} aria-label="セッション状態を更新" title="セッション状態を更新">
              <Loader2 className={cn('size-3.5', listLoading && 'animate-spin')} />
            </Button>
            <Button size="icon-xs" variant="ghost" className="cursor-pointer" onClick={() => void createThread()} disabled={creating} aria-label="新しいCodex会話" title="新しいCodex会話">
              {creating ? <Loader2 className="size-3.5 animate-spin" /> : <MessageSquarePlus className="size-3.5" />}
            </Button>
          </div>
        </div>
        <div className="min-h-0 flex-1 overflow-hidden p-1">
          {sessionThreads.map((thread) => {
            const status = thread.thread_id === selectedThread.thread_id ? sessionStatus || thread.status : thread.status
            const isSelected = thread.thread_id === selectedThread.thread_id
            return (
              <button
                key={thread.thread_id}
                type="button"
                aria-current={isSelected ? 'true' : undefined}
                className={cn(
                  'mb-1 flex h-10 w-full min-w-0 cursor-pointer items-center justify-center rounded-md p-1 hover:bg-accent/70',
                  isSelected && 'bg-accent text-accent-foreground',
                )}
                title={`${thread.title || 'New Codex conversation'} · ${status}`}
                aria-label={`${thread.title || 'New Codex conversation'} · ${status}`}
                data-testid={`codex-session-${thread.thread_id}`}
                onClick={() => { if (!isSelected) void openThread(thread) }}
              >
                <CodexThreadStatus status={status} compact />
              </button>
            )
          })}
          {sessionThreads.length === 0 && <p className="px-2 py-3 text-center text-[10px] text-muted-foreground">会話なし</p>}
        </div>
      </aside>
      <div className="flex min-h-0 min-w-0 flex-1 flex-col" onClick={handleConversationClick}>
      <div className="flex shrink-0 items-center gap-2 border-b px-2 py-2">
        <Button size="icon-xs" variant="ghost" className="shrink-0 cursor-pointer" onClick={() => { openSequenceRef.current += 1; if (reconnectTimerRef.current !== null) window.clearTimeout(reconnectTimerRef.current); reconnectTimerRef.current = null; reconnectAttemptRef.current = 0; setReconnectAttempt(0); socketRef.current?.close(); socketRef.current = null; setSelectedThread(null); activeThreadIdRef.current = null; onActiveThreadChange?.(null); onThreadSelected?.(null); void loadThreads() }} aria-label="会話一覧に戻る">
          <ArrowLeft />
        </Button>
        <div className="min-w-0 flex-1">
          <div className="truncate text-xs font-semibold" title={selectedThread.title}>{selectedThread.title || 'New Codex conversation'}</div>
          <div className={cn('text-[10px]', connected ? 'text-green-700 dark:text-green-400' : writerConflict ? 'text-amber-700 dark:text-amber-400' : 'text-muted-foreground')}>{connected ? `接続中${collaborationMode === 'plan' ? ' · Plan mode' : collaborationMode === 'default' ? ' · Default mode' : ''}` : writerConflict ? '他のCodexが操作中 · 読み取り専用' : historyLoading ? '履歴を読み込み中…' : socketConnecting && reconnectAttempt > 0 ? `自動再接続中… (${reconnectAttempt}/${CODEX_RECONNECT_DELAYS_MS.length})` : socketConnecting ? '接続中…' : '切断中'}</div>
        </div>
        <Button size="icon-xs" variant="ghost" className="shrink-0 cursor-pointer" onClick={() => void renameSelectedThread()} disabled={renamingThread} aria-label="会話名を変更" title="会話名を変更">
          {renamingThread ? <Loader2 className="animate-spin" /> : <Pencil />}
        </Button>
        {turns.length > 0 && <Button size="icon-xs" variant="ghost" className="shrink-0 cursor-pointer text-muted-foreground hover:text-destructive" onClick={() => void deleteThread(selectedThread)} disabled={deletingThreadId !== null} aria-label="Codex会話を削除" title="Codex会話を完全に削除">
          {deletingThreadId === selectedThread.thread_id ? <Loader2 className="animate-spin" /> : <Trash2 />}
        </Button>}
        {!connected && !historyLoading && <Button size="icon-xs" variant="ghost" className="shrink-0 cursor-pointer" onClick={() => void openThread(selectedThread)} aria-label={writerConflict ? 'Codexに再接続（元のCodex終了後）' : 'Codexに再接続'}><RefreshCw /></Button>}
      </div>
      {writerConflict && (
        <div role="status" className="mx-2 mt-2 flex items-center gap-2 rounded-md border border-amber-300 bg-amber-50 p-2 text-xs text-amber-950 dark:border-amber-700 dark:bg-amber-950/30 dark:text-amber-100">
          <p className="min-w-0 flex-1">別のCodexが操作中です。会話履歴は読み取り専用です。元のCodexを終了した後に再接続すると、myboxから操作できます。</p>
          <Button size="xs" variant="outline" className="shrink-0 cursor-pointer" onClick={() => void refreshHistory()} disabled={refreshingHistory}>
            {refreshingHistory ? <Loader2 className="mr-1 size-3 animate-spin" /> : <RefreshCw className="mr-1 size-3" />}
            履歴を更新
          </Button>
        </div>
      )}
      {error && !writerConflict && <div role="alert" className="mx-2 mt-2 rounded-md border border-red-200 bg-red-50 p-2 text-xs text-red-700">{error}</div>}
      <div
        ref={scrollRef}
        className="codex-chat-history min-h-0 min-w-0 flex-1 overflow-x-hidden overflow-y-auto px-3 py-3"
        onScroll={(event) => {
          const element = event.currentTarget
          scrollToBottomRef.current = element.scrollHeight - element.scrollTop - element.clientHeight < 80
        }}
      >
        {nextCursor && <div className="mb-3 text-center"><Button size="xs" variant="ghost" className="cursor-pointer" onClick={() => void loadOlder()}>以前の会話を読み込む</Button></div>}
        {historyLoading && <div className="py-8 text-center text-xs text-muted-foreground"><Loader2 className="mx-auto mb-2 size-4 animate-spin" />履歴を読み込み中…</div>}
        {!historyLoading && turns.length === 0 && <div className="py-8 text-center text-xs text-muted-foreground">メッセージを入力してCodexとの会話を始めましょう。</div>}
        <div className="flex flex-col gap-4">
          {turns.map((turn) => {
            const items = turn.items ?? []
            return (
              <section key={turn.id} className="flex min-w-0 flex-col gap-3" aria-label="Codex turn">
                {items.map((item, index) => <TurnItem key={item.id || `${turn.id}-${index}`} item={item} transformHref={rewriteCodexLink} />)}
                {activeStatus(turn.status) && <div className="flex items-center gap-2 text-[11px] text-muted-foreground"><Loader2 className="size-3 animate-spin" />Codexが作業中…</div>}
                {(turn.status === 'failed' || turn.status === 'interrupted') && (
                  <div role="status" className="rounded-md border border-amber-300 bg-amber-50 px-3 py-2 text-xs text-amber-950 dark:border-amber-700 dark:bg-amber-950/30 dark:text-amber-100">
                    <div className="font-medium">{turn.status === 'failed' ? 'Codexの処理が失敗しました' : 'Codexの処理が中断されました'}</div>
                    {turnFailureMessage(turn) && <div className="mt-1 whitespace-pre-wrap break-words">{turnFailureMessage(turn)}</div>}
                  </div>
                )}
                {turn.status === 'completed' && !turnHasAgentMessage(turn) && (
                  <div role="status" className="rounded-md border border-amber-300 bg-amber-50 px-3 py-2 text-xs text-amber-950 dark:border-amber-700 dark:bg-amber-950/30 dark:text-amber-100">
                    Codexの処理は完了しましたが、応答メッセージがありません。履歴を更新して状態を確認してください。
                  </div>
                )}
              </section>
            )
          })}
          {pendingRequests.map((request) => <ApprovalRequestView key={request.request_id} request={request} busy={busyRequests.has(request.request_id)} onRespond={respond} />)}
        </div>
      </div>
      <div className="shrink-0 border-t p-2">
        <div className="mb-1.5 flex min-w-0 items-center gap-1.5" data-testid="codex-command-strip">
          <label htmlFor="codex-mode-selector" className="sr-only">Codexモード</label>
          <select
            id="codex-mode-selector"
            aria-label="CodexモードまたはGoal管理"
            value={goalEditorOpen ? 'goal' : collaborationMode ?? 'default'}
            onChange={(event) => {
              if (event.target.value === 'goal') openGoalEditor()
              else changeMode(event.target.value === 'plan' ? 'plan' : 'default')
            }}
            disabled={!connected || controlBusy}
            className="h-8 w-24 shrink-0 rounded-md border bg-background px-1.5 text-xs"
            title="Codexモードを切り替え"
          >
            <option value="default">通常</option>
            <option value="plan">Plan</option>
            <option value="goal">Goal</option>
          </select>
          <span className="shrink-0 text-[10px] text-muted-foreground">最近</span>
          {recentCommands.length > 0 ? (
            <div className="codex-recent-command-list flex min-w-0 flex-1 items-center gap-1 overflow-x-auto" aria-label="最近使ったコマンド">
              {recentCommands.map((command) => (
                <Button
                  key={command.id}
                  size="xs"
                  variant="outline"
                  className="h-8 shrink-0 cursor-pointer px-2 text-[11px]"
                  disabled={!connected || sending || controlBusy}
                  title={command.label}
                  aria-label={`最近使ったコマンド: ${command.label}`}
                  onClick={() => command.kind === 'compact' ? compactThread() : sendQuickPrompt(command.prompt, command.id)}
                >
                  {command.shortLabel}
                </Button>
              ))}
            </div>
          ) : (
            <span className="min-w-0 flex-1 truncate text-[10px] text-muted-foreground">使ったコマンドがここに表示されます</span>
          )}
        </div>
        <div className="flex min-w-0 items-stretch gap-2 rounded-xl border bg-background p-1.5 focus-within:ring-2 focus-within:ring-ring/30">
          <textarea
            ref={composerRef}
            value={draft}
            onChange={(event) => setDraft(event.target.value)}
            onKeyDown={onComposerKeyDown}
            placeholder={active ? 'Codexに追加の指示を送信…' : 'Codexにメッセージ…'}
            className="max-h-36 min-h-9 min-w-0 flex-1 resize-y bg-transparent px-2 py-2 text-sm outline-none placeholder:text-muted-foreground"
            aria-label="Codexへのメッセージ"
            disabled={!connected}
          />
          <div className="flex shrink-0 flex-col gap-1">
            <Button
              size="icon-sm"
              variant="ghost"
              className="shrink-0 cursor-pointer"
              onClick={() => setCommandPaletteOpen(true)}
              aria-label="Codexコマンド"
              title="Codexコマンド"
            >
              <Keyboard />
            </Button>
            {active && <Button size="icon-sm" variant="ghost" className="shrink-0 cursor-pointer text-destructive" onClick={interrupt} disabled={!connected} aria-label="Codexを停止"><Square /></Button>}
              <Button size="icon-sm" className="shrink-0 cursor-pointer" onClick={sendComposer} disabled={!draft.trim() || !connected || sending || controlBusy} aria-label={active ? 'Codexに追加指示を送信' : 'Codexに送信'} title={active ? 'Steer' : 'Send'}><Send /></Button>
          </div>
        </div>
        <div className="mt-1 px-1 text-[10px] text-muted-foreground">Enterで送信 · Shift+Enterで改行{active ? ' · 送信内容は実行中のCodexへ追加されます（Steer）' : ''}</div>
      </div>
      <Sheet open={commandPaletteOpen} onOpenChange={(open) => { setCommandPaletteOpen(open); if (!open) setGoalEditorOpen(false) }}>
        <SheetContent side="bottom" className="max-h-[80vh] gap-0 overflow-y-auto p-0" data-testid="codex-command-palette">
          <SheetHeader className="border-b px-4 py-3">
            <SheetTitle>{goalEditorOpen ? 'Codex Goal' : 'Codex コマンド'}</SheetTitle>
            <SheetDescription>{goalEditorOpen ? 'この会話の長期目標を設定・管理します。' : 'モード切替とクイック操作を選べます。'}</SheetDescription>
          </SheetHeader>
          <div className="flex min-w-0 flex-col gap-4 p-4">
            {goalEditorOpen ? (
              <div className="flex flex-col gap-3">
                <label htmlFor="codex-goal-objective" className="text-xs font-semibold">目標</label>
                <textarea
                  id="codex-goal-objective"
                  value={goalDraft}
                  onChange={(event) => setGoalDraft(event.target.value)}
                  placeholder="例: この機能を実装し、必要な確認を終える"
                  maxLength={10000}
                  rows={4}
                  className="min-h-24 resize-y rounded-md border bg-background px-3 py-2 text-sm outline-none focus-visible:ring-2 focus-visible:ring-ring/50"
                />
                {goal && <p className="text-[11px] text-muted-foreground">状態: {string(goal.status) || 'active'}{typeof goal.tokensUsed === 'number' ? ` · 使用トークン ${goal.tokensUsed}` : ''}</p>}
                <div className="flex flex-wrap justify-end gap-2">
                  {goal && <Button size="xs" variant="outline" className="cursor-pointer" disabled={!connected || controlBusy} onClick={() => sendControl('goal_clear')}>Goalを削除</Button>}
                  {goal && (goal.status === 'active' || goal.status === 'paused') && <Button size="xs" variant="outline" className="cursor-pointer" disabled={!connected || controlBusy} onClick={() => sendControl('goal_status', { status: goal.status === 'paused' ? 'active' : 'paused' })}>{goal.status === 'paused' ? '再開' : '一時停止'}</Button>}
                  <Button size="xs" className="cursor-pointer" disabled={!connected || controlBusy || !goalDraft.trim()} onClick={submitGoal}>{controlBusy ? <Loader2 className="mr-1 size-3 animate-spin" /> : null}{goal ? '目標を更新' : '目標を設定'}</Button>
                </div>
                <Button variant="ghost" size="xs" className="cursor-pointer self-start" onClick={() => setGoalEditorOpen(false)}>コマンド一覧に戻る</Button>
              </div>
            ) : <>
            <div className="flex flex-col gap-2">
              <p className="text-xs font-semibold tracking-wider text-muted-foreground uppercase">会話</p>
              <Button
                variant="outline"
                size="xs"
                className="min-h-8 cursor-pointer justify-start px-2 text-xs"
                disabled={creating}
                onClick={() => {
                  setCommandPaletteOpen(false)
                  void createThread()
                }}
              >
                {creating ? <Loader2 className="mr-2 size-3 animate-spin" /> : <MessageSquarePlus className="mr-2 size-3" />}
                新しい会話
              </Button>
            </div>
            <div className="flex flex-col gap-2">
              <p className="text-xs font-semibold tracking-wider text-muted-foreground uppercase">モードと目標</p>
              <div className="grid grid-cols-2 gap-1">
                <Button variant={collaborationMode === 'plan' ? 'default' : 'outline'} size="xs" className="min-h-8 cursor-pointer justify-start px-2 text-xs" disabled={!connected || controlBusy} onClick={() => changeMode('plan')}>/plan · 計画</Button>
                <Button variant={collaborationMode === 'default' ? 'default' : 'outline'} size="xs" className="min-h-8 cursor-pointer justify-start px-2 text-xs" disabled={!connected || controlBusy} onClick={() => changeMode('default')}>/default · 通常</Button>
                <Button variant="outline" size="xs" className="min-h-8 cursor-pointer justify-start px-2 text-xs" disabled={!connected || controlBusy} onClick={openGoalEditor}>/goal · 目標を管理</Button>
              </div>
              <p className="text-[10px] text-muted-foreground">モードの変更は次のターンから適用されます。入力欄では /goal pause・/goal resume・/goal clear も使えます。</p>
            </div>
            <div className="flex flex-col gap-2">
              <p className="text-xs font-semibold tracking-wider text-muted-foreground uppercase">クイック操作</p>
              <div className="grid grid-cols-2 gap-1" data-testid="codex-command-palette-prompts">
                {CODEX_COMMANDS.map((command) => (
                  <Button
                    key={command.id}
                    variant="outline"
                    size="xs"
                    className="min-h-8 min-w-0 cursor-pointer justify-start truncate px-2 text-xs"
                    disabled={!connected || sending}
                    title={command.label}
                    onClick={() => command.kind === 'compact' ? compactThread() : sendQuickPrompt(command.prompt, command.id)}
                  >
                    {command.label}
                  </Button>
                ))}
              </div>
            </div>
            </>}
          </div>
        </SheetContent>
      </Sheet>
      </div>
    </div>
  )
}
