import { useMemo, useState } from 'react'
import { ChevronDown, ChevronsDownUp, ChevronsUpDown, FileText } from 'lucide-react'
import { DiffView } from './DiffView'
import { Button } from './ui/button'
import { cn } from '@/lib/utils'

interface FileDiffSegment {
  path: string
  name: string
  dir: string
  content: string
}

// parseDiffPath extracts the b/ path from a `diff --git a/x b/y` header line.
// For renames the second token holds the destination; git quotes paths that
// contain unusual characters, so strip a leading/trailing quote pair.
function parseDiffPath(line: string): string {
  const rest = line.slice('diff --git '.length).trim()
  const tokens = rest.match(/"(?:[^"\\]|\\.)*"|[^\s]+/g) ?? []
  const bPath = tokens[1] ?? tokens[0] ?? ''
  return bPath.replace(/^b\//, '').replace(/^"(.*)"$/, '$1')
}

// splitCommitDiff slices a multi-file unified diff into per-file segments.
// git show output carries a commit header before the first `diff --git`
// line, which is intentionally dropped.
export function splitCommitDiff(diff: string): FileDiffSegment[] {
  const segments: FileDiffSegment[] = []
  let current: string[] = []
  let path = ''

  const flush = () => {
    if (path && current.length > 0) {
      const idx = path.lastIndexOf('/')
      segments.push({
        path,
        name: idx < 0 ? path : path.slice(idx + 1),
        dir: idx < 0 ? '' : path.slice(0, idx),
        content: current.join('\n'),
      })
    }
    current = []
    path = ''
  }

  for (const line of diff.split('\n')) {
    if (line.startsWith('diff --git ')) {
      flush()
      path = parseDiffPath(line)
      current = [line]
    } else if (path) {
      current.push(line)
    }
  }
  flush()
  return segments
}

interface CommitDiffViewProps {
  diff: string
}

export function CommitDiffView({ diff }: CommitDiffViewProps) {
  const segments = useMemo(() => splitCommitDiff(diff), [diff])
  const [collapsed, setCollapsed] = useState<Set<string>>(new Set())
  const [allCollapsed, setAllCollapsed] = useState(false)

  const toggle = (path: string) =>
    setCollapsed((prev) => {
      const next = new Set(prev)
      if (next.has(path)) next.delete(path)
      else next.add(path)
      return next
    })

  if (segments.length === 0) {
    return <DiffView diff={diff} />
  }

  const isCollapsed = (path: string) => (allCollapsed ? !collapsed.has(path) : collapsed.has(path))

  const setAll = (value: boolean) => {
    setAllCollapsed(value)
    setCollapsed(new Set())
  }

  return (
    <div aria-label="Commit diff" data-testid="commit-diff-view">
      <div className="mb-2 flex items-center justify-between gap-2">
        <h3 className="text-sm font-semibold text-muted-foreground">
          {segments.length} file{segments.length === 1 ? '' : 's'}
        </h3>
        <div className="flex items-center gap-1">
          <Button variant="ghost" size="sm" onClick={() => setAll(false)} title="Expand all">
            <ChevronsUpDown />
            <span className="hidden sm:inline">Expand all</span>
          </Button>
          <Button variant="ghost" size="sm" onClick={() => setAll(true)} title="Collapse all">
            <ChevronsDownUp />
            <span className="hidden sm:inline">Collapse all</span>
          </Button>
        </div>
      </div>
      {segments.map((seg) => {
        const open = !isCollapsed(seg.path)
        return (
          <div
            key={seg.path}
            className="mb-2 overflow-hidden rounded-md border border-border"
          >
            <button
              type="button"
              onClick={() => toggle(seg.path)}
              className="flex w-full cursor-pointer items-center gap-2 px-2 py-1.5 text-left hover:bg-muted"
              aria-expanded={open}
              data-testid="commit-diff-file"
            >
              <ChevronDown
                className={cn('size-3.5 shrink-0 text-muted-foreground transition-transform', !open && '-rotate-90')}
              />
              <FileText className="size-3.5 shrink-0 text-muted-foreground" />
              <span className="min-w-0 truncate font-mono text-xs">
                {seg.dir && <span className="text-muted-foreground">{seg.dir}/</span>}
                {seg.name}
              </span>
            </button>
            {open && <DiffView diff={seg.content} className="rounded-none border-0" />}
          </div>
        )
      })}
    </div>
  )
}