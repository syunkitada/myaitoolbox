import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, fireEvent, render, screen } from '@testing-library/react'
import { type ReactNode } from 'react'
import { MemoryRouter } from 'react-router-dom'
import App from './App'
import { api } from './api/client'

vi.mock('./api/client', () => ({
  api: {
    getMeta: vi.fn(),
    getProjectGitStatus: vi.fn(),
  },
}))

vi.mock('./components/AppDialogs', () => ({
  DialogsProvider: ({ children }: { children: ReactNode }) => <>{children}</>,
  useDialogs: () => ({ prompt: vi.fn() }),
}))

vi.mock('./components/Sidebar', () => ({
  AppSidebar: ({
    gitStatus,
    myboxFocusedPaneId,
  }: {
    gitStatus: Record<string, { dirty: boolean }>
    myboxFocusedPaneId?: string | null
  }) => (
    <>
      <div data-testid="app-sidebar-git-status">{gitStatus.demo?.dirty ? 'dirty' : 'clean'}</div>
      <div data-testid="app-sidebar-focus">{myboxFocusedPaneId ?? 'none'}</div>
    </>
  ),
}))

vi.mock('./components/TerminalPanel', () => ({
  TerminalPanel: () => null,
}))

vi.mock('./components/AgentSidebar', () => ({
  AGENT_SIDEBAR_COLLAPSED_WIDTH: 56,
  AGENT_SIDEBAR_MAX_WIDTH: 960,
  agentSidebarWidthStyle: (width: number) => `min(${width}px, calc(100vw - 48px))`,
  AGENT_SIDEBAR_MIN_WIDTH: 280,
  AGENT_SIDEBAR_WIDTH_STORAGE_KEY: 'mybox_agent_sidebar_width',
  DEFAULT_AGENT_SIDEBAR_WIDTH: 400,
  AgentSidebar: ({
    onFocusChange,
    open,
    openAgentPaneId,
  }: {
    onFocusChange?: (paneId: string | null) => void
    open?: boolean
    openAgentPaneId?: string | null
  }) => (
    <>
      <button data-testid="agent-sidebar-focus" onClick={() => onFocusChange?.('w1:p2')}>sidebar focus</button>
      <button data-testid="agent-sidebar-blur" onClick={() => onFocusChange?.(null)}>sidebar blur</button>
      <div data-testid="agent-sidebar-open-state">{open ? 'open' : 'closed'}</div>
      <div data-testid="agent-sidebar-selected-pane">{openAgentPaneId ?? 'none'}</div>
    </>
  ),
}))

vi.mock('./hooks/use-agent-favicon', () => ({
  useAgentFavicon: () => undefined,
}))

vi.mock('./hooks/use-herdr', () => ({
  useHerdrOverview: () => ({
    overview: null,
    error: null,
    loading: false,
    refresh: vi.fn(),
  }),
}))

vi.mock('./pages/Dashboard', () => ({
  Dashboard: ({
    onOpenAgentPane,
    openAgentPaneId,
  }: {
    onOpenAgentPane?: (paneId: string) => void
    openAgentPaneId?: string | null
  }) => (
    <>
      <div data-testid="dashboard-open-agent-pane">{openAgentPaneId ?? 'none'}</div>
      <button data-testid="open-agent-from-viewer" onClick={() => onOpenAgentPane?.('w1:p2')}>
        open agent from viewer
      </button>
    </>
  ),
}))
vi.mock('./pages/GitPage', () => ({ GitPage: () => null }))
vi.mock('./pages/HerdrPage', () => ({
  HerdrPage: () => <div data-testid="herdr-page" />,
}))
vi.mock('./pages/KanbanBoard', () => ({ KanbanBoard: () => null }))
vi.mock('./pages/KnowledgeGraphPage', () => ({ KnowledgeGraphPage: () => null }))
vi.mock('./pages/ProjectsPage', () => ({ ProjectsPage: () => null }))
vi.mock('./pages/StatsPage', () => ({ StatsPage: () => null }))

const meta = {
  project: 'demo',
  projects: ['demo'],
  default_project: 'demo',
  tags: [],
  favorites: [],
  recent_files: [],
}

