import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { NewTaskDialog } from './NewTaskDialog'
import { api } from '../api/client'

vi.mock('../api/client', () => ({
  api: {
    createTask: vi.fn(),
    createTaskTrigger: vi.fn(),
    getTaskTemplate: vi.fn(),
    startTaskAgent: vi.fn(),
  },
}))

const DEFAULT_TASK_TEMPLATE = '---\ntitle: Task\nstatus: todo\n---\n\n## TODO\n\n- [ ]\n'
const DEFAULT_TASK_BODY = '\n## TODO\n\n- [ ]\n'

describe('NewTaskDialog', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(api.createTask).mockResolvedValue({
      id: '20260927_task',
      title: 'Task',
      status: 'todo',
      priority: 'medium',
    })
    vi.mocked(api.createTaskTrigger).mockResolvedValue({
      id: 'daily-report',
      type: 'cron',
      directory: '_task_triggers/daily-report',
      task_path: '_task_triggers/daily-report/task.md',
    })
    vi.mocked(api.getTaskTemplate).mockResolvedValue({ content: DEFAULT_TASK_TEMPLATE })
    vi.mocked(api.startTaskAgent).mockResolvedValue({ ok: true })
  })

  it('creates a cron trigger without creating or starting a normal task', async () => {
    const onTriggerCreated = vi.fn()
    render(
      <NewTaskDialog
        open
        onOpenChange={() => undefined}
        onTriggerCreated={onTriggerCreated}
      />
    )

    fireEvent.change(screen.getByLabelText('作成種別'), { target: { value: 'task_trigger' } })
    fireEvent.change(screen.getByLabelText('タスク名 (required)'), { target: { value: 'Daily report' } })
    await waitFor(() => expect(screen.getByLabelText(/task\.md本文/)).toHaveValue(DEFAULT_TASK_BODY))
    fireEvent.change(screen.getByLabelText(/Agent kind/), { target: { value: 'opencode' } })
    fireEvent.change(screen.getByLabelText(/Prompt/), { target: { value: 'do-the-task' } })
    fireEvent.click(screen.getByRole('button', { name: 'Create' }))

    await waitFor(() => expect(api.createTaskTrigger).toHaveBeenCalled())
    expect(api.createTaskTrigger).toHaveBeenCalledWith({
      id: 'daily-report',
      task: {
        name: 'Daily report',
        agent_kind: 'opencode',
        prompt: 'do-the-task',
        content: DEFAULT_TASK_TEMPLATE,
      },
      trigger: { type: 'cron', cron: '0 9 * * 1-5', timezone: 'Asia/Tokyo' },
    })
    expect(api.createTask).not.toHaveBeenCalled()
    expect(api.startTaskAgent).not.toHaveBeenCalled()
    expect(onTriggerCreated).toHaveBeenCalledWith({
      id: 'daily-report',
      type: 'cron',
      directory: '_task_triggers/daily-report',
      task_path: '_task_triggers/daily-report/task.md',
    })
  })

  it('keeps the existing normal task flow', async () => {
    render(<NewTaskDialog open onOpenChange={() => undefined} />)

    expect(screen.getByRole('option', { name: 'task' })).toBeInTheDocument()
    expect(screen.getByRole('option', { name: 'task_trigger' })).toBeInTheDocument()
    expect(screen.getByLabelText(/Agent kind/)).toHaveValue('codex')
    fireEvent.change(screen.getByLabelText('タスク名 (required)'), { target: { value: 'Task' } })
    await waitFor(() => expect(screen.getByLabelText(/task\.md本文/)).toHaveValue(DEFAULT_TASK_BODY))
    fireEvent.change(screen.getByLabelText(/task\.md本文/), { target: { value: 'custom task body' } })
    fireEvent.click(screen.getByRole('button', { name: 'Create' }))

    await waitFor(() => expect(api.createTask).toHaveBeenCalledWith({
      name: 'Task',
      agent_kind: 'codex',
      content: DEFAULT_TASK_TEMPLATE.replace(DEFAULT_TASK_BODY, 'custom task body'),
    }))
    expect(api.startTaskAgent).toHaveBeenCalledWith('20260927_task', { kind: 'codex', prompt: undefined })
    expect(api.createTaskTrigger).not.toHaveBeenCalled()
  })

  it('creates a file-created trigger with its watch settings', async () => {
    render(<NewTaskDialog open onOpenChange={() => undefined} />)

    fireEvent.change(screen.getByLabelText('作成種別'), { target: { value: 'task_trigger' } })
    fireEvent.change(screen.getByLabelText('タスク名 (required)'), { target: { value: 'Process files' } })
    await waitFor(() => expect(screen.getByLabelText(/task\.md本文/)).toHaveValue(DEFAULT_TASK_BODY))
    fireEvent.change(screen.getByLabelText('Trigger type'), { target: { value: 'file_created' } })
    fireEvent.change(screen.getByLabelText(/Watch directory/), { target: { value: 'incoming/reports' } })
    fireEvent.change(screen.getByLabelText(/Pattern/), { target: { value: '*.md' } })
    fireEvent.click(screen.getByRole('button', { name: 'Create' }))

    await waitFor(() => expect(api.createTaskTrigger).toHaveBeenCalled())
    expect(api.createTaskTrigger).toHaveBeenCalledWith({
      id: 'process-files',
      task: {
        name: 'Process files',
        agent_kind: 'codex',
        prompt: "'$task_file_path' を実施してください。",
        content: DEFAULT_TASK_TEMPLATE,
      },
      trigger: { type: 'file_created', path: 'incoming/reports', pattern: '*.md' },
    })
  })

  it('creates a manual trigger without scheduling fields', async () => {
    vi.mocked(api.createTaskTrigger).mockResolvedValueOnce({
      id: 'manual-task',
      type: 'manual',
      directory: '_task_triggers/manual-task',
      task_path: '_task_triggers/manual-task/task.md',
    })
    render(<NewTaskDialog open onOpenChange={() => undefined} />)

    fireEvent.change(screen.getByLabelText('作成種別'), { target: { value: 'task_trigger' } })
    expect(screen.getByLabelText(/Prompt/)).toHaveValue("'$task_file_path' を実施してください。")
    fireEvent.change(screen.getByLabelText('タスク名 (required)'), { target: { value: 'Manual task' } })
    await waitFor(() => expect(screen.getByLabelText(/task\.md本文/)).toHaveValue(DEFAULT_TASK_BODY))
    fireEvent.change(screen.getByLabelText('Trigger type'), { target: { value: 'manual' } })
    fireEvent.click(screen.getByRole('button', { name: 'Create' }))

    expect(screen.queryByLabelText(/Cron/)).not.toBeInTheDocument()
    expect(screen.queryByLabelText(/Watch directory/)).not.toBeInTheDocument()
    await waitFor(() => expect(api.createTaskTrigger).toHaveBeenCalledWith({
      id: 'manual-task',
      task: {
        name: 'Manual task',
        agent_kind: 'codex',
        prompt: "'$task_file_path' を実施してください。",
        content: DEFAULT_TASK_TEMPLATE,
      },
      trigger: { type: 'manual' },
    }))
  })
})
