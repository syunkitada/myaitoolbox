import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { useState, type ReactNode } from 'react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { api, type FileExecuteStreamHandlers, type FileExecutionSnapshot } from '../api/client'
import {
  FILE_EXECUTION_HISTORY_LIMIT,
  FILE_EXECUTION_OUTPUT_LIMIT,
  FileExecutionProvider,
  useFileExecution,
} from './fileExecution'

function makeSnapshot(
  id: string,
  path: string,
  status: FileExecutionSnapshot['status'] = 'running',
  output = '',
): FileExecutionSnapshot {
  return {
    id,
    path,
    status,
    output,
    exit_code: status === 'running' || status === 'stopped' ? undefined : 0,
    started_at: new Date(0).toISOString(),
  }
}

function Harness({ project = 'demo' }: { project?: string }) {
  const { runs, activeRunId, start, closeModal, open, stop, dismiss } = useFileExecution()

  return (
    <>
      <div data-testid="active-run">{activeRunId ?? 'none'}</div>
      {runs.map((run, index) => (
        <div key={`${run.id}-${index}`} data-testid={`run-${run.id}`}>
          <span>{run.path}</span>
          <span>{run.status}</span>
          <span data-testid={`run-output-${run.id}`}>{run.output}</span>
          <button onClick={() => open(run.id)}>open {run.id}</button>
          <button onClick={() => stop(run.id)}>stop {run.id}</button>
          <button onClick={() => dismiss(run.id)}>dismiss {run.id}</button>
        </div>
      ))}
      <div data-testid="run-count">{runs.length}</div>
      <button onClick={() => start('scripts/one.sh')}>start one</button>
      <button onClick={() => start('scripts/two.sh')}>start two</button>
      <button onClick={closeModal}>close modal</button>
      <span data-testid="project">{project}</span>
    </>
  )
}

function ProviderHarness() {
  const [project, setProject] = useState('demo')
  return (
    <>
      <FileExecutionProvider project={project}>
        <Harness project={project} />
      </FileExecutionProvider>
      <button onClick={() => setProject('other')}>switch project</button>
    </>
  )
}

function mockExecutionApi(
  handlers: FileExecuteStreamHandlers[],
  list: FileExecutionSnapshot[] = [],
) {
  let sequence = 0
  vi.spyOn(api, 'listFileExecutions').mockResolvedValue(list)
  vi.spyOn(api, 'startFileExecution').mockImplementation(async (path) => {
    sequence += 1
    return makeSnapshot(`run-${sequence}`, path)
  })
  vi.spyOn(api, 'executeFileStream').mockImplementation((_runId, next) => {
    handlers.push(next)
    return vi.fn()
  })
  vi.spyOn(api, 'stopFileExecution').mockImplementation(async (runId) => {
    const path = runId === 'run-1' ? 'scripts/one.sh' : 'scripts/two.sh'
    return makeSnapshot(runId, path, 'stopped')
  })
  vi.spyOn(api, 'dismissFileExecution').mockResolvedValue(undefined)
}

function renderHarness(children: ReactNode = <Harness />) {
  return render(<FileExecutionProvider project="demo">{children}</FileExecutionProvider>)
}

