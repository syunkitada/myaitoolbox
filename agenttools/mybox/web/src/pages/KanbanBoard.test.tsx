import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, within, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { KanbanBoard } from './KanbanBoard'
import { DialogsProvider } from '../components/AppDialogs'
import { api, HerdrOverview, Task } from '../api/client'

vi.mock('../api/client', () => ({
  api: {
    listTasks: vi.fn(),
    getHerdrOverview: vi.fn(),
    updateTask: vi.fn(),
    listFiles: vi.fn(),
    archiveTask: vi.fn(),
    deleteTask: vi.fn(),
  },
}))

function renderBoard(herdrOverview?: HerdrOverview | null, path = '/projects/test') {
  window.history.replaceState({}, '', path)
  return render(
    <MemoryRouter>
      <DialogsProvider>
        <KanbanBoard herdrOverview={herdrOverview} />
      </DialogsProvider>
    </MemoryRouter>,
  )
}

const tasks: Task[] = [
  { id: 't1', title: 'Todo item', status: 'todo', priority: 'high' },
  { id: 't2', title: 'Doing item', status: 'doing', priority: 'medium' },
  { id: 't3', title: 'Done item', status: 'done', priority: 'low' },
  { id: 't4', title: 'Archived item', status: 'done', priority: 'low', archived: true },
]

