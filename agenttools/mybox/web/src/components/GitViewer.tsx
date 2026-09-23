import { GitBranch, X } from 'lucide-react'
import { GitWorkspace } from '../pages/GitPage'
import { Button } from './ui/button'
import { useEscapeKey } from '../hooks/use-escape-key'

interface GitViewerProps {
  path: string
  onClose: () => void
  refreshMeta: () => Promise<void>
}

export function GitViewer({ path, onClose, refreshMeta }: GitViewerProps) {
  useEscapeKey(onClose)

  return (
    <div
      className="fixed inset-0 z-[60] flex items-center justify-center bg-black/40 p-4"
      onClick={onClose}
      data-testid="git-viewer-backdrop"
    >
      <div
        className="flex h-full max-h-[85vh] w-full max-w-4xl flex-col overflow-hidden rounded-lg border bg-card shadow-xl"
        onClick={(e) => e.stopPropagation()}
        role="dialog"
        aria-modal="true"
        aria-label="Git"
        data-testid="git-viewer"
      >
        <div className="flex shrink-0 items-center gap-2 border-b border-border px-4 py-2.5 pr-2">
          <GitBranch className="size-4 shrink-0 text-muted-foreground" />
          <h2 className="min-w-0 flex-1 truncate text-sm font-semibold">Git — {path}</h2>
          <Button
            variant="ghost"
            size="icon-sm"
            aria-label="Close Git"
            data-testid="git-viewer-close"
            onClick={onClose}
          >
            <X className="size-4" />
          </Button>
        </div>
        <div className="min-h-0 flex-1">
          <GitWorkspace refreshMeta={refreshMeta} scope={path} embedded />
        </div>
      </div>
    </div>
  )
}