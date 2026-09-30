import { useCallback, useEffect, useRef, useState } from 'react'
import { HerdrAgent, HerdrOverview, api } from '../api/client'
import { StatusDot } from './herdr-status'
import { Button } from './ui/button'
import { taskAgentName, taskDirFromPath } from '../utils/herdr-file-agent'
import { Bot, CalendarClock, ChevronRight, Loader2, Square, X } from 'lucide-react'
import { cn } from '@/lib/utils'
import { SyntaxHighlighter } from './SyntaxHighlighter'
import { paneColumnWidth } from '../utils/herdr-layout'
import { AGENT_COMMANDS } from '../utils/herdr-agent-commands'
import {
  HERDR_AGENT_OUTPUT_HEIGHT_STORAGE_KEY,
  HERDR_AGENT_PROMPT_HEIGHT_STORAGE_KEY,
  useResizableHeight,
} from '../hooks/use-resizable-height'
import {
  formatDateTimeLocal,
  formatScheduledAt,
  fromScheduledPromptResponse,
  migrateLegacyScheduledPrompts,
  type ScheduledPrompt,
} from '../utils/scheduled-prompts'

const FILE_AGENT_KEYS: { label: string; key: string }[] = [
  { label: 'Enter', key: 'enter' },
  { label: 'Tab', key: 'tab' },
  { label: '↑', key: 'up' },
  { label: '↓', key: 'down' },
  { label: 'Esc', key: 'esc' },
  { label: 'Ctrl+C', key: 'C-c' },
]

const FILE_AGENT_KIND_OPTIONS = ['opencode', 'codex'] as const

const KIND_STORAGE_KEY = 'mybox.herdr.file-agent-kind'
export interface FileAgentWidgetProps {
  /** Project-relative path of the open file the agent works on. Only files
   *  inside a task directory (_tasks/<dir>/...) can start an agent; otherwise
   *  the widget is not rendered. */
  path: string
  overview: HerdrOverview | null
  onRefresh: () => void
  webuiFocusedPaneId?: string | null
  onWebuiFocusChange?: (paneId: string | null) => void
}

