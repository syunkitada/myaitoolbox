import { beforeEach, describe, expect, it, vi } from 'vitest'
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import {
  createFileReviewThread,
  fileReviewStorageKey,
  loadFileReviewThreads,
  saveFileReviewThreads,
} from '../utils/file-review-comments'
import { FileReviewComments } from './FileReviewComments'

function renderComments({
  sourceText = 'one\ntwo',
  activeAgentPaneId = 'pane-1',
  onAppendToAgentDraft = vi.fn(),
  children = <p data-source-line="1" data-source-end-line="1">one</p>,
}: {
  sourceText?: string
  activeAgentPaneId?: string | null
  onAppendToAgentDraft?: (paneId: string, text: string) => void
  children?: React.ReactNode
} = {}) {
  return {
    ...render(
      <FileReviewComments
        project="work"
        path="docs/guide.md"
        sourceText={sourceText}
        activeAgentPaneId={activeAgentPaneId}
        onAppendToAgentDraft={onAppendToAgentDraft}
      >
        {children}
      </FileReviewComments>,
    ),
    onAppendToAgentDraft,
  }
}

function selectText(node: Node) {
  const range = document.createRange()
  range.selectNodeContents(node)
  const selection = window.getSelection()
  selection?.removeAllRanges()
  selection?.addRange(range)
}

