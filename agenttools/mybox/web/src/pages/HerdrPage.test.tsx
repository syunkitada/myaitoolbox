import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { HerdrPage } from './HerdrPage'
import { api } from '../api/client'
import { DialogsProvider } from '../components/AppDialogs'

vi.mock('../api/client', () => ({
  api: {
    readHerdrAgent: vi.fn().mockResolvedValue({ output: 'hello' }),
    getHerdrLayouts: vi.fn().mockResolvedValue({ layouts: [] }),
    listFiles: vi.fn().mockResolvedValue([]),
    promptHerdrAgent: vi.fn().mockResolvedValue({ ok: true }),
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
    window.history.pushState({}, '', '/projects/demo/herdr')
  })

  afterEach(() => {
    window.history.replaceState({}, '', '/')
    vi.clearAllMocks()
  })

  it('sends /new to the agent from the expanded agent panel', async () => {
    render(
      <MemoryRouter initialEntries={['/projects/demo/herdr']}>
        <DialogsProvider>
          <HerdrPage overview={overview} error={null} loading={false} refresh={() => Promise.resolve()} />
        </DialogsProvider>
      </MemoryRouter>,
    )

    fireEvent.click(await screen.findByTestId('agent-row-w1:p1'))
    fireEvent.click(await screen.findByTestId('agent-command-w1:p1-new'))

    await vi.waitFor(() => {
      expect(api.promptHerdrAgent).toHaveBeenCalledWith('w1:p1', '/new')
    })
  })
})