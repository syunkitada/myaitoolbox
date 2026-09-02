import { useCallback, useEffect, useRef, useState } from 'react'
import { HerdrAgent, HerdrOverview, api } from '../api/client'
import { StatusDot } from './herdr-status'
import { Button } from './ui/button'
import { fileAgentName } from '../utils/herdr-file-agent'
import { Bot, ChevronRight, Loader2, Square } from 'lucide-react'
import { cn } from '@/lib/utils'

const FILE_AGENT_KEYS: { label: string; key: string }[] = [
  { label: 'Enter', key: 'enter' },
  { label: 'Esc', key: 'esc' },
  { label: 'Ctrl+C', key: 'C-c' },
]

export interface FileAgentWidgetProps {
  /** Project-relative path of the open file the agent works on. */
  path: string
  overview: HerdrOverview | null
  onRefresh: () => void
}

export function FileAgentWidget({ path, overview, onRefresh }: FileAgentWidgetProps) {
  const filename = path.split('/').pop() ?? path
  const name = fileAgentName(filename)

  const [open, setOpen] = useState(true)
  const [starting, setStarting] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [output, setOutput] = useState<string | null>(null)
  const [outputError, setOutputError] = useState<string | null>(null)
  const [draft, setDraft] = useState('')
  const [sending, setSending] = useState(false)
  const [keySending, setKeySending] = useState<string | null>(null)
  const loadingRef = useRef(false)
  const preRef = useRef<HTMLPreElement>(null)

  const agent = (overview?.agents ?? []).find((a) => a.name === name)

  const loadOutput = useCallback(
    async (reportError = true) => {
      if (!agent || loadingRef.current) return
      loadingRef.current = true
      try {
        const res = await api.readHerdrAgent(agent.pane_id)
        setOutput(res.output)
        setOutputError(null)
      } catch (e) {
        if (reportError) setOutputError(e instanceof Error ? e.message : String(e))
      } finally {
        loadingRef.current = false
      }
    },
    [agent],
  )

  const stop = useCallback(() => {
    if (!agent) return
    setSending(true)
    setError(null)
    void api
      .sendKeysHerdrAgent(agent.pane_id, ['C-c', 'C-c'])
      .then(() => onRefresh())
      .catch((e) => setError(e instanceof Error ? e.message : String(e)))
      .finally(() => setSending(false))
  }, [agent, onRefresh])

  useEffect(() => {
    setOutput(null)
    setOutputError(null)
    if (!agent || !open || !overview?.available) return
    void loadOutput()
    const id = setInterval(() => {
      if (!document.hidden) void loadOutput(false)
    }, 1500)
    return () => clearInterval(id)
  }, [agent, open, overview?.available, loadOutput])

  useEffect(() => {
    const el = preRef.current
    if (el) el.scrollTop = el.scrollHeight
  }, [output])

  const start = useCallback(() => {
    if (starting) return
    setStarting(true)
    setError(null)
    void api
      .startHerdrFileAgent(path)
      .then(() => {
        setOpen(true)
        onRefresh()
      })
      .catch((e) => setError(e instanceof Error ? e.message : String(e)))
      .finally(() => setStarting(false))
  }, [path, starting, onRefresh])

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

  const header = (
    <button
      type="button"
      className="flex w-full cursor-pointer items-center gap-1.5 px-1 py-1.5 text-left"
      onClick={() => setOpen((o) => !o)}
      aria-expanded={open}
      aria-label={open ? `Collapse agent for ${filename}` : `Expand agent for ${filename}`}
    >
      <ChevronRight className={cn('size-3.5 shrink-0 text-muted-foreground transition-transform', open && 'rotate-90')} />
      <Bot className="size-3.5 shrink-0 text-muted-foreground" />
      <span className="min-w-0 flex-1 truncate text-xs font-semibold" title={filename}>
        {filename}
      </span>
      {agent ? (
        <StatusDot status={agent.status} />
      ) : (
        !starting && (
          <span className="text-[10px] tracking-wider text-muted-foreground uppercase">off</span>
        )
      )}
      {starting && <Loader2 className="size-3 shrink-0 animate-spin" />}
    </button>
  )

  // herdr is unavailable: keep the header so the widget stays discoverable.
  if (!overview?.available) {
    return (
      <div className="file-agent-widget mb-3 rounded-md border border-border bg-muted/40">
        {header}
      </div>
    )
  }

  if (!agent) {
    return (
      <div className="file-agent-widget mb-3 rounded-md border border-border bg-muted/40">
        {header}
        {open && (
          <div className="px-1.5 pb-2">
            <Button
              variant="outline"
              size="xs"
              className="w-full cursor-pointer"
              onClick={start}
              disabled={starting}
              aria-label={`Start agent for ${filename}`}
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
              herdr will open a tab named “{filename}” and run an opencode agent on it.
            </p>
          </div>
        )}
      </div>
    )
  }

  const agentName: HerdrAgent = agent

  return (
    <div className="file-agent-widget mb-3 rounded-md border border-border bg-muted/40">
      {header}
      {open && (
        <>
          {error && <p className="px-1.5 text-[11px] text-red-600">{error}</p>}
          {outputError && <p className="px-1.5 text-[11px] text-red-600">{outputError}</p>}
          <pre
            ref={preRef}
            className="mx-1.5 max-h-40 overflow-auto rounded border bg-background p-1.5 text-[11px] whitespace-pre-wrap"
          >
            {output ?? 'loading…'}
          </pre>
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
            <Button
              variant="ghost"
              size="xs"
              className="ml-auto cursor-pointer text-[10px] text-muted-foreground"
              onClick={stop}
              disabled={sending}
              aria-label={`Stop agent ${agentName.name}`}
              title={`Stop ${agentName.name}`}
            >
              <Square className="size-3" />
              Stop
            </Button>
          </div>
          <div className="flex items-start gap-1.5 p-1.5">
            <textarea
              aria-label={`Prompt ${agentName.name}`}
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
              className="min-h-0 flex-1 resize-none rounded-md border border-input bg-background px-2 py-1.5 text-xs outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50"
            />
            <Button
              size="xs"
              className="cursor-pointer"
              onClick={() => void sendPrompt()}
              disabled={sending || draft.trim() === ''}
              aria-label={`Send prompt to ${agentName.name}`}
            >
              Send
            </Button>
          </div>
        </>
      )}
    </div>
  )
}