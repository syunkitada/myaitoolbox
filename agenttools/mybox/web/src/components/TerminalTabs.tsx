import { useEffect, useImperativeHandle, useRef, useState, forwardRef } from 'react'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import '@xterm/xterm/css/xterm.css'
import { DndContext, closestCenter, PointerSensor, useSensor, useSensors, DragEndEvent } from '@dnd-kit/core'
import { SortableContext, horizontalListSortingStrategy, useSortable } from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import { terminalWsUrl } from '../utils/routes'
import { Button } from './ui/button'
import { cn } from '@/lib/utils'
import { Copy, ClipboardPaste, GripVertical, Maximize2, Minimize2, PanelTopClose } from 'lucide-react'
import { Separator } from './ui/separator'
import { useIsMobile } from '../hooks/use-mobile'
import { useEscapeKey } from '../hooks/use-escape-key'

export interface TerminalTabData {
  id: number
  title: string
  command?: string
  sessionId?: string
}

interface TerminalTabsProps {
  tabs: TerminalTabData[]
  activeId: number
  maximized: boolean
  collapsed: boolean
  onAdd: () => void
  onClose: (id: number) => void
  onActivate: (id: number) => void
  onReorder: (activeId: number, overId: number) => void
  onToggleMaximize: () => void
  onToggleVisible: () => void
}

type ConnStatus = 'connecting' | 'connected' | 'closed' | 'error'

const STATUS_LABEL: Record<ConnStatus, string> = {
  connecting: 'Connecting…',
  connected: '',
  closed: 'Connection closed',
  error: 'Connection failed',
}

const TERMINAL_REFIT_MAX_FRAMES = 30
const TERMINAL_REFIT_RETRY_DELAY_MS = 250
const TERMINAL_WIDTH_TOLERANCE_PX = 12

export interface TerminalViewHandle {
  paste: () => void
  copySelection: () => void
}

export function terminalHostHasLayout(host: Pick<HTMLElement, 'clientWidth' | 'clientHeight'>): boolean {
  return host.clientWidth > 0 && host.clientHeight > 0
}

export function terminalHostWidthIsAligned(hostWidth: number, panelWidth: number): boolean {
  return hostWidth > 0
    && panelWidth > 0
    && Math.abs(hostWidth - panelWidth) <= TERMINAL_WIDTH_TOLERANCE_PX
}

export function terminalRefitRetryAllowed(
  host: Pick<HTMLElement, 'clientWidth' | 'clientHeight'>,
  active: boolean,
  documentVisible = true,
): boolean {
  return documentVisible && active && terminalHostHasLayout(host)
}

export function terminalHostLayoutIsStable(
  previous: Pick<HTMLElement, 'clientWidth' | 'clientHeight'> | null,
  current: Pick<HTMLElement, 'clientWidth' | 'clientHeight'>,
): boolean {
  return terminalHostHasLayout(current)
    && previous !== null
    && previous.clientWidth === current.clientWidth
    && previous.clientHeight === current.clientHeight
}

export function terminalFitIsReady(
  proposed: Pick<{ cols: number; rows: number }, 'cols' | 'rows'> | undefined,
  actual: Pick<{ cols: number; rows: number }, 'cols' | 'rows'>,
): boolean {
  return proposed !== undefined
    && proposed.cols > 0
    && proposed.rows > 0
    && proposed.cols === actual.cols
    && proposed.rows === actual.rows
}

export function terminalLayoutRefitDecision(
  previous: Pick<HTMLElement, 'clientWidth' | 'clientHeight'> | null,
  current: Pick<HTMLElement, 'clientWidth' | 'clientHeight'>,
  attempts: number,
): { fit: boolean; retry: boolean; attempts: number } {
  if (terminalHostLayoutIsStable(previous, current)) {
    return { fit: true, retry: false, attempts }
  }
  if (attempts < TERMINAL_REFIT_MAX_FRAMES) {
    return { fit: false, retry: true, attempts: attempts + 1 }
  }
  return { fit: false, retry: false, attempts }
}

