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

describe('Explorer task trigger actions', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('runs a trigger directory from the context menu and shows the run result', async () => {
    const run = vi.spyOn(api, 'runTaskTrigger').mockResolvedValue({
      id: 'run-1',
      project: 'demo',
      trigger_id: 'manual-report',
      event_id: 'manual:event',
      status: 'dispatched',
      task_id: 'task-1',
      agent_name: 'agent-1',
      started_at: '2026-09-27T09:00:00Z',
      finished_at: '2026-09-27T09:00:01Z',
    })
    const onChanged = vi.fn().mockResolvedValue(undefined)
    const onSelect = vi.fn()

    render(
      <DialogsProvider>
        <Explorer
          entries={[
            { kind: 'dir', name: '_task_triggers', path: '_task_triggers', markdown: false },
            { kind: 'dir', name: 'manual-report', path: '_task_triggers/manual-report', markdown: false },
            { kind: 'dir', name: 'incoming', path: '_task_triggers/manual-report/incoming', markdown: false },
          ]}
          selected=""
          onSelect={onSelect}
          title="Files"
          favorites={[]}
          recentFiles={[]}
          onChanged={onChanged}
          showHidden={true}
          onToggleHidden={vi.fn()}
        />
      </DialogsProvider>,
    )

    fireEvent.click(screen.getByText('_task_triggers'))
    fireEvent.click(screen.getByText('manual-report'))
    fireEvent.contextMenu(screen.getByText('manual-report').closest('li')!)
    expect(screen.getByTestId('task-trigger-run')).toBeInTheDocument()
    fireEvent.click(screen.getByTestId('task-trigger-run'))

    await waitFor(() => expect(run).toHaveBeenCalledWith('manual-report'))
    expect(await screen.findByRole('dialog', { name: 'Run trigger _task_triggers/manual-report' })).toHaveTextContent('task-1')
    expect(onChanged).toHaveBeenCalledWith('_tasks/task-1/task.md')
    expect(onSelect).toHaveBeenCalledWith('_tasks/task-1/task.md')

    fireEvent.contextMenu(screen.getByText('incoming').closest('li')!)
    expect(screen.queryByTestId('task-trigger-run')).not.toBeInTheDocument()
  })
})

describe('BrowserPage file reveal', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('refreshes ancestor directories and reveals a requested file', async () => {
    const path = '_task_triggers/daily-report/task.md'
    const list = vi.spyOn(api, 'listFiles').mockImplementation(async (opts) => {
      switch (opts?.path ?? '') {
        case '':
          return [{ path: '_task_triggers', name: '_task_triggers', kind: 'dir' }]
        case '_task_triggers':
          return [{ path: '_task_triggers/daily-report', name: 'daily-report', kind: 'dir' }]
        case '_task_triggers/daily-report':
          return [{ path, name: 'task.md', kind: 'file' }]
        default:
          return []
      }
    })
    vi.spyOn(api, 'getFileGitStatus').mockResolvedValue({})
    const onSelect = vi.fn()
    const onRevealPathHandled = vi.fn()

    render(
      <DialogsProvider>
        <BrowserPage
          title="Files"
          selected=""
          onSelect={onSelect}
          onBack={vi.fn()}
          favorites={[]}
          recentFiles={[]}
          refreshMeta={vi.fn().mockResolvedValue(undefined)}
          revealPath={path}
          onRevealPathHandled={onRevealPathHandled}
        />
      </DialogsProvider>,
    )

    await waitFor(() => expect(onSelect).toHaveBeenCalledWith(path))
    expect(list).toHaveBeenCalledWith({ path: '', showHidden: true })
    expect(list).toHaveBeenCalledWith({ path: '_task_triggers', showHidden: true })
    expect(list).toHaveBeenCalledWith({ path: '_task_triggers/daily-report', showHidden: true })
    expect(onRevealPathHandled).toHaveBeenCalledWith(path)
  })
})

