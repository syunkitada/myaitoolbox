import { useEffect, useState } from 'react'
import { Rocket, X } from 'lucide-react'
import { api, type Task } from '../api/client'
import { Button } from './ui/button'
import { Input } from './ui/input'
import { Label } from './ui/label'
import { useEscapeKey } from '../hooks/use-escape-key'

const AGENT_KIND_OPTIONS = ['opencode', 'codex'] as const

export interface NewTaskDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  onCreated?: (task: Task) => void
  onError?: (message: string) => void
}

export function NewTaskDialog({
  open,
  onOpenChange,
  onCreated,
  onError,
}: NewTaskDialogProps) {
  const [name, setName] = useState('')
  const [agentKind, setAgentKind] = useState('')
  const [prompt, setPrompt] = useState('')
  const [creating, setCreating] = useState(false)

  useEffect(() => {
    if (open) {
      setName('')
      setAgentKind('')
      setPrompt('')
    }
  }, [open])

  const reset = () => {
    setName('')
    setAgentKind('')
    setPrompt('')
  }

  const close = () => {
    reset()
    onOpenChange(false)
  }

  useEscapeKey(close, open)

  const reportError = (e: unknown) => {
    onError?.(e instanceof Error ? e.message : String(e))
  }

  const submit = async () => {
    const trimmed = name.trim()
    if (!trimmed) return
    setCreating(true)
    try {
      const task = await api.createTask({
        name: trimmed,
        agent_kind: agentKind || undefined,
      })
      if (agentKind || prompt.trim()) {
        try {
          await api.startTaskAgent(task.id, {
            kind: agentKind || undefined,
            prompt: prompt.trim() || undefined,
          })
        } catch (e) {
          // The task was created; surface the agent failure but keep the task.
          reportError(e)
        }
      }
      reset()
      onOpenChange(false)
      onCreated?.(task)
    } catch (e) {
      reportError(e)
    } finally {
      setCreating(false)
    }
  }

  if (!open) return null

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4"
      onClick={close}
    >
      <div
        className="w-full max-w-md rounded-lg border bg-card p-5 shadow-xl"
        onClick={(e) => e.stopPropagation()}
        role="dialog"
        aria-modal="true"
        aria-label="New task"
      >
        <div className="mb-4 flex items-center justify-between">
          <h2 className="text-lg font-semibold">New task</h2>
          <Button variant="ghost" size="icon" aria-label="Close" onClick={close}>
            <X className="h-4 w-4" />
          </Button>
        </div>

        <div className="space-y-4">
          <div>
            <Label htmlFor="new-task-name" className="mb-1.5 block text-sm">
              タスク名 (required)
            </Label>
            <Input
              id="new-task-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter') void submit()
              }}
              placeholder="e.g. 認証フローの改修"
              autoFocus
            />
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div>
              <Label htmlFor="new-task-agent-kind" className="mb-1.5 block text-sm">
                Agent kind <span className="text-muted-foreground">(optional)</span>
              </Label>
              <select
                id="new-task-agent-kind"
                className="h-9 w-full rounded-md border bg-transparent px-3 text-sm"
                value={agentKind}
                onChange={(e) => setAgentKind(e.target.value)}
              >
                <option value="">none</option>
                {AGENT_KIND_OPTIONS.map((k) => (
                  <option key={k} value={k}>
                    {k}
                  </option>
                ))}
              </select>
            </div>
            <div>
              <Label htmlFor="new-task-prompt" className="mb-1.5 block text-sm">
                Prompt <span className="text-muted-foreground">(optional)</span>
              </Label>
              <Input
                id="new-task-prompt"
                value={prompt}
                onChange={(e) => setPrompt(e.target.value)}
                placeholder="template name or @name"
                disabled={!agentKind}
              />
            </div>
          </div>

          <p className="text-xs text-muted-foreground">
            Agent kind 指定時、herdr の新規タブ（タスクディレクトリ名）で agent を起動してタスクに結び付け、Prompt
            （テンプレート名 or インライン指示）を即座に送信します。タスクディレクトリ内のファイルは同じ agent
            を共有します。
          </p>

          <div className="flex justify-end gap-2 pt-1">
            <Button variant="outline" size="sm" onClick={close}>
              Cancel
            </Button>
            <Button size="sm" onClick={() => void submit()} disabled={!name.trim() || creating}>
              <Rocket />
              <span>{creating ? 'Creating…' : 'Create'}</span>
            </Button>
          </div>
        </div>
      </div>
    </div>
  )
}