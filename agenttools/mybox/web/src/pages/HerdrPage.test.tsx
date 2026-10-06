import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { HerdrPage } from './HerdrPage'
import { api } from '../api/client'
import { DialogsProvider } from '../components/AppDialogs'

vi.mock('../api/client', () => ({
  api: {
    readHerdrAgent: vi.fn().mockResolvedValue({ output: 'hello' }),
    focusHerdrAgent: vi.fn().mockResolvedValue({ ok: true }),
    getHerdrLayouts: vi.fn().mockResolvedValue({ layouts: [] }),
    sendKeysHerdrAgent: vi.fn().mockResolvedValue({ ok: true }),
    sendKeysHerdrPane: vi.fn().mockResolvedValue({ ok: true }),
    sendTextHerdrPane: vi.fn().mockResolvedValue({ ok: true }),
    listFiles: vi.fn().mockResolvedValue([]),
    moveFile: vi.fn().mockResolvedValue(undefined),
    renameHerdrAgent: vi.fn().mockResolvedValue({ ok: true }),
    renameHerdrTab: vi.fn().mockResolvedValue({ ok: true }),
    promptHerdrAgent: vi.fn().mockResolvedValue({ ok: true }),
    listHerdrScheduledPrompts: vi.fn().mockResolvedValue([]),
    createHerdrScheduledPrompt: vi.fn().mockImplementation((target: string, text: string, scheduled_at: string) =>
      Promise.resolve({ id: 'scheduled-1', target, text, scheduled_at })),
    deleteHerdrScheduledPrompt: vi.fn().mockResolvedValue(undefined),
  },
}))

const overview = {
  available: true,
  workspaces: [
    {
      workspace_id: 'w1',
      label: 'demo',
      number: 1,
      agent_status: 'idle',
      focused: true,
      tab_count: 1,
      pane_count: 1,
    },
  ],
  agents: [
    {
      name: 'myagent',
      status: 'idle',
      workspace_id: 'w1',
      cwd: '/proj',
      focused: false,
      pane_id: 'w1:p1',
    },
  ],
  tabs: [{ tab_id: 'w1:t1', workspace_id: 'w1', label: '1', number: 1, agent_status: 'idle' }],
  panes: [{ pane_id: 'w1:p1', tab_id: 'w1:t1', workspace_id: 'w1', cwd: '/proj', agent_status: 'idle' }],
}

const linkedOverview = {
  ...overview,
  agents: [{ ...overview.agents[0], name: 'f20260919_foo', custom_name: 'f20260919_foo' }],
  tabs: [{ ...overview.tabs[0], label: '20260919_foo' }],
}

class TestResizeObserver {
  readonly callback: ResizeObserverCallback
  target?: Element

  constructor(callback: ResizeObserverCallback) {
    this.callback = callback
    resizeObservers.push(this)
  }

  observe(target: Element) {
    this.target = target
  }

  unobserve() {}

  disconnect() {}
}

const resizeObservers: TestResizeObserver[] = []

function mockMatchMedia() {
  Object.defineProperty(window, 'matchMedia', {
    writable: true,
    value: vi.fn().mockImplementation((query: string) => ({
      matches: false,
      media: query,
      onchange: null,
      addListener: vi.fn(),
      removeListener: vi.fn(),
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      dispatchEvent: vi.fn(),
    })),
  })
}

describe('HerdrPage agent commands', () => {
  beforeEach(() => {
    mockMatchMedia()
    vi.stubGlobal('ResizeObserver', TestResizeObserver)
    window.history.pushState({}, '', '/projects/demo/herdr')
  })

  afterEach(() => {
    window.history.replaceState({}, '', '/')
    localStorage.clear()
    resizeObservers.length = 0
    vi.unstubAllGlobals()
    vi.clearAllMocks()
    vi.useRealTimers()
  })

  it('keeps the workspace tabs and panes panel collapsed by default', () => {
    render(
      <MemoryRouter initialEntries={['/projects/demo/herdr']}>
        <DialogsProvider>
          <HerdrPage
            overview={overview}
            error={null}
            loading={false}
            refresh={() => Promise.resolve()}
          />
        </DialogsProvider>
      </MemoryRouter>,
    )

    expect(screen.getByTestId('herdr-workspaces-toggle')).toHaveAttribute('data-state', 'closed')
  })

  it('keeps the pane input readable on mobile and blurs it after sending', async () => {
    render(
      <MemoryRouter initialEntries={['/projects/demo/herdr']}>
        <DialogsProvider>
          <HerdrPage
            overview={overview}
            error={null}
            loading={false}
            refresh={() => Promise.resolve()}
          />
        </DialogsProvider>
      </MemoryRouter>,
    )

    fireEvent.click(screen.getByTestId('herdr-workspaces-toggle'))
    const input = screen.getByRole('textbox', { name: 'Input pane w1:p1' })
    expect(input).toHaveClass('text-base', 'md:text-xs')

    fireEvent.change(input, { target: { value: 'send this text' } })
    input.focus()
    fireEvent.click(screen.getByRole('button', { name: /^Send$/ }))

    await waitFor(() => expect(api.sendTextHerdrPane).toHaveBeenCalledWith('w1:p1', 'send this text'))
    expect(input).not.toHaveFocus()
  })

  it('renames a linked task directory, agent, and tab together', async () => {
    render(
      <MemoryRouter initialEntries={['/projects/demo/herdr']}>
        <DialogsProvider>
          <HerdrPage
            overview={linkedOverview}
            error={null}
            loading={false}
            refresh={() => Promise.resolve()}
          />
        </DialogsProvider>
      </MemoryRouter>,
    )

    fireEvent.click(screen.getByTestId('herdr-workspaces-toggle'))
    fireEvent.click(await screen.findByRole('button', { name: 'Rename tab w1:t1' }))
    const input = await screen.findByTestId('app-dialog-input')
    fireEvent.change(input, { target: { value: '20260920_bar' } })
    fireEvent.click(screen.getByTestId('app-dialog-ok'))

    await waitFor(() => {
      expect(api.moveFile).toHaveBeenCalledWith('_tasks/20260919_foo', '_tasks/20260920_bar')
      expect(api.renameHerdrAgent).toHaveBeenCalledWith('w1:p1', 'f20260920_bar')
      expect(api.renameHerdrTab).toHaveBeenCalledWith('w1:t1', '20260920_bar')
    })
  })

})