describe('Explorer Git status', () => {
  it('shows a Git badge for a collapsed directory with changed descendants', () => {
    render(
      <DialogsProvider>
        <Explorer
          entries={[{ kind: 'dir', name: 'docs', path: 'docs', markdown: false }]}
          selected=""
          onSelect={vi.fn()}
          title="Files"
          favorites={[]}
          recentFiles={[]}
          gitStatus={{ 'docs/reference/guide.md': 'modified' }}
        />
      </DialogsProvider>,
    )

    expect(screen.getByRole('img', { name: 'git: modified' })).toBeInTheDocument()
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

  const renderFileViewer = (status: Record<string, string>, fileContent = 'current body') => {
    vi.spyOn(api, 'listFiles').mockResolvedValue([
      { path: 'notes.txt', name: 'notes.txt', kind: 'file' },
    ])
    vi.spyOn(api, 'getFileGitStatus').mockResolvedValue(status)
    vi.spyOn(api, 'getFileContent').mockResolvedValue({ path: 'notes.txt', content: fileContent })
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

  it('copies non-Markdown file contents as-is', async () => {
    const originalClipboard = navigator.clipboard
    const writeText = vi.fn().mockResolvedValue(undefined)
    const fileContent = '## Keep this syntax\n\n**Do not convert this text.**'
    Object.defineProperty(navigator, 'clipboard', {
      configurable: true,
      value: { writeText },
    })

    try {
      renderFileViewer({}, fileContent)

      const button = await screen.findByRole('button', { name: 'Copy file contents' })
      fireEvent.click(button)

      await waitFor(() => expect(writeText).toHaveBeenCalledWith(fileContent))
      expect(screen.queryByRole('dialog', { name: 'Copy file contents' })).not.toBeInTheDocument()
    } finally {
      Object.defineProperty(navigator, 'clipboard', {
        configurable: true,
        value: originalClipboard,
      })
    }
  })

  it('shows loading and error states for the Git diff panel', () => {
    const { rerender } = render(<GitDiffPanel files={[]} loading />)
    expect(screen.getByText('Loading Git diff…')).toBeInTheDocument()

    rerender(<GitDiffPanel files={[]} error="git status failed" />)
    expect(screen.getByText('git status failed')).toBeInTheDocument()
  })
})

describe('task progress in file viewer', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('shows progress for task.md in the file viewer', async () => {
    const path = '_tasks/20260927_demo/task.md'
    vi.spyOn(api, 'listFiles').mockResolvedValue([{ path, name: 'task.md', kind: 'file' }])
    vi.spyOn(api, 'getFileContent').mockResolvedValue({
      path,
      content: '---\ntitle: demo\n---\n\n- [ ] first\n- [x] second',
    })
    vi.spyOn(api, 'recordRecent').mockResolvedValue(undefined)
    vi.spyOn(api, 'getFileGitStatus').mockResolvedValue({})

    const router = createMemoryRouter(
      [{
        path: '*',
        element: (
          <BrowserPage
            title="Files"
            selected={path}
            onSelect={vi.fn()}
            onBack={vi.fn()}
            favorites={[]}
            recentFiles={[]}
            refreshMeta={vi.fn().mockResolvedValue(undefined)}
          />
        ),
      }],
      { initialEntries: [`/projects/proj/dashboard/files/${path}`] },
    )

    render(
      <DialogsProvider>
        <RouterProvider router={router} />
      </DialogsProvider>,
    )

    expect(await screen.findByText('1/2')).toBeInTheDocument()
    expect(screen.getByTestId('task-progress')).toBeInTheDocument()
  })

  it('does not show task progress for other Markdown files', async () => {
    const path = 'docs/guide.md'
    vi.spyOn(api, 'listFiles').mockResolvedValue([{ path, name: 'guide.md', kind: 'file' }])
    vi.spyOn(api, 'getFileContent').mockResolvedValue({
      path,
      content: '- [ ] checklist item',
    })
    vi.spyOn(api, 'recordRecent').mockResolvedValue(undefined)
    vi.spyOn(api, 'getFileGitStatus').mockResolvedValue({})

    const router = createMemoryRouter(
      [{
        path: '*',
        element: (
          <BrowserPage
            title="Files"
            selected={path}
            onSelect={vi.fn()}
            onBack={vi.fn()}
            favorites={[]}
            recentFiles={[]}
            refreshMeta={vi.fn().mockResolvedValue(undefined)}
          />
        ),
      }],
      { initialEntries: [`/projects/proj/dashboard/files/${path}`] },
    )

    render(
      <DialogsProvider>
        <RouterProvider router={router} />
      </DialogsProvider>,
    )

    await screen.findByText('checklist item')
    expect(screen.queryByTestId('task-progress')).not.toBeInTheDocument()
  })
})

describe('task actions in file viewer', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  const path = '_tasks/20260927_demo/task.md'
  const content = '---\ntitle: demo\nstatus: doing\npriority: medium\n---\n\nTask body'

  const renderTaskFile = (onSelect = vi.fn()) => {
    vi.spyOn(api, 'listFiles').mockImplementation(async (opts) => {
      if (opts?.path === `_tasks/20260927_demo/tmp`) return []
      return [
        { path: '_tasks', name: '_tasks', kind: 'dir' },
        { path, name: 'task.md', kind: 'file' },
      ]
    })
    vi.spyOn(api, 'getFileContent').mockResolvedValue({ path, content })
    vi.spyOn(api, 'recordRecent').mockResolvedValue(undefined)
    vi.spyOn(api, 'getFileGitStatus').mockResolvedValue({})

    const router = createMemoryRouter(
      [{
        path: '*',
        element: (
          <BrowserPage
            title="Files"
            selected={path}
            onSelect={onSelect}
            onBack={vi.fn()}
            favorites={[]}
            recentFiles={[]}
            refreshMeta={vi.fn().mockResolvedValue(undefined)}
          />
        ),
      }],
      { initialEntries: [`/projects/proj/dashboard/files/${path}`] },
    )

    render(
      <DialogsProvider>
        <RouterProvider router={router} />
      </DialogsProvider>,
    )

    return onSelect
  }

  it('changes the task status from the Files tab', async () => {
    const update = vi.spyOn(api, 'updateTask').mockResolvedValue({
      id: '20260927_demo',
      title: 'demo',
      status: 'review',
      priority: 'medium',
    })
    renderTaskFile()

    const status = await screen.findByRole('combobox', { name: 'Task status' })
    expect(status).toHaveValue('doing')
    fireEvent.change(status, { target: { value: 'review' } })

    await waitFor(() => expect(update).toHaveBeenCalledWith('20260927_demo', { status: 'review' }))
  })

  it('archives the task from the Files tab after confirmation', async () => {
    const archive = vi.spyOn(api, 'archiveTask').mockResolvedValue(undefined)
    vi.spyOn(api, 'deleteRecent').mockResolvedValue(undefined)
    const onSelect = renderTaskFile()

    await screen.findByRole('combobox', { name: 'Task status' })
    fireEvent.click(screen.getByRole('button', { name: 'File actions' }))
    fireEvent.click(screen.getByTestId('task-archive'))
    fireEvent.click(await screen.findByTestId('app-dialog-ok'))

    await waitFor(() => expect(archive).toHaveBeenCalledWith('20260927_demo'))
    expect(onSelect).toHaveBeenCalledWith('')
  })
})
