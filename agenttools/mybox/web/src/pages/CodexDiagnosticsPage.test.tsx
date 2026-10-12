import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { api } from '../api/client'
import { CodexDiagnosticsPage } from './CodexDiagnosticsPage'

vi.mock('../api/client', () => ({
  api: {
    getCodexDiagnostics: vi.fn(),
    getCodexDiagnosticContent: vi.fn(),
    getCodexSkillFiles: vi.fn(),
  },
}))

const diagnostics = {
  project: 'demo',
  fetched_at: '2026-10-12T00:00:00Z',
  codex: {
    status: 'ok',
    available: true,
    version: '0.162.1',
    model: 'gpt-5',
    model_provider: 'openai',
    authenticated: true,
    feature_flags: ['hooks'],
    status_reasons: [],
  },
  config_files: [{ scope: 'user', path: '/tmp/config.toml', exists: true, readable: true, status: 'loaded', is_symlink: true }],
  instructions: [{ scope: 'project', path: '/tmp/AGENTS.md', exists: true, status: 'present', is_symlink: true }],
  skills: [{ name: 'demo-skill', description: 'A demo skill', scope: 'user', path: '/tmp/SKILL.md', status: 'available', is_symlink: true }],
  skills_truncated: false,
  mcp_servers: [{ name: 'demo', scope: 'effective', source: 'codex mcp list', transport: 'stdio', command: 'node', status: 'enabled', auth_status: 'Unsupported', health_status: 'not_checked' }],
  errors: [],
}

