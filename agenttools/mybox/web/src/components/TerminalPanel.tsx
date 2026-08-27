import { useCallback, useEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { TerminalTabs, TerminalTabData } from './TerminalTabs'
import { subscribeNavActions } from '@/lib/nav-actions'
import { cn } from '@/lib/utils'

function useMediaQuery(query: string): boolean {
  const [matches, setMatches] = useState(
    () => typeof window !== 'undefined' && window.matchMedia(query).matches,
  )
  useEffect(() => {
    const mql = window.matchMedia(query)
    const onChange = () => setMatches(mql.matches)
    onChange()
    mql.addEventListener('change', onChange)
    return () => mql.removeEventListener('change', onChange)
  }, [query])
  return matches
}

export function TerminalPanel({ className }: { className?: string }) {
  const [terminals, setTerminals] = useState<TerminalTabData[]>([])
  const [activeTerminal, setActiveTerminal] = useState<number | null>(null)
  const [maximized, setMaximized] = useState(false)
  const [collapsed, setCollapsed] = useState(false)
  const [height, setHeight] = useState(320)
  const terminalIdRef = useRef(0)
  const dragRef = useRef<{ startY: number; startHeight: number } | null>(null)
  const isMobile = useMediaQuery('(max-width: 1023px)')
  const [keyboard, setKeyboard] = useState<{ top: number; height: number } | null>(null)

  // When the on-screen keyboard is shown on mobile, shrink the full-screen
  // terminal to the visible viewport so the keyboard doesn't cover the input.
  useEffect(() => {
    if (!isMobile) {
      setKeyboard(null)
      return
    }
    const vv = window.visualViewport
    if (!vv) return
    const update = () => {
      const open = vv.height < window.innerHeight - 1
      setKeyboard(
        open ? { top: vv.offsetTop, height: vv.height } : null,
      )
    }
    update()
    vv.addEventListener('resize', update)
    vv.addEventListener('scroll', update)
    return () => {
      vv.removeEventListener('resize', update)
      vv.removeEventListener('scroll', update)
    }
  }, [isMobile])

  const handleDragStart = useCallback(
    (e: React.MouseEvent | React.TouchEvent) => {
      e.preventDefault()
      const clientY = 'touches' in e ? e.touches[0].clientY : e.clientY
      dragRef.current = { startY: clientY, startHeight: height }

      const onMove = (ev: MouseEvent | TouchEvent) => {
        if (!dragRef.current) return
        const y = 'touches' in ev ? ev.touches[0].clientY : ev.clientY
        const delta = dragRef.current.startY - y
        const newHeight = Math.min(Math.max(dragRef.current.startHeight + delta, 80), window.innerHeight * 0.7)
        setHeight(newHeight)
      }

      const onUp = () => {
        dragRef.current = null
        document.removeEventListener('mousemove', onMove)
        document.removeEventListener('mouseup', onUp)
        document.removeEventListener('touchmove', onMove)
        document.removeEventListener('touchend', onUp)
      }

      document.addEventListener('mousemove', onMove)
      document.addEventListener('mouseup', onUp)
      document.addEventListener('touchmove', onMove, { passive: false })
      document.addEventListener('touchend', onUp)
    },
    [height],
  )

  const addTerminal = useCallback((title?: string, command?: string) => {
    terminalIdRef.current += 1
    const id = terminalIdRef.current
    setTerminals((prev) => [...prev, { id, title: title ?? `Terminal ${id}`, command }])
    setActiveTerminal(id)
  }, [])

  useEffect(
    () =>
      subscribeNavActions((action) => {
        if (action === 'open-terminal') addTerminal()
        if (action === 'open-chat-opencode') addTerminal('OpenCode', 'opencode')
        if (action === 'open-chat-codex') addTerminal('Codex', 'codex')
      }),
    [addTerminal],
  )

  const closeTerminal = useCallback((id: number) => {
    setTerminals((prev) => {
      const next = prev.filter((t) => t.id !== id)
      if (next.length === 0) {
        setMaximized(false)
        setCollapsed(false)
      }
      setActiveTerminal((current) => {
        if (next.length === 0) return null
        if (current === id) return next[next.length - 1].id
        return current
      })
      return next
    })
  }, [])

  const toggleMaximize = useCallback(() => {
    setCollapsed(false)
    setMaximized((m) => !m)
  }, [])

  const toggleCollapse = useCallback(() => setCollapsed((c) => !c), [])

  const closeAll = useCallback(() => {
    setTerminals([])
    setActiveTerminal(null)
    setMaximized(false)
    setCollapsed(false)
  }, [])

  const activateTerminal = useCallback((id: number) => setActiveTerminal(id), [])

  if (terminals.length === 0 || activeTerminal === null) return null

  const content = (
    <div
      className={cn(
        'flex flex-col',
        isMobile && 'fixed z-50 bg-background',
        isMobile && !keyboard && 'inset-0 h-svh',
        className,
      )}
      style={isMobile && keyboard ? { top: keyboard.top, left: 0, width: '100%', height: keyboard.height } : undefined}
    >
      <div
        className="h-0.5 shrink-0 cursor-row-resize bg-border/50 transition-colors hover:bg-border hover:h-1 max-lg:hidden"
        onMouseDown={handleDragStart}
        onTouchStart={handleDragStart}
      />
      <div
        className={cn(
          'flex flex-col max-lg:min-h-0 max-lg:flex-1',
          !isMobile && maximized && !collapsed ? 'min-h-0 flex-1' : 'shrink-0 bg-card',
        )}
        style={!isMobile && !(maximized && !collapsed) ? { height } : undefined}
      >
        <TerminalTabs
          tabs={terminals}
          activeId={activeTerminal}
          maximized={maximized}
          collapsed={collapsed}
          onAdd={addTerminal}
          onClose={closeTerminal}
          onCloseAll={closeAll}
          onActivate={activateTerminal}
          onToggleMaximize={toggleMaximize}
          onToggleCollapse={toggleCollapse}
        />
      </div>
    </div>
  )

  if (isMobile) {
    return createPortal(content, document.body)
  }
  return content
}