const TerminalView = forwardRef<TerminalViewHandle, { active: boolean; command?: string; sessionId?: string }>(function TerminalView({ active, command, sessionId }, ref) {
  const hostRef = useRef<HTMLDivElement>(null)
  const termRef = useRef<Terminal | null>(null)
  const activeRef = useRef(active)
  const refitRequestRef = useRef<(() => void) | null>(null)
  const cancelRefitRef = useRef<(() => void) | null>(null)
  const hiddenInputRef = useRef<HTMLTextAreaElement>(null)
  const pasteInputRef = useRef<HTMLTextAreaElement>(null)
  const [status, setStatus] = useState<ConnStatus>('connecting')
  const [notice, setNotice] = useState<{ text: string; kind: 'ok' | 'err' } | null>(null)
  const [pasteMode, setPasteMode] = useState(false)
  const [osc52Modal, setOsc52Modal] = useState<{ text: string } | null>(null)

  activeRef.current = active

  useEscapeKey(() => {
    if (osc52Modal) {
      setOsc52Modal(null)
    } else if (pasteMode) {
      setPasteMode(false)
    }
  }, pasteMode || Boolean(osc52Modal))

  const showNotice = (text: string, kind: 'ok' | 'err' = 'ok') => {
    setNotice({ text, kind })
    window.setTimeout(() => setNotice((n) => (n?.text === text ? null : n)), 1800)
  }

  const copyText = async (text: string): Promise<boolean> => {
    if (!text) return false
    const ta = hiddenInputRef.current
    if (ta) {
      ta.value = text
      ta.readOnly = false
      ta.style.cssText = 'position:fixed;left:0;top:0;width:1px;height:1px;opacity:0.01;'
      document.body.appendChild(ta)
      ta.focus()
      ta.select()
      ta.setSelectionRange(0, text.length)
      let ok = false
      try {
        ok = document.execCommand('copy')
      } catch { /* ignore */ }
      document.body.removeChild(ta)
      ta.style.cssText = 'position:absolute;height:1px;width:1px;-left-[9999px];top:0;opacity:0;'
      if (ok) return true
    }
    if (navigator.clipboard?.writeText) {
      try {
        await navigator.clipboard.writeText(text)
        return true
      } catch { /* ignore */ }
    }
    return false
  }

  const handlePasteSubmit = () => {
    const ta = pasteInputRef.current
    const term = termRef.current
    if (!term) return
    const text = ta?.value ?? ''
    setPasteMode(false)
    term.focus()
    if (text) {
      term.paste(text)
      showNotice('Pasted')
    } else {
      showNotice('No text to paste', 'err')
    }
  }

  const pasteClipboard = () => {
    if (!termRef.current) return
    setPasteMode(true)
    setTimeout(() => {
      const ta = pasteInputRef.current
      if (ta) {
        ta.value = ''
        ta.focus()
      }
    }, 50)
  }

  const copySelection = async (sel?: string) => {
    const text = sel ?? termRef.current?.getSelection() ?? ''
    const ok = await copyText(text)
    showNotice(ok ? 'Copied' : 'Nothing to copy', ok ? 'ok' : 'err')
  }

  useImperativeHandle(ref, () => ({
    paste: pasteClipboard,
    copySelection,
  }))

  useEffect(() => {
    const host = hostRef.current
    if (!host) return

    const term = new Terminal({
      cursorBlink: true,
      fontSize: 13,
      fontFamily: 'Menlo, Monaco, "Cascadia Mono", "Courier New", monospace',
      scrollback: 10000,
      theme: {
        background: '#0a0e17',
        foreground: '#e4e4e4',
        cursor: '#e4e4e4',
        selectionBackground: '#264f78',
      },
    })
    const fit = new FitAddon()
    term.loadAddon(fit)
    term.open(host)

    term.parser.registerOscHandler(52, (data) => {
      const args = data.split(';')
      if (args.length < 2 || args[1] === '?') return true
      let text = ''
      try {
        text = decodeURIComponent(escape(atob(args[1])))
      } catch { /* ignore */ }
      text = text.trim()
      if (text) setOsc52Modal({ text })
      return true
    })
    termRef.current = term

    const panel = host.closest<HTMLElement>('.terminal-panel')
    const fitToWidth = () => {
      if (!terminalHostHasLayout(host)) return false
      if (panel && !terminalHostWidthIsAligned(host.clientWidth, panel.clientWidth)) return false
      const core = (term as unknown as {
        _core?: {
          _charSizeService?: { measure: () => void }
          _renderService?: {
            clear: () => void
            handleCharSizeChanged: () => void
            handleResize: (cols: number, rows: number) => void
          }
          viewport?: { scrollBarWidth?: number }
        }
      })._core
      // A terminal opened during a context switch can retain stale font
      // metrics even when its host already has a measurable layout. Refresh
      // xterm's character measurement before FitAddon calculates dimensions.
      core?._charSizeService?.measure()
      core?._renderService?.handleCharSizeChanged()
      term.resize(term.cols, term.rows)
      if (core?.viewport) {
        core.viewport.scrollBarWidth = 0
      }
      fit.fit()
      const proposed = fit.proposeDimensions()
      const actual = { cols: term.cols, rows: term.rows }
      host.dataset.terminalProposedCols = proposed ? String(proposed.cols) : ''
      host.dataset.terminalProposedRows = proposed ? String(proposed.rows) : ''
      if (!terminalFitIsReady(proposed, actual)) return false
      // FitAddon skips Terminal.resize when the calculated cell count has not
      // changed. A terminal can still have stale row dimensions after a
      // route/tab transition in that case; clear and update the renderer
      // explicitly so this follows the same repaint path as a real resize
      // drag.
      core?._renderService?.handleCharSizeChanged()
      core?._renderService?.clear()
      core?._renderService?.handleResize(term.cols, term.rows)
      if (term.rows > 0) {
        term.refresh(0, term.rows - 1)
      }
      const renderedWidth = term.element?.querySelector<HTMLElement>('.xterm-screen')?.clientWidth ?? 0
      if (!terminalHostWidthIsAligned(renderedWidth, host.clientWidth)) return false
      host.dataset.terminalCols = String(term.cols)
      host.dataset.terminalRows = String(term.rows)
      return true
    }

    const ws = new WebSocket(terminalWsUrl(command, sessionId))
    ws.binaryType = 'arraybuffer'

    const send = (msg: unknown) => {
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify(msg))
      }
    }

    const sendResize = () => {
      if (!termRef.current) return
      send({ type: 'resize', cols: term.cols, rows: term.rows })
    }

    let refitFrame = 0
    let refitAttempts = 0
    let previousLayout: Pick<HTMLElement, 'clientWidth' | 'clientHeight'> | null = null
    let disposed = false
    let refitRetryTimer = 0
    let refit: () => void = () => {}
    let requestRefit: () => void = () => {}

    const clearRefitRetry = () => {
      if (refitRetryTimer !== 0) {
        window.clearTimeout(refitRetryTimer)
        refitRetryTimer = 0
      }
    }

    const scheduleRefitRetry = () => {
      if (refitAttempts < TERMINAL_REFIT_MAX_FRAMES) {
        refitAttempts += 1
        refitFrame = window.requestAnimationFrame(refit)
        return
      }
      previousLayout = null
      refitAttempts = 0
      const currentLayout = {
        clientWidth: host.clientWidth,
        clientHeight: host.clientHeight,
      }
      if (refitRetryTimer === 0 && !disposed && terminalRefitRetryAllowed(currentLayout, activeRef.current, !document.hidden)) {
        refitRetryTimer = window.setTimeout(() => {
          refitRetryTimer = 0
          requestRefit()
        }, TERMINAL_REFIT_RETRY_DELAY_MS)
      }
    }

    refit = () => {
      refitFrame = 0
      const currentLayout = {
        clientWidth: host.clientWidth,
        clientHeight: host.clientHeight,
      }
      if (document.hidden || !activeRef.current) {
        clearRefitRetry()
        previousLayout = null
        refitAttempts = 0
        return
      }
      const decision = terminalLayoutRefitDecision(previousLayout, currentLayout, refitAttempts)
      previousLayout = currentLayout
      refitAttempts = decision.attempts
      if (decision.retry) {
        refitFrame = window.requestAnimationFrame(refit)
        return
      }
      if (!decision.fit) {
        scheduleRefitRetry()
        return
      }
      if (!fitToWidth()) {
        scheduleRefitRetry()
        return
      }
      clearRefitRetry()
      sendResize()
      previousLayout = null
      refitAttempts = 0
    }

    requestRefit = () => {
      if (document.hidden || !activeRef.current || refitFrame !== 0) return
      clearRefitRetry()
      previousLayout = null
      refitAttempts = 0
      refitFrame = window.requestAnimationFrame(refit)
    }

    const cancelRefit = () => {
      clearRefitRetry()
      if (refitFrame !== 0) {
        window.cancelAnimationFrame(refitFrame)
        refitFrame = 0
      }
      previousLayout = null
      refitAttempts = 0
    }
    refitRequestRef.current = requestRefit
    cancelRefitRef.current = cancelRefit
    void document.fonts?.ready.then(() => {
      if (!disposed) requestRefit()
    })

    term.onData((data) => send({ type: 'input', data }))

    term.attachCustomKeyEventHandler((ev) => {
      if (ev.ctrlKey && !ev.metaKey && ev.code === 'Enter' && ev.type === 'keydown') {
        ev.preventDefault()
        send({ type: 'input', data: '\u001b[13;5u' })
        return false
      }
      const mod = ev.ctrlKey || ev.metaKey
      if (mod && !ev.shiftKey && ev.code === 'KeyV') {
        pasteClipboard()
        return false
      }
      if (mod && ev.shiftKey && ev.code === 'KeyC') {
        copySelection()
        return false
      }
      if (mod && ev.shiftKey && ev.code === 'KeyV') {
        pasteClipboard()
        return false
      }
      return true
    })

    // Copy the selection to the OS clipboard automatically when text is selected.
    let selectionTimer: number | undefined
    term.onSelectionChange(() => {
      if (!term.getSelection()) return
      window.clearTimeout(selectionTimer)
      selectionTimer = window.setTimeout(() => {
        const sel = term.getSelection()
        if (sel) void copyText(sel)
      }, 150)
    })

    ws.onopen = () => {
      setStatus('connected')
      requestRefit()
    }
    ws.onmessage = (ev) => {
      if (ev.data instanceof ArrayBuffer) {
        term.write(new Uint8Array(ev.data))
      } else if (typeof ev.data === 'string') {
        term.write(ev.data)
      }
    }
    ws.onclose = () => setStatus('closed')
    ws.onerror = () => setStatus('error')

    const ro = new ResizeObserver(() => {
      requestRefit()
    })
    ro.observe(host)
    if (panel && panel !== host) ro.observe(panel)
    const screen = term.element?.querySelector<HTMLElement>('.xterm-screen')
    if (screen) ro.observe(screen)
    const onWindowResize = () => requestRefit()
    const onVisibilityChange = () => {
      if (document.hidden) {
        cancelRefit()
      } else {
        requestRefit()
      }
    }
    window.addEventListener('resize', onWindowResize)
    window.visualViewport?.addEventListener('resize', onWindowResize)
    document.addEventListener('visibilitychange', onVisibilityChange)

    return () => {
      disposed = true
      window.clearTimeout(selectionTimer)
      ro.disconnect()
      window.removeEventListener('resize', onWindowResize)
      window.visualViewport?.removeEventListener('resize', onWindowResize)
      document.removeEventListener('visibilitychange', onVisibilityChange)
      ws.close()
      term.dispose()
      cancelRefit()
      if (refitRequestRef.current === requestRefit) refitRequestRef.current = null
      if (cancelRefitRef.current === cancelRefit) cancelRefitRef.current = null
      termRef.current = null
    }
  }, [command, sessionId])
  // Re-fit when the tab becomes visible again.
  useEffect(() => {
    if (!active) {
      cancelRefitRef.current?.()
      return
    }
    refitRequestRef.current?.()
    return () => cancelRefitRef.current?.()
  }, [active])

  return (
    <div className="relative h-full min-w-0 w-full">
      <textarea
        ref={hiddenInputRef}
        tabIndex={-1}
        aria-hidden="true"
        className="absolute h-px w-px -left-[9999px] top-0 opacity-0"
      />
      <div
        ref={hostRef}
        className="terminal-xterm h-full w-full"
        onContextMenu={(e) => e.preventDefault()}
      />
      {pasteMode && (
        <div className="absolute inset-0 z-50 flex flex-col items-center justify-center bg-black/80 p-4">
          <p className="mb-3 text-sm text-white">
            Long-press below and tap Paste
          </p>
          <textarea
            ref={pasteInputRef}
            className="mb-3 w-full max-w-none resize rounded border border-border bg-white p-3 text-base text-black focus:outline-none md:text-sm"
            rows={6}
            placeholder="Tap here, then long-press → Paste"
            autoFocus
            onKeyDown={(e) => {
              if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
                e.preventDefault()
                handlePasteSubmit()
              }
            }}
          />
          <div className="flex gap-2">
            <Button
              size="sm"
              className="bg-green-600 text-white hover:bg-green-700"
              onClick={handlePasteSubmit}
            >
              Send to terminal
            </Button>
            <Button
              size="sm"
              variant="outline"
              onClick={() => setPasteMode(false)}
            >
              Cancel
            </Button>
          </div>
        </div>
      )}
      {osc52Modal && (
        <div className="absolute inset-0 z-50 flex flex-col items-center justify-center bg-black/80 p-4">
          <div className="flex w-full max-w-md flex-col overflow-hidden rounded-md border border-border bg-card">
            <div className="flex items-center justify-between border-b border-border/70 px-4 py-2">
              <span className="text-sm font-medium text-foreground">Copy from terminal</span>
              <button
                className="flex h-6 w-6 cursor-pointer items-center justify-center rounded text-muted-foreground hover:bg-border/60 hover:text-foreground"
                onClick={() => setOsc52Modal(null)}
                aria-label="Close"
              >
                ×
              </button>
            </div>
            <pre className="max-h-64 overflow-auto whitespace-pre-wrap break-all px-4 py-3 text-xs text-foreground">
              {osc52Modal.text}
            </pre>
            <div className="flex gap-2 border-t border-border/70 px-4 py-3">
              <Button
                size="sm"
                className="bg-green-600 text-white hover:bg-green-700"
                onClick={() => {
                  void copyText(osc52Modal.text)
                  setOsc52Modal(null)
                }}
              >
                Copy to clipboard
              </Button>
              <Button
                size="sm"
                variant="outline"
                onClick={() => setOsc52Modal(null)}
              >
                Close
              </Button>
            </div>
          </div>
        </div>
      )}
      {status !== 'connected' && (
        <div className="pointer-events-none absolute top-2 right-3 rounded bg-black/60 px-2 py-0.5 text-xs text-red-300">
          {STATUS_LABEL[status]}
        </div>
      )}
      {notice && (
        <div
          className={cn(
            'pointer-events-none absolute bottom-2 left-1/2 -translate-x-1/2 rounded bg-black/70 px-3 py-1 text-xs',
            notice.kind === 'err' ? 'text-red-300' : 'text-green-300',
          )}
        >
          {notice.text}
        </div>
      )}
    </div>
  )
})

