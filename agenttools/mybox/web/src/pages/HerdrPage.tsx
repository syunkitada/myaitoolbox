import { useCallback, useEffect, useRef, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { dirName, getProject } from '../utils/routes'
import type { HerdrAgent, HerdrOverview, HerdrPane, HerdrTab, HerdrWorkspace } from '../api/client'
import { api } from '../api/client'
import { StatusBadge, StatusDot } from '../components/herdr-status'
import { Button } from '../components/ui/button'
import { RefreshCw, Send } from 'lucide-react'
import { cn } from '@/lib/utils'

interface HerdrPageProps {
  overview: HerdrOverview | null
  error: string | null
  loading: boolean
  refresh: () => Promise<void>
}

interface AgentDetailProps {
  agent: HerdrAgent
}

function AgentDetail({ agent }: AgentDetailProps) {
  const [output, setOutput] = useState<string | null>(null)
  const [outputError, setOutputError] = useState<string | null>(null)
  const [draft, setDraft] = useState('')
  const [sending, setSending] = useState(false)
  const [notice, setNotice] = useState<string | null>(null)

  const loadOutput = useCallback(async () => {
    try {
      const res = await api.readHerdrAgent(agent.pane_id)
      setOutput(res.output)
      setOutputError(null)
    } catch (e) {
      setOutputError(e instanceof Error ? e.message : String(e))
    }
  }, [agent.pane_id])

  useEffect(() => {
    void loadOutput()
  }, [loadOutput])

  const sendPrompt = useCallback(async () => {
    const text = draft.trim()
    if (!text || sending) return
    setSending(true)
    setNotice(null)
    try {
      await api.promptHerdrAgent(agent.pane_id, text)
      setDraft('')
      setNotice('prompt submitted')
      setTimeout(() => setNotice(null), 3000)
      void loadOutput()
    } catch (e) {
      setNotice(e instanceof Error ? e.message : String(e))
    } finally {
      setSending(false)
    }
  }, [draft, sending, agent.pane_id, loadOutput])

  return (
    <div className="herdr-agent-detail mt-2 rounded-md border bg-muted/40 p-3" data-testid={`agent-detail-${agent.pane_id}`}>
      <div className="mb-2 flex items-center justify-between gap-2">
        <span className="text-xs font-semibold tracking-wider text-muted-foreground uppercase">Terminal output</span>
        <Button variant="ghost" size="xs" className="cursor-pointer" onClick={() => void loadOutput()}>
          Reload output
        </Button>
      </div>
      {outputError && <p className="mb-2 text-xs text-red-600">{outputError}</p>}
      <pre className="max-h-64 overflow-auto rounded border bg-background p-2 text-xs whitespace-pre-wrap">
        {output ?? 'loading...'}
      </pre>
      <div className="mt-3 flex items-start gap-2">
        <textarea
          aria-label={`Prompt ${agent.name}`}
          data-testid="herdr-prompt-input"
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={(e) => {
            if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') void sendPrompt()
          }}
          placeholder="Send a prompt to this agent (Ctrl+Enter to submit)"
          rows={2}
          className="min-h-0 flex-1 resize-y rounded-md border bg-background px-2 py-1.5 text-sm outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50"
        />
        <Button size="sm" className="cursor-pointer self-end" disabled={sending || !draft.trim()} onClick={() => void sendPrompt()}>
          <Send />
          Send
        </Button>
      </div>
      {notice && <p className="herdr-prompt-notice mt-1 text-xs text-muted-foreground">{notice}</p>}
    </div>
  )
}

type OnChanged = () => Promise<void>
type OnError = (message: string) => void

async function runOp(fn: () => Promise<unknown>, onError: OnError, onChanged: OnChanged) {
  try {
    await fn()
    await onChanged()
  } catch (e) {
    onError(e instanceof Error ? e.message : String(e))
  }
}

interface PaneRowProps {
  pane: HerdrPane
  focused: boolean
  onFocus: () => void
  autoReload: boolean
  onChanged: OnChanged
  onError: OnError
}

function PaneRow({ pane, focused, onFocus, autoReload, onChanged, onError }: PaneRowProps) {
  const [open, setOpen] = useState(true)
  const [output, setOutput] = useState<string | null>(null)
  const [mode, setMode] = useState<'send-text-enter' | 'send-text' | 'send-keys' | 'prompt'>('send-text-enter')
  const [draft, setDraft] = useState('')
  const [sending, setSending] = useState(false)
  const [notice, setNotice] = useState<string | null>(null)
  const loadingRef = useRef(false)
  const preRef = useRef<HTMLPreElement>(null)
  // While true the viewport follows new output; scrolling up pauses the follow.
  const pinnedRef = useRef(true)

  const handlePreScroll = useCallback(() => {
    const el = preRef.current
    if (!el) return
    pinnedRef.current = el.scrollHeight - el.scrollTop - el.clientHeight < 32
  }, [])

  // Keep the terminal pinned to its latest output while auto reloading.
  useEffect(() => {
    const el = preRef.current
    if (!el || !pinnedRef.current) return
    el.scrollTop = el.scrollHeight
  }, [output])

  const loadOutput = useCallback(async (reportError = true) => {
    // Skip if a reload is already in flight (e.g. the 1s focus poller).
    if (loadingRef.current) return
    loadingRef.current = true
    try {
      const res = await api.readHerdrPane(pane.pane_id)
      setOutput(res.output)
    } catch (e) {
      // Background polling stays silent; only manual reloads surface errors.
      if (reportError) onError(e instanceof Error ? e.message : String(e))
    } finally {
      loadingRef.current = false
    }
  }, [pane.pane_id, onError])

  useEffect(() => {
    if (!open) return
    void loadOutput()
    // The focused pane keeps its terminal output fresh by polling every second.
    if (!focused || !autoReload) return
    const id = setInterval(() => {
      if (!document.hidden) void loadOutput(false)
    }, 1000)
    return () => clearInterval(id)
  }, [open, focused, autoReload, loadOutput])

  const handleSend = useCallback(async (customText?: string, customMode?: 'send-text-enter' | 'send-text' | 'send-keys' | 'prompt') => {
    const targetMode = customMode ?? mode
    const textToSend = customText ?? draft.trim()
    if (!textToSend || sending) return

    setSending(true)
    setNotice(null)
    try {
      if (targetMode === 'send-text-enter') {
        await api.sendTextHerdrPane(pane.pane_id, textToSend)
        await api.sendKeysHerdrPane(pane.pane_id, ['Enter'])
      } else if (targetMode === 'prompt') {
        await api.promptHerdrAgent(pane.pane_id, textToSend)
      } else if (targetMode === 'send-text') {
        await api.sendTextHerdrPane(pane.pane_id, textToSend)
      } else if (targetMode === 'send-keys') {
        const keys = textToSend.split(/\s+/).filter(Boolean)
        await api.sendKeysHerdrPane(pane.pane_id, keys)
      }
      if (!customText) setDraft('')
      setNotice(`${targetMode} sent`)
      setTimeout(() => setNotice(null), 3000)
      if (open) void loadOutput()
      await onChanged()
    } catch (e) {
      onError(e instanceof Error ? e.message : String(e))
    } finally {
      setSending(false)
    }
  }, [draft, sending, mode, pane.pane_id, open, loadOutput, onChanged, onError])

  const renamePane = () => {
    const label = window.prompt(`Rename pane ${pane.pane_id}`, pane.title ?? '')
    if (label === null) return
    if (!label.trim()) return
    void runOp(() => api.renameHerdrPane(pane.pane_id, label.trim()), onError, onChanged)
  }

  const closePane = () => {
    if (!window.confirm(`Close pane ${pane.pane_id}?`)) return
    void runOp(() => api.closeHerdrPane(pane.pane_id), onError, onChanged)
  }

  return (
    <div
      className={cn(
        'herdr-pane-row flex flex-col rounded border bg-card text-card-foreground shadow-xs transition-all',
        focused ? 'border-primary ring-1 ring-primary/40' : 'border-border',
      )}
      data-testid={`herdr-pane-${pane.pane_id}`}
      data-focused={focused ? 'true' : 'false'}
    >
      {/* Pane Header / Titlebar */}
      <div
        className="flex cursor-pointer flex-wrap items-center justify-between gap-1.5 border-b bg-muted/40 px-2.5 py-1.5 text-xs"
        onClick={onFocus}
        title="Focus this pane"
        data-testid={`herdr-pane-header-${pane.pane_id}`}
      >
        <div className="flex items-center gap-1.5 min-w-0">
          <StatusDot status={pane.agent_status ?? 'unknown'} />
          <span className="font-mono font-semibold text-foreground">{pane.pane_id}</span>
          {dirName(pane.cwd) && (
            <span className="truncate text-[11px] text-muted-foreground" title={pane.cwd ?? ''}>
              [{dirName(pane.cwd)}]
            </span>
          )}
          {pane.title && <span className="truncate text-muted-foreground">({pane.title})</span>}
          {focused && (
            <span className="rounded bg-primary/10 px-1 py-0.2 text-[10px] font-medium text-primary">
              active
            </span>
          )}
        </div>
        <div className="herdr-pane-actions flex items-center gap-1">
          <Button
            variant={open ? 'secondary' : 'ghost'}
            size="xs"
            className="h-6 cursor-pointer px-1.5 text-[11px]"
            onClick={() => setOpen((o) => !o)}
            title="Toggle output terminal window"
          >
            {open ? 'Hide Terminal' : 'View Terminal'}
          </Button>
          <Button
            variant="ghost"
            size="xs"
            className="h-6 cursor-pointer px-1.5 text-[11px]"
            title="Split right"
            aria-label={`Split ${pane.pane_id} right`}
            onClick={() => void runOp(() => api.splitHerdrPane(pane.pane_id, 'right'), onError, onChanged)}
          >
            Split →
          </Button>
          <Button
            variant="ghost"
            size="xs"
            className="h-6 cursor-pointer px-1.5 text-[11px]"
            title="Split down"
            aria-label={`Split ${pane.pane_id} down`}
            onClick={() => void runOp(() => api.splitHerdrPane(pane.pane_id, 'down'), onError, onChanged)}
          >
            Split ↓
          </Button>
          <Button
            variant="ghost"
            size="xs"
            className="h-6 cursor-pointer px-1.5 text-[11px]"
            onClick={renamePane}
            title="Rename pane"
          >
            Rename
          </Button>
          <Button
            variant="ghost"
            size="xs"
            className="herdr-pane-close h-6 cursor-pointer px-1.5 text-[11px] text-red-600 hover:bg-red-50 hover:text-red-700"
            onClick={closePane}
            title="Close pane"
          >
            Close
          </Button>
        </div>
      </div>

      {/* Pane Content / Terminal Output */}
      {open ? (
        <div className="herdr-pane-detail p-2">
          <div className="mb-1 flex items-center justify-between">
            <span className="text-[10px] uppercase font-mono tracking-wider text-muted-foreground">Terminal Output</span>
            <Button variant="ghost" size="xs" className="h-5 px-1 text-[10px]" onClick={() => void loadOutput()}>
              <RefreshCw className="mr-1 size-3" /> Reload
            </Button>
          </div>
          <pre
            ref={preRef}
            onScroll={handlePreScroll}
            className="max-h-64 overflow-auto rounded bg-black p-2.5 font-mono text-xs text-green-400 whitespace-pre-wrap"
          >
            {output ?? 'loading terminal output...'}
          </pre>
        </div>
      ) : (
        <div className="p-2.5 text-xs text-muted-foreground font-mono flex items-center justify-between">
          <span>Status: {pane.agent_status ?? 'idle'}</span>
          <button
            type="button"
            onClick={() => setOpen(true)}
            className="cursor-pointer text-primary hover:underline"
          >
            Open terminal preview →
          </button>
        </div>
      )}

      {/* Input / Prompt Section */}
      <div className="border-t bg-muted/20 p-2">
        <div className="mb-1.5 flex flex-wrap items-center justify-between gap-1 text-[11px]">
          <div className="flex items-center gap-1">
            <span className="text-muted-foreground font-medium">Input mode:</span>
            <select
              value={mode}
              onChange={(e) => setMode(e.target.value as 'send-text-enter' | 'send-text' | 'send-keys' | 'prompt')}
              className="rounded border bg-background px-1.5 py-0.5 text-xs outline-none cursor-pointer font-medium"
            >
              <option value="send-text-enter">send-text + Enter (default)</option>
              <option value="send-text">send-text (literal text only)</option>
              <option value="send-keys">send-keys (keys e.g. Enter, C-c)</option>
              <option value="prompt">prompt (agent prompt)</option>
            </select>
          </div>
          <div className="flex items-center gap-1">
            <Button
              variant="outline"
              size="xs"
              className="h-5 px-1.5 text-[10px] font-mono"
              title="Send Enter key press"
              disabled={sending}
              onClick={() => void handleSend('Enter', 'send-keys')}
            >
              [Enter]
            </Button>
            <Button
              variant="outline"
              size="xs"
              className="h-5 px-1.5 text-[10px] font-mono"
              title="Send Ctrl+C key press"
              disabled={sending}
              onClick={() => void handleSend('C-c', 'send-keys')}
            >
              [Ctrl+C]
            </Button>
          </div>
        </div>

        <div className="flex items-center gap-1.5">
          <input
            type="text"
            aria-label={`Input pane ${pane.pane_id}`}
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') void handleSend()
            }}
            placeholder={
              mode === 'send-text-enter'
                ? 'Send text + Enter to pane...'
                : mode === 'send-text'
                  ? 'Send literal text to pane (without Enter)...'
                  : mode === 'send-keys'
                    ? 'Send keys (e.g. Enter, C-c, Down)...'
                    : 'Send agent prompt...'
            }
            className="flex-1 rounded border bg-background px-2.5 py-1 text-xs outline-none focus-visible:border-ring focus-visible:ring-1 focus-visible:ring-ring"
          />
          <Button
            size="xs"
            className="h-7 cursor-pointer px-2"
            disabled={sending || !draft.trim()}
            onClick={() => void handleSend()}
          >
            <Send className="mr-1 size-3" />
            Send
          </Button>
        </div>
        {notice && <p className="mt-1 text-[11px] text-muted-foreground">{notice}</p>}
      </div>
    </div>
  )
}

