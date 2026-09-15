import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, within, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { KanbanBoard } from './KanbanBoard'
import { DialogsProvider } from '../components/AppDialogs'
import { api, Task } from '../api/client'

vi.mock('../api/client', () => ({
  api: {
    listTasks: vi.fn(),
    updateTask: vi.fn(),
    listFiles: vi.fn(),
    archiveTask: vi.fn(),
  },
}))

function renderBoard() {
  return render(
    <MemoryRouter>
      <DialogsProvider>
        <KanbanBoard />
      </DialogsProvider>
    </MemoryRouter>,
  )
}

const tasks: Task[] = [
  { id: 't1', title: 'Todo item', status: 'todo', priority: 'high' },
  { id: 't2', title: 'Doing item', status: 'doing', priority: 'medium' },
  { id: 't3', title: 'Done item', status: 'done', priority: 'low' },
  { id: 't4', title: 'Archived item', status: 'done', priority: 'low', archived: true },
  { id: 'adhoc1', title: 'Review PR', status: 'doing', priority: 'urgent', type: 'adhoc' },
]

describe('KanbanBoard', () => {
  beforeEach(() => {
    vi.mocked(api.listTasks).mockResolvedValue(tasks)
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

  it('marks adhoc tasks with an adhoc badge in their column', async () => {
    renderBoard()
    await screen.findByText('Review PR')
    const doingCol = screen.getByTestId('column-doing')
    expect(within(doingCol).getByText('Review PR')).toBeInTheDocument()
    expect(within(doingCol).getByText('adhoc')).toBeInTheDocument()
  })

  it('archives a regular task after a simple confirmation when tmp has no files', async () => {
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
      { path: 'tasks/t1/tmp/agent.log', name: 'agent.log', kind: 'file' },
      { path: 'tasks/t1/tmp/out', name: 'out', kind: 'file' },
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
      { path: 'tasks/t1/tmp/agent.log', name: 'agent.log', kind: 'file' },
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
})
