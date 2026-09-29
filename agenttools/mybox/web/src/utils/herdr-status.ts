export interface HerdrStatusAgent {
  pane_id: string
  status: string
}

export interface HerdrStatusMemory {
  previousStatuses: Map<string, string>
  unseenDonePaneIds: Set<string>
  acknowledgedDonePaneIds: Set<string>
}

export function createHerdrStatusMemory(): HerdrStatusMemory {
  return {
    previousStatuses: new Map(),
    unseenDonePaneIds: new Set(),
    acknowledgedDonePaneIds: new Set(),
  }
}

// Record completion notifications independently of Herdr's focused state.
// Herdr may turn an unread `done` state into `idle` when its pane is focused;
// mybox keeps that completion visible until the matching UI panel is focused.
export function updateHerdrStatusMemory(
  memory: HerdrStatusMemory,
  agents: readonly HerdrStatusAgent[],
  myboxFocusedPaneId: string | null,
): void {
  const currentPaneIds = new Set(agents.map((agent) => agent.pane_id))
  for (const paneId of memory.previousStatuses.keys()) {
    if (!currentPaneIds.has(paneId)) {
      memory.previousStatuses.delete(paneId)
      memory.unseenDonePaneIds.delete(paneId)
      memory.acknowledgedDonePaneIds.delete(paneId)
    }
  }

  for (const agent of agents) {
    const paneId = agent.pane_id
    const previousStatus = memory.previousStatuses.get(paneId)
    const isMyboxFocused = paneId === myboxFocusedPaneId

    if (agent.status === 'done') {
      if (isMyboxFocused) {
        memory.unseenDonePaneIds.delete(paneId)
        memory.acknowledgedDonePaneIds.add(paneId)
      } else if (previousStatus === 'done' && memory.acknowledgedDonePaneIds.has(paneId)) {
        // The user already saw this completion while the panel was focused.
        // Leaving the panel must not make the same completion unread again.
        memory.unseenDonePaneIds.delete(paneId)
      } else {
        memory.unseenDonePaneIds.add(paneId)
      }
    } else if (agent.status === 'idle') {
      if (isMyboxFocused) {
        memory.unseenDonePaneIds.delete(paneId)
        if (previousStatus === 'done') memory.acknowledgedDonePaneIds.add(paneId)
      } else if (previousStatus === 'done' && !memory.acknowledgedDonePaneIds.has(paneId)) {
        memory.unseenDonePaneIds.add(paneId)
      }
    } else if (agent.status === 'working' || agent.status === 'blocked') {
      memory.unseenDonePaneIds.delete(paneId)
      memory.acknowledgedDonePaneIds.delete(paneId)
    }

    memory.previousStatuses.set(paneId, agent.status)
  }
}

export function displayedHerdrAgentStatus(
  agent: HerdrStatusAgent,
  memory: HerdrStatusMemory,
  myboxFocusedPaneId: string | null,
): string {
  if (agent.pane_id === myboxFocusedPaneId) return agent.status
  if (agent.status === 'idle' && memory.unseenDonePaneIds.has(agent.pane_id)) return 'done'
  return agent.status
}
