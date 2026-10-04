import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import type { ComponentProps } from 'react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { AgentSidebar } from './AgentSidebar'
import { DialogsProvider } from './AppDialogs'
import { api } from '../api/client'
import type { HerdrOverview } from '../api/client'

vi.mock('../api/client', () => ({
  api: {
    getHerdrLayouts: vi.fn().mockResolvedValue({ layouts: [] }),
    focusHerdrAgent: vi.fn().mockResolvedValue({ ok: true }),
    readHerdrAgent: vi.fn().mockResolvedValue({ output: 'hello' }),
    promptHerdrAgent: vi.fn().mockResolvedValue({ ok: true }),
    sendKeysHerdrAgent: vi.fn().mockResolvedValue({ ok: true }),
    listHerdrScheduledPrompts: vi.fn().mockResolvedValue([]),
    createHerdrScheduledPrompt: vi.fn().mockImplementation((target: string, text: string, scheduled_at: string) =>
      Promise.resolve({ id: 'scheduled-1', target, text, scheduled_at })),
    deleteHerdrScheduledPrompt: vi.fn().mockResolvedValue(undefined),
    renameHerdrAgent: vi.fn().mockResolvedValue({ ok: true }),
    listFiles: vi.fn().mockResolvedValue([]),
    startHerdrFileAgent: vi.fn().mockResolvedValue({ ok: true }),
    closeHerdrPane: vi.fn().mockResolvedValue({ ok: true }),
    closeHerdrTab: vi.fn().mockResolvedValue({ ok: true }),
  },
}))

const overview: HerdrOverview = {
  available: true,
  workspaces: [
    { workspace_id: 'w1', label: 'demo', agent_status: 'working' },
    { workspace_id: 'w2', label: 'other', agent_status: 'working' },
  ],
  agents: [
    { name: 'demo-agent', status: 'working', workspace_id: 'w1', pane_id: 'w1:p1' },
    { name: 'other-agent', status: 'working', workspace_id: 'w2', pane_id: 'w2:p1' },
  ],
  tabs: [],
  panes: [],
}

function mockMatchMedia(matches = false) {
  Object.defineProperty(window, 'matchMedia', {
    writable: true,
    value: vi.fn().mockImplementation((query: string) => ({
      matches,
      media: query,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      addListener: vi.fn(),
      removeListener: vi.fn(),
    })),
  })
}

function renderSidebar(
  options: Partial<ComponentProps<typeof AgentSidebar>> = {},
  initialEntries = ['/projects/demo/dashboard'],
) {
  return render(
    <MemoryRouter initialEntries={initialEntries}>
      <DialogsProvider>
        <AgentSidebar
          project="demo"
          overview={overview}
          error={null}
          loading={false}
          open={true}
          onOpenChange={vi.fn()}
          displayMode="auto"
          onDisplayModeChange={vi.fn()}
          openAgentPaneId={null}
          onOpenAgentChange={vi.fn()}
          refresh={() => Promise.resolve()}
          {...options}
        />
      </DialogsProvider>
    </MemoryRouter>,
  )
}

