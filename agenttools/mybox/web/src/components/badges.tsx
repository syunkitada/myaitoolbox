import { cn } from '@/lib/utils'
import { Badge } from '@/components/ui/badge'

export function statusStyle(status: string): string {
  switch (status) {
    case 'todo':
      return 'border-transparent bg-blue-100 text-blue-800'
    case 'doing':
      return 'border-transparent bg-green-100 text-green-800'
    case 'blocked':
      return 'border-transparent bg-red-100 text-red-800'
    case 'review':
      return 'border-transparent bg-amber-100 text-amber-800'
    case 'done':
      return 'border-transparent bg-slate-200 text-slate-700'
    default:
      return ''
  }
}

export function StatusBadge({ status, className }: { status: string; className?: string }) {
  return (
    <Badge variant="outline" className={cn('badge', `status-${status}`, statusStyle(status), className)}>
      {status}
    </Badge>
  )
}

export function PriorityBadge({ priority, className }: { priority: string; className?: string }) {
  const style =
    priority === 'high' || priority === 'urgent'
      ? 'border-transparent bg-red-100 text-red-800'
      : 'border-transparent bg-slate-100 text-slate-600'
  return (
    <Badge variant="outline" className={cn('badge', `priority-${priority}`, style, className)}>
      {priority}
    </Badge>
  )
}

export function TagBadge({ children, className }: { children: React.ReactNode; className?: string }) {
  return (
    <Badge variant="outline" className={cn('badge tag border-transparent bg-indigo-100 text-indigo-700', className)}>
      {children}
    </Badge>
  )
}

const DUE_STYLES: Record<string, string> = {
  overdue: 'border-transparent bg-red-100 text-red-700',
  soon: 'border-transparent bg-amber-100 text-amber-700',
  far: 'border-transparent bg-green-100 text-green-700',
}

export function DueBadge({ due, dueClass, className }: { due: string; dueClass: string; className?: string }) {
  return (
    <Badge
      variant="outline"
      className={cn('badge due', DUE_STYLES[dueClass] ?? 'border-transparent bg-slate-100 text-slate-600', className)}
    >
      {due}
    </Badge>
  )
}

export function ProjectBadge({ children, className }: { children: React.ReactNode; className?: string }) {
  return (
    <Badge
      variant="outline"
      className={cn('badge project-badge border-transparent bg-blue-100 font-semibold text-blue-700', className)}
    >
      {children}
    </Badge>
  )
}

export function PendingBadge({ children, className }: { children: React.ReactNode; className?: string }) {
  return (
    <Badge variant="outline" className={cn('badge pending-until border-transparent bg-amber-100 text-amber-700', className)}>
      {children}
    </Badge>
  )
}