interface SortableTerminalTabProps {
  tab: TerminalTabData
  active: boolean
  onClose: (id: number) => void
  onActivate: (id: number) => void
}

function SortableTerminalTab({ tab, active, onClose, onActivate }: SortableTerminalTabProps) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({
    id: tab.id,
  })

  return (
    <div
      ref={setNodeRef}
      style={{
        transform: CSS.Transform.toString(transform),
        transition,
        opacity: isDragging ? 0.5 : 1,
      }}
      className={cn(
        'group flex items-center rounded-t-md border border-b-0 text-xs',
        active
          ? 'border-border bg-[#0a0e17] text-green-400'
          : 'border-transparent bg-muted/60 text-muted-foreground hover:text-foreground',
      )}
    >
      <button
        type="button"
        className="flex h-8 cursor-pointer items-center px-3"
        onClick={() => onActivate(tab.id)}
        aria-label={`Activate terminal ${tab.id}`}
      >
        {tab.title}
      </button>
      <button
        type="button"
        className="flex h-6 w-5 cursor-grab touch-none items-center justify-center rounded text-muted-foreground hover:bg-border/60 hover:text-foreground active:cursor-grabbing"
        {...attributes}
        {...listeners}
        aria-label={`Reorder terminal ${tab.id}`}
        title="Reorder terminal"
      >
        <GripVertical className="h-3.5 w-3.5" />
      </button>
      <button
        type="button"
        className="mr-1 flex h-6 w-6 cursor-pointer items-center justify-center rounded text-muted-foreground hover:bg-border/60 hover:text-foreground"
        onClick={() => onClose(tab.id)}
        aria-label={`Close terminal ${tab.id}`}
      >
        ×
      </button>
    </div>
  )
}