describe('AgentSidebar', () => {
  beforeEach(() => {
    mockMatchMedia()
    localStorage.clear()
    vi.clearAllMocks()
  })

  afterEach(() => {
    Object.defineProperty(window, 'innerWidth', { configurable: true, value: 1024 })
    window.history.replaceState({}, '', '/')
  })

  it('shows only agents belonging to the current project', () => {
    renderSidebar()

    expect(screen.getByTestId('agent-sidebar-agent-w1:p1')).toBeInTheDocument()
    expect(screen.queryByTestId('agent-sidebar-agent-w2:p1')).not.toBeInTheDocument()
  })

  it('does not render an internal close button', () => {
    renderSidebar()

    expect(screen.queryByRole('button', { name: 'Close agent sidebar' })).not.toBeInTheDocument()
  })

  it('allows the sidebar to be resized up to 960 pixels', () => {
    localStorage.setItem('mybox_agent_sidebar_width', '960')
    renderSidebar()

    const resizeHandle = screen.getByRole('separator', { name: 'Resize agent sidebar' })
    expect(resizeHandle).toHaveAttribute('aria-valuemax', '960')
    expect(resizeHandle).toHaveAttribute('aria-valuenow', '960')
  })

  it('reports and resizes the width that is visible in a narrow desktop viewport', () => {
    Object.defineProperty(window, 'innerWidth', { configurable: true, value: 768 })
    localStorage.setItem('mybox_agent_sidebar_width', '960')
    renderSidebar()

    const resizeHandle = screen.getByRole('separator', { name: 'Resize agent sidebar' })
    expect(resizeHandle).toHaveAttribute('aria-valuenow', '720')

    fireEvent.keyDown(resizeHandle, { key: 'ArrowRight' })
    expect(resizeHandle).toHaveAttribute('aria-valuenow', '710')
  })

  it('does not render the Sheet close button on mobile', async () => {
    Object.defineProperty(window, 'innerWidth', { configurable: true, value: 375 })
    mockMatchMedia(true)
    renderSidebar()

    expect(await screen.findByTestId('agent-sidebar-sheet')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /^Close$/ })).not.toBeInTheDocument()
  })

  it('keeps agent statuses visible when the sidebar is closed', () => {
    const onOpenChange = vi.fn()
    const onOpenAgentChange = vi.fn()
    renderSidebar({ open: false, onOpenChange, onOpenAgentChange })

    expect(screen.getByTestId('agent-sidebar-collapsed')).toBeInTheDocument()
    expect(screen.getByTestId('agent-sidebar-status-w1:p1')).toBeInTheDocument()
    expect(screen.queryByTestId('agent-sidebar-content')).not.toBeInTheDocument()

    fireEvent.click(screen.getByTestId('agent-sidebar-status-w1:p1'))
    expect(onOpenAgentChange).toHaveBeenCalledWith('w1:p1')
    expect(onOpenChange).toHaveBeenCalledWith(true)
  })

  it('starts the current Files task with codex and selects the returned agent', async () => {
    const taskAgent = {
      name: 'f20260919_foo',
      status: 'working',
      workspace_id: 'w1',
      pane_id: 'w1:p2',
    }
    const taskOverview: HerdrOverview = {
      ...overview,
      agents: [],
    }
    const refresh = vi.fn().mockResolvedValue(undefined)
    const onOpenAgentChange = vi.fn()
    vi.mocked(api.startHerdrFileAgent).mockResolvedValueOnce({ ok: true, agent: taskAgent })

    renderSidebar(
      { overview: taskOverview, refresh, onOpenAgentChange },
      ['/projects/demo/dashboard/files/_tasks/20260919_foo/task.md'],
    )

    expect(screen.getByLabelText('Task agent kind')).toHaveValue('codex')
    fireEvent.click(screen.getByRole('button', { name: 'Start agent for 20260919_foo' }))

    await waitFor(() => {
      expect(api.startHerdrFileAgent).toHaveBeenCalledWith('_tasks/20260919_foo/task.md', 'codex')
      expect(refresh).toHaveBeenCalled()
      expect(onOpenAgentChange).toHaveBeenCalledWith('w1:p2')
    })
  })

  it('stops an agent, closes its empty tab, and clears the selection', async () => {
    const taskAgentOverview: HerdrOverview = {
      ...overview,
      agents: [{ ...overview.agents[0], name: 'f20260919_foo' }],
      tabs: [{ tab_id: 'w1:t1', workspace_id: 'w1', label: '20260919_foo', pane_count: 1 }],
      panes: [{ pane_id: 'w1:p1', tab_id: 'w1:t1', workspace_id: 'w1', agent_status: 'working' }],
    }
    const refresh = vi.fn().mockResolvedValue(undefined)
    const onOpenAgentChange = vi.fn()

    renderSidebar({ overview: taskAgentOverview, refresh, openAgentPaneId: 'w1:p1', onOpenAgentChange })

    fireEvent.click(await screen.findByRole('button', { name: 'Stop agent f20260919_foo' }))

    await waitFor(() => {
      expect(api.sendKeysHerdrAgent).toHaveBeenCalledWith('w1:p1', ['C-c', 'C-c'])
      expect(api.closeHerdrPane).toHaveBeenCalledWith('w1:p1')
      expect(api.closeHerdrTab).toHaveBeenCalledWith('w1:t1')
      expect(refresh).toHaveBeenCalled()
      expect(onOpenAgentChange).toHaveBeenCalledWith(null)
    })
  })

  it('keeps a tab open when stopping one of its multiple panes', async () => {
    const taskAgentOverview: HerdrOverview = {
      ...overview,
      agents: [{ ...overview.agents[0], name: 'f20260919_foo' }],
      tabs: [{ tab_id: 'w1:t1', workspace_id: 'w1', label: '20260919_foo', pane_count: 2 }],
      panes: [
        { pane_id: 'w1:p1', tab_id: 'w1:t1', workspace_id: 'w1', agent_status: 'working' },
        { pane_id: 'w1:p2', tab_id: 'w1:t1', workspace_id: 'w1', agent_status: 'unknown' },
      ],
    }

    renderSidebar({ overview: taskAgentOverview, refresh: vi.fn().mockResolvedValue(undefined) })
    fireEvent.click(await screen.findByRole('button', { name: 'Stop agent f20260919_foo' }))

    await waitFor(() => expect(api.closeHerdrPane).toHaveBeenCalledWith('w1:p1'))
    expect(api.closeHerdrTab).not.toHaveBeenCalled()
  })

  it('does not render a closed sidebar strip on mobile', async () => {
    Object.defineProperty(window, 'innerWidth', { configurable: true, value: 375 })
    mockMatchMedia(true)
    renderSidebar({ open: false })

    await waitFor(() => {
      expect(screen.queryByTestId('agent-sidebar-collapsed')).not.toBeInTheDocument()
    })
    expect(screen.queryByTestId('agent-sidebar-content')).not.toBeInTheDocument()
  })

  it('opens the selected agent and exposes the Herdr agent controls', async () => {
    const onOpenAgentChange = vi.fn()
    const first = renderSidebar({ onOpenAgentChange })

    fireEvent.click(screen.getByTestId('agent-sidebar-row-w1:p1'))
    expect(onOpenAgentChange).toHaveBeenCalledWith('w1:p1')

    first.unmount()
    renderSidebar({ openAgentPaneId: 'w1:p1' })
    expect(await screen.findByTestId('herdr-agent-output-w1:p1')).toBeInTheDocument()
    expect(screen.getByTestId('agent-detail-w1:p1')).toHaveClass('-mx-2', 'px-1')
    expect(screen.getByTestId('herdr-agent-output-w1:p1')).toHaveClass('px-1')
    expect(screen.getByRole('combobox', { name: 'Agent output display mode' })).toHaveValue('auto')
    expect(screen.getByTestId('herdr-prompt-actions')).toHaveClass('w-full', 'flex-col')
    expect(screen.getByTestId('herdr-prompt-input')).toHaveClass('w-full', 'min-w-0', 'min-h-6', 'py-0', 'leading-5')
    expect(screen.getByTestId('herdr-prompt-input')).toHaveStyle({ height: '24px' })
    expect(screen.getByRole('button', { name: 'Send' })).toHaveClass('min-w-0', 'flex-1')
    expect(screen.getByRole('button', { name: '/status' })).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: '/status' }))
    await waitFor(() => expect(api.promptHerdrAgent).toHaveBeenCalledWith('w1:p1', '/status'))
  })

  it('keeps focus diagnostics out of the agent list', async () => {
    const focusedOverview: HerdrOverview = {
      ...overview,
      agents: overview.agents.map((agent) => agent.pane_id === 'w1:p1' ? { ...agent, focused: true } : agent),
    }
    renderSidebar({ overview: focusedOverview, openAgentPaneId: 'w1:p1' })

    expect(await screen.findByTestId('herdr-agent-output-w1:p1')).toBeInTheDocument()
    expect(screen.queryByText('· focused')).not.toBeInTheDocument()
    expect(screen.queryByText('· mybox focused')).not.toBeInTheDocument()
  })

  it('keeps one rename action for the selected agent', async () => {
    renderSidebar({ openAgentPaneId: 'w1:p1' })

    expect(await screen.findByTestId('herdr-agent-output-w1:p1')).toBeInTheDocument()
    expect(screen.getAllByRole('button', { name: 'Rename agent demo-agent' })).toHaveLength(1)
  })

  it('uses the header refresh to reload both agents and the selected output', async () => {
    const refresh = vi.fn().mockResolvedValue(undefined)
    renderSidebar({ openAgentPaneId: 'w1:p1', refresh })

    expect(await screen.findByTestId('herdr-agent-output-w1:p1')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Reload output' })).not.toBeInTheDocument()
    vi.mocked(api.readHerdrAgent).mockClear()
    refresh.mockClear()

    fireEvent.click(screen.getByRole('button', { name: 'Refresh agents' }))

    await waitFor(() => expect(refresh).toHaveBeenCalled())
    await waitFor(() => expect(api.readHerdrAgent).toHaveBeenCalledWith('w1:p1'))
  })

  it('auto-grows the prompt input on mobile as its content needs more lines', async () => {
    Object.defineProperty(window, 'innerWidth', { configurable: true, value: 390 })
    mockMatchMedia(true)
    renderSidebar({ openAgentPaneId: 'w1:p1' })

    const promptInput = await screen.findByTestId('herdr-prompt-input')
    expect(promptInput).toHaveStyle({ height: '48px' })

    Object.defineProperty(promptInput, 'scrollHeight', { configurable: true, value: 96 })
    fireEvent.change(promptInput, { target: { value: 'a prompt that wraps on a phone' } })
    await waitFor(() => expect(promptInput).toHaveStyle({ height: '96px' }))

    Object.defineProperty(promptInput, 'scrollHeight', { configurable: true, value: 48 })
    fireEvent.change(promptInput, { target: { value: '' } })
    await waitFor(() => expect(promptInput).toHaveStyle({ height: '48px' }))
  })

  it('reveals scheduling controls from the send action menu', async () => {
    const user = userEvent.setup()
    renderSidebar({ openAgentPaneId: 'w1:p1' })

    expect(await screen.findByTestId('herdr-prompt-input')).toBeInTheDocument()
    expect(screen.queryByLabelText('Schedule send time')).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /^Schedule send$/ })).not.toBeInTheDocument()

    fireEvent.change(screen.getByTestId('herdr-prompt-input'), { target: { value: 'send this later' } })
    await user.click(screen.getByRole('button', { name: 'More send actions' }))
    await user.click(await screen.findByRole('menuitem', { name: 'Schedule send' }))

    expect(screen.getByLabelText('Schedule send time')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /^Schedule send$/ })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Cancel scheduling' })).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Cancel scheduling' }))
    expect(screen.queryByLabelText('Schedule send time')).not.toBeInTheDocument()
  })

  it('condenses mobile agent controls into Enter and a command palette', async () => {
    Object.defineProperty(window, 'innerWidth', { configurable: true, value: 390 })
    mockMatchMedia(true)
    renderSidebar({ openAgentPaneId: 'w1:p1' })

    expect(await screen.findByRole('button', { name: 'Press Enter on demo-agent' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Press ↑ on demo-agent' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Press ↓ on demo-agent' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Press Esc on demo-agent' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '/status' })).not.toBeInTheDocument()

    const commandPaletteToggle = screen.getByRole('button', { name: 'Keys & commands' })
    expect(commandPaletteToggle).not.toHaveTextContent('Keys & commands')
    expect(commandPaletteToggle).toHaveAttribute('data-size', 'icon-sm')
    fireEvent.click(commandPaletteToggle)
    const palette = await screen.findByTestId('agent-command-palette')
    expect(palette).toBeInTheDocument()
    expect(screen.queryByRole('searchbox', { name: 'Filter keys and commands' })).not.toBeInTheDocument()
    const paletteKeys = screen.getByTestId('agent-command-palette-keys')
    expect(paletteKeys).toHaveClass('grid-cols-3', 'gap-1')
    const escapeButton = screen.getByRole('button', { name: 'Press Esc on demo-agent' })
    expect(escapeButton).toBeInTheDocument()
    expect(escapeButton).toHaveAttribute('data-size', 'xs')
    expect(escapeButton).toHaveClass('min-h-8', 'justify-center', 'px-2')
    expect(within(palette).queryByRole('button', { name: 'Press ↑ on demo-agent' })).not.toBeInTheDocument()
    expect(within(palette).queryByRole('button', { name: 'Press ↓ on demo-agent' })).not.toBeInTheDocument()
    const paletteCommands = screen.getByTestId('agent-command-palette-commands')
    expect(paletteCommands).toHaveClass('grid-cols-2', 'gap-1')
    const statusButton = screen.getByRole('button', { name: '/status' })
    expect(statusButton).toBeInTheDocument()
    expect(statusButton).toHaveAttribute('data-size', 'xs')
    expect(statusButton).toHaveClass('min-h-8', 'min-w-0', 'justify-start', 'px-2', 'truncate')

    fireEvent.click(screen.getByRole('button', { name: 'Press Esc on demo-agent' }))
    await waitFor(() => expect(api.sendKeysHerdrAgent).toHaveBeenCalledWith('w1:p1', ['esc']))
    await waitFor(() => expect(screen.queryByTestId('agent-command-palette')).not.toBeInTheDocument())
  })

  it('switches the selected agent output to Herdr formatting', async () => {
    const onDisplayModeChange = vi.fn()
    const first = renderSidebar({ openAgentPaneId: 'w1:p1', onDisplayModeChange })

    const select = await screen.findByRole('combobox', { name: 'Agent output display mode' })
    fireEvent.change(select, { target: { value: 'herdr' } })

    expect(onDisplayModeChange).toHaveBeenCalledWith('herdr')

    first.unmount()
    const herdrView = renderSidebar({ openAgentPaneId: 'w1:p1', displayMode: 'herdr' })
    expect(await screen.findByRole('combobox', { name: 'Agent output display mode' })).toHaveValue('herdr')
    expect(herdrView.container.querySelector('select')).toHaveValue('herdr')
    expect(screen.getByTestId('herdr-agent-output-w1:p1')).toHaveClass('max-w-full', 'overflow-x-hidden')
    expect(herdrView.container.querySelector('pre')).toHaveClass('whitespace-pre')
  })

  it('uses the agent URL deep link to open the sidebar selection', async () => {
    const onOpenChange = vi.fn()
    const onOpenAgentChange = vi.fn()
    render(
      <MemoryRouter initialEntries={['/projects/demo/herdr?agent=w1%3Ap1']}>
        <DialogsProvider>
          <AgentSidebar
            project="demo"
            overview={overview}
            error={null}
            loading={false}
            open={false}
            onOpenChange={onOpenChange}
            displayMode="auto"
            onDisplayModeChange={vi.fn()}
            openAgentPaneId={null}
            onOpenAgentChange={onOpenAgentChange}
            refresh={() => Promise.resolve()}
          />
        </DialogsProvider>
      </MemoryRouter>,
    )

    await waitFor(() => {
      expect(onOpenAgentChange).toHaveBeenCalledWith('w1:p1')
      expect(onOpenChange).toHaveBeenCalledWith(true)
    })
  })
})
