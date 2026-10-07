import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { useState } from 'react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { FileExecutionRun } from '../state/fileExecution'
import { FileExecutionModal } from './FileExecutionModal'

function makeRun(id: string, output: string): FileExecutionRun {
  return {
    id,
    path: `scripts/${id}.sh`,
    status: 'completed',
    output,
    result: {
      path: `scripts/${id}.sh`,
      exit_code: 0,
      output,
    },
    startedAt: 0,
  }
}

function ModalHarness() {
  const [run, setRun] = useState(() => makeRun('one', 'one output'))
  return (
    <>
      <button onClick={() => setRun(makeRun('two', 'two output'))}>switch run</button>
      <FileExecutionModal run={run} onClose={vi.fn()} onStop={vi.fn()} onDismiss={vi.fn()} />
    </>
  )
}

describe('FileExecutionModal', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('resets the copied indicator when another execution is opened', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined)
    Object.defineProperty(navigator, 'clipboard', {
      configurable: true,
      value: { writeText },
    })

    render(<ModalHarness />)
    fireEvent.click(screen.getByRole('button', { name: 'Copy output' }))
    expect(await screen.findByRole('button', { name: 'Copied' })).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'switch run' }))
    await waitFor(() => expect(screen.getByRole('button', { name: 'Copy output' })).toBeInTheDocument())
    expect(screen.queryByRole('button', { name: 'Copied' })).not.toBeInTheDocument()
  })
})
