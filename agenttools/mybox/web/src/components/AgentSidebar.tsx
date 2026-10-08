import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useLocation, useSearchParams } from 'react-router-dom'
import { Bot, CheckCircle2, CircleX, FileText, Loader2, PanelRightOpen, RefreshCw, Square, X } from 'lucide-react'
import type { HerdrAgent, HerdrOverview } from '../api/client'
import { api } from '../api/client'
import { StatusBadge, StatusDot } from './herdr-status'
import { Button } from './ui/button'
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from './ui/sheet'
import { cn } from '@/lib/utils'
import { useIsMobile } from '../hooks/use-mobile'
import { useResizableWidth } from '../hooks/use-resizable-width'
import { HerdrAgentDetail } from './HerdrAgentDetail'
import { AgentLaunchDialog } from './AgentLaunchDialog'
import {
  hasTaskFile,
  linkedTaskAgentForTab,
  linkedTaskRenameForLabel,
  taskDirectoryForFilePath,
  type LinkedTaskAgent,
} from '../utils/herdr-file-agent'
import { useDialogs } from './AppDialogs'
import type { AgentOutputDisplayMode } from '../utils/agent-output-display'
import { projectAgentsFor } from '../utils/agent-sidebar-status'
import { useFileExecution, type FileExecutionRun } from '../state/fileExecution'

export const AGENT_SIDEBAR_WIDTH_STORAGE_KEY = 'mybox_agent_sidebar_width'
export const DEFAULT_AGENT_SIDEBAR_WIDTH = 400
export const AGENT_SIDEBAR_MIN_WIDTH = 280
export const AGENT_SIDEBAR_MAX_WIDTH = 960
export const AGENT_SIDEBAR_COLLAPSED_WIDTH = 56
const AGENT_SIDEBAR_VIEWPORT_GUTTER = 48

export function agentSidebarWidthStyle(width: number): string {
  return `min(${width}px, calc(100vw - ${AGENT_SIDEBAR_VIEWPORT_GUTTER}px))`
}

export function agentSidebarDisplayedWidth(width: number, viewportWidth = window.innerWidth): number {
  return Math.min(width, AGENT_SIDEBAR_MAX_WIDTH, Math.max(AGENT_SIDEBAR_MIN_WIDTH, viewportWidth - AGENT_SIDEBAR_VIEWPORT_GUTTER))
}

interface AgentSidebarProps {
  project: string
  overview: HerdrOverview | null
  error: string | null
  loading: boolean
  open: boolean
  onOpenChange: (open: boolean) => void
  displayMode: AgentOutputDisplayMode
  onDisplayModeChange: (mode: AgentOutputDisplayMode) => void
  onWidthChange?: (width: number) => void
  openAgentPaneId: string | null
  drafts: Record<string, string>
  onDraftChange: (paneId: string, draft: string) => void
  pendingOpenAgentPaneId?: string | null
  onOpenAgentChange: (paneId: string | null) => void
  onOpenAgentPane?: (paneId: string) => void
  refresh: () => Promise<void>
  onFocusChange?: (paneId: string | null) => void
  onFilePathClick?: (path: string) => void
}

function runError(error: unknown): string {
  return error instanceof Error ? error.message : String(error)
}

function executionStatusLabel(run: FileExecutionRun): string {
  if (run.status === 'running') return 'Running'
  if (run.status === 'stopped') return 'Stopped'
  if (run.result?.timed_out) return `Timed out (exit ${run.result.exit_code})`
  if (run.result) return `Exit code ${run.result.exit_code}`
  return 'Failed'
}

function ExecutionStatusIcon({ run }: { run: FileExecutionRun }) {
  if (run.status === 'running') return <Loader2 className="size-3.5 animate-spin text-primary" aria-hidden="true" />
  if (run.status === 'completed') return <CheckCircle2 className="size-3.5 text-green-600" aria-hidden="true" />
  if (run.status === 'stopped') return <Square className="size-3.5 text-amber-600" aria-hidden="true" />
  return <CircleX className="size-3.5 text-red-600" aria-hidden="true" />
}

