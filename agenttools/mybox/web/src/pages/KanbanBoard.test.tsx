import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, within } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { KanbanBoard } from './KanbanBoard'
import { api, Task } from '../api/client'

vi.mock('../api/client', () => ({
  api: {
    listTasks: vi.fn(),
    updateTask: vi.fn(),
  },
}))

const tasks: Task[] = [
  { id: 't1', title: 'Todo item', status: 'todo', priority: 'high' },
  { id: 't2', title: 'Doing item', status: 'doing', priority: 'medium' },
  { id: 't3', title: 'Done item', status: 'done', priority: 'low' },
  { id: 't4', title: 'Archived item', status: 'done', priority: 'low', archived: true },
]

describe('KanbanBoard', () => {
  beforeEach(() => {
    vi.mocked(api.listTasks).mockResolvedValue(tasks)
  })

  it('renders all status columns', async () => {
    render(
      <MemoryRouter>
        <KanbanBoard />
      </MemoryRouter>,
    )
    expect(await screen.findByText('Todo item')).toBeInTheDocument()
    for (const s of ['todo', 'doing', 'blocked', 'review', 'done']) {
      expect(screen.getByText(s)).toBeInTheDocument()
    }
  })

  it('places each task in its status column and hides archived', async () => {
    render(
      <MemoryRouter>
        <KanbanBoard />
      </MemoryRouter>,
    )
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
})
