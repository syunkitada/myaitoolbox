import { useEffect, useRef, useState } from 'react'
import { Rocket, X } from 'lucide-react'
import { api, type Task, type TaskTrigger, type TaskTriggerType } from '../api/client'
import { Button } from './ui/button'
import { Input } from './ui/input'
import { Label } from './ui/label'
import { useEscapeKey } from '../hooks/use-escape-key'

const AGENT_KIND_OPTIONS = ['opencode', 'codex'] as const
const DEFAULT_AGENT_KIND = 'codex'
const DEFAULT_CRON = '0 9 * * 1-5'
const DEFAULT_TIMEZONE = 'Asia/Tokyo'
const DEFAULT_WATCH_PATH = 'incoming'
const DEFAULT_PATTERN = '*'
const DEFAULT_TRIGGER_PROMPT = "'$task_file_path' を実施してください。"

type CreationKind = 'task' | 'task_trigger'

const FRONT_MATTER_PATTERN = /^---\r?\n[\s\S]*?\r?\n---(?:\r?\n|$)/

function splitTaskTemplate(content: string): { header: string; body: string } {
  const match = content.match(FRONT_MATTER_PATTERN)
  if (!match) return { header: '', body: content }
  return { header: match[0], body: content.slice(match[0].length) }
}

function composeTaskContent(template: string, body: string): string {
  return splitTaskTemplate(template).header + body
}

function slugifyTriggerID(value: string): string {
  return value
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
}

export interface NewTaskDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  onCreated?: (task: Task) => void
  onTriggerCreated?: (trigger: TaskTrigger) => void
  onError?: (message: string) => void
}

