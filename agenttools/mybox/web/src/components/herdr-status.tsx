import { cn } from '@/lib/utils'

export type AgentStatus = 'working' | 'blocked' | 'idle' | 'done' | string

export const AGENT_STATUS_CLASSES: Record<string, string> = {
  working: 'bg-blue-500',
  blocked: 'bg-red-500',
  idle: 'bg-emerald-500',
  done: 'bg-violet-500',
  unknown: 'bg-zinc-400',
}

export function statusDotClass(status: AgentStatus): string {
  return AGENT_STATUS_CLASSES[status] ?? AGENT_STATUS_CLASSES.unknown
}

interface StatusDotProps {
  status: AgentStatus
  className?: string
}

export function StatusDot({ status, className }: StatusDotProps) {
  return (
    <span
      role="img"
      aria-label={`agent status ${status}`}
      title={status}
      className={cn(
        'inline-block size-2 shrink-0 rounded-full',
        statusDotClass(status),
        status === 'working' && 'animate-pulse',
        className,
      )}
    />
  )
}

const STATUS_BADGE_BASE =
  'inline-flex items-center gap-1 rounded-md border px-1.5 py-0.5 text-[11px] leading-none font-medium'

const STATUS_BADGE_STYLES: Record<string, string> = {
  working: 'border-blue-200 bg-blue-50 text-blue-700 dark:border-blue-800 dark:bg-blue-950 dark:text-blue-300',
  blocked: 'border-red-200 bg-red-50 text-red-700 dark:border-red-800 dark:bg-red-950 dark:text-red-300',
  idle: 'border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-800 dark:bg-emerald-950 dark:text-emerald-300',
  done: 'border-violet-200 bg-violet-50 text-violet-700 dark:border-violet-800 dark:bg-violet-950 dark:text-violet-300',
  unknown:
    'border-zinc-200 bg-zinc-50 text-zinc-600 dark:border-zinc-800 dark:bg-zinc-900 dark:text-zinc-400',
}

export function StatusBadge({ status }: { status: AgentStatus }) {
  return (
    <span className={cn(STATUS_BADGE_BASE, STATUS_BADGE_STYLES[status] ?? STATUS_BADGE_STYLES.unknown)}>
      {status}
    </span>
  )
}
