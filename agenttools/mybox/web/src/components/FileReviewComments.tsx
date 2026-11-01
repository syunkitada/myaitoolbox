import { ReactNode, useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { Check, MessageSquarePlus, Pencil, Reply, Trash2, X } from 'lucide-react'
import { Button } from './ui/button'
import { Textarea } from './ui/textarea'
import {
  createFileReviewThread,
  FILE_REVIEW_COMMENTS_EVENT,
  fileReviewStorageKey,
  formatFileReviewThreadForAgent,
  isFileReviewAnchorStale,
  loadFileReviewThreads,
  replyToFileReviewThread,
  resolveFileReviewThread,
  saveFileReviewThreads,
  updateFileReviewMessage,
  type FileReviewAnchor,
  type FileReviewMessage,
  type FileReviewThread,
} from '../utils/file-review-comments'
import { cn } from '@/lib/utils'

interface FileReviewCommentsProps {
  project: string
  path: string
  sourceText: string
  activeAgentPaneId: string | null
  activeCodexThreadId?: string | null
  onAppendToAgentDraft?: (paneId: string, text: string) => void
  onAppendToCodexDraft?: (threadId: string, text: string) => void
  children: ReactNode
}

interface Snapshot {
  key: string
  threads: FileReviewThread[]
  error: string | null
}

interface PortalMount {
  key: string
  element: HTMLDivElement
  threads: FileReviewThread[]
}

interface LineComposerMount {
  key: string
  element: HTMLDivElement
  anchor: Extract<FileReviewAnchor, { kind: 'lines' }>
}

type SelectionToolbarPosition =
  | { kind: 'floating'; left: number; top: number }
  | { kind: 'bottom' }

function readSnapshot(project: string, path: string, key: string): Snapshot {
  const loaded = loadFileReviewThreads(project, path)
  return { key, threads: loaded.threads, error: loaded.error }
}

function closestElement(node: Node): HTMLElement | null {
  const element = node.nodeType === Node.ELEMENT_NODE ? node as HTMLElement : node.parentElement
  return element
}

function closestSourceElement(node: Node): HTMLElement | null {
  return closestElement(node)?.closest<HTMLElement>('[data-review-line], [data-source-line]') ?? null
}

function lineAtBoundary(node: Node, offset: number, boundary: 'start' | 'end'): number | null {
  const sourceElement = closestSourceElement(node)
  if (!sourceElement) return null

  const explicitLine = sourceElement.dataset.reviewLine
  if (explicitLine) {
    const line = Number(explicitLine)
    return Number.isInteger(line) && line >= 1 ? line : null
  }

  const codeBlock = sourceElement.closest<HTMLElement>('.markdown-code-block')
  const startLine = Number(codeBlock?.dataset.reviewCodeLine ?? sourceElement.dataset.sourceLine)
  if (!Number.isInteger(startLine) || startLine < 1) return null
  if (codeBlock) {
    const code = codeBlock.querySelector('code')
    if (code && (node === code || code.contains(node))) {
      try {
        const range = document.createRange()
        range.selectNodeContents(code)
        range.setEnd(node, offset)
        return startLine + range.toString().split(/\r?\n/).length - 1
      } catch {
        return startLine
      }
    }
  }
  const endLine = Number(sourceElement.dataset.sourceEndLine)
  return boundary === 'end' && Number.isInteger(endLine) && endLine >= startLine ? endLine : startLine
}

function reviewAnchorFromSelection(root: HTMLElement, sourceText: string): Extract<FileReviewAnchor, { kind: 'lines' }> | null {
  const selection = window.getSelection()
  if (!selection || selection.rangeCount === 0) return null
  const selectedText = selection.toString().trim()
  if (!selectedText) return null
  const range = selection.getRangeAt(0)
  if (!root.contains(range.startContainer) || !root.contains(range.endContainer)) return null
  const firstLine = lineAtBoundary(range.startContainer, range.startOffset, 'start')
  const lastLine = lineAtBoundary(range.endContainer, range.endOffset, 'end')
  if (firstLine === null || lastLine === null) return null
  const startLine = Math.min(firstLine, lastLine)
  const endLine = Math.max(firstLine, lastLine)
  const quote = startLine === endLine
    ? selectedText
    : sourceText.split(/\r?\n/).slice(startLine - 1, endLine).join('\n').trim() || selectedText
  return {
    kind: 'lines',
    startLine,
    endLine,
    quote,
  }
}

function targetForLine(root: HTMLElement, anchorLine: number): HTMLElement | null {
  const candidates = Array.from(root.querySelectorAll<HTMLElement>('[data-review-line], [data-source-line]'))
  const matches = candidates.filter((candidate) => {
    const start = Number(candidate.dataset.reviewLine ?? candidate.dataset.sourceLine)
    const end = Number(candidate.dataset.sourceEndLine ?? candidate.dataset.reviewLine ?? candidate.dataset.sourceLine)
    return Number.isInteger(start) && Number.isInteger(end)
      && anchorLine >= start
      && anchorLine <= end
  })
  if (matches.length > 0) {
    return matches.sort((a, b) => {
      const aStart = Number(a.dataset.reviewLine ?? a.dataset.sourceLine)
      const aEnd = Number(a.dataset.sourceEndLine ?? a.dataset.reviewLine ?? a.dataset.sourceLine)
      const bStart = Number(b.dataset.reviewLine ?? b.dataset.sourceLine)
      const bEnd = Number(b.dataset.sourceEndLine ?? b.dataset.reviewLine ?? b.dataset.sourceLine)
      return (aEnd - aStart) - (bEnd - bStart)
    })[0]
  }

  // Keep old line comments visible near their former location after the source changes.
  return candidates
    .sort((a, b) => Math.abs(Number(a.dataset.reviewLine ?? a.dataset.sourceLine) - anchorLine)
      - Math.abs(Number(b.dataset.reviewLine ?? b.dataset.sourceLine) - anchorLine))[0] ?? null
}

function targetForThread(root: HTMLElement, thread: FileReviewThread): HTMLElement | null {
  return thread.anchor.kind === 'lines' ? targetForLine(root, thread.anchor.startLine) : null
}

function selectionToolbarPosition(range: Range, root: HTMLElement): SelectionToolbarPosition {
  if (window.innerWidth < 768) return { kind: 'bottom' }
  let rect: DOMRect | null = null
  try {
    rect = typeof range.getBoundingClientRect === 'function'
      ? range.getBoundingClientRect()
      : closestElement(range.commonAncestorContainer)?.getBoundingClientRect() ?? root.getBoundingClientRect()
  } catch {
    rect = null
  }
  if (!rect) return { kind: 'bottom' }
  if (rect.bottom < 0 || rect.top > window.innerHeight) return { kind: 'bottom' }

  const toolbarHalfWidth = 150
  const center = (rect.left + rect.right) / 2
  const left = Math.max(toolbarHalfWidth + 8, Math.min(center, window.innerWidth - toolbarHalfWidth - 8))
  const top = rect.top >= 56 ? rect.top - 48 : Math.min(rect.bottom + 8, window.innerHeight - 48)
  return { kind: 'floating', left, top }
}

function FileReviewComposer({
  anchor,
  onCancel,
  onSave,
}: {
  anchor: FileReviewAnchor
  onCancel: () => void
  onSave: (body: string) => void
}) {
  const [body, setBody] = useState('')
  const lineLabel = anchor.kind === 'lines'
    ? `Comment on lines ${anchor.startLine}${anchor.endLine === anchor.startLine ? '' : `–${anchor.endLine}`}`
    : 'Comment on this file'
  return (
    <form
      className="my-2 rounded-md border bg-card p-3 shadow-sm"
      onSubmit={(event) => {
        event.preventDefault()
        if (body.trim()) onSave(body.trim())
      }}
    >
      <div className="mb-2 flex items-center justify-between gap-2">
        <span className="text-xs font-semibold text-muted-foreground">{lineLabel}</span>
        <Button type="button" variant="ghost" size="icon-sm" aria-label="Cancel comment" onClick={onCancel}>
          <X className="size-4" />
        </Button>
      </div>
      {anchor.kind === 'lines' && (
        <blockquote className="mb-2 max-h-24 overflow-auto border-l-2 pl-2 text-xs text-muted-foreground">
          {anchor.quote}
        </blockquote>
      )}
      <Textarea
        autoFocus
        aria-label="Review comment"
        placeholder="Write a review comment…"
        value={body}
        onChange={(event) => setBody(event.target.value)}
        className="min-h-20"
      />
      <div className="mt-2 flex justify-end">
        <Button type="submit" size="sm" disabled={!body.trim()}>Add comment</Button>
      </div>
    </form>
  )
}

function ReviewThreadCard({
  thread,
  sourceText,
  activeAgentPaneId,
  activeCodexThreadId,
  onAppendToAgentDraft,
  onAppendToCodexDraft,
  onUpdate,
  onDelete,
  onReply,
  onResolve,
}: {
  thread: FileReviewThread
  sourceText: string
  activeAgentPaneId: string | null
  activeCodexThreadId?: string | null
  onAppendToAgentDraft?: (paneId: string, text: string) => void
  onAppendToCodexDraft?: (threadId: string, text: string) => void
  onUpdate: (messageId: string, body: string) => void
  onDelete: () => void
  onReply: (body: string) => void
  onResolve: () => void
}) {
  const [replyOpen, setReplyOpen] = useState(false)
  const [reply, setReply] = useState('')
  const [editingMessageId, setEditingMessageId] = useState<string | null>(null)
  const [editingBody, setEditingBody] = useState('')
  const stale = isFileReviewAnchorStale(thread.anchor, sourceText)

  return (
    <article
      className={cn('my-2 rounded-md border bg-card p-3 font-sans shadow-sm', stale && 'border-amber-400')}
      data-review-thread={thread.id}
      data-review-stale={stale ? 'true' : 'false'}
    >
      <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
        <div className="flex items-center gap-2 text-xs text-muted-foreground">
          {thread.anchor.kind === 'lines'
            ? <span>Lines {thread.anchor.startLine}{thread.anchor.endLine === thread.anchor.startLine ? '' : `–${thread.anchor.endLine}`}</span>
            : <span>File comment</span>}
          <span className={thread.resolved ? 'text-green-700 dark:text-green-400' : 'text-amber-700 dark:text-amber-400'}>
            {thread.resolved ? 'Resolved' : 'Unresolved'}
          </span>
        </div>
        <div className="flex items-center gap-1">
          {!thread.resolved && (
            <Button variant="ghost" size="xs" onClick={onResolve} aria-label="Resolve comment thread">
              <Check className="mr-1 size-3.5" /> Resolve
            </Button>
          )}
          <Button variant="ghost" size="icon-sm" onClick={onDelete} aria-label="Delete comment thread">
            <Trash2 className="size-3.5" />
          </Button>
        </div>
      </div>
      {stale && <p className="mb-2 text-xs text-amber-700 dark:text-amber-400">The quoted text no longer matches this location.</p>}
      <div className="space-y-2">
        {thread.messages.map((message, index) => (
          <ReviewMessage
            key={message.id}
            message={message}
            label={index === 0 ? 'Comment' : 'Reply'}
            editing={editingMessageId === message.id}
            editingBody={editingBody}
            onEdit={() => {
              setEditingMessageId(message.id)
              setEditingBody(message.body)
            }}
            onCancelEdit={() => setEditingMessageId(null)}
            onEditingBodyChange={setEditingBody}
            onSaveEdit={() => {
              if (!editingBody.trim()) return
              onUpdate(message.id, editingBody.trim())
              setEditingMessageId(null)
            }}
          />
        ))}
      </div>
      <div className="mt-3 flex flex-wrap items-center gap-2 border-t pt-2">
        <Button variant="ghost" size="xs" onClick={() => setReplyOpen((open) => !open)}>
          <Reply className="mr-1 size-3.5" /> Reply
        </Button>
        <Button
          variant="outline"
          size="xs"
          disabled={activeCodexThreadId
            ? !onAppendToCodexDraft
            : !activeAgentPaneId || !onAppendToAgentDraft}
          title={activeCodexThreadId || activeAgentPaneId ? 'Append this thread to the active agent draft' : 'Open an agent panel to add this thread'}
          onClick={() => {
            const context = formatFileReviewThreadForAgent(thread)
            if (activeCodexThreadId) onAppendToCodexDraft?.(activeCodexThreadId, context)
            else if (activeAgentPaneId) onAppendToAgentDraft?.(activeAgentPaneId, context)
          }}
        >
          Add to agent
        </Button>
        {!activeAgentPaneId && !activeCodexThreadId && <span className="text-xs text-muted-foreground">Open an agent panel to insert.</span>}
      </div>
      {replyOpen && (
        <form
          className="mt-2"
          onSubmit={(event) => {
            event.preventDefault()
            if (!reply.trim()) return
            onReply(reply.trim())
            setReply('')
            setReplyOpen(false)
          }}
        >
          <Textarea
            autoFocus
            aria-label="Reply to review comment"
            placeholder="Write a reply…"
            value={reply}
            onChange={(event) => setReply(event.target.value)}
            className="min-h-16"
          />
          <div className="mt-2 flex justify-end">
            <Button type="submit" size="sm" disabled={!reply.trim()}>Add reply</Button>
          </div>
        </form>
      )}
    </article>
  )
}

function ReviewMessage({
  message,
  label,
  editing,
  editingBody,
  onEdit,
  onCancelEdit,
  onEditingBodyChange,
  onSaveEdit,
}: {
  message: FileReviewMessage
  label: string
  editing: boolean
  editingBody: string
  onEdit: () => void
  onCancelEdit: () => void
  onEditingBodyChange: (body: string) => void
  onSaveEdit: () => void
}) {
  return (
    <div className="min-w-0 rounded bg-muted/50 p-2">
      <div className="mb-1 flex items-center justify-between gap-2">
        <span className="text-xs font-semibold">{label}</span>
        <div className="flex items-center gap-1 text-xs text-muted-foreground">
          <time dateTime={new Date(message.createdAt).toISOString()}>{new Date(message.createdAt).toLocaleString()}</time>
          {message.updatedAt !== message.createdAt && <span>(edited)</span>}
          {!editing && (
            <Button variant="ghost" size="icon-sm" aria-label={`Edit ${label.toLowerCase()}`} onClick={onEdit}>
              <Pencil className="size-3" />
            </Button>
          )}
        </div>
      </div>
      {editing ? (
        <div>
          <Textarea aria-label={`Edit ${label.toLowerCase()}`} value={editingBody} onChange={(event) => onEditingBodyChange(event.target.value)} className="min-h-16" />
          <div className="mt-2 flex justify-end gap-1">
            <Button type="button" variant="ghost" size="xs" onClick={onCancelEdit}>Cancel</Button>
            <Button type="button" size="xs" disabled={!editingBody.trim()} onClick={onSaveEdit}>Save</Button>
          </div>
        </div>
      ) : (
        <p className="whitespace-pre-wrap break-words text-sm">{message.body}</p>
      )}
    </div>
  )
}

export function FileReviewComments({
  project,
  path,
  sourceText,
  activeAgentPaneId,
  activeCodexThreadId,
  onAppendToAgentDraft,
  onAppendToCodexDraft,
  children,
}: FileReviewCommentsProps) {
  const storageKey = fileReviewStorageKey(project, path)
  const readCurrent = useCallback(() => readSnapshot(project, path, storageKey), [path, project, storageKey])
  const [snapshot, setSnapshot] = useState<Snapshot>(readCurrent)
  const [composerAnchor, setComposerAnchor] = useState<FileReviewAnchor | null>(null)
  const [selectedAnchor, setSelectedAnchor] = useState<Extract<FileReviewAnchor, { kind: 'lines' }> | null>(null)
  const [selectionPosition, setSelectionPosition] = useState<SelectionToolbarPosition | null>(null)
  const [stickyHeaderHeight, setStickyHeaderHeight] = useState<number | null>(null)
  const contentRef = useRef<HTMLDivElement | null>(null)
  const selectionRangeRef = useRef<Range | null>(null)
  const [portalMounts, setPortalMounts] = useState<PortalMount[]>([])
  const [lineComposerMount, setLineComposerMount] = useState<LineComposerMount | null>(null)
  const lineComposerMountRef = useRef<LineComposerMount | null>(null)

  const current = snapshot.key === storageKey ? snapshot : { key: storageKey, threads: [], error: null }

  useEffect(() => {
    setSnapshot(readCurrent())
    const refresh = (event: Event) => {
      const changedKey = (event as CustomEvent<{ key?: string }>).detail?.key
      if (changedKey === storageKey) setSnapshot(readCurrent())
    }
    const onStorage = (event: StorageEvent) => {
      if (event.key === storageKey) setSnapshot(readCurrent())
    }
    window.addEventListener(FILE_REVIEW_COMMENTS_EVENT, refresh)
    window.addEventListener('storage', onStorage)
    return () => {
      window.removeEventListener(FILE_REVIEW_COMMENTS_EVENT, refresh)
      window.removeEventListener('storage', onStorage)
    }
  }, [readCurrent, storageKey])

  const persist = useCallback((threads: FileReviewThread[]) => {
    if (current.error) return
    const error = saveFileReviewThreads(project, path, threads)
    if (error) {
      setSnapshot({ ...current, error })
      return
    }
    setSnapshot({ key: storageKey, threads, error: null })
  }, [current, path, project, storageKey])

  const updateThread = useCallback((threadId: string, update: (thread: FileReviewThread) => FileReviewThread | null) => {
    persist(current.threads.flatMap((thread) => {
      if (thread.id !== threadId) return [thread]
      const next = update(thread)
      return next ? [next] : []
    }))
  }, [current.threads, persist])

  const createThread = useCallback((anchor: FileReviewAnchor, body: string) => {
    persist([...current.threads, createFileReviewThread(path, anchor, body)])
    setComposerAnchor(null)
    setSelectedAnchor(null)
    setSelectionPosition(null)
    selectionRangeRef.current = null
    setLineComposerMount(null)
    window.getSelection()?.removeAllRanges()
  }, [current.threads, path, persist])

  const onSelectionChange = useCallback(() => {
    const root = contentRef.current
    if (!root || composerAnchor) return
    const anchor = reviewAnchorFromSelection(root, sourceText)
    const selection = window.getSelection()
    const range = anchor && selection?.rangeCount ? selection.getRangeAt(0) : null
    setSelectedAnchor(anchor)
    selectionRangeRef.current = range
    setSelectionPosition(range ? selectionToolbarPosition(range, root) : null)
  }, [composerAnchor, sourceText])

  const lineThreads = useMemo(
    () => current.threads.filter((thread) => thread.anchor.kind === 'lines'),
    [current.threads],
  )

  useLayoutEffect(() => {
    const root = contentRef.current
    if (!root) return
    const byTarget = new Map<HTMLElement, FileReviewThread[]>()
    for (const thread of lineThreads) {
      const target = targetForThread(root, thread)
      if (!target) continue
      byTarget.set(target, [...(byTarget.get(target) ?? []), thread])
    }
    const mounts: PortalMount[] = []
    let index = 0
    for (const [target, threads] of byTarget) {
      const element = document.createElement('div')
      element.dataset.fileReviewMount = 'true'
      target.insertAdjacentElement('afterend', element)
      mounts.push({ key: `${storageKey}:${index++}`, element, threads })
    }
    setPortalMounts(mounts)
    return () => mounts.forEach(({ element }) => element.remove())
  }, [children, lineThreads, sourceText, storageKey])

  useLayoutEffect(() => {
    const existingMount = lineComposerMountRef.current
    if (composerAnchor?.kind !== 'lines') {
      existingMount?.element.remove()
      lineComposerMountRef.current = null
      setLineComposerMount(null)
      return
    }
    const root = contentRef.current
    const target = root ? targetForLine(root, composerAnchor.startLine) : null
    if (!root || (!target && !existingMount)) {
      existingMount?.element.remove()
      lineComposerMountRef.current = null
      setLineComposerMount(null)
      return
    }
    const key = `${storageKey}:composer:${composerAnchor.startLine}:${composerAnchor.endLine}`
    // Reusing the portal host preserves the composer state across preview rerenders.
    let nextMount = existingMount
    if (!nextMount || nextMount.key !== key) {
      nextMount?.element.remove()
      const element = document.createElement('div')
      element.dataset.fileReviewComposerMount = 'true'
      nextMount = { key, element, anchor: composerAnchor }
    } else if (nextMount.anchor !== composerAnchor) {
      nextMount = { ...nextMount, anchor: composerAnchor }
    }
    lineComposerMountRef.current = nextMount
    if (target) {
      // Other portal mounts may sit between the anchor and composer host.
      const hostFollowsTarget = target.parentElement === nextMount.element.parentElement
        && Boolean(target.compareDocumentPosition(nextMount.element) & Node.DOCUMENT_POSITION_FOLLOWING)
      // Moving an active host can blur its textarea, so only reposition when needed.
      if (!hostFollowsTarget) {
        target.insertAdjacentElement('afterend', nextMount.element)
      }
    } else if (root.lastElementChild !== nextMount.element) {
      root.appendChild(nextMount.element)
    }
    setLineComposerMount((currentMount) => (
      currentMount?.element === nextMount.element && currentMount.anchor === nextMount.anchor
        ? currentMount
        : nextMount
    ))
  }, [children, composerAnchor, sourceText, storageKey])

  useLayoutEffect(() => () => {
    lineComposerMountRef.current?.element.remove()
    lineComposerMountRef.current = null
  }, [])

  useEffect(() => {
    if (!selectedAnchor) return
    const reposition = () => {
      const range = selectionRangeRef.current
      const root = contentRef.current
      setSelectionPosition(range && root ? selectionToolbarPosition(range, root) : { kind: 'bottom' })
    }
    window.addEventListener('resize', reposition)
    window.addEventListener('scroll', reposition, true)
    return () => {
      window.removeEventListener('resize', reposition)
      window.removeEventListener('scroll', reposition, true)
    }
  }, [selectedAnchor])

  useLayoutEffect(() => {
    const viewer = contentRef.current?.closest<HTMLElement>('.knowledge-main')
    const fileToolbar = viewer?.querySelector<HTMLElement>('.note-toolbar')
    if (!fileToolbar) return

    const measure = () => {
      const height = Math.ceil(fileToolbar.getBoundingClientRect().height)
      setStickyHeaderHeight((currentHeight) => currentHeight === height ? currentHeight : height)
    }
    measure()
    window.addEventListener('resize', measure)
    const observer = typeof ResizeObserver === 'undefined' ? null : new ResizeObserver(measure)
    observer?.observe(fileToolbar)

    return () => {
      window.removeEventListener('resize', measure)
      observer?.disconnect()
    }
  }, [storageKey])

  const placedThreadIds = new Set(portalMounts.flatMap((mount) => mount.threads.map((thread) => thread.id)))
  const unplacedThreads = lineThreads.filter((thread) => !placedThreadIds.has(thread.id))
  const fileThreads = current.threads.filter((thread) => thread.anchor.kind === 'file')
  const unresolvedCount = current.threads.filter((thread) => !thread.resolved).length

  const clearSelection = () => {
    setSelectedAnchor(null)
    setSelectionPosition(null)
    selectionRangeRef.current = null
    window.getSelection()?.removeAllRanges()
  }

  const startSelectionComment = () => {
    if (!selectedAnchor) return
    setComposerAnchor(selectedAnchor)
    setSelectedAnchor(null)
    setSelectionPosition(null)
    selectionRangeRef.current = null
  }

  const cancelComposer = () => {
    setComposerAnchor(null)
    setLineComposerMount(null)
    clearSelection()
  }

  return (
    <div className="file-review-comments min-w-0">
      <div
        data-review-toolbar
        className="sticky z-20 mb-2 border-b bg-card/95 py-2 backdrop-blur"
        style={stickyHeaderHeight === null ? undefined : { top: `${stickyHeaderHeight + 1}px` }}
      >
        <div className="flex flex-wrap items-center gap-2">
          <Button
            type="button"
            variant="outline"
            size="xs"
            disabled={Boolean(current.error)}
            onClick={() => {
              clearSelection()
              setComposerAnchor({ kind: 'file' })
            }}
          >
            <MessageSquarePlus className="mr-1 size-3.5" /> Add file comment
          </Button>
          {fileThreads.length > 0 && <span className="text-xs text-muted-foreground">{fileThreads.length} file thread{fileThreads.length === 1 ? '' : 's'}</span>}
          {unresolvedCount > 0 && (
            <span className="rounded-full bg-amber-100 px-2 py-0.5 text-xs text-amber-900 dark:bg-amber-950 dark:text-amber-200">
              {unresolvedCount} unresolved
            </span>
          )}
          {current.error && <span className="text-xs text-destructive" role="alert">{current.error}</span>}
        </div>
        {composerAnchor?.kind === 'file' && (
          <FileReviewComposer
            key="file"
            anchor={composerAnchor}
            onCancel={cancelComposer}
            onSave={(body) => createThread(composerAnchor, body)}
          />
        )}
      </div>

      {fileThreads.length > 0 && (
        <section className="mb-3" aria-label="File review comments">
          {fileThreads.map((thread) => (
            <ReviewThreadCard
              key={thread.id}
              thread={thread}
              sourceText={sourceText}
              activeAgentPaneId={activeAgentPaneId}
              activeCodexThreadId={activeCodexThreadId}
              onAppendToAgentDraft={onAppendToAgentDraft}
              onAppendToCodexDraft={onAppendToCodexDraft}
              onUpdate={(messageId, body) => updateThread(thread.id, (item) => updateFileReviewMessage(item, messageId, body))}
              onReply={(body) => updateThread(thread.id, (item) => replyToFileReviewThread(item, body))}
              onResolve={() => updateThread(thread.id, resolveFileReviewThread)}
              onDelete={() => updateThread(thread.id, () => null)}
            />
          ))}
        </section>
      )}

      <div ref={contentRef} className="file-review-content min-w-0" onMouseUp={onSelectionChange} onTouchEnd={onSelectionChange} onKeyUp={onSelectionChange}>
        {children}
      </div>

      {portalMounts.map((mount) => createPortal(
        <div className="file-review-inline-threads" key={mount.key}>
          {mount.threads.map((thread) => (
            <ReviewThreadCard
              key={thread.id}
              thread={thread}
              sourceText={sourceText}
              activeAgentPaneId={activeAgentPaneId}
              activeCodexThreadId={activeCodexThreadId}
              onAppendToAgentDraft={onAppendToAgentDraft}
              onAppendToCodexDraft={onAppendToCodexDraft}
              onUpdate={(messageId, body) => updateThread(thread.id, (item) => updateFileReviewMessage(item, messageId, body))}
              onReply={(body) => updateThread(thread.id, (item) => replyToFileReviewThread(item, body))}
              onResolve={() => updateThread(thread.id, resolveFileReviewThread)}
              onDelete={() => updateThread(thread.id, () => null)}
            />
          ))}
        </div>,
        mount.element,
        mount.key,
      ))}

      {lineComposerMount && createPortal(
        <FileReviewComposer
          key={lineComposerMount.key}
          anchor={lineComposerMount.anchor}
          onCancel={cancelComposer}
          onSave={(body) => createThread(lineComposerMount.anchor, body)}
        />,
        lineComposerMount.element,
        lineComposerMount.key,
      )}

      {selectedAnchor && selectionPosition && createPortal(
        <div
          role="toolbar"
          aria-label="Review selection actions"
          data-placement={selectionPosition.kind}
          className={cn(
            'fixed z-[100] flex items-center gap-2 rounded-md border bg-popover p-2 text-popover-foreground shadow-lg',
            selectionPosition.kind === 'bottom' && 'inset-x-3 bottom-3 justify-center rounded-lg p-3 sm:left-1/2 sm:right-auto sm:w-max sm:-translate-x-1/2',
          )}
          style={selectionPosition.kind === 'floating'
            ? { left: selectionPosition.left, top: selectionPosition.top, transform: 'translateX(-50%)' }
            : undefined}
        >
          <span className="whitespace-nowrap text-xs text-muted-foreground">
            Lines {selectedAnchor.startLine}{selectedAnchor.endLine === selectedAnchor.startLine ? '' : `–${selectedAnchor.endLine}`}
          </span>
          <Button
            type="button"
            size="xs"
            disabled={Boolean(current.error)}
            onMouseDown={(event) => event.preventDefault()}
            onClick={startSelectionComment}
          >
            <MessageSquarePlus className="mr-1 size-3.5" /> Comment on selection
          </Button>
          <Button type="button" variant="ghost" size="xs" onClick={clearSelection}>Clear</Button>
        </div>,
        document.body,
      )}

      {unplacedThreads.length > 0 && (
        <section className="mt-3 rounded-md border border-amber-400/60 p-2" aria-label="Unplaced review comments">
          <p className="mb-2 text-xs font-semibold text-amber-700 dark:text-amber-400">Some comments could not be attached to a rendered line.</p>
          {unplacedThreads.map((thread) => (
            <ReviewThreadCard
              key={thread.id}
              thread={thread}
              sourceText={sourceText}
              activeAgentPaneId={activeAgentPaneId}
              activeCodexThreadId={activeCodexThreadId}
              onAppendToAgentDraft={onAppendToAgentDraft}
              onAppendToCodexDraft={onAppendToCodexDraft}
              onUpdate={(messageId, body) => updateThread(thread.id, (item) => updateFileReviewMessage(item, messageId, body))}
              onReply={(body) => updateThread(thread.id, (item) => replyToFileReviewThread(item, body))}
              onResolve={() => updateThread(thread.id, resolveFileReviewThread)}
              onDelete={() => updateThread(thread.id, () => null)}
            />
          ))}
        </section>
      )}
    </div>
  )
}
