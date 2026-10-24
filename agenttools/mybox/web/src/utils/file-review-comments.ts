import { getBasePath } from './routes'

export const FILE_REVIEW_COMMENTS_EVENT = 'mybox:file-review-comments:changed'
const STORAGE_PREFIX = 'mybox:file-review-comments:v1'
const STORAGE_VERSION = 1

export type FileReviewAnchor =
  | { kind: 'file' }
  | { kind: 'lines'; startLine: number; endLine: number; quote: string }

export interface FileReviewMessage {
  id: string
  body: string
  createdAt: number
  updatedAt: number
}

export interface FileReviewThread {
  id: string
  path: string
  anchor: FileReviewAnchor
  messages: FileReviewMessage[]
  resolved: boolean
  createdAt: number
  updatedAt: number
}

export interface LoadedFileReviewThreads {
  threads: FileReviewThread[]
  error: string | null
}

interface StoredFileReviewThreads {
  version: number
  project: string
  path: string
  threads: FileReviewThread[]
}

export function normalizeReviewFilePath(path: string): string {
  return path
    .replaceAll('\\', '/')
    .split('/')
    .filter((segment) => segment !== '' && segment !== '.')
    .join('/')
}

export function fileReviewStorageKey(project: string, path: string, basePath = getSafeBasePath()): string {
  return `${STORAGE_PREFIX}:${encodeURIComponent(basePath)}:${encodeURIComponent(project)}:${encodeURIComponent(normalizeReviewFilePath(path))}`
}

function getSafeBasePath(): string {
  return typeof window === 'undefined' ? '' : getBasePath()
}

function getBrowserStorage(storage?: Storage | null): Storage | null {
  if (storage !== undefined) return storage
  if (typeof window === 'undefined') return null
  try {
    return window.localStorage
  } catch {
    return null
  }
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return Boolean(value && typeof value === 'object' && !Array.isArray(value))
}

function isAnchor(value: unknown): value is FileReviewAnchor {
  if (!isRecord(value)) return false
  if (value.kind === 'file') return true
  return value.kind === 'lines'
    && Number.isInteger(value.startLine)
    && Number(value.startLine) >= 1
    && Number.isInteger(value.endLine)
    && Number(value.endLine) >= Number(value.startLine)
    && typeof value.quote === 'string'
}

function isMessage(value: unknown): value is FileReviewMessage {
  return isRecord(value)
    && typeof value.id === 'string'
    && typeof value.body === 'string'
    && typeof value.createdAt === 'number'
    && typeof value.updatedAt === 'number'
}

function isThread(value: unknown): value is FileReviewThread {
  return isRecord(value)
    && typeof value.id === 'string'
    && typeof value.path === 'string'
    && isAnchor(value.anchor)
    && Array.isArray(value.messages)
    && value.messages.length > 0
    && value.messages.every(isMessage)
    && typeof value.resolved === 'boolean'
    && typeof value.createdAt === 'number'
    && typeof value.updatedAt === 'number'
}

function notifyChanged(key: string): void {
  if (typeof window === 'undefined') return
  window.dispatchEvent(new CustomEvent(FILE_REVIEW_COMMENTS_EVENT, { detail: { key } }))
}

export function loadFileReviewThreads(
  project: string,
  path: string,
  storage?: Storage | null,
): LoadedFileReviewThreads {
  const resolvedStorage = getBrowserStorage(storage)
  if (!resolvedStorage) return { threads: [], error: 'Browser storage is unavailable.' }
  const normalizedPath = normalizeReviewFilePath(path)
  const key = fileReviewStorageKey(project, normalizedPath)
  let raw: string | null
  try {
    raw = resolvedStorage.getItem(key)
  } catch {
    return { threads: [], error: 'Review comments could not be read from browser storage.' }
  }
  if (!raw) return { threads: [], error: null }

  try {
    const stored: unknown = JSON.parse(raw)
    if (
      !isRecord(stored)
      || stored.version !== STORAGE_VERSION
      || stored.project !== project
      || stored.path !== normalizedPath
      || !Array.isArray(stored.threads)
      || !stored.threads.every(isThread)
    ) {
      return { threads: [], error: 'Saved review comments have an unsupported or invalid format.' }
    }
    return { threads: stored.threads, error: null }
  } catch {
    return { threads: [], error: 'Review comments could not be read from browser storage.' }
  }
}

