import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { act, render, screen, fireEvent, waitFor, within } from '@testing-library/react'
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
    listFiles: vi.fn().mockResolvedValue([]),
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

function notifyResize(target: Element, height: number) {
  const entry = { target, contentRect: { height } } as ResizeObserverEntry
  for (const observer of resizeObservers) {
    if (observer.target === target) observer.callback([entry], {} as ResizeObserver)
  }
}

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

function localDateTimeValue(date: Date): string {
  const local = new Date(date.getTime() - date.getTimezoneOffset() * 60_000)
  return local.toISOString().slice(0, 16)
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
            webuiFocusedPaneId={null}
            onWebuiFocusChange={() => undefined}
          />
        </DialogsProvider>
      </MemoryRouter>,
    )

    expect(screen.getByTestId('herdr-workspaces-toggle')).toHaveAttribute('data-state', 'closed')
  })

  it('sends /new from the expanded agent panel and omits /init', async () => {
    const onWebuiFocusChange = vi.fn()
    render(
      <MemoryRouter initialEntries={['/projects/demo/herdr']}>
        <DialogsProvider>
          <HerdrPage
            overview={overview}
            error={null}
            loading={false}
            refresh={() => Promise.resolve()}
            webuiFocusedPaneId="w1:p1"
            onWebuiFocusChange={onWebuiFocusChange}
          />
        </DialogsProvider>
      </MemoryRouter>,
    )

    expect(screen.getByTestId('webui-focus-w1:p1')).toHaveTextContent('mybox focused')
    fireEvent.click(await screen.findByTestId('agent-row-w1:p1'))
    expect(onWebuiFocusChange).toHaveBeenCalledWith(null)
    await vi.waitFor(() => {
      expect(api.focusHerdrAgent).toHaveBeenCalledWith('w1:p1')
    })
    expect(screen.queryByRole('button', { name: '/init' })).not.toBeInTheDocument()
    for (const cmd of ['/new', '/compact', '/help', '/resume', '/plan', '/status', '進めて', '次は何をするとよいですか？', 'セルフレビューして', 'Commitして']) {
      expect(await screen.findByRole('button', { name: cmd })).toBeInTheDocument()
    }
    expect(screen.getByTestId('herdr-agent-output-w1:p1')).toHaveClass('resize-y')
    expect(screen.getByTestId('herdr-prompt-input')).toHaveClass('resize-y', 'w-full', 'sm:flex-1')
    expect(screen.getByTestId('herdr-prompt-actions')).toHaveClass('w-full', 'sm:w-auto')
    expect(screen.queryByRole('button', { name: 'Press PageDown on myagent' })).not.toBeInTheDocument()
    fireEvent.click(await screen.findByRole('button', { name: 'Press Esc on myagent' }))

    await vi.waitFor(() => {
      expect(api.sendKeysHerdrAgent).toHaveBeenCalledWith('w1:p1', ['esc'])
    })
    fireEvent.click(await screen.findByRole('button', { name: '/new' }))

    await vi.waitFor(() => {
      expect(api.promptHerdrAgent).toHaveBeenCalledWith('w1:p1', '/new')
    })

    fireEvent.click(await screen.findByRole('button', { name: '/status' }))
    await vi.waitFor(() => {
      expect(api.promptHerdrAgent).toHaveBeenCalledWith('w1:p1', '/status')
    })

    fireEvent.click(await screen.findByRole('button', { name: 'セルフレビューして' }))
    await vi.waitFor(() => {
      expect(api.promptHerdrAgent).toHaveBeenCalledWith('w1:p1', 'セルフレビューして')
    })

    fireEvent.click(await screen.findByRole('button', { name: 'Commitして' }))
    await vi.waitFor(() => {
      expect(api.promptHerdrAgent).toHaveBeenCalledWith('w1:p1', 'Commitして')
    })
  })

  it('keeps the prompt input full width on narrow screens', async () => {
    render(
      <MemoryRouter initialEntries={['/projects/demo/herdr']}>
        <DialogsProvider>
          <HerdrPage
            overview={overview}
            error={null}
            loading={false}
            refresh={() => Promise.resolve()}
            webuiFocusedPaneId={null}
            onWebuiFocusChange={() => undefined}
          />
        </DialogsProvider>
      </MemoryRouter>,
    )

    fireEvent.click(await screen.findByTestId('agent-row-w1:p1'))

    expect(screen.getByTestId('herdr-prompt-input')).toHaveClass('w-full', 'sm:flex-1')
    expect(screen.getByTestId('herdr-prompt-actions')).toHaveClass('w-full', 'sm:w-auto')
  })

  it('restores and persists the vertical sizes of the agent panel controls', async () => {
    localStorage.setItem('mybox:herdr-agent-output-height', '420')
    localStorage.setItem('mybox:herdr-agent-prompt-height', '96')

    render(
      <MemoryRouter initialEntries={['/projects/demo/herdr']}>
        <DialogsProvider>
          <HerdrPage
            overview={overview}
            error={null}
            loading={false}
            refresh={() => Promise.resolve()}
            webuiFocusedPaneId={null}
            onWebuiFocusChange={() => undefined}
          />
        </DialogsProvider>
      </MemoryRouter>,
    )

    fireEvent.click(await screen.findByTestId('agent-row-w1:p1'))
    const output = await screen.findByTestId('herdr-agent-output-w1:p1')
    const prompt = screen.getByTestId('herdr-prompt-input')
    expect(output).toHaveStyle({ height: '420px' })
    expect(prompt).toHaveStyle({ height: '96px' })

    await act(async () => {
      notifyResize(output, 444)
      notifyResize(prompt, 88)
    })

    await waitFor(() => {
      expect(localStorage.getItem('mybox:herdr-agent-output-height')).toBe('444')
      expect(localStorage.getItem('mybox:herdr-agent-prompt-height')).toBe('88')
    })
  })

  it('persists a prompt schedule on the server', async () => {
    render(
      <MemoryRouter initialEntries={['/projects/demo/herdr']}>
        <DialogsProvider>
          <HerdrPage
            overview={overview}
            error={null}
            loading={false}
            refresh={() => Promise.resolve()}
            webuiFocusedPaneId={null}
            onWebuiFocusChange={() => undefined}
          />
        </DialogsProvider>
      </MemoryRouter>,
    )

    fireEvent.click(screen.getByTestId('agent-row-w1:p1'))
    fireEvent.change(screen.getByTestId('herdr-prompt-input'), { target: { value: 'send later' } })
    fireEvent.change(screen.getByLabelText('Schedule send time'), {
      target: { value: localDateTimeValue(new Date(Date.now() + 120_000)) },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Schedule send' }))

    await waitFor(() => expect(screen.getByTestId('scheduled-prompt')).toHaveTextContent('send later'))
    expect(api.createHerdrScheduledPrompt).toHaveBeenCalledWith(
      'w1:p1',
      'send later',
      expect.any(String),
    )
    expect(api.promptHerdrAgent).not.toHaveBeenCalled()

    expect(api.promptHerdrAgent).not.toHaveBeenCalled()
  })

  it('cancels a scheduled prompt from the expanded agent panel', async () => {
    render(
      <MemoryRouter initialEntries={['/projects/demo/herdr']}>
        <DialogsProvider>
          <HerdrPage
            overview={overview}
            error={null}
            loading={false}
            refresh={() => Promise.resolve()}
            webuiFocusedPaneId={null}
            onWebuiFocusChange={() => undefined}
          />
        </DialogsProvider>
      </MemoryRouter>,
    )

    fireEvent.click(screen.getByTestId('agent-row-w1:p1'))
    fireEvent.change(screen.getByTestId('herdr-prompt-input'), { target: { value: 'cancel me' } })
    fireEvent.change(screen.getByLabelText('Schedule send time'), {
      target: { value: localDateTimeValue(new Date(Date.now() + 60 * 60_000)) },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Schedule send' }))
    const row = await screen.findByTestId('scheduled-prompt')
    fireEvent.click(within(row).getByRole('button', { name: 'Cancel scheduled prompt cancel me' }))

    await waitFor(() => expect(screen.queryByText(/cancel me/)).not.toBeInTheDocument())
    expect(api.deleteHerdrScheduledPrompt).toHaveBeenCalledWith('scheduled-1')
    expect(api.promptHerdrAgent).not.toHaveBeenCalled()
  })

  it('reports WebUI focus when the agent detail panel opens', async () => {
    const onWebuiFocusChange = vi.fn()
    render(
      <MemoryRouter initialEntries={['/projects/demo/herdr']}>
        <DialogsProvider>
          <HerdrPage
            overview={overview}
            error={null}
            loading={false}
            refresh={() => Promise.resolve()}
            webuiFocusedPaneId={null}
            onWebuiFocusChange={onWebuiFocusChange}
          />
        </DialogsProvider>
      </MemoryRouter>,
    )

    fireEvent.click(await screen.findByTestId('agent-row-w1:p1'))
    await vi.waitFor(() => {
      expect(onWebuiFocusChange).toHaveBeenCalledWith('w1:p1')
    })
  })

  it('restores the last open agent panel separately for each project', async () => {
    localStorage.clear()
    const otherOverview = {
      ...overview,
      workspaces: [{ ...overview.workspaces[0], workspace_id: 'w2', label: 'other' }],
      agents: [{ ...overview.agents[0], workspace_id: 'w2', pane_id: 'w2:p1' }],
      tabs: [{ ...overview.tabs[0], workspace_id: 'w2', tab_id: 'w2:t1' }],
      panes: [{ ...overview.panes[0], workspace_id: 'w2', tab_id: 'w2:t1', pane_id: 'w2:p1' }],
    }
    const renderHerdr = (project: string, pageOverview: typeof overview) => {
      window.history.pushState({}, '', `/projects/${project}/herdr`)
      return render(
        <MemoryRouter initialEntries={[`/projects/${project}/herdr`]}>
          <DialogsProvider>
            <HerdrPage
              overview={pageOverview}
              error={null}
              loading={false}
              refresh={() => Promise.resolve()}
              webuiFocusedPaneId={null}
              onWebuiFocusChange={() => undefined}
            />
          </DialogsProvider>
        </MemoryRouter>,
      )
    }

    const demoPage = renderHerdr('demo', overview)
    fireEvent.click(await screen.findByTestId('agent-row-w1:p1'))
    expect(await screen.findByTestId('agent-detail-w1:p1')).toBeInTheDocument()
    demoPage.unmount()

    const otherPage = renderHerdr('other', otherOverview)
    expect(screen.queryByTestId('agent-detail-w2:p1')).not.toBeInTheDocument()
    otherPage.unmount()

    renderHerdr('demo', overview)
    expect(await screen.findByTestId('agent-detail-w1:p1')).toBeInTheDocument()
  })
})
