import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { ChevronDown, FileText, RefreshCw, Send } from 'lucide-react'
import { dirName, encodePath, getProject, projectUrl } from '../utils/routes'
import type { HerdrAgent, HerdrLayout, HerdrOverview, HerdrPane, HerdrTab, HerdrWorkspace } from '../api/client'
import { api } from '../api/client'
import { StatusBadge, StatusDot } from '../components/herdr-status'
import { Button } from '../components/ui/button'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '../components/ui/collapsible'
import { useDialogs } from '../components/AppDialogs'
import { cn } from '@/lib/utils'
import { SyntaxHighlighter } from '../components/SyntaxHighlighter'
import { useIsMobile } from '../hooks/use-mobile'
import { filePathForAgent } from '../utils/herdr-file-agent'
import { AGENT_COMMANDS } from '../utils/herdr-agent-commands'
import {
  edgeNeighborsForSplit,
  layoutBoxForTab,
  MAX_RATIO,
  MIN_RATIO,
} from '../utils/herdr-layout'
import type { Divider, Rect } from '../utils/herdr-layout'

interface HerdrPageProps {
  overview: HerdrOverview | null
  error: string | null
  loading: boolean
  refresh: () => Promise<void>
}

interface AgentDetailProps {
  agent: HerdrAgent
  autoReload: boolean
  onRename?: () => void
  // Terminal column width of the agent's pane, used to render the output at
  // the same wrap columns as the real herdr pane.
  cols?: number
}

// Quick keys sent to the agent terminal via `herdr agent send-keys`.
// Key names must be accepted by the herdr CLI (e.g. Enter, esc, C-c).
const AGENT_QUICK_KEYS: { label: string; key: string }[] = [
  { label: 'Enter', key: 'enter' },
  { label: 'Esc', key: 'esc' },
  { label: 'Ctrl+C', key: 'C-c' },
  { label: 'Tab', key: 'Tab' },
  { label: '↑', key: 'Up' },
  { label: '↓', key: 'Down' },
]

