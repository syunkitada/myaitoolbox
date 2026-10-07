import { useEffect, useState } from 'react'
import { Check, Copy, Loader2, Square, Terminal, X } from 'lucide-react'
import type { FileExecutionRun } from '../state/fileExecution'
import { Button } from './ui/button'
import { useEscapeKey } from '../hooks/use-escape-key'
import { copyToClipboard } from '../utils/clipboard'
import { cn } from '@/lib/utils'

interface FileExecutionModalProps {
  run: FileExecutionRun | null
  onClose: () => void
  onStop: (runId: string) => void
  onDismiss: (runId: string) => void
}

export function FileExecutionModal({ run, onClose, onStop, onDismiss }: FileExecutionModalProps) {
  const [copied, setCopied] = useState(false)

  useEscapeKey(onClose, Boolean(run))

  useEffect(() => {
    if (!copied) return
    const timer = window.setTimeout(() => setCopied(false), 1500)
    return () => window.clearTimeout(timer)
  }, [copied])

  useEffect(() => {
    setCopied(false)
  }, [run?.id])

  if (!run) return null

  const copyOutput = () => {
    void copyToClipboard(run.output)
      .then(() => setCopied(true))
      .catch(() => undefined)
  }
  const exitedCleanly = run.result && run.result.exit_code === 0 && !run.result.timed_out
  const resultLabel = run.status === 'stopped'
    ? 'Stopped'
    : run.result?.timed_out
      ? `Timed out (exit ${run.result.exit_code})`
      : run.result
        ? `Exit code ${run.result.exit_code}`
        : 'Failed'

  return (
    <div
      className="fixed inset-0 z-[70] flex items-center justify-center bg-black/40 p-4"
      onClick={onClose}
      role="dialog"
      aria-modal="true"
      aria-label={`Execute ${run.path}`}
      data-testid="file-execution-modal"
    >
      <div
        className="flex max-h-[80vh] w-full max-w-2xl flex-col rounded-lg border bg-card shadow-xl"
        onClick={(event) => event.stopPropagation()}
      >
        <div className="flex items-center justify-between gap-2 border-b border-border px-4 py-3">
          <div className="flex min-w-0 items-center gap-2">
            <Terminal className="size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
            <span className="truncate text-sm font-semibold">{run.path}</span>
          </div>
          <button
            className="flex size-7 shrink-0 cursor-pointer items-center justify-center rounded-md text-muted-foreground hover:bg-muted hover:text-foreground"
            onClick={onClose}
            aria-label="Close execution result"
          >
            <X className="size-4" />
          </button>
        </div>
        <div className="min-h-0 flex-1 overflow-y-auto p-4">
          {run.status === 'running' && (
            <div className="flex items-center gap-2 text-sm text-muted-foreground">
              <Loader2 className="size-4 animate-spin" aria-hidden="true" />
              Running…
            </div>
          )}
          {run.status === 'stopped' && (
            <div className="rounded-md border border-amber-200 bg-amber-50 px-3.5 py-2.5 text-sm text-amber-800">
              Execution stopped.
            </div>
          )}
          {run.error && (
            <div className="rounded-md border border-red-200 bg-red-50 px-3.5 py-2.5 text-sm text-red-700">
              {run.error}
            </div>
          )}
          {run.result && (
            <div className="mt-3 flex flex-col gap-3">
              <div className="flex flex-wrap items-center gap-2">
                <span
                  className={cn(
                    'inline-flex items-center gap-1 rounded-full px-2.5 py-0.5 text-xs font-semibold',
                    run.result.timed_out
                      ? 'bg-amber-100 text-amber-700'
                      : exitedCleanly
                        ? 'bg-green-100 text-green-700'
                        : 'bg-red-100 text-red-700',
                  )}
                >
                  <span className="size-1.5 rounded-full bg-current" aria-hidden="true" />
                  {resultLabel}
                </span>
              </div>
            </div>
          )}
          {(run.output || run.result || run.status === 'stopped') && (
            <pre className="exec-result-output mt-3 m-0 max-h-96 overflow-auto rounded-md border border-border bg-muted/40 p-3 text-xs leading-relaxed whitespace-pre-wrap break-words">
              {run.output || '(no output)'}
            </pre>
          )}
          <div className="mt-4 flex flex-wrap justify-end gap-2">
            {(run.output || run.result) && (
              <Button variant="ghost" size="sm" className="cursor-pointer" onClick={copyOutput}>
                {copied ? <Check className="size-3.5" /> : <Copy className="size-3.5" />}
                {copied ? 'Copied' : 'Copy output'}
              </Button>
            )}
            {run.status === 'running' ? (
              <Button variant="outline" size="sm" className="cursor-pointer" onClick={() => onStop(run.id)}>
                <Square className="size-3.5" />
                Stop
              </Button>
            ) : (
              <Button variant="outline" size="sm" className="cursor-pointer" onClick={() => onDismiss(run.id)}>
                Dismiss
              </Button>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}
