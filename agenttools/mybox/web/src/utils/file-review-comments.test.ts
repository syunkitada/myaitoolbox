import { beforeEach, describe, expect, it } from 'vitest'
import {
  appendReviewContextToDraft,
  fileReviewStorageKey,
  formatFileReviewThreadForAgent,
  isFileReviewAnchorStale,
  loadFileReviewThreads,
  saveFileReviewThreads,
  type FileReviewThread,
} from './file-review-comments'

const thread: FileReviewThread = {
  id: 'thread-1',
  path: 'src/example.ts',
  anchor: { kind: 'lines', startLine: 4, endLine: 5, quote: 'const total = 1' },
  messages: [
    { id: 'message-1', body: 'Please explain this value.', createdAt: 1000, updatedAt: 1000 },
    { id: 'message-2', body: 'It is a temporary default.', createdAt: 2000, updatedAt: 2000 },
  ],
  resolved: false,
  createdAt: 1000,
  updatedAt: 2000,
}

describe('file review comment storage', () => {
  beforeEach(() => {
    localStorage.clear()
  })

  it('stores threads independently by project and normalized file path', () => {
    expect(saveFileReviewThreads('work', 'src/example.ts', [thread])).toBeNull()

    expect(loadFileReviewThreads('work', 'src/example.ts').threads).toEqual([thread])
    expect(loadFileReviewThreads('other', 'src/example.ts').threads).toEqual([])
    expect(loadFileReviewThreads('work', './src/example.ts').threads).toEqual([thread])
  })

  it('reports malformed saved data and leaves it untouched', () => {
    const key = fileReviewStorageKey('work', 'src/example.ts')
    localStorage.setItem(key, '{broken')

    const loaded = loadFileReviewThreads('work', 'src/example.ts')

    expect(loaded.threads).toEqual([])
    expect(loaded.error).toMatch(/could not be read/i)
    expect(localStorage.getItem(key)).toBe('{broken')
  })

  it('reports a failed write when browser storage is full or unavailable', () => {
    const blockedStorage = {
      getItem: () => null,
      setItem: () => { throw new Error('quota exceeded') },
    } as unknown as Storage

    expect(saveFileReviewThreads('work', 'src/example.ts', [thread], blockedStorage)).toMatch(/could not be saved/i)
  })

  it('marks a range stale when its quoted text no longer exists at the saved lines', () => {
    expect(isFileReviewAnchorStale(thread.anchor, 'one\ntwo\nthree\nconst total = 1\nnext')).toBe(false)
    expect(isFileReviewAnchorStale(thread.anchor, 'one\ntwo\nthree\nconst total = 2\nnext')).toBe(true)
    expect(isFileReviewAnchorStale({ kind: 'file' }, 'anything')).toBe(false)
  })

  it('formats the file, range, quote, replies, and status for the agent draft', () => {
    expect(formatFileReviewThreadForAgent(thread)).toBe(
      [
        '### File review context',
        'File: src/example.ts',
        'Lines: 4-5',
        'Quoted text:',
        '> const total = 1',
        'Thread status: unresolved',
        'Comment: Please explain this value.',
        'Reply: It is a temporary default.',
      ].join('\n'),
    )
  })

  it('appends agent context without changing the existing draft', () => {
    expect(appendReviewContextToDraft('existing prompt', 'review context')).toBe('existing prompt\n\nreview context')
    expect(appendReviewContextToDraft('existing prompt\n', 'review context')).toBe('existing prompt\n\nreview context')
    expect(appendReviewContextToDraft('', 'review context')).toBe('review context')
  })
})
