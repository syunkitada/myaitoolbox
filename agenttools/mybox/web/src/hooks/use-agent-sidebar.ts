import { useCallback, useEffect, useState } from 'react'
import {
  isAgentOutputDisplayMode,
  type AgentOutputDisplayMode,
} from '../utils/agent-output-display'

export interface AgentSidebarProjectState {
  open: boolean
  mobileOpen: boolean
  paneId: string | null
  displayMode: AgentOutputDisplayMode
  drafts: Record<string, string>
}

const STORAGE_KEY = 'mybox:agent-sidebar-state'
const LEGACY_OPEN_AGENT_STORAGE_KEY = 'mybox:herdr-open-agent'
const DEFAULT_STATE: AgentSidebarProjectState = {
  open: true,
  mobileOpen: false,
  paneId: null,
  displayMode: 'auto',
  drafts: {},
}

function readDrafts(value: unknown): Record<string, string> {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return {}
  return Object.fromEntries(
    Object.entries(value as Record<string, unknown>)
      .filter((entry): entry is [string, string] => typeof entry[1] === 'string'),
  )
}

function readRecord(key: string): Record<string, unknown> {
  if (typeof window === 'undefined') return {}
  try {
    const raw = window.localStorage.getItem(key)
    if (!raw) return {}
    const parsed = JSON.parse(raw) as unknown
    return parsed && typeof parsed === 'object' && !Array.isArray(parsed)
      ? (parsed as Record<string, unknown>)
      : {}
  } catch {
    return {}
  }
}

function loadStates(): Record<string, AgentSidebarProjectState> {
  const stored = readRecord(STORAGE_KEY)
  const legacy = readRecord(LEGACY_OPEN_AGENT_STORAGE_KEY)
  const states: Record<string, AgentSidebarProjectState> = {}

  for (const [project, value] of Object.entries(stored)) {
    if (!value || typeof value !== 'object' || Array.isArray(value)) continue
    const item = value as Record<string, unknown>
    states[project] = {
      open: item.open !== false,
      mobileOpen: item.mobileOpen === true,
      paneId: typeof item.paneId === 'string' ? item.paneId : null,
      displayMode: isAgentOutputDisplayMode(item.displayMode) ? item.displayMode : DEFAULT_STATE.displayMode,
      drafts: readDrafts(item.drafts),
    }
  }

  for (const [project, value] of Object.entries(legacy)) {
    if (states[project] || typeof value !== 'string') continue
    states[project] = { ...DEFAULT_STATE, paneId: value, drafts: {} }
  }

  return states
}

export function useAgentSidebarState(project: string, isMobile = false) {
  const [states, setStates] = useState<Record<string, AgentSidebarProjectState>>(loadStates)
  const state = states[project] ?? DEFAULT_STATE

  useEffect(() => {
    try {
      window.localStorage.setItem(STORAGE_KEY, JSON.stringify(states))
    } catch {
      // Ignore unavailable or full local storage.
    }
  }, [states])

  const patch = useCallback(
    (next: Partial<AgentSidebarProjectState>) => {
      setStates((current) => ({
        ...current,
        [project]: {
          ...(current[project] ?? DEFAULT_STATE),
          ...next,
        },
      }))
    },
    [project],
  )

  const setOpen = useCallback(
    (open: boolean) => patch(isMobile ? { mobileOpen: open } : { open }),
    [isMobile, patch],
  )
  const setPaneId = useCallback((paneId: string | null) => patch({ paneId }), [patch])
  const setDraft = useCallback((paneId: string, draft: string) => {
    setStates((current) => {
      const projectState = current[project] ?? DEFAULT_STATE
      const drafts = { ...projectState.drafts }
      if (draft === '') delete drafts[paneId]
      else drafts[paneId] = draft
      return {
        ...current,
        [project]: {
          ...projectState,
          drafts,
        },
      }
    })
  }, [project])
  const setDisplayMode = useCallback((displayMode: AgentOutputDisplayMode) => patch({ displayMode }), [patch])

  return {
    ...state,
    open: isMobile ? state.mobileOpen : state.open,
    drafts: state.drafts,
    setOpen,
    setPaneId,
    setDraft,
    setDisplayMode,
  }
}
