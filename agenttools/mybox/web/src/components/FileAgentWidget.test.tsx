import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { FileAgentWidget } from './FileAgentWidget'
import { api } from '../api/client'
import type { HerdrOverview } from '../api/client'

vi.mock('../api/client', () => ({
  api: {
    closeHerdrPane: vi.fn().mockResolvedValue({ ok: true }),
    closeHerdrTab: vi.fn().mockResolvedValue({ ok: true }),
    readHerdrAgent: vi.fn().mockResolvedValue({ output: 'hello' }),
    focusHerdrAgent: vi.fn().mockResolvedValue({ ok: true }),
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
  tabs: [{ tab_id: 'w1:t1', workspace_id: 'w1', label: '1', pane_count: 1 }],
  panes: [{ pane_id: 'w1:p1', tab_id: 'w1:t1', workspace_id: 'w1', agent_status: 'idle' }],
}

describe('FileAgentWidget commands', () => {
  it('stops the agent, removes its pane, and removes the empty tab', async () => {
    vi.clearAllMocks()
    const onRefresh = vi.fn()
    render(
      <FileAgentWidget path="_tasks/20260919_foo/task.md" overview={runningOverview} onRefresh={onRefresh} />,
    )

    fireEvent.click(await screen.findByRole('button', { name: 'Stop and remove panel for f20260919_foo' }))
    await waitFor(() => {
      expect(api.sendKeysHerdrAgent).toHaveBeenCalledWith('w1:p1', ['C-c', 'C-c'])
      expect(api.closeHerdrPane).toHaveBeenCalledWith('w1:p1')
      expect(api.closeHerdrTab).toHaveBeenCalledWith('w1:t1')
      expect(onRefresh).toHaveBeenCalled()
    })
  })

  it('keeps the tab when another pane remains in it', async () => {
    vi.clearAllMocks()
    const overviewWithSiblingPane: HerdrOverview = {
      ...runningOverview,
      tabs: [{ ...runningOverview.tabs[0], pane_count: 2 }],
      panes: [
        ...runningOverview.panes,
        { pane_id: 'w1:p2', tab_id: 'w1:t1', workspace_id: 'w1', agent_status: 'idle' },
      ],
    }
    render(
      <FileAgentWidget
        path="_tasks/20260919_foo/task.md"
        overview={overviewWithSiblingPane}
        onRefresh={() => undefined}
      />,
    )

    fireEvent.click(await screen.findByRole('button', { name: 'Stop and remove panel for f20260919_foo' }))
    await waitFor(() => {
      expect(api.closeHerdrPane).toHaveBeenCalledWith('w1:p1')
    })
    expect(api.closeHerdrTab).not.toHaveBeenCalled()
  })

  it('reports WebUI focus separately from herdr focus', async () => {
    const onWebuiFocusChange = vi.fn()
    render(
      <FileAgentWidget
        path="_tasks/20260919_foo/task.md"
        overview={runningOverview}
        onRefresh={() => undefined}
        webuiFocusedPaneId="w1:p1"
        onWebuiFocusChange={onWebuiFocusChange}
      />,
    )

    expect(await screen.findByTestId('webui-focus-indicator')).toHaveTextContent('mybox focused')
    await waitFor(() => {
      expect(onWebuiFocusChange).toHaveBeenCalledWith('w1:p1')
    })

    fireEvent.click(screen.getByRole('button', { name: 'Collapse agent for 20260919_foo' }))
    await waitFor(() => {
      expect(onWebuiFocusChange).toHaveBeenCalledWith(null)
    })
  })

  it('focuses the herdr agent when its panel is open', async () => {
    render(
      <FileAgentWidget path="_tasks/20260919_foo/task.md" overview={runningOverview} onRefresh={() => undefined} />,
    )

    await waitFor(() => {
      expect(api.focusHerdrAgent).toHaveBeenCalledWith('w1:p1')
    })
  })

  it('sends a quick prompt to the running agent when its button is pressed', async () => {
    render(
      <FileAgentWidget path="_tasks/20260919_foo/task.md" overview={runningOverview} onRefresh={() => undefined} />,
    )
    fireEvent.click(await screen.findByRole('button', { name: '次は何をするとよいですか？' }))
    await waitFor(() => {
      expect(api.promptHerdrAgent).toHaveBeenCalledWith('w1:p1', '次は何をするとよいですか？')
    })
  })

  it('renders the command buttons for a running agent', async () => {
    render(
      <FileAgentWidget path="_tasks/20260919_foo/task.md" overview={runningOverview} onRefresh={() => undefined} />,
    )
    expect(screen.queryByRole('button', { name: '/init' })).not.toBeInTheDocument()
    for (const cmd of ['/new', '/compact', '/help', '/resume', '/plan', '/status', '進めて', '次は何をするとよいですか？']) {
      expect(await screen.findByRole('button', { name: cmd })).toBeInTheDocument()
    }
  })

  it('allows the terminal output and prompt form to be resized vertically', async () => {
    render(
      <FileAgentWidget path="_tasks/20260919_foo/task.md" overview={runningOverview} onRefresh={() => undefined} />,
    )

    expect(await screen.findByTestId('file-agent-output')).toHaveClass('resize-y')
    expect(screen.getByTestId('file-agent-prompt-input')).toHaveClass('resize-y')
  })
})
