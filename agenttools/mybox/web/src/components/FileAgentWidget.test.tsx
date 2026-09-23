import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { FileAgentWidget } from './FileAgentWidget'
import { api } from '../api/client'
import type { HerdrOverview } from '../api/client'

vi.mock('../api/client', () => ({
  api: {
    readHerdrAgent: vi.fn().mockResolvedValue({ output: 'hello' }),
    getHerdrLayouts: vi.fn().mockResolvedValue({ layouts: [] }),
    promptHerdrAgent: vi.fn().mockResolvedValue({ ok: true }),
    sendKeysHerdrAgent: vi.fn().mockResolvedValue({ ok: true }),
    startHerdrFileAgent: vi.fn().mockResolvedValue({ ok: true }),
  },
}))

const runningOverview: HerdrOverview = {
  available: true,
  workspaces: [],
  agents: [
    {
      name: 'f20260919_foo',
      status: 'idle',
      workspace_id: 'w1',
      cwd: '/proj',
      focused: false,
      pane_id: 'w1:p1',
    },
  ],
  tabs: [],
  panes: [],
}

describe('FileAgentWidget commands', () => {
  it('sends /new to the running agent when its button is pressed', async () => {
    render(
      <FileAgentWidget path="tasks/20260919_foo/task.md" overview={runningOverview} onRefresh={() => undefined} />,
    )
    fireEvent.click(await screen.findByRole('button', { name: '/new' }))
    await waitFor(() => {
      expect(api.promptHerdrAgent).toHaveBeenCalledWith('w1:p1', '/new')
    })
  })

  it('renders the command buttons for a running agent', async () => {
    render(
      <FileAgentWidget path="tasks/20260919_foo/task.md" overview={runningOverview} onRefresh={() => undefined} />,
    )
    for (const cmd of ['/new', '/init', '/compact', '/help']) {
      expect(await screen.findByRole('button', { name: cmd })).toBeInTheDocument()
    }
  })
})