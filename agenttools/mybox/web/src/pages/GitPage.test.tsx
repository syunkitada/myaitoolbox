import { createEvent, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { api, type GitDetail } from '../api/client'
import { DialogsProvider } from '../components/AppDialogs'
import { GitWorkspace } from './GitPage'

vi.mock('../components/MonacoEditor', () => ({ default: () => null }))
vi.mock('../hooks/use-mobile', () => ({ useIsMobile: () => false }))

const detail: GitDetail = {
  is_repo: true,
  branch: 'main',
  remote: '',
  upstream: '',
  sync_status: 'no-upstream',
  ahead: 0,
  behind: 0,
  last_commit_message: '',
  staged: [{ path: 'notes.md', status: 'staged', code: 'M', diff: '' }],
  unstaged: [],
  untracked: [],
}

describe('GitWorkspace commit message', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('commits with Ctrl+Enter without inserting a newline', async () => {
    vi.spyOn(api, 'getGitStatus').mockResolvedValue(detail)
    vi.spyOn(api, 'getGitBranches').mockResolvedValue({ branches: [] })
    const gitCommit = vi.spyOn(api, 'gitCommit').mockResolvedValue({ ok: true, output: 'committed' })
    const refreshMeta = vi.fn().mockResolvedValue(undefined)

    render(
      <DialogsProvider>
        <GitWorkspace refreshMeta={refreshMeta} />
      </DialogsProvider>,
    )

    const message = await screen.findByTestId('git-commit-message')
    fireEvent.change(message, { target: { value: 'Commit from shortcut' } })
    const event = createEvent.keyDown(message, { key: 'Enter', ctrlKey: true })
    fireEvent(message, event)

    expect(event.defaultPrevented).toBe(true)
    await waitFor(() => expect(gitCommit).toHaveBeenCalledWith(undefined, 'Commit from shortcut', false))
    await waitFor(() => expect(message).toHaveValue(''))
  })
})