export function NewTaskDialog({
  open,
  onOpenChange,
  onCreated,
  onTriggerCreated,
  onError,
}: NewTaskDialogProps) {
  const [creationKind, setCreationKind] = useState<CreationKind>('task')
  const [name, setName] = useState('')
  const [content, setContent] = useState('')
  const contentEditedRef = useRef(false)
  const [templateContent, setTemplateContent] = useState('')
  const [templateReady, setTemplateReady] = useState(false)
  const [agentKind, setAgentKind] = useState(DEFAULT_AGENT_KIND)
  const [prompt, setPrompt] = useState('')
  const [triggerID, setTriggerID] = useState('')
  const [triggerIDEdited, setTriggerIDEdited] = useState(false)
  const [triggerType, setTriggerType] = useState<TaskTriggerType>('cron')
  const [cron, setCron] = useState(DEFAULT_CRON)
  const [timezone, setTimezone] = useState(DEFAULT_TIMEZONE)
  const [watchPath, setWatchPath] = useState(DEFAULT_WATCH_PATH)
  const [pattern, setPattern] = useState(DEFAULT_PATTERN)
  const [creating, setCreating] = useState(false)

  const reset = () => {
    setCreationKind('task')
    setName('')
    setContent('')
    contentEditedRef.current = false
    setTemplateContent('')
    setTemplateReady(false)
    setAgentKind(DEFAULT_AGENT_KIND)
    setPrompt('')
    setTriggerID('')
    setTriggerIDEdited(false)
    setTriggerType('cron')
    setCron(DEFAULT_CRON)
    setTimezone(DEFAULT_TIMEZONE)
    setWatchPath(DEFAULT_WATCH_PATH)
    setPattern(DEFAULT_PATTERN)
  }

  useEffect(() => {
    if (open) reset()
  }, [open])

  useEffect(() => {
    if (!open) return
    let cancelled = false
    setTemplateReady(false)
    void api.getTaskTemplate(name.trim()).then(
      (template) => {
        if (!cancelled) {
          setTemplateContent(template.content)
          if (!contentEditedRef.current) setContent(splitTaskTemplate(template.content).body)
          setTemplateReady(true)
        }
      },
      (e) => {
        if (!cancelled) {
          setTemplateReady(true)
          onError?.(e instanceof Error ? e.message : String(e))
        }
      },
    )
    return () => {
      cancelled = true
    }
  }, [open, name, onError])

  const close = () => {
    reset()
    onOpenChange(false)
  }

  useEscapeKey(close, open)

  const reportError = (e: unknown) => {
    onError?.(e instanceof Error ? e.message : String(e))
  }

  const changeCreationKind = (kind: CreationKind) => {
    setCreationKind(kind)
    if (kind === 'task_trigger') {
      if (!agentKind) setAgentKind(DEFAULT_AGENT_KIND)
      if (!prompt.trim()) setPrompt(DEFAULT_TRIGGER_PROMPT)
    } else if (prompt.trim() === DEFAULT_TRIGGER_PROMPT) {
      setPrompt('')
    }
  }

  const submit = async () => {
    const trimmed = name.trim()
    if (!trimmed) return
    setCreating(true)
    try {
      if (creationKind === 'task_trigger') {
        const trigger = await api.createTaskTrigger({
          id: triggerID.trim(),
          task: {
            name: trimmed,
            agent_kind: agentKind,
            content: composeTaskContent(templateContent, content),
            ...(prompt.trim() ? { prompt: prompt.trim() } : {}),
          },
          trigger:
            triggerType === 'cron'
              ? {
                  type: triggerType,
                  cron: cron.trim(),
                  timezone: timezone.trim(),
                }
              : triggerType === 'file_created'
                ? {
                    type: triggerType,
                    path: watchPath.trim(),
                    pattern: pattern.trim(),
                  }
                : { type: triggerType },
        })
        reset()
        onOpenChange(false)
        onTriggerCreated?.(trigger)
        return
      }
      const task = await api.createTask({
        name: trimmed,
        agent_kind: agentKind || undefined,
        content: composeTaskContent(templateContent, content),
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
        className="max-h-[90vh] w-full max-w-4xl overflow-y-auto rounded-lg border bg-card p-5 shadow-xl"
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
              onChange={(e) => {
                const value = e.target.value
                setName(value)
                if (!triggerIDEdited) setTriggerID(slugifyTriggerID(value))
              }}
              onKeyDown={(e) => {
                if (e.key === 'Enter') void submit()
              }}
              placeholder="e.g. Refactor authentication flow"
              autoFocus
            />
          </div>

          <div>
            <Label htmlFor="new-task-creation-kind" className="mb-1.5 block text-sm">
              作成種別
            </Label>
            <select
              id="new-task-creation-kind"
              className="h-9 w-full rounded-md border bg-transparent px-3 text-sm"
              value={creationKind}
              onChange={(e) => changeCreationKind(e.target.value as CreationKind)}
            >
              <option value="task">task</option>
              <option value="task_trigger">task_trigger</option>
            </select>
          </div>

          {creationKind === 'task_trigger' && (
            <>
              <div>
                <Label htmlFor="new-task-trigger-id" className="mb-1.5 block text-sm">
                  Trigger ID (required)
                </Label>
                <Input
                  id="new-task-trigger-id"
                  value={triggerID}
                  onChange={(e) => {
                    setTriggerIDEdited(true)
                    setTriggerID(e.target.value)
                  }}
                  placeholder="e.g. daily-report"
                />
              </div>

              <div>
                <Label htmlFor="new-task-trigger-type" className="mb-1.5 block text-sm">
                  Trigger type
                </Label>
                <select
                  id="new-task-trigger-type"
                  className="h-9 w-full rounded-md border bg-transparent px-3 text-sm"
                  value={triggerType}
                  onChange={(e) => setTriggerType(e.target.value as TaskTriggerType)}
                >
                  <option value="cron">cron</option>
                  <option value="file_created">file_created</option>
                  <option value="manual">manual</option>
                </select>
              </div>

              {triggerType === 'cron' ? (
                <div className="grid grid-cols-2 gap-3">
                  <div>
                    <Label htmlFor="new-task-trigger-cron" className="mb-1.5 block text-sm">
                      Cron (required)
                    </Label>
                    <Input
                      id="new-task-trigger-cron"
                      value={cron}
                      onChange={(e) => setCron(e.target.value)}
                      placeholder="0 9 * * 1-5"
                    />
                  </div>
                  <div>
                    <Label htmlFor="new-task-trigger-timezone" className="mb-1.5 block text-sm">
                      Timezone (required)
                    </Label>
                    <Input
                      id="new-task-trigger-timezone"
                      value={timezone}
                      onChange={(e) => setTimezone(e.target.value)}
                      placeholder="Asia/Tokyo"
                    />
                  </div>
                </div>
              ) : triggerType === 'file_created' ? (
                <div className="grid grid-cols-2 gap-3">
                  <div>
                    <Label htmlFor="new-task-trigger-path" className="mb-1.5 block text-sm">
                      Watch directory (required)
                    </Label>
                    <Input
                      id="new-task-trigger-path"
                      value={watchPath}
                      onChange={(e) => setWatchPath(e.target.value)}
                      placeholder="incoming"
                    />
                  </div>
                  <div>
                    <Label htmlFor="new-task-trigger-pattern" className="mb-1.5 block text-sm">
                      Pattern (optional)
                    </Label>
                    <Input
                      id="new-task-trigger-pattern"
                      value={pattern}
                      onChange={(e) => setPattern(e.target.value)}
                      placeholder="*"
                    />
                  </div>
                </div>
              ) : null}
            </>
          )}

          <div>
            <Label htmlFor="new-task-content" className="mb-1.5 block text-sm">
              task.md本文 (YAMLヘッダー除外)
            </Label>
            <textarea
              id="new-task-content"
              value={content}
              onChange={(e) => {
                contentEditedRef.current = true
                setContent(e.target.value)
              }}
              rows={18}
              spellCheck={false}
              className="min-h-[360px] w-full resize-y rounded-md border bg-transparent px-3 py-2 font-mono text-sm leading-6 outline-none focus:ring-2 focus:ring-ring"
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

          {creationKind === 'task_trigger' ? (
            <p className="text-xs text-muted-foreground">
              task.mdとtrigger.yamlを生成します。
              {triggerType === 'manual'
                ? ' manualはCLIまたはtriggerディレクトリの右クリックから実行したときだけタスクを生成します。'
                : ' 作成後はautomation daemonが条件に応じてタスクを生成し、Agentを起動します。'}
            </p>
          ) : (
            <p className="text-xs text-muted-foreground">
              Agent kind 指定時、herdr の新規タブ（タスクディレクトリ名）で agent を起動してタスクに結び付け、Prompt
              （テンプレート名 or インライン指示）を即座に送信します。タスクディレクトリ内のファイルは同じ agent
              を共有します。
            </p>
          )}

          <div className="flex justify-end gap-2 pt-1">
            <Button variant="outline" size="sm" onClick={close}>
              Cancel
            </Button>
            <Button
              size="sm"
              onClick={() => void submit()}
              disabled={
                !name.trim() ||
                !templateReady ||
                creating ||
                (creationKind === 'task_trigger' &&
                  (!triggerID.trim() || !agentKind ||
                    (triggerType === 'cron' ? !cron.trim() || !timezone.trim() : !watchPath.trim())))
              }
            >
              <Rocket />
              <span>{creating ? 'Creating…' : 'Create'}</span>
            </Button>
          </div>
        </div>
      </div>
    </div>
  )
}