describe('KanbanBoard', () => {
  beforeEach(() => {
    vi.mocked(api.listTasks).mockResolvedValue(tasks)
    vi.mocked(api.getHerdrOverview).mockResolvedValue({
      available: false,
      workspaces: [],
      agents: [],
      tabs: [],
      panes: [],
    })
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('renders all status columns', async () => {
    renderBoard()
    expect(await screen.findByText('Todo item')).toBeInTheDocument()
    for (const s of ['todo', 'doing', 'blocked', 'review', 'done']) {
      expect(screen.getByText(s)).toBeInTheDocument()
    }
  })

  it('places each task in its status column and hides archived', async () => {
    renderBoard()
    await screen.findByText('Todo item')
    const todoCol = screen.getByTestId('column-todo')
    expect(within(todoCol).getByText('Todo item')).toBeInTheDocument()
    expect(within(todoCol).queryByText('Doing item')).not.toBeInTheDocument()

    const doingCol = screen.getByTestId('column-doing')
    expect(within(doingCol).getByText('Doing item')).toBeInTheDocument()

    const doneCol = screen.getByTestId('column-done')
    expect(within(doneCol).getByText('Done item')).toBeInTheDocument()
    expect(within(doneCol).queryByText('Archived item')).not.toBeInTheDocument()
  })

  it('shows task checklist progress on each task card', async () => {
    vi.mocked(api.listTasks).mockResolvedValue([
      {
        id: 'progress',
        title: 'Progress item',
        status: 'todo',
        priority: 'medium',
        body: '- [ ] first\n- [x] second\n- [X] third',
      },
    ])

    renderBoard()
    const card = (await screen.findByText('Progress item')).closest('.board-card') as HTMLElement

    expect(within(card).getByTestId('task-progress')).toHaveTextContent('2/3')
  })

  it('shows the linked agent status on a task card', async () => {
    const linkedTask: Task = {
      id: '20260929_linked-agent',
      title: 'Linked agent task',
      status: 'doing',
      priority: 'high',
      agent_kind: 'opencode',
    }
    vi.mocked(api.listTasks).mockResolvedValue([linkedTask])

    renderBoard({
      available: true,
      workspaces: [],
      tabs: [],
      panes: [],
      agents: [
        {
          name: 'f20260929_linked-agent',
          status: 'working',
          workspace_id: 'w1',
          pane_id: 'w1:p1',
        },
      ],
    })

    const card = (await screen.findByText('Linked agent task')).closest('.board-card') as HTMLElement
    expect(within(card).getByTestId('task-agent-status')).toHaveTextContent('working')
  })

  it('does not show an agent status when no agent is linked to the task', async () => {
    const taskWithoutAgent: Task = {
      id: '20260929_unlinked-agent',
      title: 'Unlinked agent task',
      status: 'todo',
      priority: 'medium',
    }
    vi.mocked(api.listTasks).mockResolvedValue([taskWithoutAgent])

    renderBoard({
      available: true,
      workspaces: [],
      tabs: [],
      panes: [],
      agents: [
        {
          name: 'f20260929_different-task',
          status: 'working',
          workspace_id: 'w1',
          pane_id: 'w1:p1',
        },
      ],
    })

    const card = (await screen.findByText('Unlinked agent task')).closest('.board-card') as HTMLElement
    expect(within(card).queryByTestId('task-agent-status')).not.toBeInTheDocument()
  })

  it('shows linked agent status on the cross-project board', async () => {
    const linkedTask: Task = {
      id: '20260929_cross-project-agent',
      title: 'Cross-project agent task',
      status: 'doing',
      priority: 'high',
      project: 'other-project',
    }
    vi.mocked(api.listTasks).mockResolvedValue([linkedTask])
    vi.mocked(api.getHerdrOverview).mockResolvedValue({
      available: true,
      workspaces: [],
      tabs: [],
      panes: [],
      agents: [
        {
          name: 'f20260929_cross-project-agent',
          status: 'blocked',
          workspace_id: 'w2',
          pane_id: 'w2:p1',
        },
      ],
    })

    renderBoard(null, '/')

    const card = (await screen.findByText('Cross-project agent task')).closest('.board-card') as HTMLElement
    await waitFor(() => expect(within(card).getByTestId('task-agent-status')).toHaveTextContent('blocked'))
    expect(api.getHerdrOverview).toHaveBeenCalledWith('other-project')
  })

  it('dims a task only until its pending date, while keeping expired metadata visible', async () => {
    vi.setSystemTime(new Date(2026, 8, 27, 12))
    vi.mocked(api.listTasks).mockResolvedValue([
      { id: 'future', title: 'Future pending item', status: 'todo', priority: 'medium', pending_until: '20260928' },
      { id: 'today', title: 'Today pending item', status: 'todo', priority: 'medium', pending_until: '20260927' },
      { id: 'expired', title: 'Expired pending item', status: 'todo', priority: 'medium', pending_until: '20260926' },
      {
        id: 'reason-only',
        title: 'Reason-only pending item',
        status: 'todo',
        priority: 'medium',
        pending_reason: 'Waiting for review',
      },
    ])

    renderBoard()
    await screen.findByText('Future pending item')

    expect(screen.getByText('Future pending item').closest('.board-card')).toHaveClass('pending')
    expect(screen.getByText('Today pending item').closest('.board-card')).toHaveClass('pending')
    expect(screen.getByText('Expired pending item').closest('.board-card')).not.toHaveClass('pending')
    expect(screen.getByText('Reason-only pending item').closest('.board-card')).toHaveClass('pending')
    expect(screen.getByText('20260926')).toBeInTheDocument()
  })

  it('archives a task after a simple confirmation when tmp has no files', async () => {
    window.history.replaceState({}, '', '/projects/test')
    vi.mocked(api.listFiles).mockResolvedValue([])
    const archive = vi.mocked(api.archiveTask).mockResolvedValue(undefined)
    renderBoard()
    await screen.findByText('Todo item')

    const card = screen.getByText('Todo item').closest('.board-card') as HTMLElement
    fireEvent.click(within(card).getByRole('button', { name: '' }))
    fireEvent.click(within(card).getByRole('button', { name: 'Archive' }))

    const dialog = await screen.findByTestId('app-dialog')
    expect(within(dialog).queryByText(/tmp ディレクトリ/)).not.toBeInTheDocument()
    fireEvent.click(within(dialog).getByTestId('app-dialog-ok'))

    await waitFor(() => expect(archive).toHaveBeenCalledWith('t1'))
  })

  it('warns about tmp deletion before archiving when a task has a tmp dir', async () => {
    window.history.replaceState({}, '', '/projects/test')
    vi.mocked(api.listFiles).mockResolvedValue([
      { path: '_tasks/t1/tmp/agent.log', name: 'agent.log', kind: 'file' },
      { path: '_tasks/t1/tmp/out', name: 'out', kind: 'file' },
    ])
    const archive = vi.mocked(api.archiveTask).mockResolvedValue(undefined)
    renderBoard()
    await screen.findByText('Todo item')

    const card = screen.getByText('Todo item').closest('.board-card') as HTMLElement
    fireEvent.click(within(card).getByRole('button', { name: '' }))
    fireEvent.click(within(card).getByRole('button', { name: 'Archive' }))

    const dialog = await screen.findByTestId('app-dialog')
    expect(within(dialog).getByText(/tmp ディレクトリ（2件: agent\.log, out）/)).toBeInTheDocument()
    fireEvent.click(within(dialog).getByTestId('app-dialog-ok'))

    await waitFor(() => expect(archive).toHaveBeenCalledWith('t1'))
  })

  it('does not archive when tmp deletion is declined', async () => {
    window.history.replaceState({}, '', '/projects/test')
    vi.mocked(api.listFiles).mockResolvedValue([
      { path: '_tasks/t1/tmp/agent.log', name: 'agent.log', kind: 'file' },
    ])
    const archive = vi.mocked(api.archiveTask).mockResolvedValue(undefined)
    renderBoard()
    await screen.findByText('Todo item')

    const card = screen.getByText('Todo item').closest('.board-card') as HTMLElement
    fireEvent.click(within(card).getByRole('button', { name: '' }))
    fireEvent.click(within(card).getByRole('button', { name: 'Archive' }))

    const dialog = await screen.findByTestId('app-dialog')
    fireEvent.click(within(dialog).getByTestId('app-dialog-cancel'))

    expect(archive).not.toHaveBeenCalled()
  })

  it('deletes a task after confirmation', async () => {
    window.history.replaceState({}, '', '/projects/test')
    const del = vi.mocked(api.deleteTask).mockResolvedValue(undefined)
    renderBoard()
    await screen.findByText('Todo item')

    const card = screen.getByText('Todo item').closest('.board-card') as HTMLElement
    fireEvent.click(within(card).getByRole('button', { name: '' }))
    fireEvent.click(within(card).getByRole('button', { name: 'Delete' }))

    const dialog = await screen.findByTestId('app-dialog')
    fireEvent.click(within(dialog).getByTestId('app-dialog-ok'))

    await waitFor(() => expect(del).toHaveBeenCalledWith('t1'))
  })
})
