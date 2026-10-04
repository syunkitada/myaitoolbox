import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter, useLocation } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { AppSidebar } from './Sidebar'
import { SidebarProvider } from './ui/sidebar'

const mocks = vi.hoisted(() => ({
  getStats: vi.fn(),
  listFiles: vi.fn(),
}))

vi.mock('../api/client', () => ({ api: mocks }))

function LocationProbe() {
  const location = useLocation()
  return <output data-testid="location">{location.pathname}{location.search}</output>
}

const meta = {
  project: 'demo',
  projects: ['demo'],
  default_project: 'demo',
  tags: [],
  favorites: [],
  recent_files: [],
}

const herdr = {
  available: true,
  workspaces: [{ workspace_id: 'w1', label: 'demo', agent_status: 'working' }],
  agents: [
    {
      name: 'f20260919_foo',
      custom_name: 'f20260919_foo',
      status: 'working',
      workspace_id: 'w1',
      cwd: '/proj',
      pane_id: 'w1:p1',
    },
    {
      name: 'generic',
      status: 'working',
      workspace_id: 'w1',
      cwd: '/proj',
      pane_id: 'w1:p2',
    },
  ],
  tabs: [{ tab_id: 'w1:t1', workspace_id: 'w1', label: '20260919_foo' }],
  panes: [{ pane_id: 'w1:p1', tab_id: 'w1:t1', workspace_id: 'w1', cwd: '/proj' }],
}

describe('AppSidebar agent navigation', () => {
  beforeEach(() => {
    Object.defineProperty(window, 'matchMedia', {
      configurable: true,
      value: vi.fn().mockImplementation((query: string) => ({
        matches: false,
        media: query,
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
      })),
    })
    mocks.getStats.mockResolvedValue({
      cpu: [],
      memory: { usage_percent: 0, used: 0, total: 0 },
      disks: [],
    })
  })

  afterEach(() => {
    vi.clearAllMocks()
    window.history.replaceState({}, '', '/')
  })

  it('does not let a slower file-agent lookup override a later agent click', async () => {
    let resolveList: ((entries: Array<{ path: string; name: string; kind: 'file' | 'dir' }>) => void) | undefined
    mocks.listFiles.mockImplementation(
      () => new Promise((resolve) => {
        resolveList = resolve
      }),
    )

    render(
      <MemoryRouter initialEntries={['/projects/demo/dashboard']}>
        <SidebarProvider>
          <AppSidebar meta={meta} project="demo" herdr={herdr} gitStatus={{}} />
          <LocationProbe />
        </SidebarProvider>
      </MemoryRouter>,
    )

    fireEvent.click(screen.getByTestId('sidebar-agent-w1:p1'))
    await waitFor(() => expect(mocks.listFiles).toHaveBeenCalledTimes(1))

    fireEvent.click(screen.getByTestId('sidebar-agent-w1:p2'))
    expect(screen.getByTestId('location')).toHaveTextContent('/projects/demo/herdr?agent=w1%3Ap2')

    await act(async () => {
      resolveList?.([{ path: '_tasks/20260919_foo/task.md', name: 'task.md', kind: 'file' }])
    })

    expect(screen.getByTestId('location')).toHaveTextContent('/projects/demo/herdr?agent=w1%3Ap2')
  })

  it('shows Mybox focus state in a collapsed debug section', () => {
    const focusedHerdr = {
      ...herdr,
      agents: herdr.agents.map((agent) => agent.pane_id === 'w1:p1' ? { ...agent, focused: true } : agent),
    }

    render(
      <MemoryRouter initialEntries={['/projects/demo/dashboard']}>
        <SidebarProvider>
          <AppSidebar
            meta={meta}
            project="demo"
            herdr={focusedHerdr}
            gitStatus={{}}
            myboxFocusedPaneId="w1:p1"
            agentSidebarOpen
          />
        </SidebarProvider>
      </MemoryRouter>,
    )

    expect(screen.queryByTestId('sidebar-focus-debug-content')).not.toBeInTheDocument()
    act(() => fireEvent.click(screen.getByTestId('sidebar-focus-debug-toggle')))

    expect(screen.getByTestId('sidebar-focus-debug-content')).toHaveTextContent('f20260919_foo')
    expect(screen.getByTestId('sidebar-focus-debug-content')).toHaveTextContent('Synchronized')
  })
})
