import { type ReactNode, useCallback, useEffect, useState } from 'react'
import { Bot, ChevronDown, ChevronRight, File, FileCog, Folder, KeyRound, Link2, RefreshCw, Server, ShieldCheck, Sparkles } from 'lucide-react'
import { api, CodexDiagnosticContentKind, CodexDiagnostics, CodexSkill, CodexSkillFile } from '../api/client'
import { getProject } from '../utils/routes'
import { Badge } from '../components/ui/badge'
import { Button } from '../components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/card'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '../components/ui/tabs'
import { cn } from '@/lib/utils'

function display(value: string | null | undefined): string {
  return value?.trim() || '—'
}

function humanize(value: string): string {
  return value.replaceAll('_', ' ')
}

function StatusBadge({ status }: { status: string }) {
  const variant = status === 'ok' || status === 'available' || status === 'present' || status === 'loaded'
    ? 'default'
    : status === 'warning' || status === 'unavailable' || status === 'not_checked'
      ? 'secondary'
      : status === 'error' || status === 'invalid' || status === 'unreadable'
        ? 'destructive'
        : 'outline'
  return <Badge variant={variant}>{humanize(status)}</Badge>
}

function DetailList({ values }: { values: Array<[string, string | null | undefined]> }) {
  return (
    <dl className="grid gap-3 sm:grid-cols-2">
      {values.map(([label, value]) => (
        <div key={label} className="min-w-0">
          <dt className="text-xs text-muted-foreground">{label}</dt>
          <dd className="mt-1 break-words text-sm font-medium">{display(value)}</dd>
        </div>
      ))}
    </dl>
  )
}

function EmptyState({ children }: { children: string }) {
  return <p className="text-sm text-muted-foreground">{children}</p>
}

interface CodexContentItem {
  path: string
  scope: string
  status: string
  readable: boolean
  isSymlink: boolean
  label?: string
  description?: string
}

interface CodexContentBrowserProps {
  kind: CodexDiagnosticContentKind
  items: CodexContentItem[]
  project?: string
  refreshKey: string
  emptyMessage: string
}

function CodexContentBrowser({ kind, items, project, refreshKey, emptyMessage }: CodexContentBrowserProps) {
  const readableItems = items.filter((item) => item.readable)
  const [selectedPath, setSelectedPath] = useState('')
  const [content, setContent] = useState<Awaited<ReturnType<typeof api.getCodexDiagnosticContent>> | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const selectedItem = items.find((item) => item.path === selectedPath)

  useEffect(() => {
    setSelectedPath((current) => readableItems.some((item) => item.path === current) ? current : readableItems[0]?.path ?? '')
  }, [kind, items])

  useEffect(() => {
    if (!selectedPath) {
      setContent(null)
      setError(null)
      return
    }
    let active = true
    setLoading(true)
    setError(null)
    void api.getCodexDiagnosticContent(kind, selectedPath, project || undefined)
      .then((result) => {
        if (active) setContent(result)
      })
      .catch((cause: unknown) => {
        if (active) {
          setContent(null)
          setError(cause instanceof Error ? cause.message : String(cause))
        }
      })
      .finally(() => {
        if (active) setLoading(false)
      })
    return () => {
      active = false
    }
  }, [kind, project, refreshKey, selectedPath])

  if (items.length === 0) return <EmptyState>{emptyMessage}</EmptyState>

  return (
    <div className="grid gap-4 md:grid-cols-[minmax(15rem,0.35fr)_minmax(0,1fr)]">
      <div className="space-y-2">
        {items.map((item) => (
          <button
            key={item.path}
            type="button"
            disabled={!item.readable}
            aria-pressed={selectedPath === item.path}
            onClick={() => setSelectedPath(item.path)}
            className={cn(
              'w-full rounded-md border px-3 py-2 text-left transition-colors',
              item.readable ? 'hover:bg-muted/60' : 'cursor-not-allowed opacity-60',
              selectedPath === item.path && 'border-primary bg-muted',
            )}
          >
            <div className="flex items-start justify-between gap-2">
              <div className="min-w-0">
                <div className="flex flex-wrap items-center gap-2">
                  <div className="truncate text-sm font-medium">{item.label ?? item.scope}</div>
                  {item.isSymlink && <Badge variant="outline">Symbolic link</Badge>}
                </div>
                <div className="mt-1 break-all font-mono text-[11px] text-muted-foreground">Source: {item.path}</div>
                {item.description && <p className="mt-1 text-xs text-muted-foreground">{item.description}</p>}
              </div>
              <StatusBadge status={item.status} />
            </div>
          </button>
        ))}
      </div>

      <div className="min-w-0 rounded-md border bg-muted/20">
        {selectedPath && (
          <div className="border-b px-3 py-2">
            <div className="flex flex-wrap items-center gap-2">
              <span className="text-sm font-medium">Content</span>
              {(selectedItem?.isSymlink || content?.is_symlink) && <Badge variant="outline">Symbolic link</Badge>}
              {content?.redacted && <Badge variant="secondary">Masked</Badge>}
              {content?.truncated && <Badge variant="secondary">Truncated at 256 KiB</Badge>}
            </div>
            <div className="mt-1 break-all font-mono text-[11px] text-muted-foreground">{selectedPath}</div>
            {(selectedItem?.isSymlink || content?.is_symlink) && <p className="mt-2 text-xs text-muted-foreground">This file is a symbolic link; content is read from its target.</p>}
            {kind === 'config' && <p className="mt-2 text-xs text-muted-foreground">Secret-like values are masked before display.</p>}
          </div>
        )}
        <div className="p-3">
          {loading && <p className="text-sm text-muted-foreground">Loading content…</p>}
          {!loading && error && <p className="text-sm text-red-700">{error}</p>}
          {!loading && !error && content && (
            <pre className="max-h-[32rem] overflow-auto whitespace-pre-wrap break-words rounded border bg-background p-3 font-mono text-xs leading-relaxed">{content.content || '(empty file)'}</pre>
          )}
          {!loading && !error && !content && !selectedPath && <EmptyState>Select a readable file to view its content.</EmptyState>}
        </div>
      </div>
    </div>
  )
}

