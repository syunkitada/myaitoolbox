import { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react'
import type { ReactNode } from 'react'
import type { ExplorerState, NodeId } from '../graph/types'
import { ancestorsOf, isDirId } from '../graph/nodeId'

interface ExplorerStateValue extends ExplorerState {
  toggleNode: (id: NodeId) => void
  expandNode: (id: NodeId) => void
  collapseNode: (id: NodeId) => void
  selectNode: (id: NodeId) => void
  revealNode: (id: NodeId) => void
  expandDirs: (ids: Iterable<NodeId>) => void
  collapseDirs: (ids: Iterable<NodeId>) => void
}

const ExplorerStateContext = createContext<ExplorerStateValue | null>(null)

const GRAPH_STATE_STORAGE_KEY = 'mybox_graph_explorer_state'

interface PersistedProjectState {
  expanded: string[]
  selected: string | null
}

function loadGraphState(project: string): PersistedProjectState | null {
  try {
    const raw = window.localStorage.getItem(GRAPH_STATE_STORAGE_KEY)
    if (!raw) return null
    const map = JSON.parse(raw) as Record<string, PersistedProjectState>
    const entry = map[project]
    if (!entry || !Array.isArray(entry.expanded)) return null
    return entry
  } catch {
    return null
  }
}

function saveGraphState(project: string, state: PersistedProjectState): void {
  try {
    const raw = window.localStorage.getItem(GRAPH_STATE_STORAGE_KEY)
    const map = raw ? (JSON.parse(raw) as Record<string, PersistedProjectState>) : {}
    map[project] = state
    window.localStorage.setItem(GRAPH_STATE_STORAGE_KEY, JSON.stringify(map))
  } catch {
    // ignore
  }
}

export function ExplorerStateProvider({ children, project }: { children: ReactNode; project?: string }) {
  const initial = useMemo(() => (project ? loadGraphState(project) : null), [project])
  const [expandedNodeIds, setExpandedNodeIds] = useState<Set<NodeId>>(
    () => new Set(initial?.expanded ?? []),
  )
  const [selectedNodeId, setSelectedNodeId] = useState<NodeId | null>(initial?.selected ?? null)

  const toggleNode = useCallback((id: NodeId) => {
    if (!isDirId(id)) return
    setExpandedNodeIds((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }, [])

  const expandNode = useCallback((id: NodeId) => {
    if (!isDirId(id)) return
    setExpandedNodeIds((prev) => {
      if (prev.has(id)) return prev
      const next = new Set(prev)
      next.add(id)
      return next
    })
  }, [])

  const collapseNode = useCallback((id: NodeId) => {
    if (!isDirId(id)) return
    setExpandedNodeIds((prev) => {
      if (!prev.has(id)) return prev
      const next = new Set(prev)
      next.delete(id)
      return next
    })
  }, [])

  const selectNode = useCallback((id: NodeId) => {
    setSelectedNodeId(id)
  }, [])

  const expandDirs = useCallback((ids: Iterable<NodeId>) => {
    setExpandedNodeIds((prev) => {
      const next = new Set(prev)
      let changed = false
      for (const id of ids) {
        if (!isDirId(id)) continue
        if (!next.has(id)) {
          next.add(id)
          changed = true
        }
      }
      return changed ? next : prev
    })
  }, [])

  const collapseDirs = useCallback((ids: Iterable<NodeId>) => {
    setExpandedNodeIds((prev) => {
      const next = new Set(prev)
      let changed = false
      for (const id of ids) {
        if (!isDirId(id)) continue
        if (next.delete(id)) changed = true
      }
      return changed ? next : prev
    })
  }, [])

  const revealNode = useCallback((id: NodeId) => {
    setExpandedNodeIds((prev) => {
      const next = new Set(prev)
      for (const ancestor of ancestorsOf(id)) next.add(ancestor)
      return next
    })
    setSelectedNodeId(id)
  }, [])

  useEffect(() => {
    if (!project) return
    saveGraphState(project, { expanded: [...expandedNodeIds], selected: selectedNodeId })
  }, [project, expandedNodeIds, selectedNodeId])

  const value = useMemo<ExplorerStateValue>(
    () => ({
      expandedNodeIds,
      selectedNodeId,
      toggleNode,
      expandNode,
      collapseNode,
      selectNode,
      revealNode,
      expandDirs,
      collapseDirs,
    }),
    [
      expandedNodeIds,
      selectedNodeId,
      toggleNode,
      expandNode,
      collapseNode,
      selectNode,
      revealNode,
      expandDirs,
      collapseDirs,
    ],
  )

  return <ExplorerStateContext.Provider value={value}>{children}</ExplorerStateContext.Provider>
}

export function useExplorerState(): ExplorerStateValue {
  const ctx = useContext(ExplorerStateContext)
  if (!ctx) throw new Error('useExplorerState must be used within ExplorerStateProvider')
  return ctx
}