import { useEffect, useRef, useState } from 'react'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import { ClipboardAddon } from '@xterm/addon-clipboard'
import '@xterm/xterm/css/xterm.css'
import { terminalWsUrl } from '../utils/routes'
import { Button } from './ui/button'
import { cn } from '@/lib/utils'
import { Maximize2, Minimize2, PanelTopClose } from 'lucide-react'

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

function TerminalView({ active, command, sessionId }: { active: boolean; command?: string; sessionId?: string }) {
  const hostRef = useRef<HTMLDivElement>(null)
  const termRef = useRef<Terminal | null>(null)
  const fitRef = useRef<FitAddon | null>(null)
  const wsRef = useRef<WebSocket | null>(null)
  const hiddenInputRef = useRef<HTMLTextAreaElement>(null)
  const [status, setStatus] = useState<ConnStatus>('connecting')
  const [notice, setNotice] = useState<{ text: string; kind: 'ok' | 'err' } | null>(null)

  const showNotice = (text: string, kind: 'ok' | 'err' = 'ok') => {
    setNotice({ text, kind })
    window.setTimeout(() => setNotice((n) => (n?.text === text ? null : n)), 1800)
  }

  const copyText = async (text: string): Promise<boolean> => {
    if (!text) return false
    if (navigator.clipboard?.writeText) {
      try {
        await navigator.clipboard.writeText(text)
        return true
      } catch {
        /* fall through to textarea fallback */
      }
    }
    const ta = hiddenInputRef.current
    if (ta) {
      ta.value = text
      ta.focus()
      ta.select()
      ta.setSelectionRange(0, text.length)
      try {
        return document.execCommand('copy')
      } catch {
        return false
      }
    }
    return false
  }

  const readClipboard = (): Promise<string> =>
    new Promise((resolve) => {
      if (navigator.clipboard?.readText) {
        navigator.clipboard.readText().then(resolve).catch(() => readViaPasteEvent(resolve))
        return
      }
      readViaPasteEvent(resolve)
    })

  // Triggers a real browser paste event on the hidden textarea. In a genuine
  // paste event the browser fills clipboardData with the OS clipboard content
  // without requiring the clipboard-read permission.
  const readViaPasteEvent = (resolve: (text: string) => void) => {
    const ta = hiddenInputRef.current
    if (!ta) return
    let settled = false
    const onPaste = (e: ClipboardEvent) => {
      if (settled) return
      settled = true
      ta.removeEventListener('paste', onPaste)
      resolve(e.clipboardData?.getData('text') ?? '')
    }
    ta.addEventListener('paste', onPaste)
    ta.value = ''
    ta.focus()
    ta.select()
    try {
      document.execCommand('paste')
    } catch {
      /* the paste event may not have fired */
    }
    window.setTimeout(() => {
      if (settled) return
      settled = true
      ta.removeEventListener('paste', onPaste)
      resolve(ta.value)
    }, 60)
  }

  const copySelection = async (sel?: string) => {
    const text = sel ?? termRef.current?.getSelection() ?? ''
    const ok = await copyText(text)
    showNotice(ok ? 'Copied' : 'Nothing to copy', ok ? 'ok' : 'err')
  }

  const pasteClipboard = async () => {
    const term = termRef.current
    if (!term) return
    const text = await readClipboard()
    term.focus()
    if (text) {
      term.paste(text)
      showNotice('Pasted')
    } else {
      showNotice(
        'Cannot read clipboard. Use long-press / right-click to paste',
        'err',
      )
    }
  }

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
    const clipboard = new ClipboardAddon()
    term.loadAddon(fit)
    term.loadAddon(clipboard)
    term.open(host)
    termRef.current = term
    fitRef.current = fit

    const fitToWidth = () => {
      const core = (term as unknown as { _core?: { viewport?: { scrollBarWidth?: number } } })._core
      if (window.innerWidth < 768 && core?.viewport) {
        core.viewport.scrollBarWidth = 0
      }
      fit.fit()
    }
    fitToWidth()

    const ws = new WebSocket(terminalWsUrl(command, sessionId))
    ws.binaryType = 'arraybuffer'
    wsRef.current = ws

    const send = (msg: unknown) => {
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify(msg))
      }
    }

    const sendResize = () => {
      if (!termRef.current) return
      send({ type: 'resize', cols: term.cols, rows: term.rows })
    }

    const hostVisible = () =>
      host.offsetParent !== null && host.clientWidth > 0 && host.clientHeight > 0

    term.onData((data) => send({ type: 'input', data }))

    term.attachCustomKeyEventHandler((ev) => {
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
      if (hostVisible()) {
        sendResize()
      }
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
      if (!termRef.current) return
      if (!hostVisible()) return
      fitToWidth()
      sendResize()
    })
    ro.observe(host)

    return () => {
      window.clearTimeout(selectionTimer)
      ro.disconnect()
      ws.close()
      term.dispose()
      termRef.current = null
      fitRef.current = null
      wsRef.current = null
    }
  }, [command, sessionId])
  // Re-fit when the tab becomes visible again.
  useEffect(() => {
    if (!active) return
    const term = termRef.current
    if (!term) return
    const host = hostRef.current
    if (host && (host.offsetParent === null || host.clientWidth === 0 || host.clientHeight === 0)) {
      return
    }
    const fit = fitRef.current
    if (fit) fit.fit()
    const ws = wsRef.current
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ type: 'resize', cols: term.cols, rows: term.rows }))
    }
  }, [active])

  return (
    <div className="relative h-full">
      <textarea
        ref={hiddenInputRef}
        tabIndex={-1}
        aria-hidden="true"
        className="absolute h-px w-px -left-[9999px] top-0 opacity-0"
      />
      <div
        ref={hostRef}
        className="terminal-xterm h-full w-full"
      />
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
}

