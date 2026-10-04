export interface HerdrStatusAgent {
  pane_id: string
  status: string
}

export const HERDR_STATUS_MEMORY_STORAGE_KEY = 'mybox:herdr-status-memory'

export interface HerdrStatusMemory {
  previousStatuses: Map<string, string>
  workingPaneIds: Set<string>
  unseenDonePaneIds: Set<string>
  acknowledgedDonePaneIds: Set<string>
}

export function createHerdrStatusMemory(): HerdrStatusMemory {
  return {
    previousStatuses: new Map(),
    workingPaneIds: new Set(),
    unseenDonePaneIds: new Set(),
    acknowledgedDonePaneIds: new Set(),
  }
}

function stringSet(value: unknown): Set<string> {
  if (!Array.isArray(value)) return new Set()
  return new Set(value.filter((item): item is string => typeof item === 'string'))
}

export function loadHerdrStatusMemory(): HerdrStatusMemory {
  const memory = createHerdrStatusMemory()
  if (typeof window === 'undefined') return memory

  try {
    const raw = window.localStorage.getItem(HERDR_STATUS_MEMORY_STORAGE_KEY)
    if (!raw) return memory
    const parsed = JSON.parse(raw) as unknown
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) return memory
    const stored = parsed as Record<string, unknown>
    memory.workingPaneIds = stringSet(stored.workingPaneIds)
    memory.unseenDonePaneIds = stringSet(stored.unseenDonePaneIds)
    memory.acknowledgedDonePaneIds = stringSet(stored.acknowledgedDonePaneIds)
  } catch {
    // Ignore malformed or unavailable local storage.
  }
  return memory
}

export function persistHerdrStatusMemory(memory: HerdrStatusMemory): void {
  if (typeof window === 'undefined') return
  try {
    window.localStorage.setItem(HERDR_STATUS_MEMORY_STORAGE_KEY, JSON.stringify({
      workingPaneIds: [...memory.workingPaneIds],
      unseenDonePaneIds: [...memory.unseenDonePaneIds],
      acknowledgedDonePaneIds: [...memory.acknowledgedDonePaneIds],
    }))
  } catch {
    // Ignore unavailable or full local storage.
  }
}

// Remember that an agent was working so a subsequent `idle` response still
// represents a completion even when Herdr acknowledged `done` before Mybox's
// next poll. Mybox focus, rather than Herdr focus, is what acknowledges it.
export function updateHerdrStatusMemory(
  memory: HerdrStatusMemory,
  agents: readonly HerdrStatusAgent[],
): void {
  const currentPaneIds = new Set(agents.map((agent) => agent.pane_id))
  const rememberedPaneIds = new Set([
    ...memory.previousStatuses.keys(),
    ...memory.workingPaneIds,
    ...memory.unseenDonePaneIds,
    ...memory.acknowledgedDonePaneIds,
  ])
  for (const paneId of rememberedPaneIds) {
    if (!currentPaneIds.has(paneId)) {
      memory.previousStatuses.delete(paneId)
      memory.workingPaneIds.delete(paneId)
      memory.unseenDonePaneIds.delete(paneId)
      memory.acknowledgedDonePaneIds.delete(paneId)
    }
  }

  for (const agent of agents) {
    const paneId = agent.pane_id

    if (agent.status === 'working') {
      memory.workingPaneIds.add(paneId)
      memory.unseenDonePaneIds.delete(paneId)
      memory.acknowledgedDonePaneIds.delete(paneId)
    } else if (agent.status === 'blocked') {
      memory.workingPaneIds.delete(paneId)
      memory.unseenDonePaneIds.delete(paneId)
      memory.acknowledgedDonePaneIds.delete(paneId)
    } else if (agent.status === 'done') {
      if (!memory.acknowledgedDonePaneIds.has(paneId)) {
        memory.unseenDonePaneIds.add(paneId)
      }
    } else if (agent.status === 'idle') {
      if (memory.workingPaneIds.has(paneId) && !memory.acknowledgedDonePaneIds.has(paneId)) {
        memory.unseenDonePaneIds.add(paneId)
      }
    }

    memory.previousStatuses.set(paneId, agent.status)
  }
}

export function acknowledgeHerdrStatusMemory(
  memory: HerdrStatusMemory,
  agents: readonly HerdrStatusAgent[],
  myboxFocusedPaneId: string | null,
): void {
  if (!myboxFocusedPaneId) return
  const agent = agents.find((candidate) => candidate.pane_id === myboxFocusedPaneId)
  if (!agent) return
  if (agent.status !== 'done' && !memory.unseenDonePaneIds.has(myboxFocusedPaneId)) return

  memory.unseenDonePaneIds.delete(myboxFocusedPaneId)
  memory.acknowledgedDonePaneIds.add(myboxFocusedPaneId)
}

export function displayedHerdrAgentStatus(
  agent: HerdrStatusAgent,
  memory: HerdrStatusMemory,
  myboxFocusedPaneId: string | null,
): string {
  if (
    agent.status === 'done' &&
    (memory.acknowledgedDonePaneIds.has(agent.pane_id) || agent.pane_id === myboxFocusedPaneId)
  ) {
    return 'idle'
  }
  if (agent.status === 'idle' && memory.unseenDonePaneIds.has(agent.pane_id)) {
    return agent.pane_id === myboxFocusedPaneId ? 'idle' : 'done'
  }
  return agent.status
}
