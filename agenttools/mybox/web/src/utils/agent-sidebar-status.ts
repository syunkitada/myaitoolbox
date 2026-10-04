import type { HerdrAgent, HerdrOverview } from '../api/client'

const STATUS_PRIORITY = ['blocked', 'done', 'working', 'idle', 'unknown']

export interface AgentStatusSummary {
  status: string
  label: string
}

export function projectAgentsFor(overview: HerdrOverview | null, project: string): HerdrAgent[] {
  if (!overview || !project) return []

  const workspaceIds = new Set(
    overview.workspaces
      .filter((workspace) => workspace.label === project)
      .map((workspace) => workspace.workspace_id),
  )
  return overview.agents.filter((agent) => workspaceIds.has(agent.workspace_id))
}

export function summarizeAgentStatuses(agents: readonly Pick<HerdrAgent, 'status'>[]): AgentStatusSummary {
  if (agents.length === 0) return { status: 'unknown', label: 'Agents: no agents' }

  const counts = new Map<string, number>()
  for (const agent of agents) counts.set(agent.status, (counts.get(agent.status) ?? 0) + 1)

  const status = STATUS_PRIORITY.find((candidate) => counts.has(candidate)) ?? 'unknown'
  const orderedStatuses = [
    ...STATUS_PRIORITY,
    ...[...counts.keys()].filter((candidate) => !STATUS_PRIORITY.includes(candidate)).sort(),
  ]
  const details = orderedStatuses
    .filter((candidate) => counts.has(candidate))
    .map((candidate) => `${counts.get(candidate)} ${candidate}`)
    .join(', ')

  return { status, label: `Agents: ${details}` }
}