function AgentDetail({ agent, autoReload, onRename, cols }: AgentDetailProps) {
  const [output, setOutput] = useState<string | null>(null)
  const [outputError, setOutputError] = useState<string | null>(null)
  const [draft, setDraft] = useState('')
  const [sending, setSending] = useState(false)
  const [keySending, setKeySending] = useState<string | null>(null)
  const [commandSending, setCommandSending] = useState<string | null>(null)
  const [notice, setNotice] = useState<string | null>(null)
  const loadingRef = useRef(false)
  const preRef = useRef<HTMLDivElement>(null)
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
      const res = await api.readHerdrAgent(agent.pane_id)
      setOutput(res.output)
      setOutputError(null)
    } catch (e) {
      // Background polling stays silent; only manual reloads surface errors.
      if (reportError) setOutputError(e instanceof Error ? e.message : String(e))
    } finally {
      loadingRef.current = false
    }
  }, [agent.pane_id])

  useEffect(() => {
    void loadOutput()
    // The focused agent keeps its terminal output fresh by polling every second.
    if (!autoReload) return
    const id = setInterval(() => {
      if (!document.hidden) void loadOutput(false)
    }, 1000)
    return () => clearInterval(id)
  }, [autoReload, loadOutput])

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

  const sendKey = useCallback(async (label: string, key: string) => {
    if (keySending) return
    setKeySending(key)
    setNotice(null)
    try {
      await api.sendKeysHerdrAgent(agent.pane_id, [key])
      setNotice(`${label} sent`)
      setTimeout(() => setNotice(null), 3000)
      void loadOutput()
    } catch (e) {
      setNotice(e instanceof Error ? e.message : String(e))
    } finally {
      setKeySending(null)
    }
  }, [keySending, agent.pane_id, loadOutput])

  const sendCommand = useCallback(async (command: string) => {
    if (commandSending) return
    setCommandSending(command)
    setNotice(null)
    try {
      await api.promptHerdrAgent(agent.pane_id, command)
      setNotice(`${command} submitted`)
      setTimeout(() => setNotice(null), 3000)
      void loadOutput()
    } catch (e) {
      setNotice(e instanceof Error ? e.message : String(e))
    } finally {
      setCommandSending(null)
    }
  }, [commandSending, agent.pane_id, loadOutput])

  return (
    <div className="herdr-agent-detail mt-2 rounded-md border bg-muted/40 p-3" data-testid={`agent-detail-${agent.pane_id}`}>
      <div className="mb-2 flex items-center justify-between gap-2">
        <span className="text-xs font-semibold tracking-wider text-muted-foreground uppercase">Terminal output</span>
        <div className="flex items-center gap-1">
          {onRename && (
            <Button variant="ghost" size="xs" className="cursor-pointer" onClick={onRename}>
              Rename agent
            </Button>
          )}
          <Button variant="ghost" size="xs" className="cursor-pointer" onClick={() => void loadOutput()}>
            Reload output
          </Button>
        </div>
      </div>
      {outputError && <p className="mb-2 text-xs text-red-600">{outputError}</p>}
      <div
        ref={preRef}
        onScroll={handlePreScroll}
        className="max-h-64 overflow-auto rounded border bg-background p-2 text-xs"
      >
        <SyntaxHighlighter text={output ?? 'loading...'} cols={cols} />
      </div>
      <div
        className="herdr-agent-keys mt-2 flex flex-wrap items-center gap-1"
        data-testid={`agent-keys-${agent.pane_id}`}
      >
        {AGENT_QUICK_KEYS.map((k) => (
          <Button
            key={k.key}
            variant="outline"
            size="xs"
            className="cursor-pointer px-1.5 font-mono text-[11px]"
            title={`Press ${k.label} key`}
            aria-label={`Press ${k.label} on ${agent.name}`}
            data-testid={`agent-key-${agent.pane_id}-${k.label}`}
            disabled={keySending !== null}
            onClick={() => void sendKey(k.label, k.key)}
          >
            [{keySending === k.key ? '…' : k.label}]
          </Button>
        ))}
        <span aria-hidden="true" className="h-4 w-px bg-border" />
        {AGENT_COMMANDS.map((c) => (
          <Button
            key={c.command}
            variant="outline"
            size="xs"
            className="cursor-pointer px-1.5 font-mono text-[10px] text-muted-foreground"
            title={`Run ${c.label}`}
            data-testid={`agent-command-${agent.pane_id}-${c.command.slice(1)}`}
            disabled={commandSending !== null}
            onClick={() => void sendCommand(c.command)}
          >
            {commandSending === c.command ? '…' : c.label}
          </Button>
        ))}
      </div>
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
  // fit renders the row to fill an absolutely-positioned layout box (the
  // terminal output stretches instead of capping at max-h-64).
  fit: boolean
  // Terminal column width of the pane, used to render the output at the same
  // wrap columns (and horizontal scroll width) as the real herdr pane.
  cols?: number
}

