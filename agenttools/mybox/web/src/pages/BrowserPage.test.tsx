import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { createMemoryRouter, RouterProvider } from 'react-router-dom'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { api, GitDetail } from '../api/client'
import { DialogsProvider } from '../components/AppDialogs'
import { BrowserEntry, BrowserPage, Explorer, GitDiffPanel, gitFilesForPath } from './BrowserPage'

vi.mock('../components/MonacoEditor', () => ({ default: () => null }))
vi.mock('../hooks/use-mobile', () => ({ useIsMobile: () => false }))

describe('Explorer file upload', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  const renderExplorer = () => {
    const entries: BrowserEntry[] = [
      { kind: 'dir', name: 'uploads', path: 'uploads', markdown: true },
    ]
    render(
      <DialogsProvider>
        <Explorer
          entries={entries}
          selected=""
          onSelect={vi.fn()}
          title="Files"
          favorites={[]}
          recentFiles={[]}
          onMoveFile={vi.fn()}
          onChanged={vi.fn().mockResolvedValue(undefined)}
          onLoadDir={vi.fn().mockResolvedValue(undefined)}
          showHidden={true}
          onToggleHidden={vi.fn()}
        />
      </DialogsProvider>,
    )

    const row = screen.getByText('uploads').closest('li')
    if (!row) throw new Error('upload directory row was not rendered')
    fireEvent.contextMenu(row)
    fireEvent.click(screen.getByTestId('file-upload'))
    return screen.getByLabelText('Upload files')
  }

  it('shows a success dialog after the upload completes', async () => {
    const upload = vi.spyOn(api, 'uploadFiles').mockResolvedValue(undefined)
    const input = renderExplorer()
    const file = new File(['zip data'], 'archive.zip', { type: 'application/zip' })

    fireEvent.change(input, { target: { files: [file] } })

    await waitFor(() => expect(screen.getByRole('dialog')).toHaveTextContent('Upload succeeded'))
    expect(upload).toHaveBeenCalledWith('uploads', [file])
  })

  it('shows an in-progress modal while the upload is pending', async () => {
    let finishUpload!: () => void
    const upload = vi.spyOn(api, 'uploadFiles').mockReturnValue(
      new Promise<void>((resolve) => {
        finishUpload = resolve
      }),
    )
    const input = renderExplorer()

    fireEvent.change(input, { target: { files: [new File(['zip data'], 'archive.zip')] } })

    await waitFor(() => {
      const dialog = screen.getByTestId('app-dialog')
      expect(dialog).toBeVisible()
      expect(dialog).toHaveAttribute('data-dialog-kind', 'progress')
      expect(dialog).toHaveTextContent('Uploading 1 file to uploads')
    })
    expect(upload).toHaveBeenCalledWith('uploads', expect.any(Array))

    finishUpload()
    await waitFor(() => expect(screen.getByRole('dialog')).toHaveTextContent('Upload succeeded'))
  })

  it('shows the server error in a failure dialog', async () => {
    vi.spyOn(api, 'uploadFiles').mockRejectedValue(new Error('upload exceeds the maximum size'))
    const input = renderExplorer()

    fireEvent.change(input, { target: { files: [new File(['zip data'], 'archive.zip')] } })

    await waitFor(() => expect(screen.getByRole('dialog')).toHaveTextContent('Upload failed'))
    expect(screen.getByRole('dialog')).toHaveTextContent('upload exceeds the maximum size')
  })

  it('searches file contents and reports the selected hit', async () => {
    const search = vi.spyOn(api, 'searchFiles').mockResolvedValue({
      query: 'deploy',
      results: [{ path: 'docs/guide.md', line: 4, snippet: 'Deploy here', match_count: 1 }],
      total: 1,
      truncated: false,
    })
    const onSearchHit = vi.fn()
    const onSelect = vi.fn()

    render(
      <DialogsProvider>
        <Explorer
          entries={[{ kind: 'file', name: 'guide.md', path: 'docs/guide.md', markdown: true }]}
          selected=""
          onSelect={onSelect}
          onSearchHit={onSearchHit}
          title="Files"
          favorites={[]}
          recentFiles={[]}
          onMoveFile={vi.fn()}
          showHidden={false}
          onToggleHidden={vi.fn()}
        />
      </DialogsProvider>,
    )

    const nameButton = screen.getByRole('button', { name: 'Name' })
    const textButton = screen.getByRole('button', { name: 'Text' })
    expect(nameButton.querySelectorAll('svg')).toHaveLength(1)
    expect(textButton.querySelectorAll('svg')).toHaveLength(1)

    fireEvent.click(textButton)
    const searchbox = screen.getByRole('searchbox')
    fireEvent.change(searchbox, { target: { value: 'deploy' } })
    fireEvent.submit(searchbox)

    await waitFor(() => expect(search).toHaveBeenCalledWith({ q: 'deploy', showHidden: false }))
    expect(await screen.findByText('docs/guide.md')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: /docs\/guide\.md/ }))
    expect(onSearchHit).toHaveBeenCalledWith({ path: 'docs/guide.md', line: 4, query: 'deploy' })
    expect(onSelect).not.toHaveBeenCalled()
  })
})

