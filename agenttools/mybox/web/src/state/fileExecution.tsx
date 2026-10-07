import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from 'react'
import type { ReactNode } from 'react'
import type { FileExecuteResult } from '../api/client'
import { api } from '../api/client'
import { FileExecutionModal } from '../components/FileExecutionModal'

export type FileExecutionStatus = 'running' | 'completed' | 'failed' | 'stopped'

export interface FileExecutionRun {
  id: string
  path: string
  status: FileExecutionStatus
  output: string
  result?: FileExecuteResult
  error?: string
  startedAt: number
}

export const FILE_EXECUTION_OUTPUT_LIMIT = 1024 * 1024
export const FILE_EXECUTION_HISTORY_LIMIT = 20

const OUTPUT_TRUNCATION_MARKER = '\n[output truncated; showing latest output]\n'

interface FileExecutionContextValue {
  runs: FileExecutionRun[]
  activeRunId: string | null
  start: (path: string) => string
  open: (runId: string) => void
  closeModal: () => void
  stop: (runId: string) => void
  dismiss: (runId: string) => void
}

const FileExecutionContext = createContext<FileExecutionContextValue | null>(null)

const EMPTY_CONTEXT: FileExecutionContextValue = {
  runs: [],
  activeRunId: null,
  start: () => '',
  open: () => undefined,
  closeModal: () => undefined,
  stop: () => undefined,
  dismiss: () => undefined,
}

function executionStatus(result: FileExecuteResult): FileExecutionStatus {
  return result.exit_code === 0 && !result.timed_out ? 'completed' : 'failed'
}

function appendExecutionOutput(current: string, chunk: string): string {
  if (!chunk) return current
  if (current.length + chunk.length <= FILE_EXECUTION_OUTPUT_LIMIT) return current + chunk

  const retainedLength = FILE_EXECUTION_OUTPUT_LIMIT - OUTPUT_TRUNCATION_MARKER.length
  if (chunk.length >= retainedLength) {
    return OUTPUT_TRUNCATION_MARKER + chunk.slice(-retainedLength)
  }
  return OUTPUT_TRUNCATION_MARKER + current.slice(-(retainedLength - chunk.length)) + chunk
}

function retainExecutionHistory(runs: FileExecutionRun[], activeRunId: string | null): FileExecutionRun[] {
  const terminalRuns = runs.filter((run) => run.status !== 'running')
  const removeCount = terminalRuns.length - FILE_EXECUTION_HISTORY_LIMIT
  if (removeCount <= 0) return runs

  const removeIds = new Set<string>()
  for (let index = runs.length - 1; index >= 0 && removeIds.size < removeCount; index -= 1) {
    const run = runs[index]
    if (run.status !== 'running' && run.id !== activeRunId) removeIds.add(run.id)
  }
  return runs.filter((run) => !removeIds.has(run.id))
}

export function FileExecutionProvider({ project, children }: { project?: string | null; children: ReactNode }) {
  const [runs, setRuns] = useState<FileExecutionRun[]>([])
  const [activeRunId, setActiveRunId] = useState<string | null>(null)
  const cancelersRef = useRef(new Map<string, () => void>())
  const runsRef = useRef(runs)
  const runSequenceRef = useRef(0)
  const previousProjectRef = useRef(project)
  const activeRunIdRef = useRef<string | null>(null)

  useEffect(() => {
    runsRef.current = runs
  }, [runs])

  const cancelAll = useCallback(() => {
    for (const cancel of cancelersRef.current.values()) cancel()
    cancelersRef.current.clear()
  }, [])

  useEffect(() => {
    if (previousProjectRef.current === project) return
    previousProjectRef.current = project
    cancelAll()
    setRuns([])
    activeRunIdRef.current = null
    setActiveRunId(null)
  }, [cancelAll, project])

  useEffect(() => () => cancelAll(), [cancelAll])

  const start = useCallback((path: string) => {
    const id = `file-execution-${Date.now()}-${runSequenceRef.current++}`
    setRuns((current) => [
      {
        id,
        path,
        status: 'running',
        output: '',
        startedAt: Date.now(),
      },
      ...current,
    ])
    activeRunIdRef.current = id
    setActiveRunId(id)

    const cancel = api.executeFileStream(path, {
      onOutput: (chunk) => {
        setRuns((current) => current.map((run) => (
          run.id === id && run.status === 'running'
            ? { ...run, output: appendExecutionOutput(run.output, chunk) }
            : run
        )))
      },
      onComplete: (result) => {
        cancelersRef.current.delete(id)
        setRuns((current) => retainExecutionHistory(current.map((run) => (
          run.id === id && run.status === 'running'
            ? {
                ...run,
                status: executionStatus(result),
                result: { ...result, output: run.output },
            }
            : run
        )), activeRunIdRef.current))
      },
      onError: (error) => {
        cancelersRef.current.delete(id)
        setRuns((current) => retainExecutionHistory(current.map((run) => (
          run.id === id && run.status === 'running'
            ? { ...run, status: 'failed', error: error.message }
            : run
        )), activeRunIdRef.current))
      },
    })
    cancelersRef.current.set(id, cancel)
    return id
  }, [])

  const open = useCallback((runId: string) => {
    if (runsRef.current.some((run) => run.id === runId)) {
      activeRunIdRef.current = runId
      setActiveRunId(runId)
    }
  }, [])

  const closeModal = useCallback(() => {
    activeRunIdRef.current = null
    setActiveRunId(null)
  }, [])

  const stop = useCallback((runId: string) => {
    const run = runsRef.current.find((item) => item.id === runId)
    if (!run || run.status !== 'running') return
    cancelersRef.current.get(runId)?.()
    cancelersRef.current.delete(runId)
    setRuns((current) => retainExecutionHistory(current.map((item) => (
      item.id === runId && item.status === 'running'
        ? { ...item, status: 'stopped' }
        : item
    )), activeRunIdRef.current))
  }, [])

  const dismiss = useCallback((runId: string) => {
    const run = runsRef.current.find((item) => item.id === runId)
    if (!run || run.status === 'running') return
    if (activeRunIdRef.current === runId) {
      activeRunIdRef.current = null
      setActiveRunId(null)
    }
    setRuns((current) => current.filter((item) => item.id !== runId))
  }, [])

  const value = useMemo<FileExecutionContextValue>(
    () => ({ runs, activeRunId, start, open, closeModal, stop, dismiss }),
    [activeRunId, closeModal, dismiss, open, runs, start, stop],
  )
  const activeRun = runs.find((run) => run.id === activeRunId) ?? null

  return (
    <FileExecutionContext.Provider value={value}>
      {children}
      <FileExecutionModal
        run={activeRun}
        onClose={closeModal}
        onStop={stop}
        onDismiss={dismiss}
      />
    </FileExecutionContext.Provider>
  )
}

export function useFileExecution(): FileExecutionContextValue {
  return useContext(FileExecutionContext) ?? EMPTY_CONTEXT
}
