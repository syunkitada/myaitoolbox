import { createEvent, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import userEvent from '@testing-library/user-event'
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

describe('GitWorkspace branch switching', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('switches branches from the explorer branch selector', async () => {
    const featureDetail: GitDetail = { ...detail, branch: 'feature' }
    vi.spyOn(api, 'getGitStatus')
      .mockResolvedValueOnce(detail)
      .mockResolvedValue(featureDetail)
    vi.spyOn(api, 'getGitBranches')
      .mockResolvedValueOnce({
        branches: [
          { name: 'main', current: true },
          { name: 'feature', current: false },
        ],
      })
      .mockResolvedValue({
        branches: [
          { name: 'main', current: false },
          { name: 'feature', current: true },
        ],
      })
    const gitCheckout = vi.spyOn(api, 'gitCheckout').mockResolvedValue({ ok: true, output: '' })
    const refreshMeta = vi.fn().mockResolvedValue(undefined)

    render(
      <DialogsProvider>
        <GitWorkspace refreshMeta={refreshMeta} />
      </DialogsProvider>,
    )

    const user = userEvent.setup()
    const branchSwitcher = await screen.findByTestId('git-explorer-branch-switcher')
    expect(branchSwitcher).toHaveTextContent('main')

    await user.click(branchSwitcher)
    expect(screen.getByTestId('git-new-branch')).toBeInTheDocument()
    await user.keyboard('{ArrowDown}')
    expect(screen.getByTestId('git-branch-current')).toHaveFocus()
    await user.keyboard('{ArrowDown}')
    expect(screen.getByRole('menuitem', { name: 'feature' })).toHaveFocus()
    await user.keyboard('{Escape}')
    expect(branchSwitcher).toHaveFocus()
    expect(screen.queryByRole('menu')).not.toBeInTheDocument()

    await user.click(branchSwitcher)
    await user.click(await screen.findByRole('menuitem', { name: 'feature' }))

    expect(branchSwitcher).toHaveFocus()
    await waitFor(() => expect(gitCheckout).toHaveBeenCalledWith(undefined, 'feature'))
    await waitFor(() => expect(branchSwitcher).toHaveTextContent('feature'))
  })
})
