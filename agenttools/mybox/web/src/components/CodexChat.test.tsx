import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { api } from '../api/client'
import { DialogsProvider } from './AppDialogs'
import { CodexChat } from './CodexChat'

vi.mock('../api/client', () => ({
  api: {
    listProjects: vi.fn(),
    listCodexThreads: vi.fn(),
    createCodexThread: vi.fn(),
    getCodexThreadHistory: vi.fn(),
  },
}))

const threads = [{
  thread_id: 'thread-1',
  title: 'Refactor sidebar',
  preview: 'Improve the agent panel',
  created_at: 1_800_000_000,
  updated_at: 1_800_000_100,
  status: 'idle',
  model: 'gpt-5-codex',
}]

function renderCodexChat(onThreadSelected?: (threadId: string | null) => void) {
  return render(<DialogsProvider><CodexChat project="demo" onThreadSelected={onThreadSelected} /></DialogsProvider>)
}

async function selectThread(threadId = 'thread-1') {
  const title = await screen.findByText(threadId === 'thread-1' ? 'Refactor sidebar' : 'New conversation')
  const button = title.closest('button')
  if (!button) throw new Error(`Conversation ${threadId} has no selectable button`)
  fireEvent.click(button)
}

class MockWebSocket {
  static OPEN = 1
  static latest: MockWebSocket | undefined
  readyState = 0
  onopen: (() => void) | null = null
  onclose: (() => void) | null = null
  onerror: (() => void) | null = null
  onmessage: ((event: MessageEvent) => void) | null = null
  sent: string[] = []

  constructor() {
    MockWebSocket.latest = this
    queueMicrotask(() => {
      this.readyState = MockWebSocket.OPEN
      this.onopen?.()
    })
  }

  send(value: string) {
    this.sent.push(value)
  }

  close() {
    this.readyState = 3
    this.onclose?.()
  }

  fail() {
    this.onerror?.()
    this.close()
  }

  emit(value: unknown) {
    this.onmessage?.({ data: JSON.stringify(value) } as MessageEvent)
  }
}

