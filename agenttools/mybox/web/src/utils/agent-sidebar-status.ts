import type { HerdrAgent, HerdrOverview } from '../api/client'

export const STATUS_PRIORITY = ['blocked', 'input', 'done', 'working', 'idle', 'unknown'] as const

export function normalizeAgentStatus(status: string): string {
  switch (status.trim().toLowerCase().replace(/[\s_-]/g, '')) {
    case 'working':
    case 'active':
    case 'inprogress':
    case 'running':
      return 'working'
    case 'input':
    case 'waitingonuserinput':
    case 'needsinput':
      return 'input'
    case 'blocked':
    case 'waitingonapproval':
    case 'needsapproval':
    case 'approvalrequired':
    case 'systemerror':
    case 'failed':
    case 'error':
      return 'blocked'
    case 'idle':
      return 'idle'
    case 'done':
    case 'completed':
    case 'complete':
      return 'done'
    default:
      return 'unknown'
  }
}

export function highestPriorityAgentStatus(statuses: readonly string[]): string {
  const normalized = new Set(statuses.map(normalizeAgentStatus))
  return STATUS_PRIORITY.find((status) => normalized.has(status)) ?? 'unknown'
}

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

export function summarizeAgentStatuses(agents: readonly Pick<HerdrAgent, 'status'>[], codexStatus?: string | null): AgentStatusSummary {
  const codexDisplayStatus = codexStatus ? normalizeAgentStatus(codexStatus) : null
  if (agents.length === 0 && !codexDisplayStatus) return { status: 'unknown', label: 'Agents: no agents' }

  const counts = new Map<string, number>()
  for (const agent of agents) {
    const status = normalizeAgentStatus(agent.status)
    counts.set(status, (counts.get(status) ?? 0) + 1)
  }

  const status = highestPriorityAgentStatus([
    ...[...counts.keys()],
    ...(codexDisplayStatus ? [codexDisplayStatus] : []),
  ])
  const orderedStatuses = [
    ...STATUS_PRIORITY,
    ...[...counts.keys()].filter((candidate) => !(STATUS_PRIORITY as readonly string[]).includes(candidate)).sort(),
  ]
  const details = orderedStatuses
    .filter((candidate) => counts.has(candidate))
    .map((candidate) => `${counts.get(candidate)} ${candidate}`)
    .join(', ')

  const label = agents.length > 0 ? `Agents: ${details}` : 'Agents: no agents'
  return { status, label: codexDisplayStatus ? `${label}; Codex: ${codexDisplayStatus}` : label }
}