describe('CodexDiagnosticsPage', () => {
  beforeEach(() => {
    vi.mocked(api.getCodexDiagnostics).mockResolvedValue(diagnostics)
    vi.mocked(api.getCodexDiagnosticContent).mockImplementation(async (kind, path) => ({
      kind,
      scope: kind === 'config' ? 'user' : 'project',
      path,
      content: kind === 'config' ? 'model = "gpt-5"\napi_key = "[REDACTED]"\n' : `${kind} source\n`,
      is_symlink: kind === 'skill',
      redacted: kind === 'config',
      truncated: false,
      size_bytes: 42,
      fetched_at: '2026-10-12T00:00:00Z',
    }))
    vi.mocked(api.getCodexSkillFiles).mockResolvedValue({
      path: '/tmp',
      entries: [
        { name: 'SKILL.md', path: '/tmp/SKILL.md', kind: 'file', size_bytes: 42, is_symlink: true, target: '/real/SKILL.md' },
        { name: 'references', path: '/tmp/references', kind: 'directory', size_bytes: 0, is_symlink: false, target: '' },
      ],
      truncated: false,
      fetched_at: '2026-10-12T00:00:00Z',
    })
  })

  it('shows diagnostics and keeps MCP health explicitly unchecked', async () => {
    render(<MemoryRouter initialEntries={['/projects/demo/agent']}><CodexDiagnosticsPage /></MemoryRouter>)

    expect(await screen.findByText('0.162.1')).toBeInTheDocument()
    expect(screen.getByText('gpt-5')).toBeInTheDocument()
    await userEvent.setup().click(screen.getByRole('tab', { name: /MCP/ }))
    expect(await screen.findByText('not checked')).toBeInTheDocument()
    expect(await screen.findByText(/not launched or health-checked/)).toBeInTheDocument()
  })

  it('shows why Codex runtime is warning', async () => {
    vi.mocked(api.getCodexDiagnostics).mockResolvedValue({
      ...diagnostics,
      codex: {
        ...diagnostics.codex,
        status: 'warning',
        status_reasons: ['state.rollout_db_parity (warning): missing active rows=4'],
      },
    })
    render(<MemoryRouter initialEntries={['/projects/demo/agent']}><CodexDiagnosticsPage /></MemoryRouter>)

    expect(await screen.findByText('Why this is warning')).toBeInTheDocument()
    expect(screen.getByRole('alert')).toHaveTextContent('missing active rows=4')
  })

  it('styles Codex runtime error reasons as destructive', async () => {
    vi.mocked(api.getCodexDiagnostics).mockResolvedValue({
      ...diagnostics,
      codex: {
        ...diagnostics.codex,
        status: 'error',
        status_reasons: ['auth.credentials (error): auth configuration failed'],
      },
    })
    render(<MemoryRouter initialEntries={['/projects/demo/agent']}><CodexDiagnosticsPage /></MemoryRouter>)

    const alert = await screen.findByRole('alert')
    expect(alert).toHaveTextContent('Why this is error')
    expect(alert).toHaveClass('border-red-200', 'bg-red-50', 'text-red-900')
  })

  it('allows manual refresh', async () => {
    render(<MemoryRouter initialEntries={['/projects/demo/agent']}><CodexDiagnosticsPage /></MemoryRouter>)
    await screen.findByText('0.162.1')
    fireEvent.click(screen.getByRole('button', { name: 'Refresh' }))
    await waitFor(() => expect(api.getCodexDiagnostics).toHaveBeenCalledTimes(2))
  })

  it('shows when the standard Skill list is truncated', async () => {
    const user = userEvent.setup()
    vi.mocked(api.getCodexDiagnostics).mockResolvedValue({ ...diagnostics, skills_truncated: true })
    render(<MemoryRouter initialEntries={['/projects/demo/agent']}><CodexDiagnosticsPage /></MemoryRouter>)

    await user.click(await screen.findByRole('tab', { name: 'Skills' }))
    expect(await screen.findByText(/Only the first 256 Skills/)).toBeInTheDocument()
  })

  it('loads selected config content into the same page panel', async () => {
    const user = userEvent.setup()
    render(<MemoryRouter initialEntries={['/projects/demo/agent']}><CodexDiagnosticsPage /></MemoryRouter>)

    await screen.findByText('0.162.1')
    await user.click(screen.getByRole('button', { name: /\/tmp\/config\.toml/ }))

    expect(api.getCodexDiagnosticContent).toHaveBeenCalledWith('config', '/tmp/config.toml', undefined)
    expect(screen.getAllByText('Symbolic link').length).toBeGreaterThan(0)
    expect(await screen.findByText(/api_key = "\[REDACTED\]"/)).toBeInTheDocument()
    expect(screen.getAllByText(/secret-like values are masked/i).length).toBeGreaterThan(0)
  })

  it('loads instruction and skill source text from their selected tabs', async () => {
    const user = userEvent.setup()
    render(<MemoryRouter initialEntries={['/projects/demo/agent']}><CodexDiagnosticsPage /></MemoryRouter>)

    await screen.findByText('0.162.1')
    await user.click(screen.getByRole('tab', { name: 'Instructions' }))
    await user.click(screen.getByRole('button', { name: /\/tmp\/AGENTS\.md/ }))
    expect(api.getCodexDiagnosticContent).toHaveBeenCalledWith('instruction', '/tmp/AGENTS.md', undefined)
    expect(await screen.findByText('instruction source')).toBeInTheDocument()

    await user.click(screen.getByRole('tab', { name: 'Skills' }))
    expect(screen.getAllByText('Symbolic link').length).toBeGreaterThan(0)
    await user.click(screen.getByRole('button', { name: /demo-skill/ }))
    expect(api.getCodexSkillFiles).toHaveBeenCalledWith('/tmp', undefined)
    expect(api.getCodexDiagnosticContent).toHaveBeenCalledWith('skill', '/tmp/SKILL.md', undefined)
    expect(await screen.findByText('skill source')).toBeInTheDocument()
  })

  it('expands Skill directories and loads a selected nested file', async () => {
    const user = userEvent.setup()
    vi.mocked(api.getCodexSkillFiles).mockImplementation(async (path) => ({
      path,
      entries: path === '/tmp'
        ? [{ name: 'references', path: '/tmp/references', kind: 'directory', size_bytes: 0, is_symlink: false, target: '' }]
        : [{ name: 'guide.md', path: '/tmp/references/guide.md', kind: 'file', size_bytes: 18, is_symlink: false, target: '' }],
      truncated: false,
      fetched_at: '2026-10-12T00:00:00Z',
    }))
    render(<MemoryRouter initialEntries={['/projects/demo/agent']}><CodexDiagnosticsPage /></MemoryRouter>)

    await screen.findByText('0.162.1')
    await user.click(screen.getByRole('tab', { name: 'Skills' }))
    await user.click(await screen.findByRole('button', { name: /references/ }))
    expect(api.getCodexSkillFiles).toHaveBeenCalledWith('/tmp/references', undefined)
    await user.click(await screen.findByRole('button', { name: /guide\.md/ }))
    expect(api.getCodexDiagnosticContent).toHaveBeenCalledWith('skill', '/tmp/references/guide.md', undefined)
  })

  it('shows when a Skill directory listing is truncated', async () => {
    const user = userEvent.setup()
    vi.mocked(api.getCodexSkillFiles).mockResolvedValue({
      path: '/tmp',
      entries: [{ name: 'SKILL.md', path: '/tmp/SKILL.md', kind: 'file', size_bytes: 42, is_symlink: false, target: '' }],
      truncated: true,
      fetched_at: '2026-10-12T00:00:00Z',
    })
    render(<MemoryRouter initialEntries={['/projects/demo/agent']}><CodexDiagnosticsPage /></MemoryRouter>)

    await screen.findByText('0.162.1')
    await user.click(screen.getByRole('tab', { name: 'Skills' }))
    expect(await screen.findByText('Only the first 512 entries are shown.')).toBeInTheDocument()
  })
})