function TabRow({ tab, panes, active, onSelect, onChanged, onError }: {
  tab: HerdrTab
  panes: HerdrPane[]
  active: boolean
  onSelect: () => void
  onChanged: OnChanged
  onError: OnError
}) {
  const renameTab = (e: React.MouseEvent) => {
    e.stopPropagation()
    const label = window.prompt(`Rename tab "${tab.label}"`, tab.label)
    if (label === null) return
    if (!label.trim()) return
    void runOp(() => api.renameHerdrTab(tab.tab_id, label.trim()), onError, onChanged)
  }

  const closeTab = (e: React.MouseEvent) => {
    e.stopPropagation()
    const extra =
      panes.length === 0 || (tab.pane_count ?? panes.length) <= 1
        ? ' This is the last tab of the workspace; closing it also closes the workspace.'
        : ''
    if (!window.confirm(`Close tab "${tab.label}"?${extra}`)) return
    void runOp(() => api.closeHerdrTab(tab.tab_id), onError, onChanged)
  }

  return (
    <div
      onClick={onSelect}
      className={cn(
        'group flex cursor-pointer items-center gap-1.5 rounded-t border border-b-0 px-3 py-1.5 text-xs transition-colors',
        active
          ? 'border-primary bg-background font-semibold text-foreground'
          : 'border-transparent bg-muted/50 text-muted-foreground hover:bg-muted hover:text-foreground',
      )}
      data-testid={`herdr-tab-${tab.tab_id}`}
      data-active={active ? 'true' : 'false'}
    >
      <StatusDot status={tab.agent_status ?? 'unknown'} />
      <span>
        {tab.number ?? '0'}:{tab.label}
      </span>
      {active && <span className="text-[10px] text-primary font-mono">*</span>}
      <div className="ml-1 hidden items-center gap-1 group-hover:flex">
        <button
          type="button"
          title="Rename tab"
          aria-label={`Rename tab ${tab.tab_id}`}
          onClick={renameTab}
          className="text-muted-foreground hover:text-foreground"
        >
          ✎
        </button>
        <button
          type="button"
          title="Close tab"
          aria-label={`Close tab ${tab.tab_id}`}
          onClick={closeTab}
          className="text-red-500 hover:text-red-700 font-bold"
        >
          ×
        </button>
      </div>
    </div>
  )
}