export function saveFileReviewThreads(
  project: string,
  path: string,
  threads: FileReviewThread[],
  storage?: Storage | null,
): string | null {
  const resolvedStorage = getBrowserStorage(storage)
  if (!resolvedStorage) return 'Browser storage is unavailable.'
  if (!threads.every(isThread)) return 'Review comments contain invalid data.'

  const normalizedPath = normalizeReviewFilePath(path)
  const key = fileReviewStorageKey(project, normalizedPath)
  const value: StoredFileReviewThreads = {
    version: STORAGE_VERSION,
    project,
    path: normalizedPath,
    threads: threads.map((thread) => ({ ...thread, path: normalizedPath })),
  }
  try {
    resolvedStorage.setItem(key, JSON.stringify(value))
  } catch {
    return 'Review comments could not be saved. Browser storage may be full or unavailable.'
  }
  notifyChanged(key)
  return null
}

function createId(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') return crypto.randomUUID()
  return `${Date.now().toString(36)}-${Math.random().toString(36).slice(2)}`
}

export function createFileReviewThread(
  path: string,
  anchor: FileReviewAnchor,
  body: string,
  now = Date.now(),
): FileReviewThread {
  const message: FileReviewMessage = { id: createId(), body, createdAt: now, updatedAt: now }
  return {
    id: createId(),
    path: normalizeReviewFilePath(path),
    anchor,
    messages: [message],
    resolved: false,
    createdAt: now,
    updatedAt: now,
  }
}

export function replyToFileReviewThread(thread: FileReviewThread, body: string, now = Date.now()): FileReviewThread {
  return {
    ...thread,
    messages: [...thread.messages, { id: createId(), body, createdAt: now, updatedAt: now }],
    updatedAt: now,
  }
}

export function updateFileReviewMessage(
  thread: FileReviewThread,
  messageId: string,
  body: string,
  now = Date.now(),
): FileReviewThread {
  return {
    ...thread,
    messages: thread.messages.map((message) => (
      message.id === messageId ? { ...message, body, updatedAt: now } : message
    )),
    updatedAt: now,
  }
}

export function resolveFileReviewThread(thread: FileReviewThread, now = Date.now()): FileReviewThread {
  return { ...thread, resolved: true, updatedAt: now }
}

function normalizeQuote(value: string): string {
  return value.replace(/\s+/g, ' ').trim()
}

export function isFileReviewAnchorStale(anchor: FileReviewAnchor, sourceText: string): boolean {
  if (anchor.kind === 'file') return false
  const sourceLines = sourceText.split(/\r?\n/)
  if (anchor.startLine > sourceLines.length) return true
  const source = normalizeQuote(sourceLines.slice(anchor.startLine - 1, anchor.endLine).join(' '))
  const quote = normalizeQuote(anchor.quote)
  return !quote || !source.includes(quote)
}

export function formatFileReviewThreadForAgent(thread: FileReviewThread): string {
  const lines = [
    '### File review context',
    `File: ${thread.path}`,
  ]
  if (thread.anchor.kind === 'lines') {
    lines.push(
      `Lines: ${thread.anchor.startLine}${thread.anchor.endLine === thread.anchor.startLine ? '' : `-${thread.anchor.endLine}`}`,
      'Quoted text:',
      ...thread.anchor.quote.split(/\r?\n/).map((line) => `> ${line}`),
    )
  }
  lines.push(`Thread status: ${thread.resolved ? 'resolved' : 'unresolved'}`)
  thread.messages.forEach((message, index) => {
    lines.push(`${index === 0 ? 'Comment' : 'Reply'}: ${message.body}`)
  })
  return lines.join('\n')
}

export function appendReviewContextToDraft(draft: string, context: string): string {
  if (!draft) return context
  if (draft.endsWith('\n\n')) return `${draft}${context}`
  if (draft.endsWith('\n')) return `${draft}\n${context}`
  return `${draft}\n\n${context}`
}
