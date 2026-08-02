import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { TaskList } from './TaskList'
import { api, Task } from '../api/client'

vi.mock('../api/client', () => ({
  api: {
    listTasks: vi.fn(),
    createTask: vi.fn(),
  },
}))

const tasks: Task[] = [
  {
    id: 't1',
    title: 'Write tests',
    status: 'todo',
    priority: 'high',
    tags: ['web'],
  },
  {
    id: 't2',
    title: 'Ship UI',
    status: 'doing',
    priority: 'low',
    tags: ['ui'],
  },
]

describe('TaskList', () => {
  beforeEach(() => {
    vi.mocked(api.listTasks).mockResolvedValue(tasks)
  })

  it('renders tasks with status badges', async () => {
    render(
      <MemoryRouter>
        <TaskList />
      </MemoryRouter>,
    )
    expect(await screen.findByText('Write tests')).toBeInTheDocument()
    expect(screen.getByText('Ship UI')).toBeInTheDocument()
    expect(screen.getAllByText(/todo|doing/).length).toBeGreaterThan(0)
  })

  it('filters by tag', async () => {
    render(
      <MemoryRouter>
        <TaskList />
      </MemoryRouter>,
    )
    await screen.findByText('Write tests')
    const tagInput = screen.getByLabelText('filter by tag')
    fireEvent.change(tagInput, { target: { value: 'web' } })
    await waitFor(() => expect(api.listTasks).toHaveBeenCalledWith({ status: undefined, tag: 'web' }))
  })
})