function parentDirectory(path: string): string {
  const separator = path.lastIndexOf('/')
  return separator > 0 ? path.slice(0, separator) : '/'
}

function formatFileSize(size: number): string {
  if (size < 1024) return `${size} B`
  if (size < 1024 * 1024) return `${Math.round(size / 1024)} KiB`
  return `${(size / (1024 * 1024)).toFixed(1)} MiB`
}

interface CodexSkillBrowserProps {
  skills: CodexSkill[]
  project?: string
  refreshKey: string
}

function CodexSkillBrowser({ skills, project, refreshKey }: CodexSkillBrowserProps) {
  const readableSkills = skills.filter((skill) => skill.status !== 'unreadable')
  const [selectedSkillPath, setSelectedSkillPath] = useState('')
  const [selectedFilePath, setSelectedFilePath] = useState('')
  const [entriesByDirectory, setEntriesByDirectory] = useState<Record<string, CodexSkillFile[]>>({})
  const [directoryTruncated, setDirectoryTruncated] = useState<Record<string, boolean>>({})
  const [expandedDirectories, setExpandedDirectories] = useState<Set<string>>(new Set())
  const [directoryLoading, setDirectoryLoading] = useState<Record<string, boolean>>({})
  const [directoryErrors, setDirectoryErrors] = useState<Record<string, string>>({})
  const [content, setContent] = useState<Awaited<ReturnType<typeof api.getCodexDiagnosticContent>> | null>(null)
  const [contentLoading, setContentLoading] = useState(false)
  const [contentError, setContentError] = useState<string | null>(null)

  const loadDirectory = useCallback(async (path: string) => {
    setDirectoryLoading((current) => ({ ...current, [path]: true }))
    setDirectoryErrors((current) => {
      const next = { ...current }
      delete next[path]
      return next
    })
    try {
      const result = await api.getCodexSkillFiles(path, project || undefined)
      setEntriesByDirectory((current) => ({ ...current, [path]: result.entries }))
      setDirectoryTruncated((current) => ({ ...current, [path]: result.truncated }))
    } catch (cause: unknown) {
      setDirectoryErrors((current) => ({
        ...current,
        [path]: cause instanceof Error ? cause.message : String(cause),
      }))
    } finally {
      setDirectoryLoading((current) => ({ ...current, [path]: false }))
    }
  }, [project])

  useEffect(() => {
    setSelectedSkillPath((current) => readableSkills.some((skill) => skill.path === current) ? current : readableSkills[0]?.path ?? '')
  }, [skills, refreshKey])

  useEffect(() => {
    if (!selectedSkillPath) {
      setSelectedFilePath('')
      setEntriesByDirectory({})
      setDirectoryTruncated({})
      setExpandedDirectories(new Set())
      return
    }
    const directory = parentDirectory(selectedSkillPath)
    setSelectedFilePath(selectedSkillPath)
    setEntriesByDirectory({})
    setDirectoryTruncated({})
    setDirectoryErrors({})
    setExpandedDirectories(new Set([directory]))
    void loadDirectory(directory)
  }, [loadDirectory, refreshKey, selectedSkillPath])

  useEffect(() => {
    if (!selectedFilePath) {
      setContent(null)
      setContentError(null)
      return
    }
    let active = true
    setContentLoading(true)
    setContentError(null)
    void api.getCodexDiagnosticContent('skill', selectedFilePath, project || undefined)
      .then((result) => {
        if (active) setContent(result)
      })
      .catch((cause: unknown) => {
        if (active) {
          setContent(null)
          setContentError(cause instanceof Error ? cause.message : String(cause))
        }
      })
      .finally(() => {
        if (active) setContentLoading(false)
      })
    return () => {
      active = false
    }
  }, [project, refreshKey, selectedFilePath])

  if (skills.length === 0) return <EmptyState>No skills were found in the standard locations.</EmptyState>

  const selectedSkill = skills.find((skill) => skill.path === selectedSkillPath)
  const selectedEntry = Object.values(entriesByDirectory).flat().find((entry) => entry.path === selectedFilePath)

  const toggleDirectory = (path: string) => {
    const shouldExpand = !expandedDirectories.has(path)
    setExpandedDirectories((current) => {
      const next = new Set(current)
      if (shouldExpand) next.add(path)
      else next.delete(path)
      return next
    })
    if (shouldExpand && entriesByDirectory[path] === undefined) void loadDirectory(path)
  }

  const renderDirectory = (path: string, depth = 0): ReactNode => {
    const entries = entriesByDirectory[path] ?? []
    return entries.map((entry) => {
      const isDirectory = entry.kind === 'directory'
      const isExpanded = expandedDirectories.has(entry.path)
      const icon = isDirectory ? <Folder className="size-4 shrink-0" /> : <File className="size-4 shrink-0" />
      return (
        <div key={entry.path}>
          <button
            type="button"
            disabled={!isDirectory && entry.kind !== 'file'}
            aria-pressed={!isDirectory && selectedFilePath === entry.path}
            onClick={() => isDirectory ? toggleDirectory(entry.path) : setSelectedFilePath(entry.path)}
            className={cn(
              'flex w-full items-start gap-1 rounded-md px-2 py-1.5 text-left text-sm transition-colors',
              isDirectory || entry.kind === 'file' ? 'hover:bg-muted/60' : 'cursor-not-allowed opacity-60',
              !isDirectory && selectedFilePath === entry.path && 'bg-muted',
            )}
            style={{ paddingLeft: `${8 + depth * 16}px` }}
          >
            {isDirectory ? (isExpanded ? <ChevronDown className="mt-0.5 size-4 shrink-0" /> : <ChevronRight className="mt-0.5 size-4 shrink-0" />) : <span className="size-4 shrink-0" />}
            {icon}
            <span className="min-w-0 flex-1">
              <span className="flex flex-wrap items-center gap-1.5">
                <span className="truncate">{entry.name}</span>
                {entry.is_symlink && <Badge variant="outline"><Link2 className="mr-1 size-3" />Symbolic link</Badge>}
              </span>
              {entry.is_symlink && entry.target && <span className="block truncate font-mono text-[10px] text-muted-foreground">→ {entry.target}</span>}
              {!isDirectory && <span className="block text-[10px] text-muted-foreground">{entry.kind === 'file' ? formatFileSize(entry.size_bytes) : humanize(entry.kind)}</span>}
            </span>
          </button>
          {isDirectory && isExpanded && (
            <>
              {directoryLoading[entry.path] && <div className="py-1 text-center text-[11px] text-muted-foreground">Loading…</div>}
              {directoryErrors[entry.path] && <div className="px-3 py-1 text-xs text-red-700">{directoryErrors[entry.path]}</div>}
              {!directoryLoading[entry.path] && !directoryErrors[entry.path] && (
                <>
                  {renderDirectory(entry.path, depth + 1)}
                  {directoryTruncated[entry.path] && <div className="px-3 py-1 text-[11px] text-muted-foreground">Only the first 512 entries are shown.</div>}
                </>
              )}
            </>
          )}
        </div>
      )
    })
  }

  const skillDirectory = selectedSkillPath ? parentDirectory(selectedSkillPath) : ''

  return (
    <div className="grid gap-4 xl:grid-cols-[minmax(13rem,0.28fr)_minmax(16rem,0.38fr)_minmax(0,1fr)]">
      <div className="space-y-2">
        <div className="text-xs font-medium text-muted-foreground">Skills</div>
        {skills.map((skill) => {
          const readable = skill.status !== 'unreadable'
          return (
            <button
              key={skill.path}
              type="button"
              disabled={!readable}
              aria-pressed={selectedSkillPath === skill.path}
              onClick={() => setSelectedSkillPath(skill.path)}
              className={cn(
                'w-full rounded-md border px-3 py-2 text-left transition-colors',
                readable ? 'hover:bg-muted/60' : 'cursor-not-allowed opacity-60',
                selectedSkillPath === skill.path && 'border-primary bg-muted',
              )}
            >
              <div className="flex items-start justify-between gap-2">
                <div className="min-w-0">
                  <div className="flex flex-wrap items-center gap-2 text-sm font-medium">
                    <span className="truncate">{skill.name}</span>
                    {skill.is_symlink && <Badge variant="outline">Symbolic link</Badge>}
                  </div>
                  <div className="mt-1 text-xs text-muted-foreground">Scope: {skill.scope}</div>
                  <div className="mt-1 break-all font-mono text-[11px] text-muted-foreground">Source: {skill.path}</div>
                  {skill.description && <p className="mt-1 text-xs text-muted-foreground">{display(skill.description)}</p>}
                </div>
                <StatusBadge status={skill.status} />
              </div>
            </button>
          )
        })}
      </div>

      <div className="min-w-0 rounded-md border bg-muted/20">
        <div className="border-b px-3 py-2">
          <div className="flex flex-wrap items-center gap-2 text-sm font-medium">
            <span>Skill files</span>
            {selectedSkill?.is_symlink && <Badge variant="outline">Symbolic link</Badge>}
          </div>
          <div className="mt-1 break-all font-mono text-[11px] text-muted-foreground">{skillDirectory || '—'}</div>
        </div>
        <div className="max-h-[36rem] overflow-auto p-2">
          {selectedSkill && directoryLoading[skillDirectory] && <p className="px-2 py-2 text-sm text-muted-foreground">Loading directory…</p>}
          {selectedSkill && directoryErrors[skillDirectory] && <p className="px-2 py-2 text-sm text-red-700">{directoryErrors[skillDirectory]}</p>}
          {selectedSkill && !directoryLoading[skillDirectory] && !directoryErrors[skillDirectory] && renderDirectory(skillDirectory)}
          {selectedSkill && !directoryLoading[skillDirectory] && !directoryErrors[skillDirectory] && directoryTruncated[skillDirectory] && <p className="px-2 py-2 text-xs text-muted-foreground">Only the first 512 entries are shown.</p>}
          {selectedSkill && !directoryLoading[skillDirectory] && !directoryErrors[skillDirectory] && (entriesByDirectory[skillDirectory]?.length ?? 0) === 0 && <p className="px-2 py-2 text-sm text-muted-foreground">The directory is empty.</p>}
          {!selectedSkill && <p className="px-2 py-2 text-sm text-muted-foreground">Select a readable Skill.</p>}
        </div>
      </div>

      <div className="min-w-0 rounded-md border bg-muted/20">
        {selectedFilePath && (
          <div className="border-b px-3 py-2">
            <div className="flex flex-wrap items-center gap-2">
              <span className="text-sm font-medium">Content</span>
              {(selectedEntry?.is_symlink || content?.is_symlink) && <Badge variant="outline">Symbolic link</Badge>}
              {content?.truncated && <Badge variant="secondary">Truncated at 256 KiB</Badge>}
            </div>
            <div className="mt-1 break-all font-mono text-[11px] text-muted-foreground">{selectedFilePath}</div>
            {(selectedEntry?.is_symlink || content?.is_symlink) && <p className="mt-2 text-xs text-muted-foreground">This file is a symbolic link; content is read from its target.</p>}
          </div>
        )}
        <div className="p-3">
          {contentLoading && <p className="text-sm text-muted-foreground">Loading content…</p>}
          {!contentLoading && contentError && <p className="text-sm text-red-700">{contentError}</p>}
          {!contentLoading && !contentError && content && <pre className="max-h-[32rem] overflow-auto whitespace-pre-wrap break-words rounded border bg-background p-3 font-mono text-xs leading-relaxed">{content.content || '(empty file)'}</pre>}
          {!contentLoading && !contentError && !content && !selectedFilePath && <EmptyState>Select a file to view its content.</EmptyState>}
        </div>
      </div>
    </div>
  )
}

