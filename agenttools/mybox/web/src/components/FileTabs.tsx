import { useState } from 'react'
import { Star, X } from 'lucide-react'
import { cn } from '@/lib/utils'
import { useEscapeKey } from '../hooks/use-escape-key'

interface FileTabsProps {
  tabs: string[]
  active: string
  favorites: string[]
  onSelect: (path: string) => void
  onClose: (path: string) => void
}

export function FileTabs({ tabs, active, favorites, onSelect, onClose }: FileTabsProps) {
  const [favoritesOpen, setFavoritesOpen] = useState(false)
  useEscapeKey(() => setFavoritesOpen(false), favoritesOpen)

  const selectFavorite = (path: string) => {
    setFavoritesOpen(false)
    onSelect(path)
  }

  return (
    <>
      <div className="file-tabs flex min-h-0 shrink-0 items-center gap-1 overflow-x-auto border-b border-border bg-card px-2 py-1.5">
        <button
          className="flex size-7 shrink-0 cursor-pointer items-center justify-center rounded-md text-muted-foreground hover:bg-muted/60 hover:text-foreground"
          onClick={() => setFavoritesOpen(true)}
          aria-label="Favorites"
          aria-expanded={favoritesOpen}
          title="Favorites"
          data-testid="file-tabs-favorites"
        >
          <Star className="size-4" />
        </button>
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
      {favoritesOpen && (
        <div
          className="fixed inset-0 z-[70] flex items-center justify-center bg-black/40 p-4"
          onClick={() => setFavoritesOpen(false)}
          data-testid="favorites-dialog-backdrop"
        >
          <div
            className="flex max-h-[80vh] w-full max-w-lg flex-col overflow-hidden rounded-lg border bg-card shadow-xl"
            onClick={(event) => event.stopPropagation()}
            role="dialog"
            aria-modal="true"
            aria-label="Favorites"
            data-testid="favorites-dialog"
          >
            <div className="flex shrink-0 items-center gap-2 border-b border-border px-4 py-2.5 pr-2">
              <Star className="size-4 shrink-0 text-muted-foreground" />
              <h2 className="min-w-0 flex-1 truncate text-sm font-semibold">Favorites</h2>
              <button
                className="flex size-7 shrink-0 cursor-pointer items-center justify-center rounded text-muted-foreground hover:bg-muted/60 hover:text-foreground"
                onClick={() => setFavoritesOpen(false)}
                aria-label="Close Favorites"
                title="Close Favorites"
              >
                <X className="size-4" />
              </button>
            </div>
            <div className="min-h-0 overflow-y-auto p-3">
              {favorites.length === 0 ? (
                <p className="px-1 py-4 text-center text-sm text-muted-foreground">No favorites yet.</p>
              ) : (
                <ul className="m-0 flex list-none flex-col gap-1 p-0">
                  {favorites.map((path) => (
                    <li key={path}>
                      <button
                        className="flex w-full cursor-pointer items-center rounded-md px-3 py-2 text-left text-sm text-foreground hover:bg-muted hover:text-primary"
                        onClick={() => selectFavorite(path)}
                        title={path}
                      >
                        <span className="truncate">{path}</span>
                      </button>
                    </li>
                  ))}
                </ul>
              )}
            </div>
          </div>
        </div>
      )}
    </>
  )
}
