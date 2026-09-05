import { X } from 'lucide-react'
import { cn } from '@/lib/utils'

interface FileTabsProps {
  tabs: string[]
  active: string
  onSelect: (path: string) => void
  onClose: (path: string) => void
}

export function FileTabs({ tabs, active, onSelect, onClose }: FileTabsProps) {
  if (tabs.length === 0) return null
  return (
    <div className="file-tabs flex min-h-0 shrink-0 items-center gap-1 overflow-x-auto border-b border-border bg-card px-2 py-1.5">
      {tabs.map((p) => {
        const base = p.split('/').pop() ?? p
        const isActive = p === active
        return (
          <div
            key={p}
            className={cn(
              'group flex max-w-56 shrink-0 items-center rounded-md border text-xs',
              isActive
                ? 'border-border bg-muted text-foreground'
                : 'border-transparent text-muted-foreground hover:bg-muted/60 hover:text-foreground',
            )}
          >
            <button
              className="flex min-w-0 cursor-pointer items-center px-2.5 py-1.5"
              onClick={() => onSelect(p)}
              title={p}
            >
              <span className="truncate">{base}</span>
            </button>
            <button
              className="mr-1 flex size-5 shrink-0 cursor-pointer items-center justify-center rounded text-muted-foreground hover:bg-border/60 hover:text-foreground"
              onClick={() => onClose(p)}
              aria-label={`Close ${base}`}
              title={`Remove ${p} from recent`}
            >
              <X className="size-3" />
            </button>
          </div>
        )
      })}
    </div>
  )
}