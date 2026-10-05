import { Bot, Loader2, X } from 'lucide-react'
import { Button } from './ui/button'
import { useEscapeKey } from '../hooks/use-escape-key'

export const TASK_AGENT_KIND_OPTIONS = ['codex', 'opencode'] as const
export const TASK_AGENT_KIND_STORAGE_KEY = 'mybox.herdr.file-agent-kind'

interface TaskAgentLaunchDialogProps {
  open: boolean
  taskPath: string
  taskDirectory: string
  agentKind: string
  starting: boolean
  error: string | null
  onAgentKindChange: (kind: string) => void
  onOpenChange: (open: boolean) => void
  onSubmit: () => void
}

export function TaskAgentLaunchDialog({
  open,
  taskPath,
  taskDirectory,
  agentKind,
  starting,
  error,
  onAgentKindChange,
  onOpenChange,
  onSubmit,
}: TaskAgentLaunchDialogProps) {
  useEscapeKey(() => onOpenChange(false), open && !starting)

  if (!open) return null

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4" data-testid="task-agent-launch-dialog-backdrop">
      <div
        className="w-full max-w-md rounded-lg border bg-card p-4 text-card-foreground shadow-lg"
        role="dialog"
        aria-modal="true"
        aria-labelledby="task-agent-launch-dialog-title"
        onClick={(event) => event.stopPropagation()}
      >
        <div className="flex items-start gap-3">
          <Bot className="mt-0.5 size-5 shrink-0 text-primary" />
          <div className="min-w-0 flex-1">
            <h2 id="task-agent-launch-dialog-title" className="text-base font-semibold">Start task agent</h2>
            <p className="mt-1 break-all text-xs text-muted-foreground" title={taskPath}>
              {taskDirectory}
            </p>
          </div>
          <Button
            variant="ghost"
            size="icon-xs"
            aria-label="Close"
            title="Close"
            onClick={() => onOpenChange(false)}
            disabled={starting}
          >
            <X />
          </Button>
        </div>

        <div className="mt-4">
          <label className="text-xs font-medium" htmlFor="task-agent-launch-kind">
            Agent kind
          </label>
          <select
            id="task-agent-launch-kind"
            value={agentKind}
            onChange={(event) => onAgentKindChange(event.target.value)}
            aria-label="Task agent kind"
            disabled={starting}
            className="mt-1 h-9 w-full rounded-md border border-input bg-background px-2 text-sm outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 disabled:cursor-not-allowed disabled:opacity-50"
          >
            {TASK_AGENT_KIND_OPTIONS.map((kind) => (
              <option key={kind} value={kind}>{kind}</option>
            ))}
          </select>
        </div>

        {error && (
          <p role="alert" className="mt-3 rounded-md border border-red-200 bg-red-50 px-2.5 py-2 text-xs text-red-700">
            {error}
          </p>
        )}

        <div className="mt-5 flex justify-end gap-2">
          <Button variant="ghost" size="sm" onClick={() => onOpenChange(false)} disabled={starting}>
            Cancel
          </Button>
          <Button size="sm" onClick={onSubmit} disabled={starting}>
            {starting ? (
              <>
                <Loader2 className="animate-spin" />
                Starting…
              </>
            ) : (
              <>
                <Bot />
                Start agent
              </>
            )}
          </Button>
        </div>
      </div>
    </div>
  )
}
