import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, fireEvent, render, screen } from '@testing-library/react'
import type { ReactNode } from 'react'
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
}))

vi.mock('./components/Sidebar', () => ({
  AppSidebar: ({ gitStatus }: { gitStatus: Record<string, { dirty: boolean }> }) => (
    <div data-testid="app-sidebar-git-status">{gitStatus.demo?.dirty ? 'dirty' : 'clean'}</div>
  ),
}))

vi.mock('./components/TerminalPanel', () => ({
  TerminalPanel: () => null,
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
    webuiFocusedPaneId,
    onWebuiFocusChange,
  }: {
    webuiFocusedPaneId: string | null
    onWebuiFocusChange: (paneId: string | null) => void
  }) => (
    <button data-testid="dashboard-focus" onClick={() => onWebuiFocusChange('w1:p1')}>
      {webuiFocusedPaneId ?? 'none'}
    </button>
  ),
}))
vi.mock('./pages/GitPage', () => ({ GitPage: () => null }))
vi.mock('./pages/HerdrPage', () => ({
  HerdrPage: ({ webuiFocusedPaneId }: { webuiFocusedPaneId: string | null }) => (
    <div data-testid="herdr-focus">{webuiFocusedPaneId ?? 'none'}</div>
  ),
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

  it('clears WebUI focus when leaving the current page', async () => {
    render(
      <MemoryRouter initialEntries={['/projects/demo/dashboard']}>
        <App />
      </MemoryRouter>,
    )

    fireEvent.click(screen.getByTestId('dashboard-focus'))
    expect(screen.getByTestId('dashboard-focus')).toHaveTextContent('w1:p1')

    fireEvent.click(screen.getByRole('link', { name: 'Herdr' }))
    expect(screen.getByTestId('herdr-focus')).toHaveTextContent('none')
  })
})
