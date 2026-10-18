import { useCallback, useEffect, useRef, useState } from 'react'
import { Check, Globe, Hand, ListFilter, RefreshCw, RotateCcw, ShieldAlert, UserRound, X } from 'lucide-react'
import { api, BrowserApproval, BrowserAuditEvent, BrowserAuditFilters, BrowserAuditResponse, BrowserObservation, BrowserSession } from '../api/client'
import { browserWsUrl } from '../utils/routes'
import { Button } from '../components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '../components/ui/card'
import { Input } from '../components/ui/input'

function pointerPosition(event: { currentTarget: HTMLImageElement; clientX: number; clientY: number }) {
  const rect = event.currentTarget.getBoundingClientRect()
  return {
    x: Math.max(0, Math.round(((event.clientX - rect.left) / rect.width) * 1280)),
    y: Math.max(0, Math.round(((event.clientY - rect.top) / rect.height) * 800)),
  }
}

const emptyAudit: BrowserAuditResponse = { events: [], total: 0, offset: 0, limit: 50, has_more: false }

function auditTime(value: string): string | undefined {
  if (!value) return undefined
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? undefined : date.toISOString()
}

function auditEventLabel(event: BrowserAuditEvent): string {
  if (event.command) return `${event.event} · ${event.command}`
  return event.event
}

function BrowserAuditPanel() {
  const [audit, setAudit] = useState<BrowserAuditResponse>(emptyAudit)
  const [sessionID, setSessionID] = useState('')
  const [result, setResult] = useState('')
  const [from, setFrom] = useState('')
  const [to, setTo] = useState('')
  const [query, setQuery] = useState<BrowserAuditFilters>({})
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const load = useCallback(async (nextQuery: BrowserAuditFilters, offset: number) => {
    setLoading(true)
    try {
      const next = await api.getBrowserAudit({ ...nextQuery, offset })
      setAudit(next)
      setError(null)
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause))
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { void load({}, 0) }, [load])

  const applyFilters = () => {
    const nextQuery = {
      sessionId: sessionID.trim() || undefined,
      result: result || undefined,
      from: auditTime(from),
      to: auditTime(to),
    }
    setQuery(nextQuery)
    void load(nextQuery, 0)
  }

  const clearFilters = () => {
    setSessionID('')
    setResult('')
    setFrom('')
    setTo('')
    setQuery({})
    void load({}, 0)
  }

  const page = (offset: number) => { void load(query, offset) }

  return (
    <Card data-testid="browser-audit-panel">
      <CardHeader className="flex flex-row items-center justify-between gap-2">
        <CardTitle className="flex items-center gap-2 text-sm"><ListFilter className="size-4" /> Audit log</CardTitle>
        <Button variant="ghost" size="sm" onClick={() => void load(query, audit.offset)} disabled={loading} aria-label="Refresh audit log"><RefreshCw className="size-4" /></Button>
      </CardHeader>
      <CardContent className="space-y-3">
        <div className="grid gap-2 sm:grid-cols-2 xl:grid-cols-4">
          <Input value={sessionID} onChange={(event) => setSessionID(event.target.value)} placeholder="Session ID" aria-label="Audit session ID" />
          <select value={result} onChange={(event) => setResult(event.target.value)} aria-label="Audit result" className="h-9 rounded-md border border-input bg-background px-3 text-sm">
            <option value="">All results</option>
            <option value="success">Success</option>
            <option value="blocked">Blocked</option>
            <option value="approval_required">Needs approval</option>
            <option value="pending">Pending</option>
            <option value="error">Error</option>
          </select>
          <Input type="datetime-local" value={from} onChange={(event) => setFrom(event.target.value)} aria-label="Audit from" />
          <Input type="datetime-local" value={to} onChange={(event) => setTo(event.target.value)} aria-label="Audit to" />
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Button size="sm" onClick={applyFilters}>Apply filters</Button>
          <Button size="sm" variant="outline" onClick={clearFilters}>Clear</Button>
          <span className="text-xs text-muted-foreground">Project: current project · {audit.total} events</span>
        </div>
        {error && <div className="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">{error}</div>}
        <div className="overflow-x-auto rounded-md border" data-testid="browser-audit-events">
          <table className="w-full text-left text-xs">
            <thead className="border-b bg-muted/50 text-muted-foreground">
              <tr><th className="px-3 py-2 font-medium">Time</th><th className="px-3 py-2 font-medium">Event</th><th className="px-3 py-2 font-medium">Actor</th><th className="px-3 py-2 font-medium">Result</th><th className="px-3 py-2 font-medium">Reason</th></tr>
            </thead>
            <tbody>
              {loading ? <tr><td colSpan={5} className="px-3 py-4 text-center text-muted-foreground">Loading…</td></tr> : audit.events.length === 0 ? <tr><td colSpan={5} className="px-3 py-4 text-center text-muted-foreground">No audit events</td></tr> : audit.events.map((event, index) => <tr key={`${event.timestamp}-${event.event}-${event.session_id ?? ''}-${event.command ?? ''}-${index}`} className="border-b last:border-0"><td className="whitespace-nowrap px-3 py-2 text-muted-foreground">{new Date(event.timestamp).toLocaleString()}</td><td className="px-3 py-2">{auditEventLabel(event)}</td><td className="px-3 py-2">{event.actor || '—'}</td><td className="px-3 py-2">{event.result}</td><td className="px-3 py-2">{event.reason || '—'}</td></tr>)}
            </tbody>
          </table>
        </div>
        <div className="flex items-center justify-between gap-2 text-xs text-muted-foreground">
          <span>{audit.total === 0 ? '0' : `${audit.offset + 1}–${audit.offset + audit.events.length}`} / {audit.total}</span>
          <div className="flex gap-2">
            <Button size="sm" variant="outline" onClick={() => page(Math.max(0, audit.offset - audit.limit))} disabled={loading || audit.offset === 0}>Newer</Button>
            <Button size="sm" variant="outline" onClick={() => page(audit.offset + audit.limit)} disabled={loading || !audit.has_more}>Older</Button>
          </div>
        </div>
      </CardContent>
    </Card>
  )
}

