import { useCallback, useEffect, useLayoutEffect, useRef, useState } from 'react'
import { CalendarClock, ChevronDown, Keyboard, Send, X } from 'lucide-react'
import type { HerdrAgent } from '../api/client'
import { api } from '../api/client'
import { Button, buttonVariants } from './ui/button'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from './ui/dropdown-menu'
import { SyntaxHighlighter } from './SyntaxHighlighter'
import { useIsMobile } from '../hooks/use-mobile'
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from './ui/sheet'
import { AGENT_COMMANDS } from '../utils/herdr-agent-commands'
import type { AgentOutputDisplayMode } from '../utils/agent-output-display'
import {
  AGENT_SIDEBAR_PROMPT_HEIGHT_STORAGE_KEY,
  HERDR_AGENT_OUTPUT_HEIGHT_STORAGE_KEY,
  useResizableHeight,
} from '../hooks/use-resizable-height'
import {
  formatDateTimeLocal,
  formatScheduledAt,
  fromScheduledPromptResponse,
  migrateLegacyScheduledPrompts,
  type ScheduledPrompt,
} from '../utils/scheduled-prompts'

interface HerdrAgentDetailProps {
  agent: HerdrAgent
  autoReload: boolean
  reloadToken?: number
  onFilePathClick?: (path: string) => void
  onDisplayModeChange?: (mode: AgentOutputDisplayMode) => void
  cols?: number
  displayMode?: AgentOutputDisplayMode
}

const AGENT_QUICK_KEYS: { label: string; key: string }[] = [
  { label: 'Enter', key: 'enter' },
  { label: 'Esc', key: 'esc' },
  { label: 'Ctrl+C', key: 'C-c' },
  { label: 'Tab', key: 'Tab' },
  { label: '↑', key: 'Up' },
  { label: '↓', key: 'Down' },
]

const MOBILE_INLINE_QUICK_KEYS = new Set(['enter', 'Up', 'Down'])
const MOBILE_PROMPT_MIN_HEIGHT = 48

