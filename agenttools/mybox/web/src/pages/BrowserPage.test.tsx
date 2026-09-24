import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { api } from '../api/client'
import { DialogsProvider } from '../components/AppDialogs'
import { BrowserEntry, Explorer } from './BrowserPage'

vi.mock('../components/MonacoEditor', () => ({ default: () => null }))

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
})