describe('file viewer Git diff', () => {
  const detail: GitDetail = {
    is_repo: true,
    branch: 'main',
    remote: '',
    upstream: '',
    sync_status: 'no-upstream',
    ahead: 0,
    behind: 0,
    last_commit_message: '',
    staged: [],
    unstaged: [{ path: 'notes.txt', status: 'unstaged', code: 'M', diff: '+changed' }],
    untracked: [],
  }

  const renderFileViewer = (status: Record<string, string>) => {
    vi.spyOn(api, 'listFiles').mockResolvedValue([
      { path: 'notes.txt', name: 'notes.txt', kind: 'file' },
    ])
    vi.spyOn(api, 'getFileGitStatus').mockResolvedValue(status)
    vi.spyOn(api, 'getFileContent').mockResolvedValue({ path: 'notes.txt', content: 'current body' })
    vi.spyOn(api, 'recordRecent').mockResolvedValue(undefined)
    const refreshMeta = vi.fn().mockResolvedValue(undefined)
    const router = createMemoryRouter(
      [{ path: '*', element: (
        <BrowserPage
          title="Files"
          selected="notes.txt"
          onSelect={vi.fn()}
          onBack={vi.fn()}
          favorites={[]}
          recentFiles={[]}
          refreshMeta={refreshMeta}
        />
      ) }],
      { initialEntries: ['/projects/proj/dashboard/files/notes.txt'] },
    )
    render(
      <DialogsProvider>
        <RouterProvider router={router} />
      </DialogsProvider>,
    )
  }

  it('keeps staged and unstaged diffs for the selected file', () => {
    const detail: GitDetail = {
      is_repo: true,
      branch: 'main',
      remote: '',
      upstream: '',
      sync_status: 'no-upstream',
      ahead: 0,
      behind: 0,
      last_commit_message: '',
      staged: [{ path: 'notes.md', status: 'staged', code: 'M', diff: '+staged' }],
      unstaged: [{ path: 'notes.md', status: 'unstaged', code: 'M', diff: '+unstaged' }],
      untracked: [{ path: 'other.md', status: 'untracked', code: '??', diff: '+other' }],
    }

    expect(gitFilesForPath(detail, 'notes.md')).toEqual([
      detail.staged[0],
      detail.unstaged[0],
    ])
  })

  it('renders each selected Git diff with its status', () => {
    render(
      <GitDiffPanel
        files={[
          { path: 'notes.md', status: 'staged', code: 'M', diff: '+staged' },
          { path: 'notes.md', status: 'unstaged', code: 'M', diff: '+unstaged' },
        ]}
      />,
    )

    expect(screen.getByTestId('git-file-diff')).toBeInTheDocument()
    expect(screen.getByText('Staged')).toBeInTheDocument()
    expect(screen.getByText('Unstaged')).toBeInTheDocument()
    expect(screen.getByText('+staged')).toBeInTheDocument()
    expect(screen.getByText('+unstaged')).toBeInTheDocument()
  })

  it('shows the Git button and compares the current file with its diff', async () => {
    const getGitStatus = vi.spyOn(api, 'getGitStatus').mockResolvedValue(detail)
    renderFileViewer({ 'notes.txt': 'modified' })

    const button = await screen.findByRole('button', { name: 'Show Git diff' })
    fireEvent.click(button)

    await waitFor(() => expect(getGitStatus).toHaveBeenCalledTimes(1))
    expect(screen.getByText('current body')).toBeInTheDocument()
    expect(screen.getByText('+changed')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Hide Git diff' })).toBeInTheDocument()
    const scrollContainer = screen.getByTestId('git-files-scroll-container')
    expect(scrollContainer).toHaveAttribute('data-git-diff-open', 'true')
    expect(scrollContainer).toHaveClass('overflow-hidden')
    expect(screen.getByTestId('git-file-content-pane')).toHaveClass('overflow-y-auto')
    expect(screen.getByTestId('git-file-diff-pane')).toHaveClass('overflow-y-auto')

    fireEvent.click(screen.getByRole('button', { name: 'Hide Git diff' }))
    expect(scrollContainer).toHaveAttribute('data-git-diff-open', 'false')
    expect(scrollContainer).toHaveClass('overflow-y-auto')
  })

  it('does not show the Git button for a clean file', async () => {
    renderFileViewer({})

    await screen.findByText('current body')
    expect(screen.queryByRole('button', { name: 'Show Git diff' })).not.toBeInTheDocument()
  })

  it('shows loading and error states for the Git diff panel', () => {
    const { rerender } = render(<GitDiffPanel files={[]} loading />)
    expect(screen.getByText('Loading Git diff…')).toBeInTheDocument()

    rerender(<GitDiffPanel files={[]} error="git status failed" />)
    expect(screen.getByText('git status failed')).toBeInTheDocument()
  })
})
