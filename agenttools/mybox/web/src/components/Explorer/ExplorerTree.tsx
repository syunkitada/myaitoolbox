import { useEffect, useRef, useState } from 'react'
import type { ReactNode } from 'react'
import type { FileTreeNode, NodeId } from '../../graph/types'
import { dirId, fileId } from '../../graph/nodeId'
import { ContextMenu } from '../ContextMenu'
import { cn } from '@/lib/utils'

export interface ExplorerTreeProps {
  tree: FileTreeNode[]
  expandedNodeIds: Set<NodeId>
  selectedNodeId: NodeId | null
  onToggle: (id: NodeId) => void
  onSelect: (id: NodeId) => void
  onOpenFile?: (path: string) => void
  onExpandAllDir?: (id: NodeId) => void
  onCollapseAllDir?: (id: NodeId) => void
}

function FileIcon({ markdown, expanded }: { markdown: boolean; expanded?: boolean }) {
  return (
    <span
      className={cn(
        'flex size-4 shrink-0 items-center justify-center text-[10px] font-bold',
        expanded ? 'text-primary' : 'text-muted-foreground',
      )}
      aria-hidden="true"
    >
      {markdown ? 'M' : '▤'}
    </span>
  )
}

function Caret({ open, onToggle, label }: { open: boolean; onToggle: () => void; label: string }) {
  return (
    <button
      className="flex h-6 w-6 shrink-0 cursor-pointer items-center justify-center rounded p-0 text-muted-foreground hover:bg-muted hover:text-primary"
      aria-label={open ? `Collapse ${label}` : `Expand ${label}`}
      onClick={(e) => {
        e.stopPropagation()
        onToggle()
      }}
    >
      <svg
        className={cn('h-3 w-3 shrink-0 transition-transform', open && 'rotate-90')}
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth="2.5"
        strokeLinecap="round"
        strokeLinejoin="round"
        aria-hidden="true"
      >
        <polyline points="9 18 15 12 9 6" />
      </svg>
    </button>
  )
}

export function ExplorerTree({
  tree,
  expandedNodeIds,
  selectedNodeId,
  onToggle,
  onSelect,
  onOpenFile,
  onExpandAllDir,
  onCollapseAllDir,
}: ExplorerTreeProps) {
  const listRef = useRef<HTMLUListElement | null>(null)
  const [menu, setMenu] = useState<{ x: number; y: number; id: NodeId } | null>(null)

  useEffect(() => {
    if (!selectedNodeId) return
    const el = listRef.current?.querySelector<HTMLElement>(`[data-node-id="${cssEscape(selectedNodeId)}"]`)
    el?.scrollIntoView({ block: 'nearest' })
  }, [selectedNodeId])

  const items: ReactNode[] = []
  const renderNodes = (nodes: FileTreeNode[], depth: number) => {
    for (const node of nodes) {
      const pad = depth * 14
      if (node.kind === 'dir') {
        const id = dirId(node.path)
        const open = expandedNodeIds.has(id)
        const selected = selectedNodeId === id
        items.push(
          <li
            key={id}
            data-node-id={id}
            className={cn(
              'flex min-h-[26px] items-center gap-1 rounded-md px-1 leading-[1.4] hover:bg-muted',
              selected && 'bg-accent',
            )}
            style={{ paddingLeft: pad }}
            onContextMenu={(e) => {
              e.preventDefault()
              setMenu({ x: e.clientX, y: e.clientY, id })
            }}
          >
            <Caret
              open={open}
              label={node.name}
              onToggle={() => onToggle(id)}
            />
            <button
              className={cn(
                'flex min-w-0 flex-1 cursor-pointer items-center gap-1 self-stretch bg-transparent p-0 text-left text-sm font-semibold whitespace-nowrap text-foreground overflow-hidden text-ellipsis hover:text-primary',
                selected && 'active text-primary',
              )}
              title={node.path}
              onClick={() => {
                onSelect(id)
                if (!open) onToggle(id)
              }}
            >
              <FileIcon markdown={false} expanded={open} />
              <span className="truncate">{node.name}</span>
            </button>
          </li>,
        )
        if (open) renderNodes(node.children ?? [], depth + 1)
      } else {
        const id = fileId(node.path)
        const selected = selectedNodeId === id
        items.push(
          <li
            key={id}
            data-node-id={id}
            className={cn(
              'flex min-h-[26px] items-center gap-1 rounded-md px-1 leading-[1.4] hover:bg-muted',
              selected && 'bg-accent',
            )}
            style={{ paddingLeft: pad + 24 }}
          >
            <button
              className={cn(
                'flex min-w-0 flex-1 cursor-pointer items-center gap-1 self-stretch bg-transparent p-0 text-left text-sm whitespace-nowrap text-foreground overflow-hidden text-ellipsis hover:text-primary',
                selected && 'active text-primary font-semibold',
              )}
              title={node.path}
              onDoubleClick={() => onOpenFile?.(node.path)}
              onClick={() => onSelect(id)}
            >
              <FileIcon markdown={node.markdown} />
              <span className="truncate">{node.name}</span>
            </button>
          </li>,
        )
      }
    }
  }
  renderNodes(tree, 0)

  return (
    <div className="knowledge-explorer flex h-full min-h-0 w-full flex-col overflow-y-auto bg-card p-2.5 text-sm">
      <h2 className="mb-2 px-1 text-xs font-semibold tracking-wider text-muted-foreground uppercase">
        File Explorer
      </h2>
      {tree.length === 0 ? (
        <p className="px-1 text-xs text-muted-foreground">No files.</p>
      ) : (
        <ul ref={listRef} className="knowledge-tree m-0 list-none p-0">
          {items}
        </ul>
      )}
      <p className="mt-3 border-t border-border px-1 pt-3 text-xs leading-relaxed text-muted-foreground">
        Expanding a folder in the Explorer reveals its children in the Graph.
        Markdown links between the visible files are drawn as edges.
      </p>
      {menu && (
        <ContextMenu
          x={menu.x}
          y={menu.y}
          onClose={() => setMenu(null)}
          items={[
            { label: 'Expand all', onSelect: () => onExpandAllDir?.(menu.id) },
            { label: 'Collapse all', onSelect: () => onCollapseAllDir?.(menu.id) },
          ]}
        />
      )}
    </div>
  )
}

function cssEscape(value: string): string {
  return value.replace(/["\\]/g, '\\$&').replace(/[^a-zA-Z0-9_:.\-\\]/g, (c) => `\\${c}`)
}