interface WorkspaceSectionProps {
  ws: HerdrWorkspace
  tabs: HerdrTab[]
  panes: HerdrPane[]
  current: boolean
  urlTabId: string | null
  urlPaneId: string | null
  onSelectTab: (tabId: string) => void
  onSelectPane: (paneId: string) => void
  autoReload: boolean
  onChanged: OnChanged
  onError: OnError
}

function WorkspaceSection({
  ws,
  tabs,
  panes,
  current,
  urlTabId,
  urlPaneId,
  onSelectTab,
  onSelectPane,
  autoReload,
  onChanged,
  onError,
}: WorkspaceSectionProps) {
  // Tab and pane focus are managed by the web UI (persisted in the URL),
  // not by the herdr CLI; herdr's own focused flags are ignored here.
  const sortedTabs = [...tabs].sort((a, b) => (a.number ?? 0) - (b.number ?? 0))
  const selectedTabId =
    urlTabId && sortedTabs.some((t) => t.tab_id === urlTabId)
      ? urlTabId
      : (sortedTabs[0]?.tab_id ?? null)

  const createTab = () => {
    const label = window.prompt(`New tab in workspace "${ws.label}" (optional name)`, '')
    if (label === null) return
    void runOp(
      () => api.createHerdrTab(ws.workspace_id, label.trim() || undefined),
      onError,
      onChanged,
    )
  }

  const activeTab = sortedTabs.find((t) => t.tab_id === selectedTabId) ?? sortedTabs[0]
  const activePanes = activeTab ? panes.filter((p) => p.tab_id === activeTab.tab_id) : []

  return (
    <div
      className={cn('rounded-lg border bg-card p-4 shadow-sm', current && 'border-primary ring-2 ring-primary/20')}
      data-testid={`herdr-workspace-${ws.workspace_id}`}
    >
      {/* Workspace Header */}
      <div className="mb-3 flex flex-wrap items-center justify-between gap-2 border-b pb-2">
        <div className="flex min-w-0 items-center gap-2">
          <StatusDot status={ws.agent_status} />
          <span className="truncate text-base font-bold">{ws.label}</span>
          {current && (
            <span className="shrink-0 rounded bg-accent px-1.5 py-0.5 text-[11px] font-medium text-primary">
              this project
            </span>
          )}
          <StatusBadge status={ws.agent_status} />
        </div>
        <div className="flex items-center gap-2">
          <span className="text-xs text-muted-foreground">
            #{ws.number ?? '—'} · {ws.tab_count ?? tabs.length} tabs · {ws.pane_count ?? panes.length} panes
          </span>
          <Button variant="outline" size="xs" className="cursor-pointer" onClick={createTab}>
            + New Tab
          </Button>
        </div>
      </div>

      {/* tmux Tab Bar */}
      {sortedTabs.length === 0 ? (
        <p className="rounded-md border border-dashed p-4 text-center text-xs text-muted-foreground">
          No tabs in this workspace. Click "+ New Tab" to create one.
        </p>
      ) : (
        <div className="flex flex-col">
          <div className="flex flex-wrap items-center gap-1 border-b bg-muted/30 px-2 pt-2">
            {sortedTabs.map((t) => (
              <TabRow
                key={t.tab_id}
                tab={t}
                panes={panes.filter((p) => p.tab_id === t.tab_id)}
                active={t.tab_id === activeTab?.tab_id}
                onSelect={() => onSelectTab(t.tab_id)}
                onChanged={onChanged}
                onError={onError}
              />
            ))}
          </div>

          {/* Pane View Container (tmux style grid/split) */}
          <div className="bg-background/50 p-3 rounded-b border border-t-0">
            {activePanes.length === 0 ? (
              <p className="p-4 text-center text-xs text-muted-foreground">No panes in tab "{activeTab?.label}".</p>
            ) : (
              <div
                className={cn(
                  'grid gap-3',
                  activePanes.length === 1 && 'grid-cols-1',
                  activePanes.length === 2 && 'grid-cols-1 md:grid-cols-2',
                  activePanes.length >= 3 && 'grid-cols-1 md:grid-cols-2 lg:grid-cols-3',
                )}
              >
                {activePanes.map((p) => (
                  <PaneRow
                    key={p.pane_id}
                    pane={p}
                    focused={urlPaneId === p.pane_id}
                    onFocus={() => onSelectPane(p.pane_id)}
                    autoReload={autoReload}
                    onChanged={onChanged}
                    onError={onError}
                  />
                ))}
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  )
}

export function HerdrPage({ overview, error, loading, refresh }: HerdrPageProps) {
  const project = getProject()
  const [searchParams, setSearchParams] = useSearchParams()
  const requestedAgent = searchParams.get('agent')
  // Focus state lives in the URL (?tab=<tab_id>&pane=<pane_id>) so that
  // reloading the browser restores exactly the same tab/pane focus.
  const urlTabId = searchParams.get('tab')
  const urlPaneId = searchParams.get('pane')
  const [openPane, setOpenPane] = useState<string | null>(null)
  // Auto reload refreshes the focused pane's terminal output every second.
  const [autoReload, setAutoReload] = useState(true)

  const selectTab = useCallback(
    (tabId: string) => {
      setSearchParams(
        (prev) => {
          const next = new URLSearchParams(prev)
          next.set('tab', tabId)
          next.delete('pane')
          return next
        },
        { replace: true },
      )
    },
    [setSearchParams],
  )

  const selectPane = useCallback(
    (paneId: string) => {
      setSearchParams(
        (prev) => {
          const next = new URLSearchParams(prev)
          next.set('pane', paneId)
          return next
        },
        { replace: true },
      )
    },
    [setSearchParams],
  )

  const workspaces = overview?.workspaces ?? []
  const agents = overview?.agents ?? []
  const tabs = overview?.tabs ?? []
  const panes = overview?.panes ?? []
  const matched = workspaces.find((w) => w.label === project)
  const [opError, setOpError] = useState<string | null>(null)

  const onError = useCallback((message: string) => {
    setOpError(message)
    setTimeout(() => setOpError((cur) => (cur === message ? null : cur)), 6000)
  }, [])

  // Auto-open the agent requested via ?agent=<pane_id> (sidebar deep link).
  useEffect(() => {
    if (!requestedAgent) return
    if ((overview?.agents ?? []).some((a) => a.pane_id === requestedAgent)) {
      setOpenPane(requestedAgent)
    }
  }, [requestedAgent, overview])

  return (
    <div className="page p-4 md:p-6">
      <div className="page-header mb-3 flex items-center justify-between gap-3">
        <div>
          <h1 className="text-2xl font-bold">Herdr</h1>
          <p className="page-subtitle mt-0.5 text-sm text-muted-foreground">
            {matched
              ? `Workspace “${project}” is ${matched.agent_status}. Operate herdr agents here.`
              : `Project “${project}” has no matching herdr workspace.`}
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Button
            variant={autoReload ? 'secondary' : 'outline'}
            size="sm"
            className="cursor-pointer"
            onClick={() => setAutoReload((v) => !v)}
            aria-pressed={autoReload}
            data-testid="herdr-auto-reload-toggle"
            title="Toggle auto reload of the focused pane's terminal output"
          >
            <RefreshCw className={cn(autoReload && loading && 'animate-spin')} />
            Auto reload: {autoReload ? 'ON' : 'OFF'}
          </Button>
          <Button variant="ghost" size="sm" className="cursor-pointer" onClick={() => void refresh()} title="Refresh">
            <RefreshCw className={cn(loading && 'animate-spin')} />
            Refresh
          </Button>
        </div>
      </div>

      {error && (
        <div className="error-banner my-2 flex items-center justify-between rounded-md border border-red-200 bg-red-50 px-3.5 py-2.5 text-sm text-red-700">
          {error}
        </div>
      )}
      {opError && (
        <div
          role="alert"
          className="herdr-op-error my-2 flex items-center justify-between rounded-md border border-red-200 bg-red-50 px-3.5 py-2.5 text-sm text-red-700"
        >
          {opError}
        </div>
      )}
      {overview && !overview.available && (
        <div className="my-2 rounded-md border border-amber-200 bg-amber-50 px-3.5 py-2.5 text-sm text-amber-800">
          herdr command is not available on this server.
        </div>
      )}

      <section className="herdr-workspaces mb-6">
        <h2 className="mb-2 text-sm font-semibold tracking-wider text-muted-foreground uppercase">
          Workspaces · Tabs · Panes
        </h2>
        {workspaces.filter((w) => w.label === project).length === 0 ? (
          <p className="rounded-md border border-dashed p-4 text-center text-xs text-muted-foreground">
            No herdr workspace found for project "{project}".
          </p>
        ) : (
          <div className="flex flex-col gap-3">
            {workspaces
              .filter((w) => w.label === project)
              .map((w) => (
                <WorkspaceSection
                  key={w.workspace_id}
                  ws={w}
                  tabs={tabs.filter((t) => t.workspace_id === w.workspace_id)}
                  panes={panes.filter((p) => p.workspace_id === w.workspace_id)}
                  current={true}
                  urlTabId={urlTabId}
                  urlPaneId={urlPaneId}
                  onSelectTab={selectTab}
                  onSelectPane={selectPane}
                  autoReload={autoReload}
                  onChanged={refresh}
                  onError={onError}
                />
              ))}
          </div>
        )}
      </section>

      <section className="herdr-agents">
        <h2 className="mb-2 text-sm font-semibold tracking-wider text-muted-foreground uppercase">Agents</h2>
        {(() => {
          const projectWorkspaceIds = new Set(
            workspaces.filter((w) => w.label === project).map((w) => w.workspace_id),
          )
          const projectAgents = agents.filter((a) => projectWorkspaceIds.has(a.workspace_id))

          if (projectAgents.length === 0) {
            return (
              <p className="rounded-md border border-dashed p-4 text-center text-xs text-muted-foreground">
                No herdr agents running for project "{project}".
              </p>
            )
          }

          return (
            <div className="flex flex-col gap-2">
              {projectAgents.map((a) => {
                const ws = workspaces.find((w) => w.workspace_id === a.workspace_id)
                return (
                  <div
                    key={a.pane_id}
                    className={cn(
                      'rounded-lg border bg-card p-3',
                      openPane === a.pane_id && 'border-primary ring-2 ring-primary/20',
                    )}
                    data-testid={`herdr-agent-${a.pane_id}`}
                  >
                    <button
                      type="button"
                      className="herdr-agent-row flex w-full cursor-pointer flex-wrap items-center gap-2 text-left"
                      onClick={() => setOpenPane(openPane === a.pane_id ? null : a.pane_id)}
                      aria-expanded={openPane === a.pane_id}
                    >
                      <StatusDot status={a.status} />
                      <span className="font-semibold">{a.name}</span>
                      <StatusBadge status={a.status} />
                      {ws && <span className="text-xs text-muted-foreground">in {ws.label}</span>}
                      {a.focused && <span className="text-xs text-muted-foreground">· focused</span>}
                      <span className="ml-auto truncate font-mono text-xs text-muted-foreground">
                        {a.pane_id}
                      </span>
                      {a.title && (
                        <span className="herdr-agent-title w-full truncate text-xs text-muted-foreground">
                          {a.title}
                        </span>
                      )}
                    </button>
                    {openPane === a.pane_id && <AgentDetail agent={a} />}
                  </div>
                )
              })}
            </div>
          )
        })()}
      </section>
    </div>
  )
}
