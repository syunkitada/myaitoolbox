import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor, within } from '@testing-library/react'
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
    listHerdrScheduledPrompts: vi.fn().mockResolvedValue([]),
    createHerdrScheduledPrompt: vi.fn().mockImplementation((target: string, text: string, scheduled_at: string) =>
      Promise.resolve({ id: 'scheduled-1', target, text, scheduled_at })),
    deleteHerdrScheduledPrompt: vi.fn().mockResolvedValue(undefined),
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

function localDateTimeValue(date: Date): string {
  const local = new Date(date.getTime() - date.getTimezoneOffset() * 60_000)
  return local.toISOString().slice(0, 16)
}

describe('FileAgentWidget commands', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    window.localStorage.clear()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

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

  it('sends PageDown to the running agent when its button is pressed', async () => {
    render(
      <FileAgentWidget path="_tasks/20260919_foo/task.md" overview={runningOverview} onRefresh={() => undefined} />,
    )

    fireEvent.click(await screen.findByTitle('Press PageDown'))
    await waitFor(() => {
      expect(api.sendKeysHerdrAgent).toHaveBeenCalledWith('w1:p1', ['PageDown'])
    })
  })

  it('allows the terminal output and prompt form to be resized vertically', async () => {
    render(
      <FileAgentWidget path="_tasks/20260919_foo/task.md" overview={runningOverview} onRefresh={() => undefined} />,
    )

    expect(await screen.findByTestId('file-agent-output')).toHaveClass('resize-y')
    expect(screen.getByTestId('file-agent-prompt-input')).toHaveClass('resize-y', 'w-full', 'sm:flex-1')
    expect(screen.getByTestId('file-agent-prompt-actions')).toHaveClass('w-full', 'sm:w-auto')
  })

  it('persists a prompt schedule on the server', async () => {
    const scheduledAt = new Date(Date.now() + 120_000)
    const localDateTime = localDateTimeValue(scheduledAt)
    render(
      <FileAgentWidget path="_tasks/20260919_foo/task.md" overview={runningOverview} onRefresh={() => undefined} />,
    )

    fireEvent.change(screen.getByTestId('file-agent-prompt-input'), { target: { value: 'send later' } })
    fireEvent.change(screen.getByLabelText('Schedule send time'), { target: { value: localDateTime } })
    fireEvent.click(screen.getByRole('button', { name: 'Schedule send' }))

    await waitFor(() => expect(screen.getByTestId('scheduled-prompt')).toHaveTextContent('send later'))
    expect(api.createHerdrScheduledPrompt).toHaveBeenCalledWith(
      'w1:p1',
      'send later',
      expect.any(String),
    )
    expect(api.promptHerdrAgent).not.toHaveBeenCalled()

    expect(api.promptHerdrAgent).not.toHaveBeenCalled()
    expect(screen.getByTestId('scheduled-prompt')).toHaveTextContent('send later')
  })

  it('cancels a scheduled prompt without sending it', async () => {
    const scheduledAt = localDateTimeValue(new Date(Date.now() + 60 * 60_000))
    render(
      <FileAgentWidget path="_tasks/20260919_foo/task.md" overview={runningOverview} onRefresh={() => undefined} />,
    )

    fireEvent.change(screen.getByTestId('file-agent-prompt-input'), { target: { value: 'cancel me' } })
    fireEvent.change(screen.getByLabelText('Schedule send time'), { target: { value: scheduledAt } })
    fireEvent.click(screen.getByRole('button', { name: 'Schedule send' }))
    const row = await screen.findByTestId('scheduled-prompt')
    fireEvent.click(within(row).getByRole('button', { name: 'Cancel scheduled prompt cancel me' }))

    await waitFor(() => expect(screen.queryByText(/cancel me/)).not.toBeInTheDocument())
    expect(api.deleteHerdrScheduledPrompt).toHaveBeenCalledWith('scheduled-1')
    expect(api.promptHerdrAgent).not.toHaveBeenCalled()
  })

  it('restores scheduled prompts after the widget is remounted', async () => {
    const scheduledAt = localDateTimeValue(new Date(Date.now() + 60 * 60_000))
    const first = render(
      <FileAgentWidget path="_tasks/20260919_foo/task.md" overview={runningOverview} onRefresh={() => undefined} />,
    )

    fireEvent.change(screen.getByTestId('file-agent-prompt-input'), { target: { value: 'restore me' } })
    fireEvent.change(screen.getByLabelText('Schedule send time'), { target: { value: scheduledAt } })
    fireEvent.click(screen.getByRole('button', { name: 'Schedule send' }))
    await waitFor(() => expect(screen.getByTestId('scheduled-prompt')).toHaveTextContent('restore me'))
    first.unmount()

    vi.mocked(api.listHerdrScheduledPrompts).mockResolvedValueOnce([
      {
        id: 'scheduled-1',
        target: 'f20260919_foo',
        text: 'restore me',
        scheduled_at: new Date(Date.now() + 60 * 60_000).toISOString(),
      },
    ])
    render(
      <FileAgentWidget path="_tasks/20260919_foo/task.md" overview={runningOverview} onRefresh={() => undefined} />,
    )
    expect(await screen.findByTestId('scheduled-prompt')).toHaveTextContent('restore me')
  })

  it('rejects a prompt scheduled in the past', async () => {
    const scheduledAt = localDateTimeValue(new Date(Date.now() - 60_000))
    render(
      <FileAgentWidget path="_tasks/20260919_foo/task.md" overview={runningOverview} onRefresh={() => undefined} />,
    )

    fireEvent.change(screen.getByTestId('file-agent-prompt-input'), { target: { value: 'too late' } })
    fireEvent.change(screen.getByLabelText('Schedule send time'), { target: { value: scheduledAt } })
    fireEvent.click(screen.getByRole('button', { name: 'Schedule send' }))

    expect(await screen.findByText('Schedule time must be in the future')).toBeInTheDocument()
    expect(api.promptHerdrAgent).not.toHaveBeenCalled()
  })
})
