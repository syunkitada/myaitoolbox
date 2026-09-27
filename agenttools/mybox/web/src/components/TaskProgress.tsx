import { cn } from '@/lib/utils'

interface TaskProgressProps {
  completed: number
  total: number
  className?: string
}

export function TaskProgress({ completed, total, className }: TaskProgressProps) {
  const safeTotal = Math.max(0, total)
  const safeCompleted = Math.min(Math.max(0, completed), safeTotal)
  const percentage = safeTotal === 0 ? 0 : (safeCompleted / safeTotal) * 100

  return (
    <div
      className={cn('task-progress flex min-w-0 items-center gap-2 text-xs text-muted-foreground', className)}
      data-testid="task-progress"
    >
      <span className="task-progress-count shrink-0 whitespace-nowrap" aria-label={`${safeCompleted} of ${safeTotal} tasks completed`}>
        {safeCompleted}/{safeTotal}
      </span>
      <div
        className="task-progress-track h-1.5 min-w-0 flex-1 overflow-hidden rounded-full bg-muted"
        role="progressbar"
        aria-label="Task progress"
        aria-valuemin={0}
        aria-valuemax={safeTotal}
        aria-valuenow={safeCompleted}
      >
        <div
          className="task-progress-bar h-full rounded-full bg-primary transition-[width]"
          style={{ width: `${percentage}%` }}
        />
      </div>
    </div>
  )
}
