import { describe, expect, it } from 'vitest'
import type { HerdrOverview } from '../api/client'
import { projectAgentsFor, summarizeAgentStatuses } from './agent-sidebar-status'

const overview: HerdrOverview = {
  available: true,
  workspaces: [
    { workspace_id: 'w1', label: 'demo', agent_status: 'working' },
    { workspace_id: 'w2', label: 'demo', agent_status: 'idle' },
    { workspace_id: 'w3', label: 'other', agent_status: 'blocked' },
  ],
  agents: [
    { name: 'working-agent', status: 'working', workspace_id: 'w1', pane_id: 'w1:p1' },
    { name: 'idle-agent', status: 'idle', workspace_id: 'w2', pane_id: 'w2:p1' },
    { name: 'other-agent', status: 'blocked', workspace_id: 'w3', pane_id: 'w3:p1' },
  ],
  tabs: [],
  panes: [],
}

describe('agent sidebar status', () => {
  it('filters agents by project workspace', () => {
    expect(projectAgentsFor(overview, 'demo').map((agent) => agent.pane_id)).toEqual(['w1:p1', 'w2:p1'])
  })

  it('summarizes statuses with the most important status first', () => {
    expect(summarizeAgentStatuses(projectAgentsFor(overview, 'demo'))).toEqual({
      status: 'working',
      label: 'Agents: 1 working, 1 idle',
    })
    expect(summarizeAgentStatuses([{ ...overview.agents[0], status: 'blocked' }])).toEqual({
      status: 'blocked',
      label: 'Agents: 1 blocked',
    })
  })

  it('keeps a completion notification visible over idle agents', () => {
    expect(summarizeAgentStatuses([
      { status: 'done' },
      { status: 'idle' },
    ])).toEqual({
      status: 'done',
      label: 'Agents: 1 done, 1 idle',
    })
  })

  it('keeps a completion notification visible over working agents', () => {
    expect(summarizeAgentStatuses([
      { status: 'done' },
      { status: 'working' },
    ])).toEqual({
      status: 'done',
      label: 'Agents: 1 done, 1 working',
    })
  })

  it('uses a neutral status when no project agents exist', () => {
    expect(summarizeAgentStatuses([])).toEqual({ status: 'unknown', label: 'Agents: no agents' })
  })
})