export function FileAgentWidget({
  path,
  overview,
  onRefresh,
  webuiFocusedPaneId,
  onWebuiFocusChange,
}: FileAgentWidgetProps) {
  const dir = taskDirFromPath(path)
  const name = taskAgentName(path)

  const [open, setOpen] = useState(true)
  const [starting, setStarting] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [output, setOutput] = useState<string | null>(null)
  const [outputError, setOutputError] = useState<string | null>(null)
  const [draft, setDraft] = useState('')
  const [scheduleAt, setScheduleAt] = useState('')
  const [scheduledPrompts, setScheduledPrompts] = useState<ScheduledPrompt[]>([])
  const [sending, setSending] = useState(false)
  const [scheduling, setScheduling] = useState(false)
  const [keySending, setKeySending] = useState<string | null>(null)
  const [commandSending, setCommandSending] = useState<string | null>(null)
  const [kind, setKind] = useState<string>(() => {
    const saved = window.localStorage.getItem(KIND_STORAGE_KEY)
    return saved && (FILE_AGENT_KIND_OPTIONS as readonly string[]).includes(saved) ? saved : 'opencode'
  })
  const loadingRef = useRef(false)
  const scheduledRevisionRef = useRef(0)
  const preRef = useRef<HTMLDivElement | null>(null)
  const outputSize = useResizableHeight<HTMLDivElement>({
    storageKey: HERDR_AGENT_OUTPUT_HEIGHT_STORAGE_KEY,
    defaultHeight: 320,
    minHeight: 128,
  })
  const promptSize = useResizableHeight<HTMLTextAreaElement>({
    storageKey: HERDR_AGENT_PROMPT_HEIGHT_STORAGE_KEY,
    defaultHeight: 48,
    minHeight: 40,
  })
  const agentRef = useRef<HerdrAgent | undefined>(undefined)
  const refreshRef = useRef(onRefresh)
  const prevPaneIdRef = useRef<string | null>(null)
  const reportedWebuiPaneIdRef = useRef<string | null>(null)
  // Terminal column width of the agent's pane, so the output wraps at the same
  // columns as the real herdr pane instead of the web pane's width.
  const [cols, setCols] = useState<number | undefined>(undefined)

  const agent = (overview?.agents ?? []).find((a) => a.name === name)
  agentRef.current = agent
  refreshRef.current = onRefresh

  const visibleScheduledPrompts = scheduledPrompts.filter(
    (scheduled) => scheduled.target === name || scheduled.target === agent?.pane_id,
  )

  const loadOutput = useCallback(
    async (reportError = true) => {
      const a = agentRef.current
      if (!a || loadingRef.current) return
      loadingRef.current = true
      try {
        const res = await api.readHerdrAgent(a.pane_id)
        setOutput(res.output)
        setOutputError(null)
      } catch (e) {
        if (reportError) setOutputError(e instanceof Error ? e.message : String(e))
      } finally {
        loadingRef.current = false
      }
    },
    [],
  )

  const stop = useCallback(() => {
    if (!agent) return
    const pane = (overview?.panes ?? []).find((p) => p.pane_id === agent.pane_id)
    const tab = pane ? (overview?.tabs ?? []).find((t) => t.tab_id === pane.tab_id) : undefined
    const isOnlyPaneInTab = pane
      ? (overview?.panes ?? []).filter((p) => p.tab_id === pane.tab_id).length === 1
      : false
    setSending(true)
    setError(null)
    void api
      .sendKeysHerdrAgent(agent.pane_id, ['C-c', 'C-c'])
      .then(() => api.closeHerdrPane(agent.pane_id))
      .then(async () => {
        if (isOnlyPaneInTab && tab) await api.closeHerdrTab(tab.tab_id)
        await onRefresh()
      })
      .catch((e) => setError(e instanceof Error ? e.message : String(e)))
      .finally(() => setSending(false))
  }, [agent, onRefresh, overview])

  useEffect(() => {
    const paneId = agent?.pane_id ?? null
    if (prevPaneIdRef.current !== paneId) {
      setOutput(null)
      setOutputError(null)
    }
    prevPaneIdRef.current = paneId
    if (!agent || !open || !overview?.available) return
    void loadOutput()
    const id = setInterval(() => {
      if (!document.hidden) void loadOutput(false)
    }, 1500)
    return () => clearInterval(id)
  }, [agent?.pane_id, open, overview?.available, loadOutput])

  useEffect(() => {
    const paneId = agent?.pane_id ?? null
    if (!paneId || !overview?.available) {
      setCols(undefined)
      return
    }
    let cancelled = false
    void api
      .getHerdrLayouts()
      .then((res) => {
        if (!cancelled) setCols(paneColumnWidth(res.layouts, paneId))
      })
      .catch(() => {
        if (!cancelled) setCols(undefined)
      })
    return () => {
      cancelled = true
    }
  }, [agent?.pane_id, open, overview?.available])

  useEffect(() => {
    const paneId = agent?.pane_id
    if (!paneId || !open || !overview?.available) return
    void api
      .focusHerdrAgent(paneId)
      .then(() => refreshRef.current())
      .catch((e) => setError(e instanceof Error ? e.message : String(e)))
  }, [agent?.pane_id, open, overview?.available])

  useEffect(() => {
    const paneId = open && overview?.available ? agent?.pane_id ?? null : null
    if (reportedWebuiPaneIdRef.current === paneId) return
    reportedWebuiPaneIdRef.current = paneId
    onWebuiFocusChange?.(paneId)
  }, [agent?.pane_id, open, overview?.available, onWebuiFocusChange])

  useEffect(() => {
    window.localStorage.setItem(KIND_STORAGE_KEY, kind)
  }, [kind])

  useEffect(() => {
    const el = preRef.current
    if (el) el.scrollTop = el.scrollHeight
  }, [output])

  const start = useCallback(() => {
    if (starting) return
    setStarting(true)
    setError(null)
    void api
      .startHerdrFileAgent(path, kind)
      .then(() => {
        setOpen(true)
        onRefresh()
      })
      .catch((e) => setError(e instanceof Error ? e.message : String(e)))
      .finally(() => setStarting(false))
  }, [path, kind, starting, onRefresh])

  const sendPrompt = useCallback(async () => {
    const text = draft.trim()
    if (!text || !agent || sending) return
    setSending(true)
    setError(null)
    try {
      await api.promptHerdrAgent(agent.pane_id, text)
      setDraft('')
      void loadOutput()
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    } finally {
      setSending(false)
    }
  }, [draft, agent, sending, loadOutput])

  const loadScheduledPrompts = useCallback(async () => {
    const revision = scheduledRevisionRef.current
    try {
      const prompts = await api.listHerdrScheduledPrompts()
      const current = prompts.map(fromScheduledPromptResponse).filter((prompt): prompt is ScheduledPrompt => prompt !== null)
      const migrated = await migrateLegacyScheduledPrompts((target, text, scheduledAt) =>
        api.createHerdrScheduledPrompt(target, text, scheduledAt),
      )
      if (revision === scheduledRevisionRef.current) setScheduledPrompts([...current, ...migrated])
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    }
  }, [])

  useEffect(() => {
    void loadScheduledPrompts()
    const id = setInterval(() => void loadScheduledPrompts(), 5000)
    return () => clearInterval(id)
  }, [loadScheduledPrompts])

  const schedulePrompt = useCallback(async () => {
    const text = draft.trim()
    const timestamp = Date.parse(scheduleAt)
    if (!text) return
    if (!Number.isFinite(timestamp) || timestamp <= Date.now()) {
      setError('Schedule time must be in the future')
      return
    }
    scheduledRevisionRef.current += 1
    setScheduling(true)
    setError(null)
    try {
      const created = await api.createHerdrScheduledPrompt(
        agent?.pane_id ?? name,
        text,
        new Date(timestamp).toISOString(),
      )
      const scheduled = fromScheduledPromptResponse(created)
      if (scheduled) setScheduledPrompts((current) => [...current, scheduled])
      setDraft('')
      setScheduleAt('')
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    } finally {
      setScheduling(false)
    }
  }, [agent?.pane_id, draft, name, scheduleAt])

  const cancelScheduledPrompt = useCallback(async (id: string) => {
    scheduledRevisionRef.current += 1
    try {
      await api.deleteHerdrScheduledPrompt(id)
      setScheduledPrompts((current) => current.filter((scheduled) => scheduled.id !== id))
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    }
  }, [])

  const sendKey = useCallback(
    async (key: string) => {
      if (!agent || keySending) return
      setKeySending(key)
      setError(null)
      try {
        await api.sendKeysHerdrAgent(agent.pane_id, [key])
        void loadOutput()
      } catch (e) {
        setError(e instanceof Error ? e.message : String(e))
      } finally {
        setKeySending(null)
      }
    },
    [agent, keySending, loadOutput],
  )

  const sendCommand = useCallback(
    async (command: string) => {
      if (!agent || commandSending) return
      setCommandSending(command)
      setError(null)
      try {
        await api.promptHerdrAgent(agent.pane_id, command)
        void loadOutput()
      } catch (e) {
        setError(e instanceof Error ? e.message : String(e))
      } finally {
        setCommandSending(null)
      }
    },
    [agent, commandSending, loadOutput],
  )

  // Files outside a task directory cannot start an agent: hide the widget.
  if (dir === null) return null

  const header = (
    <button
      type="button"
      className="flex w-full cursor-pointer items-center gap-1.5 px-1 py-1.5 text-left"
      onClick={() => setOpen((o) => !o)}
      aria-expanded={open}
      aria-label={open ? `Collapse agent for ${dir}` : `Expand agent for ${dir}`}
    >
      <ChevronRight className={cn('size-3.5 shrink-0 text-muted-foreground transition-transform', open && 'rotate-90')} />
      <Bot className="size-3.5 shrink-0 text-muted-foreground" />
      <span className="min-w-0 flex-1 truncate text-xs font-semibold" title={dir}>
        {dir}
      </span>
      {agent ? (
        <StatusDot status={agent.status} />
      ) : (
        !starting && (
          <span className="text-[10px] tracking-wider text-muted-foreground uppercase">off</span>
        )
      )}
      {agent?.pane_id === webuiFocusedPaneId && (
        <span
          data-testid="webui-focus-indicator"
          className="rounded border border-sky-200 bg-sky-50 px-1.5 py-0.5 text-[10px] font-medium text-sky-700 dark:border-sky-800 dark:bg-sky-950 dark:text-sky-300"
        >
          mybox focused
        </span>
      )}
      {starting && <Loader2 className="size-3 shrink-0 animate-spin" />}
    </button>
  )

  // herdr is unavailable: keep the header so the widget stays discoverable.
  if (!overview?.available) {
    return (
      <div className="file-agent-widget min-w-0 w-full mb-3 rounded-md border border-border bg-muted/40">
        {header}
      </div>
    )
  }

  if (!agent) {
    return (
      <div className="file-agent-widget min-w-0 w-full mb-3 rounded-md border border-border bg-muted/40">
        {header}
        {open && (
          <div className="px-1.5 pb-1.5">
            <div className="mb-1.5 flex items-center gap-1.5">
              <label className="text-[10px] tracking-wider text-muted-foreground uppercase">Agent kind</label>
              <select
                value={kind}
                onChange={(e) => setKind(e.target.value)}
                aria-label="Agent kind"
                className="cursor-pointer rounded border bg-background px-1.5 py-0.5 text-xs outline-none"
              >
                {FILE_AGENT_KIND_OPTIONS.map((k) => (
                  <option key={k} value={k}>
                    {k}
                  </option>
                ))}
              </select>
            </div>
            <Button
              variant="outline"
              size="xs"
              className="w-full cursor-pointer"
              onClick={start}
              disabled={starting}
              aria-label={`Start agent for ${dir}`}
            >
              {starting ? (
                <>
                  <Loader2 className="size-3 animate-spin" />
                  Starting…
                </>
              ) : (
                <>
                  <Bot className="size-3" />
                  Start agent
                </>
              )}
            </Button>
            {error && <p className="mt-1.5 text-[11px] text-red-600">{error}</p>}
            <p className="mt-1 text-[10px] leading-snug text-muted-foreground">
              herdr will open a tab named “{dir}” and run a {kind} agent shared by every file in
              the task directory.
            </p>
          </div>
        )}
      </div>
    )
  }

  const agentName: HerdrAgent = agent

  return (
    <div className="file-agent-widget min-w-0 w-full mb-3 rounded-md border border-border bg-muted/40">
      {header}
      {open && (
        <>
          {error && <p className="px-1.5 text-[11px] text-red-600">{error}</p>}
          {outputError && <p className="px-1.5 text-[11px] text-red-600">{outputError}</p>}
          <div
            ref={(element) => {
              preRef.current = element
              outputSize.ref(element)
            }}
            data-testid="file-agent-output"
            className="mx-1.5 min-h-32 max-h-[70vh] resize-y overflow-auto rounded border bg-background p-1.5 text-[11px]"
            style={{ height: outputSize.height }}
          >
            <SyntaxHighlighter text={output ?? 'loading…'} cols={cols} linkFilePaths />
          </div>
          <div className="flex flex-wrap items-center gap-1 px-1.5 pt-1.5">
            {FILE_AGENT_KEYS.map((k) => (
              <Button
                key={k.key}
                variant="outline"
                size="xs"
                className="cursor-pointer px-1.5 font-mono text-[10px]"
                title={`Press ${k.label}`}
                disabled={keySending !== null || sending}
                onClick={() => void sendKey(k.key)}
              >
                [{keySending === k.key ? '…' : k.label}]
              </Button>
            ))}
            <span aria-hidden="true" className="h-4 w-px bg-border" />
            {AGENT_COMMANDS.map((c) => (
              <Button
                key={c.id}
                variant="outline"
                size="xs"
                className="cursor-pointer px-1.5 font-mono text-[10px] text-muted-foreground"
                title={`Run ${c.label}`}
                disabled={commandSending !== null || sending}
                onClick={() => void sendCommand(c.command)}
              >
                {commandSending === c.command ? '…' : c.label}
              </Button>
            ))}
            <Button
              variant="ghost"
              size="xs"
              className="ml-auto cursor-pointer text-[10px] text-muted-foreground"
              onClick={stop}
              disabled={sending}
              aria-label={`Stop and remove panel for ${agentName.name}`}
              title={`Stop and remove panel for ${agentName.name}`}
            >
              <Square className="size-3" />
              Stop
            </Button>
          </div>
          <div className="flex flex-col gap-1.5 p-1.5 sm:flex-row sm:items-start">
            <textarea
              ref={promptSize.ref}
              aria-label={`Prompt ${agentName.name}`}
              data-testid="file-agent-prompt-input"
              value={draft}
              onChange={(e) => setDraft(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) {
                  e.preventDefault()
                  void sendPrompt()
                }
              }}
              placeholder="Prompt… (Ctrl+Enter to send)"
              rows={2}
              className="min-h-10 max-h-[40vh] w-full min-w-0 resize-y rounded-md border border-input bg-background px-2 py-1.5 text-xs outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 sm:flex-1"
              style={{ height: promptSize.height }}
            />
            <div data-testid="file-agent-prompt-actions" className="flex w-full flex-wrap items-start gap-1.5 sm:w-auto sm:flex-nowrap">
              <Button
                size="xs"
                className="cursor-pointer self-end"
                onClick={() => void sendPrompt()}
                disabled={sending || draft.trim() === ''}
                aria-label={`Send prompt to ${agentName.name}`}
              >
                Send
              </Button>
              <div className="flex min-w-0 flex-1 items-center gap-1 sm:min-w-40 sm:flex-col sm:items-stretch">
                <input
                  type="datetime-local"
                  aria-label="Schedule send time"
                  value={scheduleAt}
                  min={formatDateTimeLocal(new Date())}
                  onChange={(e) => setScheduleAt(e.target.value)}
                  className="h-6 w-full min-w-0 rounded-md border border-input bg-background px-1.5 text-[11px] outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50"
                />
                <Button
                  variant="outline"
                  size="xs"
                  className="shrink-0 cursor-pointer"
                  onClick={schedulePrompt}
                  disabled={sending || scheduling || draft.trim() === '' || scheduleAt === ''}
                  aria-label="Schedule send"
                >
                  <CalendarClock className="size-3" />
                  Schedule
                </Button>
              </div>
            </div>
          </div>
          {visibleScheduledPrompts.length > 0 && (
            <div className="space-y-1 px-1.5 pb-1.5">
              <p className="text-[10px] tracking-wider text-muted-foreground uppercase">Scheduled sends</p>
              {visibleScheduledPrompts.map((scheduled) => (
                <div
                  key={scheduled.id}
                  data-testid="scheduled-prompt"
                  className="flex min-w-0 items-center gap-1 rounded border bg-background px-1.5 py-1 text-[11px]"
                >
                  <span className="min-w-0 flex-1 truncate" title={scheduled.text}>
                    {scheduled.text}
                  </span>
                  <time className="shrink-0 text-muted-foreground" dateTime={new Date(scheduled.scheduledAt).toISOString()}>
                    {formatScheduledAt(scheduled.scheduledAt)}
                  </time>
                  <Button
                    variant="ghost"
                    size="xs"
                    className="size-5 shrink-0 cursor-pointer p-0"
                    onClick={() => cancelScheduledPrompt(scheduled.id)}
                    aria-label={`Cancel scheduled prompt ${scheduled.text}`}
                    title="Cancel scheduled prompt"
                  >
                    <X className="size-3" />
                  </Button>
                </div>
              ))}
            </div>
          )}
        </>
      )}
    </div>
  )
}