export function HerdrAgentDetail({
  agent,
  autoReload,
  reloadToken,
  onFilePathClick,
  onDisplayModeChange,
  cols,
  displayMode = 'auto',
}: HerdrAgentDetailProps) {
  const isMobile = useIsMobile()
  const [output, setOutput] = useState<string | null>(null)
  const [outputError, setOutputError] = useState<string | null>(null)
  const [draft, setDraft] = useState('')
  const [scheduleAt, setScheduleAt] = useState('')
  const [scheduledPrompts, setScheduledPrompts] = useState<ScheduledPrompt[]>([])
  const [sending, setSending] = useState(false)
  const [scheduling, setScheduling] = useState(false)
  const [keySending, setKeySending] = useState<string | null>(null)
  const [commandSending, setCommandSending] = useState<string | null>(null)
  const [notice, setNotice] = useState<string | null>(null)
  const [commandPaletteOpen, setCommandPaletteOpen] = useState(false)
  const [scheduleMode, setScheduleMode] = useState(false)
  const [mobilePromptHeight, setMobilePromptHeight] = useState(MOBILE_PROMPT_MIN_HEIGHT)
  const loadingRef = useRef(false)
  const scheduledRevisionRef = useRef(0)
  const preRef = useRef<HTMLDivElement | null>(null)
  const promptInputRef = useRef<HTMLTextAreaElement | null>(null)
  const pinnedRef = useRef(true)
  const outputSize = useResizableHeight<HTMLDivElement>({
    storageKey: HERDR_AGENT_OUTPUT_HEIGHT_STORAGE_KEY,
    defaultHeight: 256,
    minHeight: 128,
  })
  const promptSize = useResizableHeight<HTMLTextAreaElement>({
    storageKey: AGENT_SIDEBAR_PROMPT_HEIGHT_STORAGE_KEY,
    defaultHeight: 24,
    minHeight: 24,
  })
  const setPromptInputRef = useCallback((element: HTMLTextAreaElement | null) => {
    promptInputRef.current = element
    promptSize.ref(isMobile ? null : element)
  }, [isMobile, promptSize.ref])

  const resizeMobilePrompt = useCallback(() => {
    if (!isMobile) return
    const element = promptInputRef.current
    if (!element) return

    const previousHeight = element.style.height
    element.style.height = 'auto'
    const maxHeight = Math.max(MOBILE_PROMPT_MIN_HEIGHT, Math.floor(window.innerHeight * 0.4))
    const nextHeight = Math.min(Math.max(element.scrollHeight, MOBILE_PROMPT_MIN_HEIGHT), maxHeight)
    element.style.height = previousHeight
    setMobilePromptHeight((current) => current === nextHeight ? current : nextHeight)
  }, [isMobile])

  useLayoutEffect(() => {
    resizeMobilePrompt()
  }, [draft, isMobile, resizeMobilePrompt])

  useEffect(() => {
    if (!isMobile) return
    window.addEventListener('resize', resizeMobilePrompt)
    return () => window.removeEventListener('resize', resizeMobilePrompt)
  }, [isMobile, resizeMobilePrompt])
  const visibleScheduledPrompts = scheduledPrompts.filter(
    (scheduled) => scheduled.target === agent.name || scheduled.target === agent.pane_id,
  )
  const paletteQuickKeys = AGENT_QUICK_KEYS.filter((key) => !MOBILE_INLINE_QUICK_KEYS.has(key.key))
  const paletteCommands = AGENT_COMMANDS

  const handlePreScroll = useCallback(() => {
    const element = preRef.current
    if (!element) return
    pinnedRef.current = element.scrollHeight - element.scrollTop - element.clientHeight < 32
  }, [])

  useEffect(() => {
    const element = preRef.current
    if (!element || !pinnedRef.current) return
    element.scrollTop = element.scrollHeight
  }, [output])

  const loadOutput = useCallback(async (reportError = true) => {
    if (loadingRef.current) return
    loadingRef.current = true
    try {
      const response = await api.readHerdrAgent(agent.pane_id)
      setOutput(response.output)
      setOutputError(null)
    } catch (error) {
      if (reportError) setOutputError(error instanceof Error ? error.message : String(error))
    } finally {
      loadingRef.current = false
    }
  }, [agent.pane_id])

  useEffect(() => {
    void loadOutput()
    if (!autoReload) return
    const id = window.setInterval(() => {
      if (!document.hidden) void loadOutput(false)
    }, 1000)
    return () => window.clearInterval(id)
  }, [autoReload, loadOutput, reloadToken])

  const sendPrompt = useCallback(async () => {
    const text = draft.trim()
    if (!text || sending) return
    setSending(true)
    setNotice(null)
    try {
      await api.promptHerdrAgent(agent.pane_id, text)
      setDraft('')
      promptInputRef.current?.blur()
      setScheduleAt('')
      setScheduleMode(false)
      setNotice('prompt submitted')
      window.setTimeout(() => setNotice(null), 3000)
      void loadOutput()
    } catch (error) {
      setNotice(error instanceof Error ? error.message : String(error))
    } finally {
      setSending(false)
    }
  }, [agent.pane_id, draft, loadOutput, sending])

  const loadScheduledPrompts = useCallback(async () => {
    const revision = scheduledRevisionRef.current
    try {
      const prompts = await api.listHerdrScheduledPrompts()
      const current = prompts
        .map(fromScheduledPromptResponse)
        .filter((prompt): prompt is ScheduledPrompt => prompt !== null)
      const migrated = await migrateLegacyScheduledPrompts((target, text, scheduledAt) =>
        api.createHerdrScheduledPrompt(target, text, scheduledAt),
      )
      if (revision === scheduledRevisionRef.current) setScheduledPrompts([...current, ...migrated])
    } catch (error) {
      setNotice(error instanceof Error ? error.message : String(error))
    }
  }, [])

  useEffect(() => {
    void loadScheduledPrompts()
    const id = window.setInterval(() => void loadScheduledPrompts(), 5000)
    return () => window.clearInterval(id)
  }, [loadScheduledPrompts])

  const schedulePrompt = useCallback(async () => {
    const text = draft.trim()
    const timestamp = Date.parse(scheduleAt)
    if (!text) return
    if (!Number.isFinite(timestamp) || timestamp <= Date.now()) {
      setNotice('Schedule time must be in the future')
      return
    }
    scheduledRevisionRef.current += 1
    setScheduling(true)
    setNotice(null)
    try {
      const created = await api.createHerdrScheduledPrompt(agent.pane_id, text, new Date(timestamp).toISOString())
      const scheduled = fromScheduledPromptResponse(created)
      if (scheduled) setScheduledPrompts((current) => [...current, scheduled])
      setDraft('')
      promptInputRef.current?.blur()
      setScheduleAt('')
      setScheduleMode(false)
      setNotice('prompt scheduled')
    } catch (error) {
      setNotice(error instanceof Error ? error.message : String(error))
    } finally {
      setScheduling(false)
    }
  }, [agent.pane_id, draft, scheduleAt])

  const cancelScheduledPrompt = useCallback(async (id: string) => {
    scheduledRevisionRef.current += 1
    try {
      await api.deleteHerdrScheduledPrompt(id)
      setScheduledPrompts((current) => current.filter((scheduled) => scheduled.id !== id))
    } catch (error) {
      setNotice(error instanceof Error ? error.message : String(error))
    }
  }, [])

  const sendKey = useCallback(async (label: string, key: string) => {
    if (keySending) return
    setKeySending(key)
    setNotice(null)
    try {
      await api.sendKeysHerdrAgent(agent.pane_id, [key])
      setNotice(`${label} sent`)
      window.setTimeout(() => setNotice(null), 3000)
      void loadOutput()
    } catch (error) {
      setNotice(error instanceof Error ? error.message : String(error))
    } finally {
      setKeySending(null)
    }
  }, [agent.pane_id, keySending, loadOutput])

  const sendCommand = useCallback(async (command: string) => {
    if (commandSending) return
    setCommandSending(command)
    setNotice(null)
    try {
      await api.promptHerdrAgent(agent.pane_id, command)
      setNotice(`${command} submitted`)
      window.setTimeout(() => setNotice(null), 3000)
      void loadOutput()
    } catch (error) {
      setNotice(error instanceof Error ? error.message : String(error))
    } finally {
      setCommandSending(null)
    }
  }, [agent.pane_id, commandSending, loadOutput])

  const renderQuickKeyButton = (key: { label: string; key: string }, inPalette = false) => (
    <Button
      key={key.key}
      variant="outline"
      size="xs"
      className={inPalette ? 'min-h-8 cursor-pointer justify-center px-2 font-mono text-xs leading-none' : 'cursor-pointer px-1.5 font-mono text-[11px]'}
      title={`Press ${key.label} key`}
      aria-label={`Press ${key.label} on ${agent.name}`}
      data-testid={`agent-key-${agent.pane_id}-${key.label}`}
      disabled={keySending !== null}
      onClick={() => {
        if (inPalette) setCommandPaletteOpen(false)
        void sendKey(key.label, key.key)
      }}
    >
      [{keySending === key.key ? '…' : key.label}]
    </Button>
  )

  const renderCommandButton = (command: typeof AGENT_COMMANDS[number], inPalette = false) => (
    <Button
      key={command.id}
      variant="outline"
      size="xs"
      className={inPalette ? 'min-h-8 min-w-0 cursor-pointer justify-start px-2 text-xs truncate' : 'cursor-pointer px-1.5 font-mono text-[10px] text-muted-foreground'}
      title={`Run ${command.label}`}
      data-testid={`agent-command-${agent.pane_id}-${command.id}`}
      disabled={commandSending !== null}
      onClick={() => {
        if (inPalette) setCommandPaletteOpen(false)
        void sendCommand(command.command)
      }}
    >
      {commandSending === command.command ? '…' : command.label}
    </Button>
  )

  return (
    <div className="herdr-agent-detail -mx-2 mt-2 min-w-0 rounded-md border bg-muted/40 px-1 py-2" data-testid={`agent-detail-${agent.pane_id}`}>
      <div className="mb-2 flex min-w-0 flex-wrap items-center justify-between gap-2">
        <span className="min-w-0 flex-1 text-xs font-semibold tracking-wider text-muted-foreground uppercase">Terminal output</span>
        <div className="flex min-w-0 flex-wrap items-center justify-end gap-1">
          <label className="flex min-w-0 items-center gap-1 text-[10px] text-muted-foreground">
            <span className="sr-only">Agent output display mode</span>
            <select
              aria-label="Agent output display mode"
              value={displayMode}
              onChange={(event) => onDisplayModeChange?.(event.target.value as AgentOutputDisplayMode)}
              className="h-6 max-w-32 min-w-0 cursor-pointer rounded border bg-background px-1 text-base text-foreground outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 md:text-[10px]"
            >
              <option value="auto">Auto-correct</option>
              <option value="herdr">Herdr format</option>
            </select>
          </label>
        </div>
      </div>
      {outputError && <p className="mb-2 text-xs text-red-600">{outputError}</p>}
      <div
        ref={(element) => {
          preRef.current = element
          outputSize.ref(element)
        }}
        onScroll={handlePreScroll}
        data-testid={`herdr-agent-output-${agent.pane_id}`}
        className="min-h-32 max-h-[70vh] min-w-0 max-w-full resize-y overflow-x-hidden overflow-y-auto rounded border bg-background px-1 py-2 text-xs"
        style={{ height: outputSize.height }}
      >
        <SyntaxHighlighter
          text={output ?? 'loading...'}
          cols={cols}
          displayMode={displayMode}
          linkFilePaths
          onFilePathClick={onFilePathClick}
        />
      </div>
      <div className="herdr-agent-keys mt-2 flex flex-wrap items-center gap-1" data-testid={`agent-keys-${agent.pane_id}`}>
        {(isMobile ? AGENT_QUICK_KEYS.filter((key) => MOBILE_INLINE_QUICK_KEYS.has(key.key)) : AGENT_QUICK_KEYS).map((key) => renderQuickKeyButton(key))}
        {!isMobile && <span aria-hidden="true" className="h-4 w-px bg-border" />}
        {!isMobile && AGENT_COMMANDS.map((command) => renderCommandButton(command))}
        {isMobile && (
          <Button
            variant="outline"
            size="icon-sm"
            className="cursor-pointer"
            onClick={() => {
              setCommandPaletteOpen(true)
            }}
            aria-label="Keys & commands"
            title="Keys & commands"
            data-testid="agent-command-palette-toggle"
          >
            <Keyboard />
          </Button>
        )}
      </div>
      {isMobile && (
        <Sheet open={commandPaletteOpen} onOpenChange={setCommandPaletteOpen}>
          <SheetContent
            side="bottom"
            className="max-h-[80vh] gap-0 overflow-y-auto p-0"
            data-testid="agent-command-palette"
          >
            <SheetHeader className="border-b px-4 py-3">
              <SheetTitle>Keys & commands</SheetTitle>
              <SheetDescription>Send a terminal key or run an Agent command.</SheetDescription>
            </SheetHeader>
            <div className="flex min-w-0 flex-col gap-4 p-4">
              <div className="flex flex-col gap-2">
                <p className="text-xs font-semibold tracking-wider text-muted-foreground uppercase">Keys</p>
                <div className="grid grid-cols-3 gap-1" data-testid="agent-command-palette-keys">
                  {paletteQuickKeys.map((key) => renderQuickKeyButton(key, true))}
                </div>
              </div>
              <div className="flex flex-col gap-2">
                <p className="text-xs font-semibold tracking-wider text-muted-foreground uppercase">Commands</p>
                <div className="grid grid-cols-2 gap-1" data-testid="agent-command-palette-commands">
                  {paletteCommands.map((command) => renderCommandButton(command, true))}
                </div>
              </div>
            </div>
          </SheetContent>
        </Sheet>
      )}
      <div className="mt-3 flex min-w-0 flex-col gap-2">
        <textarea
          ref={setPromptInputRef}
          aria-label={`Prompt ${agent.name}`}
          data-testid="herdr-prompt-input"
          value={draft}
          onChange={(event) => setDraft(event.target.value)}
          onKeyDown={(event) => {
            if ((event.metaKey || event.ctrlKey) && event.key === 'Enter') void sendPrompt()
          }}
          placeholder="Send a prompt to this agent (Ctrl+Enter to submit)"
          rows={2}
          className={`min-h-6 max-h-[40vh] w-full min-w-0 rounded-md border bg-background px-2 py-0 text-base leading-5 outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 md:text-sm ${isMobile ? 'resize-none' : 'resize-y'}`}
          style={{ height: isMobile ? mobilePromptHeight : promptSize.height }}
        />
        <div data-testid="herdr-prompt-actions" className="flex w-full min-w-0 flex-col items-stretch gap-2">
          <div className="flex w-full min-w-0">
            <Button
              size="sm"
              className="min-w-0 flex-1 cursor-pointer rounded-r-none"
              disabled={sending || scheduling || !draft.trim()}
              onClick={() => void sendPrompt()}
            >
              <Send />
              Send
            </Button>
            <DropdownMenu>
              <DropdownMenuTrigger
                className={buttonVariants({
                  variant: 'default',
                  size: 'icon-sm',
                  className: 'shrink-0 cursor-pointer rounded-l-none border-l border-primary-foreground/25',
                })}
                disabled={sending || scheduling}
                aria-label="More send actions"
                title="More send actions"
              >
                <ChevronDown />
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                <DropdownMenuItem disabled={!draft.trim()} onSelect={() => setScheduleMode(true)}>
                  <CalendarClock />
                  Schedule send
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
          {scheduleMode && (
            <div className="grid min-w-0 grid-cols-[minmax(0,1fr)_auto] items-center gap-2">
              <input
                type="datetime-local"
                aria-label="Schedule send time"
                value={scheduleAt}
                min={formatDateTimeLocal(new Date())}
                onChange={(event) => setScheduleAt(event.target.value)}
                className="h-8 w-full min-w-0 rounded-md border bg-background px-1.5 text-base outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 md:text-xs"
              />
              <div className="flex min-w-0 items-center gap-1">
                <Button
                  variant="outline"
                  size="sm"
                  className="min-w-0 cursor-pointer px-2"
                  onClick={() => void schedulePrompt()}
                  disabled={sending || scheduling || !draft.trim() || scheduleAt === ''}
                  aria-label="Schedule send"
                >
                  <CalendarClock />
                  Schedule
                </Button>
                <Button
                  variant="ghost"
                  size="icon-sm"
                  className="shrink-0 cursor-pointer"
                  onClick={() => {
                    setScheduleMode(false)
                    setScheduleAt('')
                  }}
                  aria-label="Cancel scheduling"
                  title="Cancel scheduling"
                >
                  <X />
                </Button>
              </div>
            </div>
          )}
        </div>
      </div>
      {visibleScheduledPrompts.length > 0 && (
        <div className="mt-2 space-y-1">
          <p className="text-[11px] tracking-wider text-muted-foreground uppercase">Scheduled sends</p>
          {visibleScheduledPrompts.map((scheduled) => (
            <div key={scheduled.id} data-testid="scheduled-prompt" className="flex min-w-0 items-center gap-1 rounded border bg-background px-1.5 py-1 text-xs">
              <span className="min-w-0 flex-1 truncate" title={scheduled.text}>{scheduled.text}</span>
              <time className="shrink-0 text-muted-foreground" dateTime={new Date(scheduled.scheduledAt).toISOString()}>
                {formatScheduledAt(scheduled.scheduledAt)}
              </time>
              <Button
                variant="ghost"
                size="xs"
                className="size-5 shrink-0 cursor-pointer p-0"
                onClick={() => void cancelScheduledPrompt(scheduled.id)}
                aria-label={`Cancel scheduled prompt ${scheduled.text}`}
                title="Cancel scheduled prompt"
              >
                <X className="size-3" />
              </Button>
            </div>
          ))}
        </div>
      )}
      {notice && <p className="herdr-prompt-notice mt-1 text-xs text-muted-foreground">{notice}</p>}
    </div>
  )
}