describe('FileReviewComments', () => {
  beforeEach(() => {
    localStorage.clear()
    window.getSelection()?.removeAllRanges()
  })

  it('creates a line-range thread from the selected rendered text', async () => {
    const user = userEvent.setup()
    const { container } = renderComments()
    const line = container.querySelector('.file-review-content p')!
    selectText(line.firstChild!)
    fireEvent.mouseUp(line)

    await user.click(await screen.findByRole('button', { name: /comment on selection/i }))
    await user.type(screen.getByRole('textbox', { name: 'Review comment' }), 'Please clarify this line.')
    await user.click(screen.getByRole('button', { name: 'Add comment' }))

    const stored = loadFileReviewThreads('work', 'docs/guide.md')
    expect(stored.error).toBeNull()
    expect(stored.threads).toHaveLength(1)
    expect(stored.threads[0].anchor).toEqual({ kind: 'lines', startLine: 1, endLine: 1, quote: 'one' })
    expect(stored.threads[0].messages[0].body).toBe('Please clarify this line.')
    expect(line.nextElementSibling).toHaveAttribute('data-file-review-mount', 'true')
  })

  it('shows the selection action beside the selected text and opens the composer below it', async () => {
    const user = userEvent.setup()
    const { container } = renderComments()
    const line = container.querySelector('.file-review-content p')!
    selectText(line.firstChild!)
    fireEvent.mouseUp(line)

    const toolbar = await screen.findByRole('toolbar', { name: 'Review selection actions' })
    expect(toolbar).toHaveAttribute('data-placement', 'floating')
    expect(toolbar).toHaveClass('fixed')
    await user.click(within(toolbar).getByRole('button', { name: /comment on selection/i }))

    const composerMount = line.nextElementSibling as HTMLElement
    expect(composerMount).toHaveAttribute('data-file-review-composer-mount', 'true')
    expect(within(composerMount).getByRole('textbox', { name: 'Review comment' })).toBeInTheDocument()
  })

  it('preserves a selection comment draft when the rendered file is refreshed', async () => {
    const user = userEvent.setup()
    const { container, rerender } = renderComments()
    const line = container.querySelector('.file-review-content p')!
    selectText(line.firstChild!)
    fireEvent.mouseUp(line)

    await user.click(await screen.findByRole('button', { name: /comment on selection/i }))
    const originalMount = line.nextElementSibling
    const editor = screen.getByRole('textbox', { name: 'Review comment' })
    await user.type(editor, 'Draft in progress')

    rerender(
      <FileReviewComments project="work" path="docs/guide.md" sourceText="one\ntwo" activeAgentPaneId="pane-1">
        <p data-source-line="1" data-source-end-line="1">one</p>
      </FileReviewComments>,
    )

    expect(screen.getByRole('textbox', { name: 'Review comment' })).toHaveValue('Draft in progress')
    expect(screen.getByRole('textbox', { name: 'Review comment' })).toHaveFocus()
    expect(container.querySelector('[data-file-review-composer-mount]')).toBe(originalMount)
  })

  it('preserves selection comment focus when an existing line thread is refreshed', async () => {
    const user = userEvent.setup()
    saveFileReviewThreads('work', 'docs/guide.md', [
      createFileReviewThread(
        'docs/guide.md',
        { kind: 'lines', startLine: 1, endLine: 1, quote: 'one' },
        'Existing line comment',
        10,
      ),
    ])
    const { container, rerender } = renderComments()
    const line = container.querySelector('.file-review-content p')!
    await screen.findByText('Existing line comment')
    selectText(line.firstChild!)
    fireEvent.mouseUp(line)

    await user.click(await screen.findByRole('button', { name: /comment on selection/i }))
    const editor = screen.getByRole('textbox', { name: 'Review comment' })
    await user.type(editor, 'Draft in progress')

    rerender(
      <FileReviewComments project="work" path="docs/guide.md" sourceText="one\ntwo" activeAgentPaneId="pane-1">
        <p data-source-line="1" data-source-end-line="1">one</p>
      </FileReviewComments>,
    )

    expect(screen.getByRole('textbox', { name: 'Review comment' })).toHaveValue('Draft in progress')
    expect(screen.getByRole('textbox', { name: 'Review comment' })).toHaveFocus()
  })

  it('clears a selection comment draft after cancelling and reopening it', async () => {
    const user = userEvent.setup()
    const { container } = renderComments()
    const selectLine = () => {
      const line = container.querySelector('.file-review-content p')!
      selectText(line.firstChild!)
      fireEvent.mouseUp(line)
    }

    selectLine()
    await user.click(await screen.findByRole('button', { name: /comment on selection/i }))
    await user.type(screen.getByRole('textbox', { name: 'Review comment' }), 'Discard this draft')
    await user.click(screen.getByRole('button', { name: 'Cancel comment' }))

    selectLine()
    await user.click(await screen.findByRole('button', { name: /comment on selection/i }))

    expect(screen.getByRole('textbox', { name: 'Review comment' })).toHaveValue('')
  })

  it('pins the selection action to the bottom on narrow screens', async () => {
    const previousWidth = window.innerWidth
    Object.defineProperty(window, 'innerWidth', { configurable: true, value: 390 })
    try {
      const { container } = renderComments()
      const line = container.querySelector('.file-review-content p')!
      selectText(line.firstChild!)
      fireEvent.mouseUp(line)

      const toolbar = await screen.findByRole('toolbar', { name: 'Review selection actions' })
      expect(toolbar).toHaveAttribute('data-placement', 'bottom')
      expect(toolbar).toHaveClass('fixed')
    } finally {
      Object.defineProperty(window, 'innerWidth', { configurable: true, value: previousWidth })
    }
  })

  it('keeps file comment actions and unresolved count in a sticky viewer toolbar', async () => {
    const unresolved = createFileReviewThread('docs/guide.md', { kind: 'file' }, 'Open item', 10)
    saveFileReviewThreads('work', 'docs/guide.md', [unresolved])
    const { container } = renderComments()

    const toolbar = container.querySelector<HTMLElement>('[data-review-toolbar]')!
    expect(toolbar).toHaveClass('sticky')
    expect(toolbar).toHaveTextContent('1 unresolved')
    expect(within(toolbar).getByRole('button', { name: /add file comment/i })).toBeInTheDocument()
  })

  it('uses source line markers when a text selection crosses code lines', async () => {
    const user = userEvent.setup()
    const { container } = renderComments({
      sourceText: 'one\ntwo\nthree\nfour',
      children: (
        <pre>
          <code>
            <span data-review-line="3">three</span>
            <span data-review-line="4">four</span>
          </code>
        </pre>
      ),
    })
    const firstLine = container.querySelector('[data-review-line="3"]')!
    const lastLine = container.querySelector('[data-review-line="4"]')!
    const range = document.createRange()
    range.setStart(firstLine.firstChild!, 0)
    range.setEnd(lastLine.firstChild!, lastLine.textContent!.length)
    const selection = window.getSelection()
    selection?.removeAllRanges()
    selection?.addRange(range)
    fireEvent.mouseUp(lastLine)

    await user.click(await screen.findByRole('button', { name: /comment on selection/i }))
    await user.type(screen.getByRole('textbox', { name: 'Review comment' }), 'Check these lines.')
    await user.click(screen.getByRole('button', { name: 'Add comment' }))

    expect(loadFileReviewThreads('work', 'docs/guide.md').threads[0].anchor).toEqual({
      kind: 'lines',
      startLine: 3,
      endLine: 4,
      quote: 'three\nfour',
    })
  })

  it('anchors Markdown code selections to the first code line instead of the fence', async () => {
    const user = userEvent.setup()
    const { container } = renderComments({
      sourceText: 'heading\n```ts\nconst first = 1\nconst second = 2\n```',
      children: (
        <div className="markdown-code-block" data-source-line="2" data-review-code-line="3" data-source-end-line="5">
          <pre><code>{'const first = 1\nconst second = 2'}</code></pre>
        </div>
      ),
    })
    const code = container.querySelector('.markdown-code-block code')!.firstChild!
    const range = document.createRange()
    range.setStart(code, 0)
    range.setEnd(code, code.textContent!.length)
    const selection = window.getSelection()
    selection?.removeAllRanges()
    selection?.addRange(range)
    fireEvent.mouseUp(code.parentElement!)

    await user.click(await screen.findByRole('button', { name: /comment on selection/i }))
    await user.type(screen.getByRole('textbox', { name: 'Review comment' }), 'Check this block.')
    await user.click(screen.getByRole('button', { name: 'Add comment' }))

    expect(loadFileReviewThreads('work', 'docs/guide.md').threads[0].anchor).toEqual({
      kind: 'lines',
      startLine: 3,
      endLine: 4,
      quote: 'const first = 1\nconst second = 2',
    })
  })

  it('adds file comments, replies, resolves, and appends the thread to the selected agent', async () => {
    const user = userEvent.setup()
    const onAppendToAgentDraft = vi.fn()
    renderComments({ onAppendToAgentDraft })

    await user.click(screen.getByRole('button', { name: /add file comment/i }))
    await user.type(screen.getByRole('textbox', { name: 'Review comment' }), 'Please update the guide.')
    await user.click(screen.getByRole('button', { name: 'Add comment' }))
    await user.click(screen.getByRole('button', { name: 'Reply' }))
    await user.type(screen.getByRole('textbox', { name: 'Reply to review comment' }), 'I will update it.')
    await user.click(screen.getByRole('button', { name: 'Add reply' }))

    await user.click(screen.getByRole('button', { name: 'Add to agent' }))
    expect(onAppendToAgentDraft).toHaveBeenCalledWith(
      'pane-1',
      expect.stringContaining('File: docs/guide.md'),
    )
    expect(onAppendToAgentDraft.mock.calls[0][1]).toContain('Comment: Please update the guide.')
    expect(onAppendToAgentDraft.mock.calls[0][1]).toContain('Reply: I will update it.')

    await user.click(screen.getByRole('button', { name: 'Resolve comment thread' }))
    expect(screen.getByText('Resolved')).toBeInTheDocument()
    expect(loadFileReviewThreads('work', 'docs/guide.md').threads[0].resolved).toBe(true)
  })

  it('allows a comment to be edited and its thread to be deleted', async () => {
    const user = userEvent.setup()
    renderComments()

    await user.click(screen.getByRole('button', { name: /add file comment/i }))
    await user.type(screen.getByRole('textbox', { name: 'Review comment' }), 'Original comment.')
    await user.click(screen.getByRole('button', { name: 'Add comment' }))
    await user.click(screen.getByRole('button', { name: 'Edit comment' }))
    const editor = screen.getByRole('textbox', { name: 'Edit comment' })
    await user.clear(editor)
    await user.type(editor, 'Updated comment.')
    await user.click(screen.getByRole('button', { name: 'Save' }))

    expect(await screen.findByText('Updated comment.')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Delete comment thread' }))
    await waitFor(() => expect(screen.queryByText('Updated comment.')).not.toBeInTheDocument())
    expect(loadFileReviewThreads('work', 'docs/guide.md').threads).toEqual([])
  })

  it('marks a thread stale when the quoted source no longer matches', async () => {
    const staleThread = createFileReviewThread(
      'docs/guide.md',
      { kind: 'lines', startLine: 1, endLine: 1, quote: 'old text' },
      'Review this',
      10,
    )
    saveFileReviewThreads('work', 'docs/guide.md', [staleThread])
    const { container } = renderComments({ sourceText: 'new text' })

    await waitFor(() => expect(within(container).getByText(/quoted text no longer matches/i)).toBeInTheDocument())
  })

  it('disables agent insertion when no agent pane is open', async () => {
    const thread = createFileReviewThread('docs/guide.md', { kind: 'file' }, 'Review this', 10)
    saveFileReviewThreads('work', 'docs/guide.md', [thread])
    renderComments({ activeAgentPaneId: null })

    const button = await screen.findByRole('button', { name: 'Add to agent' })
    expect(button).toBeDisabled()
    expect(screen.getByText('Open an agent panel to insert.')).toBeInTheDocument()
  })

  it('keeps comment storage scoped to this project file', async () => {
    const thread = createFileReviewThread('docs/guide.md', { kind: 'file' }, 'Review this', 10)
    saveFileReviewThreads('work', 'docs/guide.md', [thread])
    renderComments()

    await screen.findByText('Review this')
    expect(localStorage.getItem(fileReviewStorageKey('other', 'docs/guide.md'))).toBeNull()
  })

  it('updates the same file comments in another viewer pane', async () => {
    const user = userEvent.setup()
    render(
      <>
        <FileReviewComments project="work" path="docs/guide.md" sourceText="one" activeAgentPaneId={null}>
          <p data-source-line="1">Main pane</p>
        </FileReviewComments>
        <FileReviewComments project="work" path="docs/guide.md" sourceText="one" activeAgentPaneId={null}>
          <p data-source-line="1">Reference pane</p>
        </FileReviewComments>
      </>,
    )
    const mainPane = screen.getByText('Main pane').closest<HTMLElement>('.file-review-comments')!
    await user.click(within(mainPane).getByRole('button', { name: /add file comment/i }))
    await user.type(within(mainPane).getByRole('textbox', { name: 'Review comment' }), 'Shared local thread.')
    await user.click(within(mainPane).getByRole('button', { name: 'Add comment' }))

    await waitFor(() => expect(screen.getAllByText('Shared local thread.')).toHaveLength(2))
  })
})