export function TerminalTabs({ tabs, activeId, maximized, collapsed, onAdd, onClose, onActivate, onToggleMaximize, onToggleVisible }: TerminalTabsProps) {
  return (
    <div
      className={cn(
        'terminal-panel flex h-full flex-col overflow-hidden rounded-md border border-border bg-[#0a0e17]',
        maximized ? 'max-lg:mt-0' : 'mt-3',
        'max-lg:h-full max-lg:flex-1 max-lg:rounded-none max-lg:border-0 max-lg:mt-0',
      )}
    >
      <div className="terminal-tabbar flex items-center gap-0.5 border-b border-border/70 bg-card px-2 pt-1.5">
        {tabs.map((t) => {
          const active = t.id === activeId
          return (
            <div
              key={t.id}
              className={cn(
                'group flex items-center rounded-t-md border border-b-0 text-xs',
                active
                  ? 'border-border bg-[#0a0e17] text-green-400'
                  : 'border-transparent bg-muted/60 text-muted-foreground hover:text-foreground',
              )}
            >
              <button
                className="flex h-8 cursor-pointer items-center px-3"
                onClick={() => onActivate(t.id)}
                aria-label={`Activate terminal ${t.id}`}
              >
                {t.title}
              </button>
              <button
                className="mr-1 flex h-6 w-6 cursor-pointer items-center justify-center rounded text-muted-foreground hover:bg-border/60 hover:text-foreground"
                onClick={() => onClose(t.id)}
                aria-label={`Close terminal ${t.id}`}
              >
                ×
              </button>
            </div>
          )
        })}
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
          'terminal-body flex min-h-0 flex-1 flex-col',
          collapsed && 'hidden',
        )}
      >
        {tabs.map((t) => (
          <div
            key={t.id}
            className={cn(
              t.id === activeId ? 'block min-h-0 flex-1' : 'hidden',
            )}
          >
            <TerminalView active={t.id === activeId} command={t.command} sessionId={t.sessionId} />
          </div>
        ))}
      </div>
    </div>
  )
}
