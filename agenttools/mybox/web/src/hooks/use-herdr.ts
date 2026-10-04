import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { api, HerdrOverview } from '../api/client'
import {
  acknowledgeHerdrStatusMemory,
  displayedHerdrAgentStatus,
  loadHerdrStatusMemory,
  persistHerdrStatusMemory,
  updateHerdrStatusMemory,
} from '../utils/herdr-status'

export interface HerdrState {
  overview: HerdrOverview | null
  error: string | null
  loading: boolean
}

export function useHerdrOverview(intervalMs = 5000, myboxFocusedPaneId: string | null = null) {
  const [state, setState] = useState<HerdrState>({ overview: null, error: null, loading: true })
  const seq = useRef(0)
  const statusMemory = useRef(loadHerdrStatusMemory())
  const focusedPaneIdRef = useRef(myboxFocusedPaneId)
  focusedPaneIdRef.current = myboxFocusedPaneId

  const refresh = useCallback(async () => {
    const mySeq = ++seq.current
    try {
      const overview = await api.getHerdrOverview()
      // Drop stale responses that resolve after a newer refresh was started.
      if (mySeq !== seq.current) return
      if (overview.available) {
        updateHerdrStatusMemory(statusMemory.current, overview.agents)
        acknowledgeHerdrStatusMemory(
          statusMemory.current,
          overview.agents,
          focusedPaneIdRef.current,
        )
        persistHerdrStatusMemory(statusMemory.current)
      }
      setState({ overview, error: null, loading: false })
    } catch (e) {
      if (mySeq !== seq.current) return
      setState((prev) => ({
        overview: prev.overview,
        error: e instanceof Error ? e.message : String(e),
        loading: false,
      }))
    }
  }, [])

  useEffect(() => {
    void refresh()
    const id = setInterval(() => {
      if (!document.hidden) void refresh()
    }, intervalMs)
    const onVisible = () => {
      if (!document.hidden) void refresh()
    }
    document.addEventListener('visibilitychange', onVisible)
    return () => {
      clearInterval(id)
      document.removeEventListener('visibilitychange', onVisible)
    }
  }, [refresh, intervalMs])

  useEffect(() => {
    if (!state.overview?.available) return
    acknowledgeHerdrStatusMemory(statusMemory.current, state.overview.agents, myboxFocusedPaneId)
    persistHerdrStatusMemory(statusMemory.current)
  }, [myboxFocusedPaneId, state.overview])

  const overview = useMemo(() => {
    if (!state.overview) return null
    const agents = state.overview.agents.map((agent) => ({
      ...agent,
      status: displayedHerdrAgentStatus(agent, statusMemory.current, myboxFocusedPaneId),
    }))
    const donePaneIds = new Set(
      agents.filter((agent) => agent.status === 'done').map((agent) => agent.pane_id),
    )
    const doneWorkspaceIds = new Set(
      agents.filter((agent) => agent.status === 'done').map((agent) => agent.workspace_id),
    )
    const doneTabIds = new Set(
      state.overview.panes
        .filter((pane) => donePaneIds.has(pane.pane_id))
        .map((pane) => pane.tab_id),
    )
    return {
      ...state.overview,
      agents,
      workspaces: state.overview.workspaces.map((workspace) => ({
        ...workspace,
        agent_status:
          workspace.agent_status === 'idle' && doneWorkspaceIds.has(workspace.workspace_id)
            ? 'done'
            : workspace.agent_status,
      })),
      tabs: state.overview.tabs.map((tab) => ({
        ...tab,
        agent_status:
          tab.agent_status === 'idle' && doneTabIds.has(tab.tab_id) ? 'done' : tab.agent_status,
      })),
      panes: state.overview.panes.map((pane) => ({
        ...pane,
        agent_status:
          pane.agent_status === 'idle' && donePaneIds.has(pane.pane_id) ? 'done' : pane.agent_status,
      })),
    }
  }, [myboxFocusedPaneId, state.overview])

  return { ...state, overview, refresh }
}