export function CodexDiagnosticsPage() {
  const [diagnostics, setDiagnostics] = useState<CodexDiagnostics | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const project = getProject()

  const refresh = useCallback(async () => {
    setLoading(true)
    try {
      setDiagnostics(await api.getCodexDiagnostics(project ?? undefined))
      setError(null)
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    } finally {
      setLoading(false)
    }
  }, [project])

  useEffect(() => {
    void refresh()
  }, [refresh])

  return (
    <div className="mx-auto max-w-[1280px] space-y-6 p-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="flex items-center gap-2 text-2xl font-bold">
            <Bot className="size-6" />
            Agent
          </h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Read-only Codex CLI configuration and availability diagnostics for {display(project)}.
          </p>
        </div>
        <Button variant="outline" size="sm" onClick={() => void refresh()} disabled={loading}>
          <RefreshCw className={cn('size-4', loading && 'animate-spin')} />
          Refresh
        </Button>
      </div>

      {error && (
        <div className="rounded-md border border-red-200 bg-red-50 px-3.5 py-2.5 text-sm text-red-700">
          {error}
        </div>
      )}

      {loading && !diagnostics && <p className="text-sm text-muted-foreground">Loading Codex diagnostics…</p>}

      {diagnostics && (
        <>
          {diagnostics.errors.length > 0 && (
            <div className="rounded-md border border-amber-200 bg-amber-50 px-3.5 py-3 text-sm text-amber-900">
              <div className="font-medium">Some diagnostics are unavailable</div>
              <ul className="mt-2 list-disc space-y-1 pl-5">
                {diagnostics.errors.map((item) => <li key={`${item.section}:${item.message}`}>{item.section}: {item.message}</li>)}
              </ul>
            </div>
          )}

          <Card>
            <CardHeader className="flex flex-row items-start justify-between gap-4">
              <div>
                <CardTitle className="flex items-center gap-2"><ShieldCheck className="size-4" /> Codex runtime</CardTitle>
                <CardDescription>Collected at {new Date(diagnostics.fetched_at).toLocaleString()}</CardDescription>
              </div>
              <StatusBadge status={diagnostics.codex.status} />
            </CardHeader>
            <CardContent className="space-y-4">
              <DetailList values={[
                ['Available', diagnostics.codex.available ? 'Yes' : 'No'],
                ['Version', diagnostics.codex.version],
                ['Executable', diagnostics.codex.executable_path],
                ['App server', diagnostics.codex.app_server_version],
                ['App server status', diagnostics.codex.app_server_status],
                ['Authentication', diagnostics.codex.authenticated == null ? 'Unknown' : diagnostics.codex.authenticated ? 'Configured' : 'Not configured'],
                ['Auth mode', diagnostics.codex.auth_mode],
                ['Model', diagnostics.codex.model],
                ['Provider', diagnostics.codex.model_provider],
                ['Reasoning effort', diagnostics.codex.reasoning_effort],
                ['Sandbox', diagnostics.codex.sandbox_mode],
                ['Approval policy', diagnostics.codex.approval_policy],
              ]} />
              <div>
                <div className="mb-2 text-xs text-muted-foreground">Enabled feature flags</div>
                {diagnostics.codex.feature_flags.length > 0 ? (
                  <div className="flex flex-wrap gap-1.5">
                    {diagnostics.codex.feature_flags.map((feature) => <Badge key={feature} variant="outline">{feature}</Badge>)}
                  </div>
                ) : <EmptyState>No enabled feature flags reported.</EmptyState>}
              </div>
            </CardContent>
          </Card>

          <Tabs defaultValue="config" className="space-y-4">
            <TabsList>
              <TabsTrigger value="config"><FileCog className="size-4" /> Config</TabsTrigger>
              <TabsTrigger value="instructions"><KeyRound className="size-4" /> Instructions</TabsTrigger>
              <TabsTrigger value="skills"><Sparkles className="size-4" /> Skills</TabsTrigger>
              <TabsTrigger value="mcp"><Server className="size-4" /> MCP</TabsTrigger>
            </TabsList>

            <TabsContent value="config">
              <Card>
                <CardHeader>
                  <CardTitle>Configuration files</CardTitle>
                  <CardDescription>Choose a readable file to inspect its content. Secret-like values are masked. Collected at {new Date(diagnostics.fetched_at).toLocaleString()}.</CardDescription>
                </CardHeader>
                <CardContent>
                  <CodexContentBrowser
                    kind="config"
                    items={diagnostics.config_files.map((file) => ({
                      path: file.path,
                      scope: file.scope,
                      status: file.status,
                      readable: file.exists && file.readable,
                      isSymlink: file.is_symlink,
                      label: file.scope,
                    }))}
                    project={project ?? undefined}
                    refreshKey={diagnostics.fetched_at}
                    emptyMessage="No known configuration files."
                  />
                </CardContent>
              </Card>
            </TabsContent>

            <TabsContent value="instructions">
              <Card>
                <CardHeader>
                  <CardTitle>Applicable AGENTS.md</CardTitle>
                  <CardDescription>Override files are selected using Codex discovery precedence. Select a readable file to inspect the source text. Collected at {new Date(diagnostics.fetched_at).toLocaleString()}.</CardDescription>
                </CardHeader>
                <CardContent>
                  <CodexContentBrowser
                    kind="instruction"
                    items={diagnostics.instructions.map((file) => ({
                      path: file.path,
                      scope: file.scope,
                      status: file.status,
                      readable: file.exists && file.status === 'present',
                      isSymlink: file.is_symlink,
                      label: file.scope,
                    }))}
                    project={project ?? undefined}
                    refreshKey={diagnostics.fetched_at}
                    emptyMessage="No instruction paths found."
                  />
                </CardContent>
              </Card>
            </TabsContent>

            <TabsContent value="skills">
              <Card>
                <CardHeader>
                  <CardTitle>Skills</CardTitle>
                  <CardDescription>
                    Select a Skill, browse its directory, and inspect read-only file contents. Collected at {new Date(diagnostics.fetched_at).toLocaleString()}.
                    {diagnostics.skills_truncated && ' Only the first 256 Skills from one or more standard locations are shown.'}
                  </CardDescription>
                </CardHeader>
                <CardContent>
                  <CodexSkillBrowser
                    skills={diagnostics.skills}
                    project={project ?? undefined}
                    refreshKey={diagnostics.fetched_at}
                  />
                </CardContent>
              </Card>
            </TabsContent>

            <TabsContent value="mcp">
              <Card>
                <CardHeader>
                  <CardTitle>MCP servers</CardTitle>
                  <CardDescription>Configuration and auth state only. MCP servers are not launched or health-checked. Collected at {new Date(diagnostics.fetched_at).toLocaleString()}.</CardDescription>
                </CardHeader>
                <CardContent>
                  {diagnostics.mcp_servers.length === 0 ? <EmptyState>No MCP servers are configured.</EmptyState> : (
                    <div className="overflow-x-auto">
                      <table className="w-full text-left text-sm">
                        <thead className="border-b text-xs text-muted-foreground"><tr><th className="py-2 pr-4">Name</th><th className="py-2 pr-4">Scope</th><th className="py-2 pr-4">Source</th><th className="py-2 pr-4">Transport</th><th className="py-2 pr-4">Command</th><th className="py-2 pr-4">Status</th><th className="py-2 pr-4">Auth</th><th className="py-2">Health</th></tr></thead>
                        <tbody>{diagnostics.mcp_servers.map((server) => <tr key={server.name} className="border-b last:border-0"><td className="py-2 pr-4 font-medium">{server.name}</td><td className="py-2 pr-4">{server.scope}</td><td className="py-2 pr-4">{server.source}</td><td className="py-2 pr-4">{server.transport}</td><td className="py-2 pr-4 font-mono text-xs">{display(server.command)}</td><td className="py-2 pr-4"><StatusBadge status={server.status} /></td><td className="py-2 pr-4">{server.auth_status}</td><td className="py-2"><StatusBadge status={server.health_status} /></td></tr>)}</tbody>
                      </table>
                    </div>
                  )}
                </CardContent>
              </Card>
            </TabsContent>
          </Tabs>
        </>
      )}
    </div>
  )
}
