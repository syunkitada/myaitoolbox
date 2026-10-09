import { Bot, Loader2, X } from 'lucide-react'
import { Button } from './ui/button'
import { Input } from './ui/input'
import { Label } from './ui/label'
import { useEscapeKey } from '../hooks/use-escape-key'

interface AgentLaunchDialogProps {
  open: boolean
  kinds: readonly string[]
  tabs: readonly string[]
  agentKind: string
  tab: string
  loadingKinds: boolean
  starting: boolean
  error: string | null
  onAgentKindChange: (kind: string) => void
  onTabChange: (tab: string) => void
  onOpenChange: (open: boolean) => void
  onSubmit: () => void
}

export function AgentLaunchDialog({
  open,
  kinds,
  tabs,
  agentKind,
  tab,
  loadingKinds,
  starting,
  error,
  onAgentKindChange,
  onTabChange,
  onOpenChange,
  onSubmit,
}: AgentLaunchDialogProps) {
  useEscapeKey(() => onOpenChange(false), open && !starting)

  if (!open) return null

  return (
    <div className="fixed inset-0 z-[60] flex items-center justify-center bg-black/40 p-4" data-testid="agent-launch-dialog-backdrop">
      <div
        className="w-full max-w-md rounded-lg border bg-card p-4 text-card-foreground shadow-lg"
        role="dialog"
        aria-modal="true"
        aria-labelledby="agent-launch-dialog-title"
        onClick={(event) => event.stopPropagation()}
      >
        <div className="flex items-start gap-3">
          <Bot className="mt-0.5 size-5 shrink-0 text-primary" />
          <div className="min-w-0 flex-1">
            <h2 id="agent-launch-dialog-title" className="text-base font-semibold">Start agent</h2>
            <p className="mt-1 text-xs text-muted-foreground">Start an agent in this project’s Herdr workspace.</p>
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

        <div className="mt-4 space-y-3">
          <div>
            <Label htmlFor="agent-launch-kind" className="text-xs font-medium">Agent kind</Label>
            <select
              id="agent-launch-kind"
              value={agentKind}
              onChange={(event) => onAgentKindChange(event.target.value)}
              aria-label="Agent kind"
              disabled={loadingKinds || starting}
              className="mt-1 h-9 w-full rounded-md border border-input bg-background px-2 text-base outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 disabled:cursor-not-allowed disabled:opacity-50 md:text-sm"
            >
              {kinds.map((kind) => <option key={kind} value={kind}>{kind}</option>)}
            </select>
          </div>

          <div>
            <Label htmlFor="agent-launch-tab" className="text-xs font-medium">Tab</Label>
            <Input
              id="agent-launch-tab"
              value={tab}
              onChange={(event) => onTabChange(event.target.value)}
              list="agent-launch-tabs"
              aria-label="Agent tab"
              placeholder="main"
              disabled={starting}
              className="mt-1"
            />
            <datalist id="agent-launch-tabs">
              {tabs.map((tabName) => <option key={tabName} value={tabName}>{tabName}</option>)}
            </datalist>
          </div>
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
          <Button size="sm" onClick={onSubmit} disabled={starting || loadingKinds || !agentKind.trim() || !tab.trim()}>
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