export function WebBrowserPage() {
  const [session, setSession] = useState<BrowserSession | null>(null)
  const [url, setUrl] = useState('https://example.com')
  const [frameURL, setFrameURL] = useState<string | null>(null)
  const [observation, setObservation] = useState<BrowserObservation | null>(null)
  const [approval, setApproval] = useState<BrowserApproval | null>(null)
  const [approvalModeLoading, setApprovalModeLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const imageRef = useRef<HTMLImageElement>(null)

  const refreshSession = useCallback(async () => {
    setLoading(true)
    try {
      const result = await api.getBrowserSession()
      setSession(result.session)
      setApproval(result.session?.pending_approval ?? null)
      if (result.session?.url && result.session.url !== 'about:blank') setUrl(result.session.url)
      setError(result.session?.error ?? null)
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause))
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { void refreshSession() }, [refreshSession])

  useEffect(() => {
    if (!session) return
    const streamError = 'ブラウザストリームに接続できません。再接続しています'
    let disposed = false
    let retryDelay = 500
    let retryTimer: number | undefined
    let ws: WebSocket | null = null

    const connect = () => {
      if (disposed) return
      ws = new WebSocket(browserWsUrl(session.id))
      ws.binaryType = 'blob'
      ws.onopen = () => {
        setError((current) => current === streamError ? null : current)
      }
      ws.onmessage = (event) => {
        retryDelay = 500
        if (typeof event.data !== 'string') {
          const next = URL.createObjectURL(event.data as Blob)
          setFrameURL((current) => {
            if (current) URL.revokeObjectURL(current)
            return next
          })
          return
        }
        try {
          const message = JSON.parse(event.data) as Partial<BrowserSession> & { type?: string; approval_id?: string; action?: string; target?: string; reason?: string; data?: BrowserObservation }
          if (message.type === 'approval_required' && message.approval_id) {
            setApproval({ approval_id: message.approval_id, action: message.action || 'action', target: message.target || 'page' })
          }
          if (message.type === 'error') {
            setSession((current) => current ? { ...current, state: 'error', error: message.error || 'browser worker disconnected' } : current)
            setApproval(null)
            setObservation(null)
            setError(message.error || 'browser worker disconnected')
            setFrameURL((current) => {
              if (current) URL.revokeObjectURL(current)
              return null
            })
          }
          if (message.type === 'expired') {
            setSession(null)
            setApproval(null)
            setObservation(null)
            setError(message.reason === 'idle_timeout' ? 'ブラウザセッションはアイドルタイムアウトで終了しました' : 'ブラウザセッションが終了しました')
            setFrameURL((current) => {
              if (current) URL.revokeObjectURL(current)
              return null
            })
          }
          if (message.type === 'approval_approved' || message.type === 'approval_rejected') setApproval(null)
          if (message.type === 'approval_failed') {
            setApproval(null)
            setError('The approved browser action failed.')
          }
          if (message.type === 'page' || message.type === 'session' || message.type === 'control' || message.type === 'approval_mode' || message.type === 'approval_approved' || message.type === 'approval_rejected' || message.type === 'approval_failed') {
            setSession((current) => current ? { ...current, state: message.state || current.state, owner: message.owner || current.owner, url: message.url || current.url, title: message.title || current.title, approval_mode: message.approval_mode || current.approval_mode } : current)
          }
        } catch {
          setError('ブラウザストリームのメッセージを解釈できません')
        }
      }
      ws.onerror = () => setError(streamError)
      ws.onclose = () => {
        if (disposed) return
        setError(streamError)
        retryTimer = window.setTimeout(connect, retryDelay)
        retryDelay = Math.min(retryDelay * 2, 5000)
      }
    }

    connect()
    return () => {
      disposed = true
      if (retryTimer !== undefined) window.clearTimeout(retryTimer)
      ws?.close()
    }
  }, [session?.id])

  useEffect(() => () => { if (frameURL) URL.revokeObjectURL(frameURL) }, [frameURL])

  const create = async () => {
    setError(null)
    setLoading(true)
    try {
      const next = await api.createBrowserSession()
      setSession(next)
      setApproval(next.pending_approval ?? null)
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause))
    } finally {
      setLoading(false)
    }
  }

  const command = async (name: string, payload?: unknown) => {
    if (!session) return
    try {
      await api.browserCommand(session.id, name, payload)
      setError(null)
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause))
    }
  }

  const navigate = async () => {
    if (!session) return
    await command('navigate', { url })
  }

  const observe = async () => {
    if (!session) return
    try {
      const result = await api.browserCommand<BrowserObservation>(session.id, 'observe')
      setObservation(result)
      setError(null)
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause))
    }
  }

  const takeover = async () => {
    if (!session) return
    try { const next = await api.takeoverBrowser(session.id); setSession(next); setApproval(next.pending_approval ?? null) } catch (cause) { setError(cause instanceof Error ? cause.message : String(cause)) }
  }

  const returnControl = async () => {
    if (!session) return
    try { const next = await api.returnBrowserControl(session.id); setSession(next); setApproval(next.pending_approval ?? null) } catch (cause) { setError(cause instanceof Error ? cause.message : String(cause)) }
  }

  const setApprovalMode = async (enabled: boolean) => {
    if (!session) return
    setApprovalModeLoading(true)
    try {
      const next = await api.setBrowserApprovalMode(session.id, enabled ? 'auto_safe' : 'manual')
      setSession(next)
      setError(null)
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause))
    } finally {
      setApprovalModeLoading(false)
    }
  }

  const close = async () => {
    if (!session) return
    try { await api.closeBrowserSession(session.id); setSession(null); setObservation(null); setApproval(null) } catch (cause) { setError(cause instanceof Error ? cause.message : String(cause)) }
  }

  const restart = async () => {
    if (!session) return
    setLoading(true)
    try {
      const next = await api.restartBrowserSession(session.id)
      setSession(next)
      setApproval(next.pending_approval ?? null)
      setObservation(null)
      setFrameURL((current) => {
        if (current) URL.revokeObjectURL(current)
        return null
      })
      setError(null)
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause))
    } finally {
      setLoading(false)
    }
  }

  const resolveApproval = async (approved: boolean) => {
    if (!session || !approval) return
    try {
      if (approved) await api.approveBrowserAction(session.id, approval.approval_id)
      else await api.rejectBrowserAction(session.id, approval.approval_id)
      setApproval(null)
      await refreshSession()
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause))
    }
  }

  const userInput = (payload: unknown) => {
    if (session?.owner === 'user' && session.state !== 'error') void command('input', payload)
  }

  return (
    <div className="flex h-full min-h-0 flex-col gap-3 overflow-y-auto p-4 md:p-6" data-testid="web-browser-page">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="flex items-center gap-2">
          <Globe className="size-5" />
          <h1 className="text-lg font-semibold">Web browser</h1>
          {session && <span className="text-xs text-muted-foreground">{session.state} · {session.owner} control</span>}
        </div>
        <div className="flex flex-wrap gap-2">
          {!session ? <Button onClick={() => void create()} disabled={loading}><Globe /> Start browser</Button> : (
            <>
              {session.state === 'error' ? <Button variant="secondary" onClick={() => void restart()} disabled={loading}><RefreshCw /> Restart browser</Button> : session.owner === 'user' ? <Button variant="secondary" onClick={() => void returnControl()}><Hand /> Return to AI</Button> : <Button variant="secondary" onClick={() => void takeover()}><UserRound /> Take over</Button>}
              <Button variant="outline" onClick={() => void close()}><X /> Close</Button>
            </>
          )}
        </div>
      </div>

      {error && <div className="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">{error}</div>}

      {!session ? (
        <Card className="max-w-2xl">
          <CardHeader><CardTitle>Start a project browser</CardTitle></CardHeader>
          <CardContent className="flex flex-col gap-3 text-sm text-muted-foreground">
            <p>AI and the user share one persistent browser profile per project. The initial version only permits public HTTPS pages.</p>
            <Button variant="outline" className="w-fit" onClick={() => void api.resetBrowserProfile().then(() => refreshSession()).catch((cause) => setError(cause instanceof Error ? cause.message : String(cause)))}><RotateCcw /> Reset profile</Button>
          </CardContent>
        </Card>
      ) : (
        <div className="grid min-h-[32rem] flex-none gap-3 xl:grid-cols-[minmax(0,1fr)_22rem]">
          <Card className="flex min-h-0 flex-col overflow-hidden">
            <CardContent className="flex min-h-0 flex-1 flex-col gap-2 p-2">
              <div className="flex gap-2">
                <Input value={url} onChange={(event) => setUrl(event.target.value)} onKeyDown={(event) => { if (event.key === 'Enter') void navigate() }} aria-label="Browser URL" />
                <Button onClick={() => void navigate()} disabled={session.owner !== 'user' || session.state === 'error'}>Go</Button>
                <Button variant="outline" onClick={() => void observe()} disabled={session.owner !== 'user' || session.state === 'error'}><RefreshCw /></Button>
              </div>
              <div className="flex min-h-0 flex-1 items-center justify-center overflow-auto rounded-md bg-slate-950 p-1">
                {session.state === 'error' ? (
                  <div className="max-w-md px-6 text-center text-sm text-slate-300">
                    <p className="font-medium text-white">The browser worker disconnected.</p>
                    <p className="mt-2">Restart the browser to reconnect. Your project profile is retained.</p>
                  </div>
                ) : frameURL ? (
                  <img
                    ref={imageRef}
                    src={frameURL}
                    alt="Live browser page"
                    tabIndex={0}
                    className="max-h-full max-w-full object-contain outline-none"
                    onPointerDown={(event) => { event.currentTarget.setPointerCapture(event.pointerId); userInput({ kind: 'mouse', event: 'down', ...pointerPosition(event), button: event.button === 2 ? 'right' : 'left' }) }}
                    onPointerUp={(event) => userInput({ kind: 'mouse', event: 'up', ...pointerPosition(event), button: event.button === 2 ? 'right' : 'left' })}
                    onWheel={(event) => { event.preventDefault(); userInput({ kind: 'wheel', ...pointerPosition(event), delta_x: event.deltaX, delta_y: event.deltaY }) }}
                    onKeyDown={(event) => userInput({ kind: 'key', event: 'down', key: event.key, code: event.code, text: event.key.length === 1 ? event.key : '', modifiers: (event.altKey ? 1 : 0) | (event.ctrlKey ? 2 : 0) | (event.metaKey ? 4 : 0) | (event.shiftKey ? 8 : 0) })}
                    onKeyUp={(event) => userInput({ kind: 'key', event: 'up', key: event.key, code: event.code, modifiers: (event.altKey ? 1 : 0) | (event.ctrlKey ? 2 : 0) | (event.metaKey ? 4 : 0) | (event.shiftKey ? 8 : 0) })}
                  />
                ) : <span className="text-sm text-slate-400">Waiting for browser frame…</span>}
              </div>
            </CardContent>
          </Card>

          <Card className="min-h-0 overflow-auto">
            <CardHeader className="flex flex-row items-center justify-between gap-3">
              <CardTitle className="text-sm">AI interaction</CardTitle>
              <label className="flex items-center gap-2 text-xs">
                <input
                  type="checkbox"
                  role="switch"
                  aria-label="Auto-approve safe actions"
                  data-testid="browser-auto-approve-toggle"
                  checked={session.approval_mode === 'auto_safe'}
                  disabled={approvalModeLoading || session.state === 'error'}
                  onChange={(event) => void setApprovalMode(event.currentTarget.checked)}
                />
                Auto-approve safe actions
              </label>
            </CardHeader>
            <CardContent className="space-y-3 text-sm">
              <p className="text-xs text-muted-foreground">AI navigation and recognized low-risk links, menu, or disclosure controls run without asking. Form submissions and actions that appear to send, publish, buy, or delete still need approval. Page labels are only a heuristic; a website can still cause unexpected effects. Take over at any time. Pending approvals remain manual.</p>
              {approval && <div className="rounded-md border border-amber-300 bg-amber-50 p-3 text-amber-950"><div className="flex items-center gap-2 font-medium"><ShieldAlert className="size-4" /> Approval required</div><p className="mt-2">{approval.action}: {approval.target}</p><div className="mt-3 flex gap-2"><Button size="sm" onClick={() => void resolveApproval(true)}><Check /> Approve</Button><Button size="sm" variant="outline" onClick={() => void resolveApproval(false)}><X /> Reject</Button></div></div>}
              <div><div className="font-medium">Current page</div><div className="break-all text-xs text-muted-foreground">{session.url || 'about:blank'}</div><div className="text-xs text-muted-foreground">{session.title}</div></div>
              {observation ? <><div className="font-medium">Accessibility snapshot</div><pre className="max-h-64 overflow-auto whitespace-pre-wrap rounded bg-muted p-2 text-xs">{observation.aria_snapshot || observation.text || 'No page text'}</pre><div className="font-medium">References ({observation.refs.length})</div><div className="space-y-1">{observation.refs.slice(0, 30).map((ref) => <div key={ref.ref} className="flex gap-2 text-xs"><code>{ref.ref}</code><span className="truncate">{ref.text || ref.tag}</span>{ref.requires_approval && !(session.approval_mode === 'auto_safe' && ref.auto_approvable) && <ShieldAlert className="size-3 shrink-0 text-amber-600" />}{session.approval_mode === 'auto_safe' && ref.auto_approvable && <span className="shrink-0 text-emerald-700">auto</span>}</div>)}</div></> : <p className="text-muted-foreground">Observe the page to expose semantic references for the AI.</p>}
            </CardContent>
          </Card>
        </div>
      )}
      <BrowserAuditPanel />
    </div>
  )
}
