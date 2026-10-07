import { act, fireEvent, render, screen, within } from '@testing-library/react'
import { useState, type ReactNode } from 'react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { api, type FileExecuteStreamHandlers } from '../api/client'
import {
  FILE_EXECUTION_HISTORY_LIMIT,
  FILE_EXECUTION_OUTPUT_LIMIT,
  FileExecutionProvider,
  useFileExecution,
} from './fileExecution'

function Harness({ project = 'demo' }: { project?: string }) {
  const { runs, activeRunId, start, closeModal, open, stop, dismiss } = useFileExecution()

  return (
    <>
      <div data-testid="active-run">{activeRunId ?? 'none'}</div>
      {runs.map((run) => (
        <div key={run.id} data-testid={`run-${run.id}`}>
          <span>{run.path}</span>
          <span>{run.status}</span>
          <span data-testid={`run-output-${run.id}`}>{run.output}</span>
          <button onClick={() => open(run.id)}>open {run.id}</button>
          <button onClick={() => stop(run.id)}>stop {run.id}</button>
          <button onClick={() => dismiss(run.id)}>dismiss {run.id}</button>
        </div>
      ))}
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

function renderHarness(children: ReactNode = <Harness />) {
  return render(<FileExecutionProvider project="demo">{children}</FileExecutionProvider>)
}

describe('file execution state', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('keeps the execution and its stream alive when the modal is closed', () => {
    const handlers: FileExecuteStreamHandlers[] = []
    const cancel = vi.fn()
    vi.spyOn(api, 'executeFileStream').mockImplementation((_path, next) => {
      handlers.push(next)
      return cancel
    })

    renderHarness()
    fireEvent.click(screen.getByRole('button', { name: 'start one' }))
    expect(screen.getByTestId('active-run')).not.toHaveTextContent('none')

    fireEvent.click(screen.getByRole('button', { name: 'close modal' }))
    expect(cancel).not.toHaveBeenCalled()

    act(() => handlers[0].onOutput('still running\n'))
    expect(screen.getByTestId(/^run-file-execution-/)).toHaveTextContent('still running')
  })

  it('caps retained output while keeping the newest output visible', () => {
    const handlers: FileExecuteStreamHandlers[] = []
    vi.spyOn(api, 'executeFileStream').mockImplementation((_path, next) => {
      handlers.push(next)
      return vi.fn()
    })

    renderHarness()
    fireEvent.click(screen.getByRole('button', { name: 'start one' }))
    const run = screen.getByTestId(/^run-file-execution-/)
    const runId = run.getAttribute('data-testid')!.slice(4)
    const newestOutput = 'newest output\n'

    act(() => handlers[0].onOutput('x'.repeat(FILE_EXECUTION_OUTPUT_LIMIT)))
    act(() => handlers[0].onOutput(newestOutput))

    const output = screen.getByTestId(`run-output-${runId}`)
    expect(output.textContent!.length).toBeLessThanOrEqual(FILE_EXECUTION_OUTPUT_LIMIT)
    expect(output).toHaveTextContent('output truncated')
    expect(output.textContent).toContain(newestOutput)
  })

  it('keeps multiple executions independent and supports stop and dismiss', () => {
    const cancelers: ReturnType<typeof vi.fn>[] = []
    vi.spyOn(api, 'executeFileStream').mockImplementation(() => {
      const cancel = vi.fn()
      cancelers.push(cancel)
      return cancel
    })

    renderHarness()
    fireEvent.click(screen.getByRole('button', { name: 'start one' }))
    fireEvent.click(screen.getByRole('button', { name: 'start two' }))

    expect(screen.getAllByTestId(/^run-file-execution-/)).toHaveLength(2)
    const firstRun = screen.getByText('scripts/one.sh').closest<HTMLElement>('[data-testid^="run-"]')
    if (!firstRun) throw new Error('first run was not rendered')
    const firstRunId = firstRun.getAttribute('data-testid')!.slice(4)
    fireEvent.click(within(firstRun).getByRole('button', { name: `stop ${firstRunId}` }))
    expect(cancelers[0]).toHaveBeenCalledOnce()
    expect(firstRun).toHaveTextContent('stopped')

    fireEvent.click(within(firstRun).getByRole('button', { name: `dismiss ${firstRunId}` }))
    expect(screen.queryByTestId(`run-${firstRunId}`)).not.toBeInTheDocument()
    expect(screen.getAllByTestId(/^run-file-execution-/)).toHaveLength(1)
  })

  it('keeps only the newest completed execution history', () => {
    const handlers: FileExecuteStreamHandlers[] = []
    vi.spyOn(api, 'executeFileStream').mockImplementation((_path, next) => {
      handlers.push(next)
      return vi.fn()
    })

    renderHarness()
    fireEvent.click(screen.getByRole('button', { name: 'start one' }))
    const firstRun = screen.getByTestId(/^run-file-execution-/)
    const firstRunId = firstRun.getAttribute('data-testid')!.slice(4)

    for (let i = 1; i < FILE_EXECUTION_HISTORY_LIMIT + 1; i += 1) {
      fireEvent.click(screen.getByRole('button', { name: 'start one' }))
    }
    act(() => {
      for (const handler of handlers) {
        handler.onComplete({ path: 'scripts/one.sh', exit_code: 0, output: '' })
      }
    })

    expect(screen.getAllByTestId(/^run-file-execution-/)).toHaveLength(FILE_EXECUTION_HISTORY_LIMIT)
    expect(screen.queryByTestId(`run-${firstRunId}`)).not.toBeInTheDocument()
  })

  it('clears runs and cancels active streams when the project changes', () => {
    const cancel = vi.fn()
    vi.spyOn(api, 'executeFileStream').mockReturnValue(cancel)

    render(<ProviderHarness />)
    fireEvent.click(screen.getByRole('button', { name: 'start one' }))
    fireEvent.click(screen.getByRole('button', { name: 'switch project' }))

    expect(cancel).toHaveBeenCalledOnce()
    expect(screen.queryByTestId(/run-/)).not.toBeInTheDocument()
    expect(screen.getByTestId('project')).toHaveTextContent('other')
  })
})