describe('App project status refresh', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    window.history.pushState({}, '', '/projects/demo/dashboard')
    Object.defineProperty(window, 'matchMedia', {
      configurable: true,
      value: vi.fn().mockReturnValue({
        matches: false,
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
      }),
    })
    vi.mocked(api.getMeta).mockResolvedValue(meta)
    vi.mocked(api.getProjectGitStatus).mockResolvedValueOnce({
      demo: { dirty: false, modified: 0, staged: 0, untracked: 0 },
    })
    vi.mocked(api.getProjectGitStatus).mockResolvedValueOnce({
      demo: { dirty: true, modified: 1, staged: 0, untracked: 0 },
    })
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.clearAllMocks()
    window.history.replaceState({}, '', '/')
  })

  it('refreshes project git status automatically', async () => {
    render(
      <MemoryRouter initialEntries={['/projects/demo/dashboard']}>
        <App />
      </MemoryRouter>,
    )

    expect(api.getProjectGitStatus).toHaveBeenCalledTimes(1)
    expect(screen.getByTestId('app-sidebar-git-status')).toHaveTextContent('clean')

    await act(async () => {
      vi.advanceTimersByTime(5000)
    })

    expect(api.getProjectGitStatus).toHaveBeenCalledTimes(2)
    expect(screen.getByTestId('app-sidebar-git-status')).toHaveTextContent('dirty')
  })

  it('uses the selected agent sidebar pane as the mybox focus', () => {
    render(
      <MemoryRouter initialEntries={['/projects/demo/dashboard']}>
        <App />
      </MemoryRouter>,
    )

    fireEvent.click(screen.getByTestId('agent-sidebar-focus'))
    expect(screen.getByTestId('app-sidebar-focus')).toHaveTextContent('w1:p2')

    fireEvent.click(screen.getByTestId('agent-sidebar-blur'))
    expect(screen.getByTestId('app-sidebar-focus')).toHaveTextContent('none')
  })

  it('opens and selects an agent when the Files viewer requests it', () => {
    render(
      <MemoryRouter initialEntries={['/projects/demo/dashboard']}>
        <App />
      </MemoryRouter>,
    )

    fireEvent.click(screen.getByTestId('open-agent-from-viewer'))

    expect(screen.getByTestId('agent-sidebar-open-state')).toHaveTextContent('open')
    expect(screen.getByTestId('agent-sidebar-selected-pane')).toHaveTextContent('w1:p2')
    expect(screen.getByTestId('dashboard-open-agent-pane')).toHaveTextContent('w1:p2')
  })

  it('uses the saved agent sidebar width before the first layout render', () => {
    localStorage.removeItem('mybox_sidebar_width')
    localStorage.setItem('mybox_agent_sidebar_width', '560')

    render(
      <MemoryRouter initialEntries={['/projects/demo/dashboard']}>
        <App />
      </MemoryRouter>,
    )

    const wrapper = document.querySelector('[data-slot="sidebar-wrapper"]')
    expect(wrapper).not.toBeNull()
    expect((wrapper as HTMLElement).style.getPropertyValue('--sidebar-right-width')).toBe(
      'min(560px, calc(100vw - 48px))',
    )
    expect((wrapper as HTMLElement).style.getPropertyValue('--sidebar-width')).toBe(
      'min(320px, calc(100% - var(--sidebar-right-width, 0px)))',
    )
  })

  it('does not reserve sidebar width when a closed agent sidebar uses the mobile layout', async () => {
    Object.defineProperty(window, 'innerWidth', { configurable: true, value: 390 })
    Object.defineProperty(window, 'matchMedia', {
      configurable: true,
      value: vi.fn().mockReturnValue({
        matches: true,
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
      }),
    })
    localStorage.setItem('mybox:agent-sidebar-state', JSON.stringify({ demo: { open: false } }))

    render(
      <MemoryRouter initialEntries={['/projects/demo/dashboard']}>
        <App />
      </MemoryRouter>,
    )

    const wrapper = document.querySelector('[data-slot="sidebar-wrapper"]') as HTMLElement
    await act(async () => undefined)
    expect(wrapper.style.getPropertyValue('--sidebar-right-width')).toBe('0px')
  })

  it('does not reserve sidebar width while the mobile agent sheet is open', async () => {
    Object.defineProperty(window, 'innerWidth', { configurable: true, value: 390 })
    Object.defineProperty(window, 'matchMedia', {
      configurable: true,
      value: vi.fn().mockReturnValue({
        matches: true,
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
      }),
    })

    render(
      <MemoryRouter initialEntries={['/projects/demo/dashboard']}>
        <App />
      </MemoryRouter>,
    )

    const wrapper = document.querySelector('[data-slot="sidebar-wrapper"]') as HTMLElement
    await act(async () => undefined)
    expect(wrapper.style.getPropertyValue('--sidebar-right-width')).toBe('0px')
  })
})