export function TerminalTabs({ tabs, activeId, maximized, collapsed, onAdd, onClose, onActivate, onReorder, onToggleMaximize, onToggleVisible }: TerminalTabsProps) {
  const termRefs = useRef<Map<number, TerminalViewHandle>>(new Map())
  const isMobile = useIsMobile()
  const sensors = useSensors(
    useSensor(PointerSensor, {
      activationConstraint: { distance: 5 },
    }),
  )

  const handleDragEnd = (event: DragEndEvent) => {
    if (!event.over) return
    const activeId = Number(event.active.id)
    const overId = Number(event.over.id)
    if (!Number.isSafeInteger(activeId) || !Number.isSafeInteger(overId)) return
    onReorder(activeId, overId)
  }

  const activeTermRef = termRefs.current.get(activeId)

  return (
    <div
      className={cn(
        'terminal-panel flex h-full min-w-0 w-full flex-col overflow-hidden rounded-md border border-border bg-[#0a0e17]',
        maximized ? 'max-lg:mt-0' : 'mt-3',
        'max-lg:h-full max-lg:flex-1 max-lg:rounded-none max-lg:border-0 max-lg:mt-0',
      )}
    >
      <div className="terminal-tabbar flex items-center gap-0.5 border-b border-border/70 bg-card px-2 pt-1.5">
        <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={handleDragEnd}>
          <SortableContext items={tabs.map((tab) => tab.id)} strategy={horizontalListSortingStrategy}>
            {tabs.map((tab) => (
              <SortableTerminalTab
                key={tab.id}
                tab={tab}
                active={tab.id === activeId}
                onClose={onClose}
                onActivate={onActivate}
              />
            ))}
          </SortableContext>
        </DndContext>
        <div className="flex items-center gap-1.5">
          <Button
            variant="ghost"
            size="icon-xs"
            className="h-8 w-8 cursor-pointer text-muted-foreground hover:text-foreground"
            onClick={() => onAdd()}
            aria-label="New terminal"
            title="New terminal"
          >
            +
          </Button>
          <Separator orientation="vertical" className="mx-0.5 h-4" />
          <Button
            variant="ghost"
            size="icon-xs"
            className="h-8 w-8 cursor-pointer text-muted-foreground hover:text-foreground"
            onClick={() => activeTermRef?.paste()}
            aria-label="Paste"
            title="Paste"
          >
            <ClipboardPaste className="h-4 w-4" />
          </Button>
          {isMobile && (
            <Button
              variant="ghost"
              size="icon-xs"
              className="h-8 w-8 cursor-pointer text-muted-foreground hover:text-foreground"
              onClick={() => activeTermRef?.copySelection()}
              aria-label="Copy"
              title="Copy"
            >
              <Copy className="h-4 w-4" />
            </Button>
          )}
          <Separator orientation="vertical" className="mx-0.5 h-4 max-lg:hidden" />
          <Button
            variant="ghost"
            size="icon-xs"
            className="h-8 w-8 cursor-pointer text-muted-foreground hover:text-foreground max-lg:hidden"
            onClick={onToggleMaximize}
            aria-label={maximized ? 'Restore terminal' : 'Maximize terminal'}
            title={maximized ? 'Restore terminal' : 'Maximize terminal'}
          >
            {maximized ? <Minimize2 className="h-4 w-4" /> : <Maximize2 className="h-4 w-4" />}
          </Button>
          <Button
            variant="ghost"
            size="icon-xs"
            className="h-8 w-8 cursor-pointer text-muted-foreground hover:text-foreground"
            onClick={onToggleVisible}
            aria-label="Hide terminal"
            title="Hide terminal"
          >
            <PanelTopClose className="h-4 w-4" />
          </Button>
        </div>
      </div>
      <div
        className={cn(
          'terminal-body relative flex min-h-0 w-full min-w-0 flex-1 flex-col',
          collapsed && 'hidden',
        )}
      >
        {tabs.map((t) => (
          <div
            key={t.id}
            aria-hidden={t.id !== activeId}
            className={cn(
              'absolute inset-0 min-w-0',
              t.id === activeId ? 'block' : 'hidden',
            )}
          >
            <TerminalView
              ref={(handle) => {
                if (handle) {
                  termRefs.current.set(t.id, handle)
                } else {
                  termRefs.current.delete(t.id)
                }
              }}
              active={t.id === activeId && !collapsed}
              command={t.command}
              sessionId={t.sessionId}
            />
          </div>
        ))}
      </div>
    </div>
  )
}