function PaneRow({ pane, focused, onFocus, autoReload, onChanged, onError, fit, cols }: PaneRowProps) {
  const isMobile = useIsMobile()
  const { prompt, confirm } = useDialogs()
  const [open, setOpen] = useState(true)
  const [output, setOutput] = useState<string | null>(null)
  const [mode, setMode] = useState<'send-text-enter' | 'send-text' | 'send-keys' | 'prompt'>('send-text-enter')
  const [draft, setDraft] = useState('')
  const [sending, setSending] = useState(false)
  const [keySending, setKeySending] = useState<string | null>(null)
  const [notice, setNotice] = useState<string | null>(null)
  const loadingRef = useRef(false)
  const preRef = useRef<HTMLDivElement>(null)
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

  const renamePane = async () => {
    const label = await prompt(`Rename pane ${pane.pane_id}`, pane.title ?? '')
    if (label === null) return
    if (!label.trim()) return
    void runOp(() => api.renameHerdrPane(pane.pane_id, label.trim()), onError, onChanged)
  }

  const closePane = async () => {
    if (!(await confirm(`Close pane ${pane.pane_id}?`))) return
    void runOp(() => api.closeHerdrPane(pane.pane_id), onError, onChanged)
  }

  const sendKey = useCallback(async (label: string, key: string) => {
    if (keySending) return
    setKeySending(key)
    setNotice(null)
    try {
      await api.sendKeysHerdrPane(pane.pane_id, [key])
      setNotice(`${label} sent`)
      setTimeout(() => setNotice(null), 3000)
      if (open) void loadOutput()
    } catch (e) {
      setNotice(e instanceof Error ? e.message : String(e))
    } finally {
      setKeySending(null)
    }
  }, [keySending, pane.pane_id, open, loadOutput])

  return (
    <div
      className={cn(
        'herdr-pane-row flex flex-col rounded border bg-card text-card-foreground shadow-xs transition-all',
        focused ? 'border-primary ring-1 ring-primary/40' : 'border-border',
        fit && 'h-full min-h-0 overflow-hidden',
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
          {!isMobile && (
            <>
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
            </>
          )}
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
        <div className={cn('herdr-pane-detail p-2', fit && 'flex min-h-0 flex-1 flex-col')}>
          <div className="mb-1 flex shrink-0 items-center justify-between">
            <span className="text-[10px] uppercase font-mono tracking-wider text-muted-foreground">Terminal Output</span>
            <Button variant="ghost" size="xs" className="h-5 px-1 text-[10px]" onClick={() => void loadOutput()}>
              <RefreshCw className="mr-1 size-3" /> Reload
            </Button>
          </div>
          <div
            ref={preRef}
            onScroll={handlePreScroll}
            className={cn(
              'overflow-auto rounded bg-black p-2.5 font-mono text-xs text-green-400',
              fit ? 'min-h-0 flex-1' : 'max-h-64',
            )}
          >
            <SyntaxHighlighter text={output ?? 'loading terminal output...'} cols={cols} />
          </div>
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
          <div className="herdr-pane-keys flex flex-wrap items-center gap-1">
            {AGENT_QUICK_KEYS.map((k) => (
              <Button
                key={k.key}
                variant="outline"
                size="xs"
                className="cursor-pointer px-1.5 font-mono text-[10px]"
                title={`Press ${k.label}`}
                aria-label={`Press ${k.label} on ${pane.pane_id}`}
                disabled={keySending !== null}
                onClick={() => void sendKey(k.label, k.key)}
              >
                [{keySending === k.key ? '…' : k.label}]
              </Button>
            ))}
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
                ? 'Send text + Enter...'
                : mode === 'send-text'
                  ? 'Send literal text...'
                  : mode === 'send-keys'
                    ? 'Send keys (e.g. Enter, C-c)...'
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
  const { prompt, confirm } = useDialogs()

  const renameTab = async (e: React.MouseEvent) => {
    e.stopPropagation()
    const label = await prompt(`Rename tab "${tab.label}"`, tab.label)
    if (label === null) return
    if (!label.trim()) return
    void runOp(() => api.renameHerdrTab(tab.tab_id, label.trim()), onError, onChanged)
  }

  const closeTab = async (e: React.MouseEvent) => {
    e.stopPropagation()
    const extra =
      panes.length === 0 || (tab.pane_count ?? panes.length) <= 1
        ? ' This is the last tab of the workspace; closing it also closes the workspace.'
        : ''
    if (!(await confirm(`Close tab "${tab.label}"?${extra}`))) return
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

// Live drag of a split divider: the dragged split renders at overrideRatio and
// the real herdr resize (pane, direction, ratio delta) is sent on pointer up.
interface DragState {
  splitId: string
  axis: 'vertical' | 'horizontal'
  baseRatio: number
  baseRegion: Rect
  startClient: number
  cellPx: number
  overrideRatio: number
}

interface WorkspaceSectionProps {
  ws: HerdrWorkspace
  tabs: HerdrTab[]
  panes: HerdrPane[]
  layouts: HerdrLayout[]
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
  layouts,
  current,
  urlTabId,
  urlPaneId,
  onSelectTab,
  onSelectPane,
  autoReload,
  onChanged,
  onError,
}: WorkspaceSectionProps) {
  const isMobile = useIsMobile()
  const { prompt } = useDialogs()
  // Tab and pane focus are managed by the web UI (persisted in the URL),
  // not by the herdr CLI; herdr's own focused flags are ignored here.
  const sortedTabs = [...tabs].sort((a, b) => (a.number ?? 0) - (b.number ?? 0))
  const selectedTabId =
    urlTabId && sortedTabs.some((t) => t.tab_id === urlTabId)
      ? urlTabId
      : (sortedTabs[0]?.tab_id ?? null)

  const createTab = async () => {
    const label = await prompt(`New tab in workspace "${ws.label}" (optional name)`, '')
    if (label === null) return
    void runOp(
      () => api.createHerdrTab(ws.workspace_id, label.trim() || undefined),
      onError,
      onChanged,
    )
  }

  const activeTab = sortedTabs.find((t) => t.tab_id === selectedTabId) ?? sortedTabs[0]
  const activePanes = activeTab ? panes.filter((p) => p.tab_id === activeTab.tab_id) : []

  // Reproduce herdr's split structure from the tab's layout (real rects and
  // proportions). When the layout is missing or incomplete the classic grid is
  // used instead. A zoomed tab only exposes its focused pane.
  const activeLayout = layouts.find((l) => l.tab_id === activeTab?.tab_id)
  const [drag, setDrag] = useState<DragState | null>(null)
  const layoutRef = useRef<HTMLDivElement>(null)
  // Rendering the dragged split at an alternative ratio previews the resize
  // live; the real resize is sent to herdr once the pointer is released.
  const overrides = useMemo<Record<string, number>>(() => {
    if (!drag) return {}
    return { [drag.splitId]: drag.overrideRatio }
  }, [drag])
  const placed = layoutBoxForTab(activeLayout, overrides)
  const useLayout =
    !isMobile &&
    placed != null &&
    (placed.zoomed || activePanes.every((p) => placed.panes.some((b) => b.paneId === p.pane_id)))

  // Terminal column width per pane so output wraps at (and scrolls to) the
  // same width as the real herdr pane.
  const paneCols = useMemo(() => {
    const map = new Map<string, number>()
    for (const p of activeLayout?.panes ?? []) {
      if (p.rect.width > 0) map.set(p.pane_id, p.rect.width)
    }
    return map
  }, [activeLayout])

  const startDrag = (e: React.PointerEvent, divider: Divider) => {
    if (e.button !== 0 || !placed) return
    e.preventDefault()
    e.currentTarget.setPointerCapture(e.pointerId)
    const el = layoutRef.current
    if (!el) return
    const box = el.getBoundingClientRect()
    const area = placed.area
    const cellPx =
      (divider.axis === 'vertical' ? box.width : box.height) /
      (divider.axis === 'vertical' ? area.width : area.height)
    setDrag({
      splitId: divider.splitId,
      axis: divider.axis,
      baseRatio: divider.ratio,
      baseRegion: { x: divider.x, y: divider.y, width: divider.width, height: divider.height },
      startClient: divider.axis === 'vertical' ? e.clientX : e.clientY,
      cellPx,
      overrideRatio: divider.ratio,
    })
  }

  const moveDrag = (e: React.PointerEvent) => {
    if (!drag) return
    const delta = (drag.axis === 'vertical' ? e.clientX : e.clientY) - drag.startClient
    const deltaCells = delta / drag.cellPx
    const regionDim = drag.axis === 'vertical' ? drag.baseRegion.width : drag.baseRegion.height
    const ratio = Math.min(
      MAX_RATIO,
      Math.max(MIN_RATIO, drag.baseRatio + deltaCells / regionDim),
    )
    setDrag((prev) => (prev && Math.abs(prev.overrideRatio - ratio) > 1e-4 ? { ...prev, overrideRatio: ratio } : prev))
  }

  const endDrag = (e: React.PointerEvent) => {
    if (!drag) return
    if (typeof e.currentTarget.releasePointerCapture === 'function') {
      e.currentTarget.releasePointerCapture(e.pointerId)
    }
    const deltaRatio = drag.overrideRatio - drag.baseRatio
    const edges = activeLayout ? edgeNeighborsForSplit(activeLayout, drag.splitId) : undefined
    setDrag(null)
    const recipe = deltaRatio > 0 ? edges?.positive : edges?.negative
    if (!recipe || Math.abs(deltaRatio) < 1e-4) return
    void runOp(
      () => api.resizeHerdrPane(recipe.paneId, recipe.direction, Math.abs(deltaRatio)),
      onError,
      onChanged,
    )
  }

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
            ) : useLayout && placed ? (
              <div
                ref={layoutRef}
                className={cn('herdr-layout relative font-mono text-[13px]', drag && 'select-none')}
                style={{
                  // Match the layout's horizontal width to the real herdr
                  // terminal (area.width columns × the monospace cell width)
                  // without ever growing past the visible container: when the
                  // real layout is wider, overflow is handled by horizontal
                  // scrolling inside each panel's terminal display instead.
                  width: `min(100%, ${placed.area.width}ch)`,
                  aspectRatio: String(placed.aspectRatio),
                }}
                data-layout={placed.zoomed ? 'zoomed' : 'split'}
                data-testid={`herdr-layout-${ws.workspace_id}`}
              >
                {activePanes.map((p) => {
                  const box = placed.panes.find((b) => b.paneId === p.pane_id)
                  if (!box) return null
                  return (
                    <div
                      key={p.pane_id}
                      className="absolute min-w-0"
                      style={{ left: box.left, top: box.top, width: box.width, height: box.height }}
                    >
                      <PaneRow
                        pane={p}
                        focused={urlPaneId === p.pane_id}
                        onFocus={() => onSelectPane(p.pane_id)}
                        autoReload={autoReload}
                        fit
                        cols={paneCols.get(p.pane_id)}
                        onChanged={onChanged}
                        onError={onError}
                      />
                    </div>
                  )
                })}
                {!placed.zoomed &&
                  placed.dividers.map((d) => {
                    const isDragging = drag?.splitId === d.splitId
                    return (
                      <div
                        key={d.splitId}
                        className={cn(
                          'herdr-divider group absolute z-20 flex touch-none items-center justify-center',
                          d.axis === 'vertical' ? 'cursor-col-resize' : 'cursor-row-resize',
                          isDragging && 'z-30',
                        )}
                        style={
                          d.axis === 'vertical'
                            ? { left: d.pos, top: '0%', height: '100%', width: 10, transform: 'translateX(-50%)' }
                            : { top: d.pos, left: '0%', width: '100%', height: 10, transform: 'translateY(-50%)' }
                        }
                        data-testid={`herdr-divider-${d.splitId}`}
                        onPointerDown={(e) => startDrag(e, d)}
                        onPointerMove={moveDrag}
                        onPointerUp={endDrag}
                        onPointerCancel={endDrag}
                      >
                        <div
                          className={cn(
                            'rounded bg-muted-foreground/0 transition-colors group-hover:bg-primary/60 group-active:bg-primary',
                            d.axis === 'vertical' ? 'h-full w-0.5' : 'h-0.5 w-full',
                            isDragging && 'bg-primary',
                          )}
                        />
                      </div>
                    )
                  })}
              </div>
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
                    fit={false}
                    cols={paneCols.get(p.pane_id)}
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
  const { prompt } = useDialogs()
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

  // Tab layouts drive the tmux-style pane reproduction; they are fetched
  // separately from the overview so the (cheaper) overview stays the single
  // polling source in the sidebar and dashboard.
  const [layouts, setLayouts] = useState<HerdrLayout[]>([])

  const refreshLayouts = useCallback(async () => {
    try {
      const res = await api.getHerdrLayouts()
      setLayouts(res.layouts ?? [])
    } catch {
      setLayouts([])
    }
  }, [])

  useEffect(() => {
    void refreshLayouts()
  }, [refreshLayouts, project])

  const refreshAll = useCallback(async () => {
    await refresh()
    await refreshLayouts()
  }, [refresh, refreshLayouts])

  // Load the project's files once so each agent can be linked back to the
  // task directory it was started for (reverse of taskAgentName).
  const [files, setFiles] = useState<Array<{ path: string }> | null>(null)
  useEffect(() => {
    let cancelled = false
    api
      .listFiles()
      .then((entries) => {
        if (!cancelled) setFiles(entries.filter((e) => e.kind === 'file'))
      })
      .catch(() => {
        if (!cancelled) setFiles([])
      })
    return () => {
      cancelled = true
    }
  }, [])

  const agentFilePath = useMemo(() => {
    const map = new Map<string, string>()
    if (files) {
      for (const a of agents) {
        const found = filePathForAgent(files, a.name)
        if (found) map.set(a.pane_id, found)
      }
    }
    return map
  }, [files, agents])

  // Map each agent's pane to its terminal column width so terminal output can
  // wrap at the same columns as the real herdr pane.
  const agentCols = useMemo(() => {
    const map = new Map<string, number>()
    for (const l of layouts) {
      for (const p of l.panes) {
        if (p.rect.width > 0) map.set(p.pane_id, p.rect.width)
      }
    }
    return map
  }, [layouts])

  const navigate = useNavigate()

  const onError = useCallback((message: string) => {
    setOpError(message)
    setTimeout(() => setOpError((cur) => (cur === message ? null : cur)), 6000)
  }, [])

  // With no matching workspace the server bootstraps this project's first
  // workspace, whose root tab becomes the requested new tab.
  const createFirstTab = async () => {
    const label = await prompt(`New tab for project "${project}" (optional name)`, '')
    if (label === null) return
    void runOp(
      () => api.createHerdrTab(undefined, label.trim() || undefined, undefined, project),
      onError,
      refreshAll,
    )
  }

  const renameAgent = async (agent: HerdrAgent, e?: React.MouseEvent) => {
    e?.stopPropagation()
    const nextName = await prompt(
      `Rename agent "${agent.name}" (leave empty to reset to default)`,
      agent.name,
    )
    if (nextName === null) return
    const trimmed = nextName.trim()
    void runOp(
      () => api.renameHerdrAgent(agent.pane_id, trimmed || undefined, !trimmed),
      onError,
      refreshAll,
    )
  }

  // Auto-open the agent requested via ?agent=<pane_id> (sidebar deep link).
  useEffect(() => {
    if (!requestedAgent) return
    if ((overview?.agents ?? []).some((a) => a.pane_id === requestedAgent)) {
      setOpenPane(requestedAgent)
    }
  }, [requestedAgent, overview])

  return (
    <div className="page p-4 md:p-6">
      <div className="page-header mb-3 flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-2xl font-bold">Herdr</h1>
          <p className="page-subtitle mt-0.5 text-sm text-muted-foreground">
            {matched
              ? `Workspace “${project}” is ${matched.agent_status}. Operate herdr agents here.`
              : `Project “${project}” has no matching herdr workspace.`}
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
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
          <Button variant="ghost" size="sm" className="cursor-pointer" onClick={() => void refreshAll()} title="Refresh">
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
        <Collapsible defaultOpen={false}>
          <CollapsibleTrigger asChild>
            <button
              type="button"
              className="group flex w-full cursor-pointer items-center gap-1.5 text-left"
              data-testid="herdr-workspaces-toggle"
            >
              <ChevronDown className="size-4 shrink-0 text-muted-foreground transition-transform duration-200 group-data-[state=open]:rotate-180" />
              <h2 className="text-sm font-semibold tracking-wider text-muted-foreground uppercase">
                Tabs · Panes
              </h2>
            </button>
          </CollapsibleTrigger>
          <CollapsibleContent>
            <div className="mt-2">
              {workspaces.filter((w) => w.label === project).length === 0 ? (
                <div className="rounded-md border border-dashed p-4 text-center" data-testid="herdr-no-workspace">
                  <p className="text-xs text-muted-foreground">
                    No herdr workspace found for project "{project}".
                  </p>
                  {overview?.available && (
                    <Button
                      variant="outline"
                      size="sm"
                      className="mt-2 cursor-pointer"
                      onClick={createFirstTab}
                      data-testid="herdr-create-first-tab"
                    >
                      + New Tab
                    </Button>
                  )}
                </div>
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
                        layouts={layouts}
                        current={true}
                        urlTabId={urlTabId}
                        urlPaneId={urlPaneId}
                        onSelectTab={selectTab}
                        onSelectPane={selectPane}
                        autoReload={autoReload}
                        onChanged={refreshAll}
                        onError={onError}
                      />
                    ))}
                </div>
              )}
            </div>
          </CollapsibleContent>
        </Collapsible>
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
                    <div className="flex w-full items-center justify-between gap-2">
                      <button
                        type="button"
                        className="herdr-agent-row flex min-w-0 flex-1 cursor-pointer flex-wrap items-center gap-2 text-left"
                        onClick={() => setOpenPane(openPane === a.pane_id ? null : a.pane_id)}
                        aria-expanded={openPane === a.pane_id}
                        data-testid={`agent-row-${a.pane_id}`}
                      >
                        <StatusDot status={a.status} />
                        <span className="font-semibold">{a.name}</span>
                        <StatusBadge status={a.status} />
                        {ws && <span className="text-xs text-muted-foreground">in {ws.label}</span>}
                        {a.focused && <span className="text-xs text-muted-foreground">· focused</span>}
                        <span className="truncate font-mono text-xs text-muted-foreground">
                          {a.pane_id}
                        </span>
                        {a.title && (
                          <span className="herdr-agent-title w-full truncate text-xs text-muted-foreground">
                            {a.title}
                          </span>
                        )}
                      </button>
                      {(() => {
                        const filePath = agentFilePath.get(a.pane_id)
                        return filePath ? (
                          <Button
                            variant="ghost"
                            size="xs"
                            className="h-6 shrink-0 cursor-pointer px-1.5 text-[11px]"
                            onClick={(e) => {
                              e.stopPropagation()
                              navigate(projectUrl(`/dashboard/files/${encodePath(filePath)}`))
                            }}
                            title={`Open ${filePath} in Files`}
                            aria-label={`Open ${dirName(filePath)} in Files`}
                            data-testid={`agent-file-${a.pane_id}`}
                          >
                            <FileText className="mr-1 size-3" />
                            {dirName(filePath)}
                          </Button>
                        ) : null
                      })()}
                      <Button
                        variant="ghost"
                        size="xs"
                        className="h-6 shrink-0 cursor-pointer px-1.5 text-[11px]"
                        onClick={(e) => renameAgent(a, e)}
                        title="Rename agent"
                        aria-label={`Rename agent ${a.name}`}
                        data-testid={`agent-rename-${a.pane_id}`}
                      >
                        Rename
                      </Button>
                    </div>
                    {openPane === a.pane_id && (
                      <AgentDetail
                        agent={a}
                        autoReload={autoReload}
                        onRename={() => renameAgent(a)}
                        cols={agentCols.get(a.pane_id)}
                      />
                    )}
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
