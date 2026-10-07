import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from 'react'
import type { ReactNode } from 'react'
import type { FileExecuteResult, FileExecutionSnapshot } from '../api/client'
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
const FILE_EXECUTION_RECONNECT_INITIAL_DELAY_MS = 1000
const FILE_EXECUTION_RECONNECT_MAX_DELAY_MS = 30000
const FILE_EXECUTION_RECONNECT_STABILITY_DELAY_MS = 5000

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

function snapshotToRun(snapshot: FileExecutionSnapshot): FileExecutionRun {
  const startedAt = Date.parse(snapshot.started_at)
  const result = snapshot.exit_code === undefined
    ? undefined
    : {
        path: snapshot.path,
        exit_code: snapshot.exit_code,
        output: snapshot.output,
        timed_out: snapshot.timed_out,
      }
  return {
    id: snapshot.id,
    path: snapshot.path,
    status: snapshot.status,
    output: snapshot.output,
    result,
    error: snapshot.error,
    startedAt: Number.isNaN(startedAt) ? Date.now() : startedAt,
  }
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
  const activeRunIdRef = useRef<string | null>(null)
  const runSequenceRef = useRef(0)
  const mountedRef = useRef(true)
  const projectRef = useRef(project)
  const reconnectTimersRef = useRef(new Map<string, ReturnType<typeof setTimeout>>())
  const reconnectAttemptsRef = useRef(new Map<string, number>())
  const reconnectStabilityTimersRef = useRef(new Map<string, ReturnType<typeof setTimeout>>())
  const connectRunRef = useRef<(runId: string) => void>(() => undefined)
  const recoverRunRef = useRef<(runId: string, requestProject: string | null | undefined) => void>(() => undefined)

  useEffect(() => {
    runsRef.current = runs
  }, [runs])

  useEffect(() => {
    projectRef.current = project
  }, [project])

  const cancelAll = useCallback(() => {
    for (const cancel of cancelersRef.current.values()) cancel()
    cancelersRef.current.clear()
    for (const timer of reconnectTimersRef.current.values()) clearTimeout(timer)
    reconnectTimersRef.current.clear()
    reconnectAttemptsRef.current.clear()
    for (const timer of reconnectStabilityTimersRef.current.values()) clearTimeout(timer)
    reconnectStabilityTimersRef.current.clear()
  }, [])

  const clearReconnectStabilityTimer = useCallback((runId: string) => {
    const timer = reconnectStabilityTimersRef.current.get(runId)
    if (timer !== undefined) {
      clearTimeout(timer)
      reconnectStabilityTimersRef.current.delete(runId)
    }
  }, [])

  const applySnapshot = useCallback((snapshot: FileExecutionSnapshot) => {
    setRuns((current) => retainExecutionHistory(
      current.map((run) => run.id === snapshot.id ? snapshotToRun(snapshot) : run),
      activeRunIdRef.current,
    ))
    if (snapshot.status !== 'running') {
      cancelersRef.current.delete(snapshot.id)
      const timer = reconnectTimersRef.current.get(snapshot.id)
      if (timer !== undefined) {
        clearTimeout(timer)
        reconnectTimersRef.current.delete(snapshot.id)
      }
      reconnectAttemptsRef.current.delete(snapshot.id)
      clearReconnectStabilityTimer(snapshot.id)
    }
  }, [clearReconnectStabilityTimer])

  const scheduleReconnect = useCallback((runId: string, requestProject: string | null | undefined) => {
    if (reconnectTimersRef.current.has(runId)) return
    const attempt = reconnectAttemptsRef.current.get(runId) ?? 0
    const delay = Math.min(
      FILE_EXECUTION_RECONNECT_INITIAL_DELAY_MS * (2 ** attempt),
      FILE_EXECUTION_RECONNECT_MAX_DELAY_MS,
    )
    reconnectAttemptsRef.current.set(runId, attempt + 1)
    const timer = setTimeout(() => {
      reconnectTimersRef.current.delete(runId)
      if (!mountedRef.current || projectRef.current !== requestProject) return
      recoverRunRef.current(runId, requestProject)
    }, delay)
    reconnectTimersRef.current.set(runId, timer)
  }, [])

  const recoverDisconnectedRun = useCallback((runId: string, requestProject: string | null | undefined) => {
    if (!mountedRef.current || projectRef.current !== requestProject) return
    void api.listFileExecutions().then((snapshots) => {
      if (!mountedRef.current || projectRef.current !== requestProject) return
      const snapshot = snapshots.find((item) => item.id === runId)
      if (!snapshot) {
        reconnectAttemptsRef.current.delete(runId)
        clearReconnectStabilityTimer(runId)
        setRuns((current) => current.filter((run) => run.id !== runId))
        return
      }
      applySnapshot(snapshot)
      if (snapshot.status === 'running') {
        connectRunRef.current(runId)
        if (!cancelersRef.current.has(runId)) scheduleReconnect(runId, requestProject)
      }
    }).catch(() => scheduleReconnect(runId, requestProject))
  }, [applySnapshot, clearReconnectStabilityTimer, scheduleReconnect])

  const connectRun = useCallback((runId: string) => {
    if (cancelersRef.current.has(runId)) return
    const requestProject = projectRef.current
    let cancel: () => void
    try {
      cancel = api.executeFileStream(runId, {
        onOutput: (chunk) => {
          setRuns((current) => current.map((run) => (
            run.id === runId && run.status === 'running'
              ? { ...run, output: appendExecutionOutput(run.output, chunk) }
              : run
          )))
        },
        onSnapshot: (snapshot) => {
          if (snapshot.status === 'running') {
            clearReconnectStabilityTimer(runId)
            const timer = setTimeout(() => {
              reconnectStabilityTimersRef.current.delete(runId)
              if (mountedRef.current && projectRef.current === requestProject && cancelersRef.current.has(runId)) {
                reconnectAttemptsRef.current.delete(runId)
              }
            }, FILE_EXECUTION_RECONNECT_STABILITY_DELAY_MS)
            reconnectStabilityTimersRef.current.set(runId, timer)
          }
          applySnapshot(snapshot)
        },
        onError: () => {
          // A disconnected stream does not mean the server-side job failed.
          cancelersRef.current.delete(runId)
          clearReconnectStabilityTimer(runId)
          scheduleReconnect(runId, requestProject)
        },
      })
    } catch {
      scheduleReconnect(runId, requestProject)
      return
    }
    cancelersRef.current.set(runId, cancel)
  }, [applySnapshot, clearReconnectStabilityTimer, scheduleReconnect])

  useEffect(() => {
    connectRunRef.current = connectRun
    recoverRunRef.current = recoverDisconnectedRun
  }, [connectRun, recoverDisconnectedRun])

  useEffect(() => {
    let active = true
    let listRetryTimer: ReturnType<typeof setTimeout> | undefined
    let listRetryAttempt = 0
    cancelAll()
    setRuns([])
    activeRunIdRef.current = null
    setActiveRunId(null)

    const loadRuns = () => {
      if (!active || !mountedRef.current) return
      void api.listFileExecutions().then((snapshots) => {
        if (!active || !mountedRef.current) return
        listRetryAttempt = 0
        const restored = snapshots.map(snapshotToRun)
        setRuns((current) => [
          ...restored,
          ...current.filter((run) => run.id.startsWith('file-execution-pending-')),
        ])
        for (const run of restored) {
          if (run.status === 'running') connectRun(run.id)
        }
      }).catch(() => {
        if (!active || !mountedRef.current) return
        const delay = Math.min(
          FILE_EXECUTION_RECONNECT_INITIAL_DELAY_MS * (2 ** listRetryAttempt),
          FILE_EXECUTION_RECONNECT_MAX_DELAY_MS,
        )
        listRetryAttempt += 1
        listRetryTimer = setTimeout(() => {
          listRetryTimer = undefined
          loadRuns()
        }, delay)
      })
    }
    loadRuns()

    return () => {
      active = false
      if (listRetryTimer !== undefined) clearTimeout(listRetryTimer)
      cancelAll()
    }
  }, [cancelAll, connectRun, project])

  useEffect(() => {
    mountedRef.current = true
    return () => {
      mountedRef.current = false
      cancelAll()
    }
  }, [cancelAll])

  const start = useCallback((path: string) => {
    const pendingID = `file-execution-pending-${Date.now()}-${runSequenceRef.current++}`
    const requestProject = projectRef.current
    setRuns((current) => [
      {
        id: pendingID,
        path,
        status: 'running',
        output: '',
        startedAt: Date.now(),
      },
      ...current,
    ])
    activeRunIdRef.current = pendingID
    setActiveRunId(pendingID)

    void api.startFileExecution(path).then((snapshot) => {
      if (!mountedRef.current || projectRef.current !== requestProject) return
      const nextRun = snapshotToRun(snapshot)
      setRuns((current) => {
        const nextRuns: FileExecutionRun[] = []
        let replaced = false
        for (const run of current) {
          if (run.id === pendingID || run.id === snapshot.id) {
            if (!replaced) {
              nextRuns.push(nextRun)
              replaced = true
            }
            continue
          }
          nextRuns.push(run)
        }
        if (!replaced) nextRuns.push(nextRun)
        return nextRuns
      })
      if (activeRunIdRef.current === pendingID) {
        activeRunIdRef.current = snapshot.id
        setActiveRunId(snapshot.id)
      }
      if (snapshot.status === 'running') connectRun(snapshot.id)
    }).catch((error: unknown) => {
      if (!mountedRef.current || projectRef.current !== requestProject) return
      const message = error instanceof Error ? error.message : String(error)
      setRuns((current) => retainExecutionHistory(
        current.map((run) => run.id === pendingID ? { ...run, status: 'failed', error: message } : run),
        activeRunIdRef.current,
      ))
    })
    return pendingID
  }, [connectRun])

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
    if (!run || run.status !== 'running' || run.id.startsWith('file-execution-pending-')) return
    void api.stopFileExecution(runId).then(applySnapshot).catch(() => undefined)
  }, [applySnapshot])

  const dismiss = useCallback((runId: string) => {
    const run = runsRef.current.find((item) => item.id === runId)
    if (!run || run.status === 'running') return
    void api.dismissFileExecution(runId).then(() => {
      if (activeRunIdRef.current === runId) {
        activeRunIdRef.current = null
        setActiveRunId(null)
      }
      setRuns((current) => current.filter((item) => item.id !== runId))
    }).catch(() => undefined)
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