describe('file execution state', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('keeps the execution and its stream alive when the modal is closed', async () => {
    const handlers: FileExecuteStreamHandlers[] = []
    mockExecutionApi(handlers)

    renderHarness()
    fireEvent.click(screen.getByRole('button', { name: 'start one' }))
    await waitFor(() => expect(screen.getByTestId('active-run')).not.toHaveTextContent('none'))
    fireEvent.click(screen.getByRole('button', { name: 'close modal' }))
    expect(screen.getByTestId('active-run')).toHaveTextContent('none')
    expect(handlers).toHaveLength(1)

    act(() => handlers[0].onOutput('still running\n'))
    expect(screen.getByTestId('run-run-1')).toHaveTextContent('still running')
  })

  it('refreshes a running execution after its stream disconnects', async () => {
    const handlers: FileExecuteStreamHandlers[] = []
    mockExecutionApi(handlers)
    const completed = makeSnapshot('run-1', 'scripts/one.sh', 'completed', 'finished\n')
    vi.mocked(api.listFileExecutions)
      .mockResolvedValueOnce([])
      .mockResolvedValueOnce([completed])

    renderHarness()
    fireEvent.click(screen.getByRole('button', { name: 'start one' }))
    await waitFor(() => expect(handlers).toHaveLength(1))

    act(() => handlers[0].onError(new Error('connection lost')))
    expect(api.listFileExecutions).toHaveBeenCalledTimes(1)

    await waitFor(() => expect(screen.getByTestId('run-run-1')).toHaveTextContent('completed'), { timeout: 2500 })
    expect(screen.getByTestId('run-run-1')).toHaveTextContent('finished')
    expect(api.listFileExecutions).toHaveBeenCalledTimes(2)
  })

  it('keeps the backoff attempt while a reconnect is not stable', async () => {
    vi.useFakeTimers()
    try {
      const handlers: FileExecuteStreamHandlers[] = []
      mockExecutionApi(handlers)
      const running = makeSnapshot('run-1', 'scripts/one.sh')
      vi.mocked(api.listFileExecutions)
        .mockResolvedValueOnce([])
        .mockResolvedValue([running])

      renderHarness()
      fireEvent.click(screen.getByRole('button', { name: 'start one' }))
      await act(async () => {
        await Promise.resolve()
        await Promise.resolve()
      })
      expect(handlers).toHaveLength(1)

      act(() => handlers[0].onError(new Error('connection lost')))
      expect(api.listFileExecutions).toHaveBeenCalledTimes(1)

      await act(async () => { await vi.advanceTimersByTimeAsync(1000) })
      expect(api.listFileExecutions).toHaveBeenCalledTimes(2)
      expect(handlers).toHaveLength(2)

      act(() => handlers[1].onSnapshot(running))
      act(() => handlers[1].onError(new Error('connection lost again')))
      await act(async () => { await vi.advanceTimersByTimeAsync(1999) })
      expect(api.listFileExecutions).toHaveBeenCalledTimes(2)

      await act(async () => { await vi.advanceTimersByTimeAsync(1) })
      expect(api.listFileExecutions).toHaveBeenCalledTimes(3)
    } finally {
      vi.useRealTimers()
    }
  })

  it('retries restoring executions after a temporary initial list failure', async () => {
    vi.useFakeTimers()
    try {
      const handlers: FileExecuteStreamHandlers[] = []
      mockExecutionApi(handlers)
      vi.mocked(api.listFileExecutions)
        .mockRejectedValueOnce(new Error('temporary failure'))
        .mockResolvedValueOnce([makeSnapshot('server-run', 'scripts/reload.sh')])

      renderHarness()
      await act(async () => { await Promise.resolve() })
      expect(api.listFileExecutions).toHaveBeenCalledTimes(1)

      await act(async () => { await vi.advanceTimersByTimeAsync(1000) })
      expect(api.listFileExecutions).toHaveBeenCalledTimes(2)
      expect(screen.getByTestId('run-server-run')).toHaveTextContent('running')
      expect(handlers).toHaveLength(1)
    } finally {
      vi.useRealTimers()
    }
  })

  it('restores server-owned executions after the provider is remounted', async () => {
    const handlers: FileExecuteStreamHandlers[] = []
    mockExecutionApi(handlers, [makeSnapshot('server-run', 'scripts/reload.sh')])

    const first = renderHarness()
    await waitFor(() => expect(screen.getByTestId('run-server-run')).toHaveTextContent('running'))
    expect(handlers).toHaveLength(1)
    first.unmount()

    renderHarness()
    await waitFor(() => expect(screen.getAllByTestId('run-server-run')).toHaveLength(1))
    expect(handlers).toHaveLength(2)
  })

  it('deduplicates a restored execution when its start response races with initial loading', async () => {
    const handlers: FileExecuteStreamHandlers[] = []
    mockExecutionApi(handlers)
    let resolveList!: (snapshots: FileExecutionSnapshot[]) => void
    let resolveStart!: (snapshot: FileExecutionSnapshot) => void
    vi.mocked(api.listFileExecutions).mockReturnValue(new Promise((resolve) => {
      resolveList = resolve
    }))
    vi.mocked(api.startFileExecution).mockReturnValue(new Promise((resolve) => {
      resolveStart = resolve
    }))

    renderHarness()
    fireEvent.click(screen.getByRole('button', { name: 'start one' }))
    await waitFor(() => expect(screen.getByTestId(/^run-file-execution-pending-/)).toBeInTheDocument())

    act(() => resolveList([makeSnapshot('run-1', 'scripts/one.sh')]))
    await waitFor(() => expect(screen.getByTestId('run-run-1')).toBeInTheDocument())
    act(() => resolveStart(makeSnapshot('run-1', 'scripts/one.sh')))
    await waitFor(() => expect(screen.getByTestId('run-count')).toHaveTextContent('1'))
  })

  it('caps retained output while keeping the newest output visible', async () => {
    const handlers: FileExecuteStreamHandlers[] = []
    mockExecutionApi(handlers)

    renderHarness()
    fireEvent.click(screen.getByRole('button', { name: 'start one' }))
    await waitFor(() => expect(handlers).toHaveLength(1))
    const runId = 'run-1'
    const newestOutput = 'newest output\n'

    act(() => handlers[0].onSnapshot(makeSnapshot(runId, 'scripts/one.sh')))
    act(() => handlers[0].onOutput('x'.repeat(FILE_EXECUTION_OUTPUT_LIMIT)))
    act(() => handlers[0].onOutput(newestOutput))

    const output = screen.getByTestId(`run-output-${runId}`)
    expect(output.textContent!.length).toBeLessThanOrEqual(FILE_EXECUTION_OUTPUT_LIMIT)
    expect(output).toHaveTextContent('output truncated')
    expect(output.textContent).toContain(newestOutput)
  })

  it('keeps multiple executions independent and supports stop and dismiss', async () => {
    const handlers: FileExecuteStreamHandlers[] = []
    mockExecutionApi(handlers)

    renderHarness()
    fireEvent.click(screen.getByRole('button', { name: 'start one' }))
    fireEvent.click(screen.getByRole('button', { name: 'start two' }))
    await waitFor(() => expect(screen.getAllByTestId(/^run-run-/)).toHaveLength(2))

    const firstRun = screen.getByText('scripts/one.sh').closest<HTMLElement>('[data-testid^="run-"]')
    if (!firstRun) throw new Error('first run was not rendered')
    const firstRunId = firstRun.getAttribute('data-testid')!.slice(4)
    fireEvent.click(within(firstRun).getByRole('button', { name: `stop ${firstRunId}` }))
    await waitFor(() => expect(firstRun).toHaveTextContent('stopped'))
    expect(api.stopFileExecution).toHaveBeenCalledWith(firstRunId)

    fireEvent.click(within(firstRun).getByRole('button', { name: `dismiss ${firstRunId}` }))
    await waitFor(() => expect(screen.queryByTestId(`run-${firstRunId}`)).not.toBeInTheDocument())
    expect(screen.getAllByTestId(/^run-file-execution|^run-run-/)).toHaveLength(1)
  })

  it('keeps a completed execution when dismissing it fails', async () => {
    const handlers: FileExecuteStreamHandlers[] = []
    mockExecutionApi(handlers)
    vi.mocked(api.dismissFileExecution).mockRejectedValueOnce(new Error('temporary failure'))

    renderHarness()
    fireEvent.click(screen.getByRole('button', { name: 'start one' }))
    await waitFor(() => expect(handlers).toHaveLength(1))
    act(() => handlers[0].onSnapshot(makeSnapshot('run-1', 'scripts/one.sh', 'completed', 'finished\n')))
    await waitFor(() => expect(screen.getByTestId('run-run-1')).toHaveTextContent('completed'))

    fireEvent.click(screen.getByRole('button', { name: 'dismiss run-1' }))
    await waitFor(() => expect(api.dismissFileExecution).toHaveBeenCalledWith('run-1'))
    expect(screen.getByTestId('run-run-1')).toBeInTheDocument()
  })

  it('keeps only the newest completed execution history', async () => {
    const handlers: FileExecuteStreamHandlers[] = []
    mockExecutionApi(handlers)

    renderHarness()
    for (let i = 0; i < FILE_EXECUTION_HISTORY_LIMIT + 1; i += 1) {
      fireEvent.click(screen.getByRole('button', { name: 'start one' }))
    }
    await waitFor(() => expect(handlers).toHaveLength(FILE_EXECUTION_HISTORY_LIMIT + 1))
    act(() => {
      for (let i = 0; i < handlers.length; i += 1) {
        handlers[i].onSnapshot(makeSnapshot(`run-${i + 1}`, 'scripts/one.sh', 'completed', `output ${i}`))
      }
    })

    expect(screen.getAllByTestId(/^run-run-/)).toHaveLength(FILE_EXECUTION_HISTORY_LIMIT)
    expect(screen.queryByTestId('run-run-1')).not.toBeInTheDocument()
  })

  it('clears runs and detaches streams when the project changes', async () => {
    const handlers: FileExecuteStreamHandlers[] = []
    mockExecutionApi(handlers)

    render(<ProviderHarness />)
    fireEvent.click(screen.getByRole('button', { name: 'start one' }))
    await waitFor(() => expect(screen.getByTestId('run-run-1')).toBeInTheDocument())
    fireEvent.click(screen.getByRole('button', { name: 'switch project' }))

    await waitFor(() => expect(screen.queryByTestId('run-run-1')).not.toBeInTheDocument())
    expect(screen.getByTestId('project')).toHaveTextContent('other')
  })
})
