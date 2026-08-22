import { useEffect, useRef, useState } from 'react'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import '@xterm/xterm/css/xterm.css'
import { terminalWsUrl } from '../utils/routes'
import { Button } from './ui/button'
import { cn } from '@/lib/utils'
import { ChevronDown, ChevronUp, Maximize2, Minimize2 } from 'lucide-react'

export interface TerminalTabData {
  id: number
  title: string
  command?: string
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
  onToggleCollapse: () => void
}

type ConnStatus = 'connecting' | 'connected' | 'closed' | 'error'

const STATUS_LABEL: Record<ConnStatus, string> = {
  connecting: 'Connecting…',
  connected: '',
  closed: 'Connection closed',
  error: 'Connection failed',
}

function TerminalView({ active, maximized, command }: { active: boolean; maximized: boolean; command?: string }) {
  const hostRef = useRef<HTMLDivElement>(null)
  const termRef = useRef<Terminal | null>(null)
  const fitRef = useRef<FitAddon | null>(null)
  const wsRef = useRef<WebSocket | null>(null)
  const [status, setStatus] = useState<ConnStatus>('connecting')

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
    termRef.current = term
    fitRef.current = fit
    fit.fit()

    const ws = new WebSocket(terminalWsUrl(command))
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
      fit.fit()
      sendResize()
    })
    ro.observe(host)

    return () => {
      ro.disconnect()
      ws.close()
      term.dispose()
      termRef.current = null
      fitRef.current = null
      wsRef.current = null
    }
  }, [command])

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
    <div className={cn('relative', maximized && 'h-full')}>
      <div
        ref={hostRef}
        onContextMenu={(e) => e.preventDefault()}
        className={cn(
          'terminal-xterm w-full',
          maximized ? 'h-[55vh] lg:h-full' : 'max-lg:h-[40vh] lg:h-64',
        )}
      />
      {status !== 'connected' && (
        <div className="pointer-events-none absolute top-2 right-3 rounded bg-black/60 px-2 py-0.5 text-xs text-red-300">
          {STATUS_LABEL[status]}
        </div>
      )}
    </div>
  )
}

export function TerminalTabs({ tabs, activeId, maximized, collapsed, onAdd, onClose, onActivate, onToggleMaximize, onToggleCollapse }: TerminalTabsProps) {
  return (
    <div
      className={cn(
        'terminal-panel overflow-hidden rounded-md border border-border bg-[#0a0e17]',
        'max-lg:sticky max-lg:bottom-0 max-lg:z-20 max-lg:-m-4 max-lg:rounded-none max-lg:border-x-0 max-lg:border-b-0',
        maximized && !collapsed ? 'flex flex-col lg:h-full' : 'mt-3',
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
        <Button
          variant="ghost"
          size="icon-xs"
          className="ml-1 h-8 w-8 cursor-pointer text-muted-foreground hover:text-foreground"
          onClick={onAdd}
          aria-label="New terminal"
          title="New terminal"
        >
          +
        </Button>
        <Button
          variant="ghost"
          size="icon-xs"
          className="h-8 w-8 cursor-pointer text-muted-foreground hover:text-foreground"
          onClick={onToggleCollapse}
          aria-label={collapsed ? 'Expand terminal' : 'Collapse terminal'}
          title={collapsed ? 'Expand terminal' : 'Collapse terminal'}
        >
          {collapsed ? <ChevronUp className="h-4 w-4" /> : <ChevronDown className="h-4 w-4" />}
        </Button>
        <Button
          variant="ghost"
          size="icon-xs"
          className="ml-auto h-8 w-8 cursor-pointer text-muted-foreground hover:text-foreground"
          onClick={onToggleMaximize}
          aria-label={maximized ? 'Restore terminal' : 'Maximize terminal'}
          title={maximized ? 'Restore terminal' : 'Maximize terminal'}
        >
          {maximized ? <Minimize2 className="h-4 w-4" /> : <Maximize2 className="h-4 w-4" />}
        </Button>
      </div>
      <div
        className={cn(
          'terminal-body',
          collapsed && 'hidden',
          maximized && !collapsed && 'flex min-h-0 flex-1 flex-col',
        )}
      >
        {tabs.map((t) => (
          <div
            key={t.id}
            className={cn(
              t.id === activeId ? 'block' : 'hidden',
              maximized && t.id === activeId && 'min-h-0 flex-1',
            )}
          >
            <TerminalView active={t.id === activeId} maximized={maximized} command={t.command} />
          </div>
        ))}
      </div>
    </div>
  )
}