describe('CodexChat', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(api.listProjects).mockResolvedValue([{ name: 'demo', path: '/workspace/demo' }])
    vi.mocked(api.listCodexThreads).mockResolvedValue({ threads, truncated: false })
    vi.mocked(api.createCodexThread).mockResolvedValue(threads[0])
    vi.mocked(api.getCodexThreadHistory).mockResolvedValue({
      thread: threads[0],
      turns: [{
        id: 'turn-1',
        status: 'inProgress',
        items: [
          { id: 'user-1', type: 'userMessage', content: [{ type: 'text', text: 'Update the sidebar' }] },
          { id: 'assistant-1', type: 'agentMessage', text: 'I will update the **sidebar**.' },
        ],
      }],
      next_cursor: null,
    })
    vi.stubGlobal('WebSocket', MockWebSocket)
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('lists project conversations and renders readable Codex history', async () => {
    renderCodexChat()

    await selectThread()

    expect(await screen.findByText('Update the sidebar')).toBeInTheDocument()
    expect(screen.getByTestId('codex-chat-conversation').querySelector('.markdown-body')).toHaveTextContent('I will update the sidebar.')
    expect(await screen.findByRole('button', { name: 'Codexを停止' })).toBeInTheDocument()
    expect(api.getCodexThreadHistory).toHaveBeenCalledWith('thread-1', { limit: 100 }, 'demo')
  })

  it('shows connection progress and explains a WebSocket failure', async () => {
    renderCodexChat()
    await selectThread()

    expect(await screen.findByText('接続中…')).toBeInTheDocument()
    await act(async () => {
      MockWebSocket.latest?.fail()
    })

    expect(await screen.findByRole('alert')).toHaveTextContent(/Codexとの接続が(失敗しました|切れました)/)
  })

  it('refreshes the open conversation history when the tab becomes visible', async () => {
    const latestTurn = {
      id: 'turn-2',
      status: 'completed',
      items: [{ id: 'assistant-2', type: 'agentMessage', text: 'The latest response' }],
    }
    vi.mocked(api.getCodexThreadHistory)
      .mockResolvedValueOnce({
        thread: threads[0],
        turns: [{
          id: 'turn-1',
          status: 'completed',
          items: [{ id: 'assistant-1', type: 'agentMessage', text: 'An earlier response' }],
        }],
        next_cursor: 'older-page-cursor',
      })
      .mockResolvedValueOnce({
        thread: threads[0],
        turns: [
          { id: 'turn-1', status: 'completed', items: [{ id: 'assistant-1', type: 'agentMessage', text: 'An earlier response' }] },
          latestTurn,
        ],
        next_cursor: 'updated-first-page-cursor',
      })
    renderCodexChat()
    await selectThread()
    expect(await screen.findByText('An earlier response')).toBeInTheDocument()

    await act(async () => {
      document.dispatchEvent(new Event('visibilitychange'))
    })

    expect(await screen.findByText('The latest response')).toBeInTheDocument()
    expect(api.getCodexThreadHistory).toHaveBeenCalledTimes(2)

    fireEvent.click(screen.getByRole('button', { name: '以前の会話を読み込む' }))
    await waitFor(() => expect(api.getCodexThreadHistory).toHaveBeenNthCalledWith(
      3,
      'thread-1',
      { cursor: 'older-page-cursor', limit: 100 },
      'demo',
    ))
  })

  it('keeps history readable and offers refresh when another Codex owns the thread', async () => {
    renderCodexChat()
    await selectThread()
    expect(await screen.findByText('Update the sidebar')).toBeInTheDocument()

    await act(async () => {
      MockWebSocket.latest?.emit({ type: 'writer_conflict' })
    })

    expect(await screen.findByRole('status')).toHaveTextContent('別のCodexが操作中')
    expect(screen.getByRole('textbox', { name: 'Codexへのメッセージ' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Codexを停止' })).toBeDisabled()

    fireEvent.click(screen.getByRole('button', { name: '履歴を更新' }))
    await waitFor(() => expect(api.getCodexThreadHistory).toHaveBeenCalledTimes(2))
  })

  it('keeps Enter for newlines and submits the composer with Ctrl+Enter', async () => {
    renderCodexChat()
    await selectThread()
    await screen.findByText(/I will update the/)

    const socket = MockWebSocket.latest
    await act(async () => {
      socket?.emit({ type: 'ready' })
    })

    const composer = screen.getByRole('textbox', { name: 'Codexへのメッセージ' })
    fireEvent.change(composer, { target: { value: 'draft message' } })

    expect(fireEvent.keyDown(composer, { key: 'Enter' })).toBe(true)
    expect(socket?.sent).not.toContain(JSON.stringify({ type: 'prompt', text: 'draft message' }))
    expect(composer).toHaveValue('draft message')

    expect(fireEvent.keyDown(composer, { key: 'Enter', ctrlKey: true })).toBe(false)
    await waitFor(() => expect(socket?.sent).toContain(JSON.stringify({ type: 'prompt', text: 'draft message' })))
    expect(composer).toHaveValue('')
  })

  it('sends a selected quick prompt from the Codex command palette', async () => {
    renderCodexChat()
    await selectThread()
    await screen.findByText(/I will update the/)

    const socket = MockWebSocket.latest
    await act(async () => {
      socket?.emit({ type: 'ready' })
    })

    const composer = screen.getByRole('textbox', { name: 'Codexへのメッセージ' })
    fireEvent.change(composer, { target: { value: 'existing draft' } })
    fireEvent.click(screen.getByRole('button', { name: 'Codexコマンド' }))

    const palette = await screen.findByTestId('codex-command-palette')
    expect(within(palette).getByRole('button', { name: '進めて' })).toBeInTheDocument()
    fireEvent.click(within(palette).getByRole('button', { name: '進めて' }))

    await waitFor(() => expect(socket?.sent).toContain(JSON.stringify({ type: 'prompt', text: '進めて' })))
    expect(composer).toHaveValue('existing draft')
    await waitFor(() => expect(screen.queryByTestId('codex-command-palette')).not.toBeInTheDocument())
  })

  it('compacts the open thread through the Codex app-server command', async () => {
    renderCodexChat()
    await selectThread()
    await screen.findByText(/I will update the/)

    const socket = MockWebSocket.latest
    await act(async () => {
      socket?.emit({ type: 'ready' })
    })
    fireEvent.click(screen.getByRole('button', { name: 'Codexコマンド' }))
    const palette = await screen.findByTestId('codex-command-palette')
    fireEvent.click(within(palette).getByRole('button', { name: '/compact' }))

    await waitFor(() => expect(socket?.sent).toContain(JSON.stringify({ type: 'compact' })))
  })

  it('starts a new conversation from the Codex command palette', async () => {
    const newThread = {
      ...threads[0],
      thread_id: 'thread-2',
      title: 'New conversation',
      preview: '',
      created_at: 1_800_000_200,
      updated_at: 1_800_000_200,
    }
    const onThreadSelected = vi.fn()
    vi.mocked(api.createCodexThread).mockResolvedValue(newThread)
    vi.mocked(api.getCodexThreadHistory).mockImplementation(async (threadId) => ({
      thread: threadId === newThread.thread_id ? newThread : threads[0],
      turns: threadId === newThread.thread_id ? [] : [{
        id: 'turn-1',
        status: 'inProgress',
        items: [
          { id: 'user-1', type: 'userMessage', content: [{ type: 'text', text: 'Update the sidebar' }] },
          { id: 'assistant-1', type: 'agentMessage', text: 'I will update the **sidebar**.' },
        ],
      }],
      next_cursor: null,
    }))
    renderCodexChat(onThreadSelected)
    await selectThread()
    await screen.findByText(/I will update the/)
    fireEvent.click(screen.getByRole('button', { name: 'Codexコマンド' }))
    const palette = await screen.findByTestId('codex-command-palette')
    fireEvent.click(within(palette).getByRole('button', { name: '新しい会話' }))

    await waitFor(() => expect(api.createCodexThread).toHaveBeenCalledWith('demo'))
    expect(await screen.findByText('New conversation')).toBeInTheDocument()
    expect(screen.getByTestId('codex-session-thread-2')).toHaveAttribute('aria-current', 'true')
    await waitFor(() => expect(onThreadSelected).toHaveBeenLastCalledWith('thread-2'))
    expect(api.getCodexThreadHistory).toHaveBeenLastCalledWith('thread-2', { limit: 100 }, 'demo')
    await waitFor(() => expect(screen.queryByTestId('codex-command-palette')).not.toBeInTheDocument())
  })

  it('sends approval responses through the app-server socket', async () => {
    renderCodexChat()
    await selectThread()
    await screen.findByText(/I will update the/)

    const socket = MockWebSocket.latest
    expect(socket).toBeDefined()
    socket?.emit({
      type: 'server_request',
      request_id: 'approval-1',
      method: 'item/commandExecution/requestApproval',
      params: { command: 'npm test', cwd: '/workspace/demo', reason: 'Run the test suite', availableDecisions: ['accept', 'decline'] },
    })
    fireEvent.click(await screen.findByRole('button', { name: '今回許可' }))
    await waitFor(() => expect(socket?.sent).toContain(JSON.stringify({ type: 'respond', request_id: 'approval-1', decision: 'accept', answers: undefined })))
    socket?.emit({ type: 'response', request_id: 'approval-1' })
    await waitFor(() => expect(screen.queryByRole('region', { name: 'Codex approval request' })).not.toBeInTheDocument())
  })

  it('updates the conversation to use Codex automatic approval review when selected', async () => {
    renderCodexChat()
    await selectThread()
    await screen.findByText(/I will update the/)

    const socket = MockWebSocket.latest
    await act(async () => socket?.emit({ type: 'ready', approvals_reviewer: 'user' }))
    fireEvent.change(screen.getByRole('combobox', { name: '承認モード' }), { target: { value: 'auto_review' } })
    await waitFor(() => expect(socket?.sent.some((message) => message.includes('"type":"set_approvals_reviewer"') && message.includes('"approvals_reviewer":"auto_review"'))).toBe(true))
    const control = JSON.parse(socket?.sent.at(-1) ?? '{}') as { request_id: string }
    await act(async () => socket?.emit({ type: 'control_response', control: 'set_approvals_reviewer', request_id: control.request_id, result: { approvals_reviewer: 'auto_review' } }))
    expect(screen.getByRole('combobox', { name: '承認モード' })).toHaveValue('auto_review')
  })
})