export function AgentSidebar({
  project,
  overview,
  error,
  loading,
  open,
  onOpenChange,
  displayMode,
  onDisplayModeChange,
  onWidthChange,
  openAgentPaneId,
  drafts,
  onDraftChange,
  pendingOpenAgentPaneId,
  onOpenAgentChange,
  onOpenAgentPane,
  refresh,
  onFocusChange,
  onFilePathClick,
}: AgentSidebarProps) {
  const isMobile = useIsMobile()
  const { pathname } = useLocation()
  const [searchParams] = useSearchParams()
  const { prompt } = useDialogs()
  const fileExecution = useFileExecution()
  const [autoReload, setAutoReload] = useState(true)
  const [reloadToken, setReloadToken] = useState(0)
  const [operationError, setOperationError] = useState<string | null>(null)
  const [viewportWidth, setViewportWidth] = useState(() => window.innerWidth)
  const [stoppingPaneId, setStoppingPaneId] = useState<string | null>(null)
  const [agentLaunchOpen, setAgentLaunchOpen] = useState(false)
  const [agentKinds, setAgentKinds] = useState<string[]>([])
  const [agentKind, setAgentKind] = useState('codex')
  const [agentTab, setAgentTab] = useState('main')
  const [loadingAgentKinds, setLoadingAgentKinds] = useState(false)
  const [startingAgent, setStartingAgent] = useState(false)
  const [agentLaunchError, setAgentLaunchError] = useState<string | null>(null)
  const requestedAgent = pathname.endsWith('/herdr') ? searchParams.get('agent') : null
  const handledRequestRef = useRef<string | null>(null)
  const focusedPaneRef = useRef<string | null>(null)
  const displayWidth = useCallback(
    (value: number) => agentSidebarDisplayedWidth(value, viewportWidth),
    [viewportWidth],
  )
  const width = useResizableWidth({
    storageKey: AGENT_SIDEBAR_WIDTH_STORAGE_KEY,
    defaultWidth: DEFAULT_AGENT_SIDEBAR_WIDTH,
    minWidth: AGENT_SIDEBAR_MIN_WIDTH,
    maxWidth: AGENT_SIDEBAR_MAX_WIDTH,
    handleSide: 'left',
    displayWidth,
  })

  useEffect(() => {
    const handleResize = () => setViewportWidth(window.innerWidth)
    window.addEventListener('resize', handleResize)
    return () => window.removeEventListener('resize', handleResize)
  }, [])

  const displayedWidth = displayWidth(width.width)

  const refreshAgents = useCallback(async () => {
    setReloadToken((current) => current + 1)
    await refresh()
  }, [refresh])

  useEffect(() => {
    onWidthChange?.(displayedWidth)
  }, [displayedWidth, onWidthChange])

  const agents = overview?.agents ?? []
  const tabs = overview?.tabs ?? []
  const panes = overview?.panes ?? []
  const projectAgents = useMemo(() => projectAgentsFor(overview, project), [overview, project])
  const selectedAgent = projectAgents.find((agent) => agent.pane_id === openAgentPaneId)
  const linkedTasks = useMemo(() => {
    const result = new Map<string, LinkedTaskAgent>()
    for (const tab of tabs) {
      const linked = linkedTaskAgentForTab(tab.tab_id, agents, panes, tabs)
      if (linked) result.set(tab.tab_id, linked)
    }
    return result
  }, [agents, panes, tabs])
  const agentFilePath = useMemo(() => {
    const result = new Map<string, string>()
    for (const linked of linkedTasks.values()) result.set(linked.agent.pane_id, linked.taskFile)
    return result
  }, [linkedTasks])
  const reportError = useCallback((message: string) => {
    setOperationError(message)
    window.setTimeout(() => setOperationError((current) => (current === message ? null : current)), 6000)
  }, [])

  const projectTabLabels = useMemo(() => {
    const workspaceIds = new Set(
      (overview?.workspaces ?? [])
        .filter((workspace) => workspace.label === project)
        .map((workspace) => workspace.workspace_id),
    )
    return [...new Set(
      tabs
        .filter((tab) => workspaceIds.has(tab.workspace_id))
        .sort((a, b) => (a.number ?? 0) - (b.number ?? 0))
        .map((tab) => tab.label),
    )]
  }, [overview?.workspaces, project, tabs])

  useEffect(() => {
    if (!agentLaunchOpen) return
    let cancelled = false
    setAgentLaunchError(null)
    setAgentTab('main')
    setAgentKind('codex')
    setLoadingAgentKinds(true)
    void api.getHerdrAgentKinds().then(({ kinds }) => {
      if (cancelled) return
      const available = [...new Set(kinds.filter((kind) => kind.trim()))]
      setAgentKinds(available)
      setAgentKind((current) => available.includes(current) ? current : available.includes('codex') ? 'codex' : (available[0] ?? ''))
      if (available.length === 0) setAgentLaunchError('No Herdr agent kinds are available.')
    }).catch((error) => {
      if (!cancelled) setAgentLaunchError(runError(error))
    }).finally(() => {
      if (!cancelled) setLoadingAgentKinds(false)
    })
    return () => {
      cancelled = true
    }
  }, [agentLaunchOpen])

  const startAgent = useCallback(async () => {
    if (startingAgent) return
    const kind = agentKind.trim()
    const tab = agentTab.trim()
    if (!kind || !tab) return
    setStartingAgent(true)
    setAgentLaunchError(null)
    try {
      const result = await api.startHerdrAgent(kind, tab)
      if (!result.agent?.pane_id) throw new Error('The agent started without a pane.')
      setAgentLaunchOpen(false)
      if (onOpenAgentPane) {
        onOpenAgentPane(result.agent.pane_id)
      } else {
        onOpenAgentChange(result.agent.pane_id)
        onOpenChange(true)
      }
      try {
        await refresh()
      } catch (refreshError) {
        reportError(`Agent started, but its status could not be refreshed: ${runError(refreshError)}`)
      }
    } catch (error) {
      setAgentLaunchError(runError(error))
    } finally {
      setStartingAgent(false)
    }
  }, [agentKind, agentTab, onOpenAgentChange, onOpenAgentPane, onOpenChange, refresh, reportError, startingAgent])

  useEffect(() => {
    if (overview?.available && openAgentPaneId && !selectedAgent) {
      if (pendingOpenAgentPaneId === openAgentPaneId) return
      onOpenAgentChange(null)
    }
  }, [onOpenAgentChange, openAgentPaneId, overview?.available, pendingOpenAgentPaneId, selectedAgent])

  useEffect(() => {
    if (!requestedAgent) {
      handledRequestRef.current = null
      return
    }
    const requestKey = `${pathname}:${requestedAgent}`
    if (handledRequestRef.current === requestKey) return
    if (!projectAgents.some((agent) => agent.pane_id === requestedAgent)) return
    handledRequestRef.current = requestKey
    onOpenAgentChange(requestedAgent)
    onOpenChange(true)
  }, [onOpenAgentChange, onOpenChange, pathname, projectAgents, requestedAgent])

  const selectedPaneId = selectedAgent?.pane_id ?? null
  useEffect(() => {
    if (!open || !overview?.available || !selectedPaneId) {
      if (focusedPaneRef.current !== null) {
        focusedPaneRef.current = null
        onFocusChange?.(null)
      }
      return
    }
    focusedPaneRef.current = selectedPaneId
    onFocusChange?.(selectedPaneId)
    void api.focusHerdrAgent(selectedPaneId).then(() => refresh()).catch((error) => reportError(runError(error)))
  }, [onFocusChange, open, overview?.available, refresh, reportError, selectedPaneId])

  const renameAgent = useCallback(async (agent: HerdrAgent) => {
    const nextName = await prompt(`Rename agent "${agent.name}" (leave empty to reset to default)`, agent.name)
    if (nextName === null) return
    const trimmed = nextName.trim()
    const linkedTask = [...linkedTasks.values()].find((link) => link.agent.pane_id === agent.pane_id)
    if (linkedTask) {
      const expectedName = linkedTaskRenameForLabel(linkedTask, linkedTask.tab.label)?.newAgentName
      if (!expectedName || trimmed !== expectedName) {
        reportError('File-agent names follow their linked Herdr tab. Rename the tab to move the task directory.')
        return
      }
    }
    try {
      await api.renameHerdrAgent(agent.pane_id, trimmed || undefined, !trimmed)
      await refresh()
    } catch (error) {
      reportError(runError(error))
    }
  }, [linkedTasks, prompt, refresh, reportError])

  const openLinkedTaskFile = useCallback(async (filePath: string) => {
    if (!onFilePathClick) return
    try {
      const entries = await api.listFiles({
        path: taskDirectoryForFilePath(filePath),
        showHidden: true,
        project,
      })
      if (!hasTaskFile(entries, filePath)) {
        reportError(`Linked task file not found: ${filePath}`)
        return
      }
      onFilePathClick(filePath)
    } catch (error) {
      reportError(runError(error))
    }
  }, [onFilePathClick, project, reportError])

  const stopAgent = useCallback(async (agent: HerdrAgent) => {
    if (stoppingPaneId) return
    setStoppingPaneId(agent.pane_id)
    setOperationError(null)
    try {
      await api.sendKeysHerdrAgent(agent.pane_id, ['C-c', 'C-c'])
      await api.closeHerdrPane(agent.pane_id)
      if (openAgentPaneId === agent.pane_id) {
        onOpenAgentChange(null)
      }
      await refresh()
    } catch (error) {
      reportError(runError(error))
    } finally {
      setStoppingPaneId(null)
    }
  }, [onOpenAgentChange, openAgentPaneId, refresh, reportError, stoppingPaneId])

  if (!project) return null

  const content = (
    <div className="flex h-full min-h-0 min-w-0 w-full flex-col bg-card" data-testid="agent-sidebar-content">
      <div className="shrink-0 border-b px-3 py-2">
        <div className="flex items-center gap-2">
          <Bot className="size-4 shrink-0" />
          <h2 className="min-w-0 flex-1 truncate text-sm font-semibold">Agents</h2>
          <Button
            size="xs"
            className="cursor-pointer px-1.5 text-[10px]"
            onClick={() => setAgentLaunchOpen(true)}
            aria-label="Start agent"
            title="Start agent"
          >
            <Bot />
            Start agent
          </Button>
          <Button
            variant={autoReload ? 'secondary' : 'ghost'}
            size="xs"
            className="cursor-pointer px-1.5 text-[10px]"
            onClick={() => setAutoReload((current) => !current)}
            aria-pressed={autoReload}
            title="Toggle auto reload of the selected agent output"
          >
            <RefreshCw className={cn(autoReload && loading && 'animate-spin')} />
            Auto: {autoReload ? 'ON' : 'OFF'}
          </Button>
          <Button
            variant="ghost"
            size="icon-xs"
            className="cursor-pointer"
            onClick={() => void refreshAgents()}
            aria-label="Refresh agents"
            title="Refresh agents and selected output"
          >
            <RefreshCw className={cn(loading && 'animate-spin')} />
          </Button>
        </div>
        <p className="mt-1 truncate text-[11px] text-muted-foreground" title={project}>{project}</p>
      </div>

      <div className="min-h-0 min-w-0 flex-1 overflow-y-auto p-3">
        {error && <div className="mb-2 rounded-md border border-red-200 bg-red-50 px-2.5 py-2 text-xs text-red-700">{error}</div>}
        {operationError && <div role="alert" className="mb-2 rounded-md border border-red-200 bg-red-50 px-2.5 py-2 text-xs text-red-700">{operationError}</div>}
        {overview && !overview.available && (
          <div className="mb-2 rounded-md border border-amber-200 bg-amber-50 px-2.5 py-2 text-xs text-amber-800">
            herdr command is not available on this server.
          </div>
        )}
        {!overview && loading && <p className="py-4 text-center text-xs text-muted-foreground">Loading agents…</p>}
        {overview?.available && projectAgents.length === 0 && (
          <p className="rounded-md border border-dashed p-4 text-center text-xs text-muted-foreground">
            No herdr agents running for project "{project}".
          </p>
        )}
        {fileExecution.runs.length > 0 && (
          <section className="mb-3" aria-labelledby="script-executions-heading" data-testid="agent-sidebar-executions">
            <h3 id="script-executions-heading" className="mb-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
              Script executions
            </h3>
            <div className="flex min-w-0 flex-col gap-2">
              {fileExecution.runs.map((run) => (
                <div key={run.id} className="min-w-0 rounded-lg border bg-background p-2.5" data-testid={`agent-sidebar-execution-${run.id}`}>
                  <div className="flex min-w-0 items-center gap-2">
                    <button
                      type="button"
                      className="flex min-w-0 flex-1 cursor-pointer items-center gap-2 text-left"
                      onClick={() => fileExecution.open(run.id)}
                      aria-label={`Open execution ${run.path} (${executionStatusLabel(run)})`}
                    >
                      <ExecutionStatusIcon run={run} />
                      <span className="min-w-0 flex-1 truncate font-mono text-xs">{run.path}</span>
                      <span className="shrink-0 text-[11px] text-muted-foreground">{executionStatusLabel(run)}</span>
                    </button>
                    {run.status === 'running' ? (
                      <Button
                        variant="ghost"
                        size="xs"
                        className="h-6 shrink-0 cursor-pointer px-1.5 text-[11px]"
                        onClick={() => fileExecution.stop(run.id)}
                        aria-label={`Stop execution ${run.path}`}
                      >
                        <Square className="mr-1 size-3" />
                        Stop
                      </Button>
                    ) : (
                      <Button
                        variant="ghost"
                        size="icon-xs"
                        className="shrink-0 cursor-pointer"
                        onClick={() => fileExecution.dismiss(run.id)}
                        aria-label={`Dismiss execution ${run.path}`}
                        title="Dismiss execution"
                      >
                        <X />
                      </Button>
                    )}
                  </div>
                </div>
              ))}
            </div>
          </section>
        )}
        {overview?.available && projectAgents.length > 0 && (
          <div className="flex min-w-0 flex-col gap-2">
            {projectAgents.map((agent) => {
              const filePath = agentFilePath.get(agent.pane_id)
              return (
                <div
                  key={agent.pane_id}
                  className={cn('min-w-0 rounded-lg border bg-background p-3', openAgentPaneId === agent.pane_id && 'border-primary ring-2 ring-primary/20')}
                  data-testid={`agent-sidebar-agent-${agent.pane_id}`}
                >
                  <div className="flex w-full items-center justify-between gap-2">
                    <button
                      type="button"
                      className="flex min-w-0 flex-1 cursor-pointer flex-wrap items-center gap-2 text-left"
                      onClick={() => onOpenAgentChange(openAgentPaneId === agent.pane_id ? null : agent.pane_id)}
                      aria-expanded={openAgentPaneId === agent.pane_id}
                      data-testid={`agent-sidebar-row-${agent.pane_id}`}
                    >
                      <StatusDot status={agent.status} />
                      <span className="truncate font-semibold">{agent.name}</span>
                      <StatusBadge status={agent.status} />
                      <span className="truncate font-mono text-xs text-muted-foreground">{agent.pane_id}</span>
                      {agent.title && <span className="w-full truncate text-xs text-muted-foreground">{agent.title}</span>}
                    </button>
                    {filePath && (
                      <Button
                        variant="ghost"
                        size="xs"
                        className="h-6 shrink-0 cursor-pointer px-1.5 text-[11px]"
                        onClick={(event) => {
                          event.stopPropagation()
                          void openLinkedTaskFile(filePath)
                        }}
                        title={`Open ${filePath} in Files`}
                        aria-label={`Open ${filePath} in Files`}
                      >
                        <FileText className="mr-1 size-3" />
                        Files
                      </Button>
                    )}
                    <Button
                      variant="ghost"
                      size="xs"
                      className="h-6 shrink-0 cursor-pointer px-1.5 text-[11px]"
                      onClick={() => void renameAgent(agent)}
                      title="Rename agent"
                      aria-label={`Rename agent ${agent.name}`}
                    >
                      Rename agent
                    </Button>
                    <Button
                      variant="ghost"
                      size="xs"
                      className="h-6 shrink-0 cursor-pointer px-1.5 text-[11px] text-red-600 hover:bg-red-50 hover:text-red-700"
                      onClick={() => void stopAgent(agent)}
                      disabled={stoppingPaneId !== null}
                      title="Stop agent"
                      aria-label={`Stop agent ${agent.name}`}
                    >
                      {stoppingPaneId === agent.pane_id ? <Loader2 className="mr-1 size-3 animate-spin" /> : <Square className="mr-1 size-3" />}
                      Stop
                    </Button>
                  </div>
                  {openAgentPaneId === agent.pane_id && (
                    <HerdrAgentDetail
                      agent={agent}
                      autoReload={autoReload}
                      reloadToken={reloadToken}
                      draft={drafts[agent.pane_id] ?? ''}
                      onDraftChange={(draft) => onDraftChange(agent.pane_id, draft)}
                      displayMode={displayMode}
                      onDisplayModeChange={onDisplayModeChange}
                      onFilePathClick={onFilePathClick}
                    />
                  )}
                </div>
              )
            })}
          </div>
        )}
      </div>
    </div>
  )

  const launchDialog = (
    <AgentLaunchDialog
      open={agentLaunchOpen}
      kinds={agentKinds}
      tabs={projectTabLabels}
      agentKind={agentKind}
      tab={agentTab}
      loadingKinds={loadingAgentKinds}
      starting={startingAgent}
      error={agentLaunchError}
      onAgentKindChange={setAgentKind}
      onTabChange={setAgentTab}
      onOpenChange={setAgentLaunchOpen}
      onSubmit={() => void startAgent()}
    />
  )

  if (isMobile) {
    return <>
      <Sheet open={open} onOpenChange={onOpenChange}>
        <SheetContent
          side="right"
          showCloseButton={false}
          className="w-[92vw] max-w-lg gap-0 border-l bg-card p-0"
          data-testid="agent-sidebar-sheet"
        >
          <SheetHeader className="sr-only">
            <SheetTitle>Agent sidebar</SheetTitle>
            <SheetDescription>Operate agents for the current project.</SheetDescription>
          </SheetHeader>
          {content}
        </SheetContent>
      </Sheet>
      {launchDialog}
    </>
  }

  if (!open) {
    return (
      <aside
        className="relative hidden h-svh shrink-0 border-l bg-card md:flex"
        style={{ width: AGENT_SIDEBAR_COLLAPSED_WIDTH }}
        data-testid="agent-sidebar-collapsed"
      >
        <div className="flex h-full w-full flex-col items-center gap-3 py-2">
          <Button
            variant="ghost"
            size="icon-xs"
            className="shrink-0 cursor-pointer"
            onClick={() => onOpenChange(true)}
            aria-label="Open agent sidebar"
            title="Open agent sidebar"
            data-testid="agent-sidebar-collapsed-toggle"
          >
            <PanelRightOpen />
          </Button>
          <div className="flex min-h-0 flex-col items-center gap-3 overflow-y-auto" aria-label="Agent statuses">
            {projectAgents.map((agent) => (
              <button
                key={agent.pane_id}
                type="button"
                className={cn(
                  'flex size-8 shrink-0 cursor-pointer items-center justify-center rounded-md outline-none hover:bg-accent focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50',
                  openAgentPaneId === agent.pane_id && 'bg-accent ring-2 ring-primary/30',
                )}
                onClick={() => {
                  onOpenAgentChange(agent.pane_id)
                  onOpenChange(true)
                }}
                aria-label={`Open ${agent.name} (${agent.status})`}
                title={`${agent.name}: ${agent.status}`}
                data-testid={`agent-sidebar-status-${agent.pane_id}`}
              >
                <StatusDot status={agent.status} className="size-3" />
              </button>
            ))}
          </div>
          {fileExecution.runs.length > 0 && (
            <div className="flex min-h-0 flex-col items-center gap-3 overflow-y-auto" aria-label="Script executions">
              {fileExecution.runs.map((run) => (
                <button
                  key={run.id}
                  type="button"
                  className="flex size-8 shrink-0 cursor-pointer items-center justify-center rounded-md outline-none hover:bg-accent focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50"
                  onClick={() => fileExecution.open(run.id)}
                  aria-label={`Open execution ${run.path} (${executionStatusLabel(run)})`}
                  title={`${run.path}: ${executionStatusLabel(run)}`}
                  data-testid={`agent-sidebar-execution-status-${run.id}`}
                >
                  <ExecutionStatusIcon run={run} />
                </button>
              ))}
            </div>
          )}
        </div>
      </aside>
    )
  }

  return <>
    <aside
      className={cn('relative hidden h-svh shrink-0 border-l md:flex', !width.resizing && 'transition-[width] duration-200 ease-linear')}
      style={{ width: agentSidebarWidthStyle(displayedWidth) }}
      data-testid="agent-sidebar"
    >
      <div
        role="separator"
        aria-label="Resize agent sidebar"
        aria-orientation="vertical"
        aria-valuemin={AGENT_SIDEBAR_MIN_WIDTH}
        aria-valuemax={AGENT_SIDEBAR_MAX_WIDTH}
        aria-valuenow={displayedWidth}
        tabIndex={0}
        className={cn('absolute inset-y-0 left-0 z-10 w-2 -translate-x-1/2 cursor-col-resize touch-none rounded-sm outline-none hover:bg-border/60 focus-visible:bg-border', width.resizing && 'bg-border/70')}
        onPointerDown={width.handlePointerDown}
        onKeyDown={width.handleKeyDown}
      />
      {content}
    </aside>
    {launchDialog}
  </>
}
