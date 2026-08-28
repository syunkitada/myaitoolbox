import { useCallback, useEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { TerminalTabs, TerminalTabData } from './TerminalTabs'
import { subscribeNavActions } from '@/lib/nav-actions'
import { getProject } from '@/utils/routes'
import { api } from '@/api/client'
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

interface ProjectTermState {
  tabs: TerminalTabData[]
  activeId: number | null
  collapsed: boolean
  visible: boolean
}

type ProjectTermMap = Record<string, ProjectTermState>

const STORAGE_KEY = 'mybox_terminals_v1'
const EMPTY: ProjectTermState = { tabs: [], activeId: null, collapsed: false, visible: false }

function loadProjectMap(): ProjectTermMap {
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY)
    if (raw) {
      const parsed = JSON.parse(raw) as ProjectTermMap
      // Normalize older persisted entries that predate the `visible` flag so a
      // restored terminal panel is shown rather than hidden.
      for (const key of Object.keys(parsed)) {
        const p = parsed[key]
        if (p && Array.isArray(p.tabs) && p.tabs.length > 0 && typeof p.visible !== 'boolean') {
          p.visible = true
        }
      }
      return parsed ?? {}
    }
  } catch {
    // ignore malformed storage
  }
  return {}
}

function newSessionId(): string {
  return window.crypto?.randomUUID?.() ?? `term-${Date.now()}-${Math.random().toString(36).slice(2)}`
}

export function TerminalPanel({ className }: { className?: string }) {
  const project = getProject() ?? ''
  const [byProject, setByProject] = useState<ProjectTermMap>(loadProjectMap)
  const [maximized, setMaximized] = useState(false)
  const [height, setHeight] = useState(320)
  const idRef = useRef(0)
  const dragRef = useRef<{ startY: number; startHeight: number } | null>(null)
  const isMobile = useMediaQuery('(max-width: 1023px)')
  const [keyboard, setKeyboard] = useState<{ top: number; height: number } | null>(null)

  const state: ProjectTermState = byProject[project] ?? EMPTY
  const { tabs, activeId, collapsed, visible } = state

  // Persist the per-project terminal state (survives a browser reload).
  useEffect(() => {
    try {
      window.localStorage.setItem(STORAGE_KEY, JSON.stringify(byProject))
    } catch {
      // ignore quota / serialization errors
    }
  }, [byProject])

  const patchProject = useCallback(
    (patch: (s: ProjectTermState) => ProjectTermState) => {
      setByProject((prev) => {
        const cur = prev[project] ?? EMPTY
        return { ...prev, [project]: patch(cur) }
      })
    },
    [project],
  )

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
      setKeyboard(open ? { top: vv.offsetTop, height: vv.height } : null)
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

  const addTerminal = useCallback(
    (title?: string, command?: string) => {
      idRef.current += 1
      const id = idRef.current
      const tab: TerminalTabData = {
        id,
        title: title ?? `Terminal ${id}`,
        command,
        sessionId: newSessionId(),
      }
      patchProject((s) => ({ ...s, tabs: [...s.tabs, tab], activeId: id, collapsed: false, visible: true }))
    },
    [patchProject],
  )

  useEffect(() => {
    return subscribeNavActions((action) => {
      if (action === 'open-terminal') {
        // Show/hide the entire terminal panel. If there are no terminals at all
        // for this project, create a new one (and show it).
        setByProject((prev) => {
          const cur = prev[project] ?? EMPTY
          if (cur.tabs.length === 0) {
            idRef.current += 1
            const id = idRef.current
            return {
              ...prev,
              [project]: {
                tabs: [{ id, title: `Terminal ${id}`, sessionId: newSessionId() }],
                activeId: id,
                collapsed: false,
                visible: true,
              },
            }
          }
          return { ...prev, [project]: { ...cur, visible: !cur.visible } }
        })
      } else if (action === 'open-chat-opencode') {
        addTerminal('OpenCode', 'opencode')
      } else if (action === 'open-chat-codex') {
        addTerminal('Codex', 'codex')
      }
    })
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [project, addTerminal])

  const closeTerminal = useCallback(
    (id: number) => {
      const tab = tabs.find((t) => t.id === id)
      if (tab?.sessionId) {
        void api.destroyTerminal(tab.sessionId).catch(() => undefined)
      }
      patchProject((s) => {
        const next = s.tabs.filter((t) => t.id !== id)
        return {
          tabs: next,
          activeId: next.length === 0 ? null : s.activeId === id ? next[next.length - 1].id : s.activeId,
          collapsed: next.length === 0 ? false : s.collapsed,
          visible: next.length === 0 ? false : s.visible,
        }
      })
    },
    [tabs, patchProject],
  )

  const toggleMaximize = useCallback(() => {
    patchProject((s) => ({ ...s, collapsed: false, visible: true }))
    setMaximized((m) => !m)
  }, [patchProject])

  const activateTerminal = useCallback(
    (id: number) => patchProject((s) => ({ ...s, activeId: id })),
    [patchProject],
  )

  if (tabs.length === 0 || activeId === null || !visible) return null

  const fullscreen = isMobile || maximized

  const content = (
    <div
      className={cn(
        'flex flex-col',
        fullscreen && 'fixed inset-0 z-50 h-svh bg-background',
        className,
      )}
      style={isMobile && keyboard && !maximized ? { top: keyboard.top, left: 0, width: '100%', height: keyboard.height } : undefined}
    >
      <div
        data-testid="terminal-resize-handle"
        className="h-0.5 shrink-0 cursor-row-resize bg-border/50 transition-colors hover:bg-border hover:h-1 max-lg:hidden"
        onMouseDown={handleDragStart}
        onTouchStart={handleDragStart}
      />
      <div
        className={cn(
          'flex flex-col max-lg:min-h-0 max-lg:flex-1',
          !fullscreen ? 'shrink-0 bg-card' : 'min-h-0 flex-1',
        )}
        style={!fullscreen ? { height } : undefined}
      >
        <TerminalTabs
          tabs={tabs}
          activeId={activeId}
          maximized={maximized}
          collapsed={collapsed}
          onAdd={() => addTerminal()}
          onClose={closeTerminal}
          onActivate={activateTerminal}
          onToggleMaximize={toggleMaximize}
          onToggleVisible={() => patchProject((s) => ({ ...s, visible: !s.visible }))}
        />
      </div>
    </div>
  )

  if (isMobile) {
    return createPortal(content, document.body)
  }
  return content
}
