import { useMemo } from 'react'
import { cn } from '@/lib/utils'

type Row =
  | { kind: 'header'; text: string }
  | { kind: 'hunk'; text: string }
  | { kind: 'meta'; text: string }
  | { kind: 'add'; text: string }
  | { kind: 'del'; text: string }
  | { kind: 'ctx'; text: string }

// buildRows converts a unified diff string into display rows. Header lines
// (diff --git / index / --- / +++ / new file / deleted file / Binary) are
// dimmed, hunk headers highlighted, and +/- lines colored.
function buildRows(diff: string): Row[] {
  const rows: Row[] = []
  for (const raw of diff.split('\n')) {
    const line = raw.replace(/\r$/, '')
    if (
      line.startsWith('diff ') || line.startsWith('index ') || line.startsWith('--- ') ||
      line.startsWith('+++ ') || line.startsWith('new file') || line.startsWith('deleted file') ||
      line.startsWith('similarity index') || line.startsWith('rename ') || line.startsWith('copy ') ||
      line === 'Binary files differ'
    ) {
      rows.push({ kind: 'header', text: line })
    } else if (line.startsWith('@@')) {
      rows.push({ kind: 'hunk', text: line })
    } else if (line.startsWith('+')) {
      rows.push({ kind: 'add', text: line })
    } else if (line.startsWith('-')) {
      rows.push({ kind: 'del', text: line })
    } else {
      rows.push({ kind: 'ctx', text: line })
    }
  }
  return rows
}

interface DiffViewProps {
  diff: string
  className?: string
}

const rowClass: Record<Row['kind'], string> = {
  hunk: 'bg-cyan-500/10 text-cyan-700 dark:text-cyan-300',
  add: 'bg-green-500/10 text-green-700 dark:text-green-300',
  del: 'bg-red-500/10 text-red-700 dark:text-red-300',
  header: 'text-muted-foreground/80',
  meta: 'text-muted-foreground/60',
  ctx: '',
}

export function DiffView({ diff, className }: DiffViewProps) {
  const rows = useMemo(() => buildRows(diff), [diff])
  if (!diff) {
    return (
      <div className={cn('p-4 text-sm text-muted-foreground', className)}>
        No diff available.
      </div>
    )
  }
  return (
    <div
      className={cn('overflow-auto rounded-md border bg-muted/40 font-mono text-xs leading-relaxed', className)}
      aria-label="Diff"
      data-testid="diff-view"
    >
      <pre className="min-w-full py-1">
        {rows.map((row, i) => (
          <div key={i} className={cn('px-2 py-px whitespace-pre-wrap break-all', rowClass[row.kind])}>
            {row.text === '' ? ' ' : row.text}
          </div>
        ))}
      </pre>
    </div>
  )
}