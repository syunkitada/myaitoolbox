import { lazy, Suspense, useCallback, useEffect, useLayoutEffect, useRef, useState } from 'react'
import { Routes, Route, useLocation, useNavigate, Navigate, NavLink } from 'react-router-dom'
import { api, Meta, ProjectGitStatus } from './api/client'
import { encodePath, getProject, projectUrl, rememberCurrentTab, rememberedFilesUrl } from './utils/routes'
import { AppSidebar } from './components/Sidebar'
import { useHerdrOverview } from './hooks/use-herdr'
import { useAgentFavicon } from './hooks/use-agent-favicon'
import { Button } from './components/ui/button'

import { Separator } from './components/ui/separator'
import { SidebarInset, SidebarProvider, SidebarTrigger } from './components/ui/sidebar'
import { TerminalPanel } from './components/TerminalPanel'
import {
  AgentSidebar,
  AGENT_SIDEBAR_COLLAPSED_WIDTH,
  AGENT_SIDEBAR_MAX_WIDTH,
  AGENT_SIDEBAR_MIN_WIDTH,
  AGENT_SIDEBAR_WIDTH_STORAGE_KEY,
  DEFAULT_AGENT_SIDEBAR_WIDTH,
  agentSidebarWidthStyle,
} from './components/AgentSidebar'
import { StatusDot } from './components/herdr-status'
import { DialogsProvider } from './components/AppDialogs'
import { HerdrPage } from './pages/HerdrPage'
import { Bot, Folder, GitBranch, Network, PanelRightClose, PanelRightOpen, SquareKanban, TerminalSquare } from 'lucide-react'

import { dispatchNavAction } from './lib/nav-actions'
import { cn } from '@/lib/utils'
import { useAgentSidebarState } from './hooks/use-agent-sidebar'
import { readResizableWidth } from './hooks/use-resizable-width'
import { useIsMobile } from './hooks/use-mobile'
import { projectAgentsFor, summarizeAgentStatuses } from './utils/agent-sidebar-status'

const Dashboard = lazy(() => import('./pages/Dashboard').then((m) => ({ default: m.Dashboard })))
const KnowledgeGraphPage = lazy(() =>
  import('./pages/KnowledgeGraphPage').then((m) => ({ default: m.KnowledgeGraphPage })),
)
const KanbanBoard = lazy(() => import('./pages/KanbanBoard').then((m) => ({ default: m.KanbanBoard })))
const StatsPage = lazy(() => import('./pages/StatsPage').then((m) => ({ default: m.StatsPage })))
const ProjectsPage = lazy(() => import('./pages/ProjectsPage').then((m) => ({ default: m.ProjectsPage })))
const GitPage = lazy(() => import('./pages/GitPage').then((m) => ({ default: m.GitPage })))

// Canonical project tab sections; position in this list is the tab index.
const TAB_SECTIONS = ['dashboard', 'board', 'graph', 'git', 'herdr']
const GIT_STATUS_REFRESH_INTERVAL_MS = 5000

export default function App() {
  const [meta, setMeta] = useState<Meta | null>(null)
  const [gitStatus, setGitStatus] = useState<Record<string, ProjectGitStatus>>({})
  const [error, setError] = useState<string | null>(null)
  const gitStatusRequestSeq = useRef(0)
  const { pathname } = useLocation()
  const navigate = useNavigate()
  const project = getProject()
  const isMobile = useIsMobile()
  const [agentSidebarFocusedPaneId, setAgentSidebarFocusedPaneId] = useState<string | null>(null)
  const [embeddedWebuiFocusedPaneId, setEmbeddedWebuiFocusedPaneId] = useState<string | null>(null)
  const [agentSidebarWidth, setAgentSidebarWidth] = useState(() => readResizableWidth({
    storageKey: AGENT_SIDEBAR_WIDTH_STORAGE_KEY,
    defaultWidth: DEFAULT_AGENT_SIDEBAR_WIDTH,
    minWidth: AGENT_SIDEBAR_MIN_WIDTH,
    maxWidth: AGENT_SIDEBAR_MAX_WIDTH,
  }))
  const agentSidebar = useAgentSidebarState(project, isMobile)
  const myboxFocusedPaneId = agentSidebar.open && agentSidebarFocusedPaneId
    ? agentSidebarFocusedPaneId
    : embeddedWebuiFocusedPaneId
  const herdr = useHerdrOverview(5000, myboxFocusedPaneId)
  const agentStatus = summarizeAgentStatuses(projectAgentsFor(herdr.overview, project))
  useAgentFavicon(herdr.overview)

  const handleAgentSidebarFocusChange = useCallback((paneId: string | null) => {
    setAgentSidebarFocusedPaneId(paneId)
  }, [])

  const handleEmbeddedFocusChange = useCallback((paneId: string | null) => {
    setEmbeddedWebuiFocusedPaneId(paneId)
  }, [])

  const refreshGitStatus = useCallback(async () => {
    const seq = ++gitStatusRequestSeq.current
    const next = await api.getProjectGitStatus()
    if (seq === gitStatusRequestSeq.current) setGitStatus(next)
  }, [])

  const refreshMeta = useCallback(async () => {
    try {
      const [m] = await Promise.all([api.getMeta(), refreshGitStatus()])
      setMeta(m)
      setError(null)
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    }
  }, [refreshGitStatus])

  const handleRecentChanged = useCallback((path: string) => {
    setMeta((current) => {
      if (!current) return current
      return {
        ...current,
        recent_files: [path, ...(current.recent_files ?? []).filter((item) => item !== path)].slice(0, 50),
      }
    })
  }, [])

  useEffect(() => {
    void refreshMeta()
  }, [refreshMeta, project])

  useEffect(() => {
    const interval = window.setInterval(() => {
      if (!document.hidden) void refreshGitStatus().catch(() => undefined)
    }, GIT_STATUS_REFRESH_INTERVAL_MS)
    const onVisible = () => {
      if (!document.hidden) void refreshGitStatus().catch(() => undefined)
    }
    document.addEventListener('visibilitychange', onVisible)
    return () => {
      window.clearInterval(interval)
      document.removeEventListener('visibilitychange', onVisible)
    }
  }, [refreshGitStatus])

  // Save the current tab path whenever the user navigates within a project
  useEffect(() => {
    if (project) rememberCurrentTab()
  }, [project, pathname])

  // Agent sidebar focus remains active while navigating between sections of a
  // project, but neither focus source leaks into another project.
  useLayoutEffect(() => {
    setAgentSidebarFocusedPaneId(null)
    setEmbeddedWebuiFocusedPaneId(null)
  }, [project])

  // Show <projectIndex>:<tabIndex> as the title; 0 outside a project
  useEffect(() => {
    if (!project || !meta) {
      document.title = '0'
      return
    }
    const index = meta.projects.indexOf(project)
    if (index < 0) {
      document.title = '0'
      return
    }
    const section = pathname.split('/')[3] || ''
    const tab = TAB_SECTIONS.indexOf(section)
    document.title = `${index + 1}:${tab < 0 ? 0 : tab}`
  }, [project, meta, pathname])

  const projectTabs = [
    {
      to: rememberedFilesUrl(),
      label: 'Files',
      icon: Folder,
      active: pathname.includes('/dashboard'),
    },
    { to: projectUrl('/board'), label: 'Board', icon: SquareKanban, active: pathname === projectUrl('/board') },
    { to: projectUrl('/graph'), label: 'Graph', icon: Network, active: pathname === projectUrl('/graph') },
    { to: projectUrl('/git'), label: 'Git', icon: GitBranch, active: pathname === projectUrl('/git') },
    { to: projectUrl('/herdr'), label: 'Herdr', icon: Bot, active: pathname === projectUrl('/herdr') },
  ]

  return (
    <DialogsProvider>
      <SidebarProvider
        style={{
          '--sidebar-width': '20rem',
          '--sidebar-right-width': isMobile
            ? '0px'
            : agentSidebar.open ? agentSidebarWidthStyle(agentSidebarWidth) : `${AGENT_SIDEBAR_COLLAPSED_WIDTH}px`,
        } as React.CSSProperties}
      >
      <AppSidebar
        meta={meta}
        project={project}
        herdr={herdr.overview}
        gitStatus={gitStatus}
        myboxFocusedPaneId={myboxFocusedPaneId}
        agentSidebarOpen={agentSidebar.open}
      />
      <SidebarInset>
        <header className="sticky top-0 z-30 flex h-14 shrink-0 items-center gap-2 border-b bg-background px-4">
          <div className="flex flex-1 items-center gap-2">
            <SidebarTrigger />
            <Separator orientation="vertical" className="mr-2 data-[orientation=vertical]:h-4" />
            {project && (
              <nav aria-label="Project sections" className="project-tabs ml-4 flex min-w-0 items-center gap-1 overflow-x-auto">
                {projectTabs.map((t) => (
                  <NavLink
                    key={t.label}
                    to={t.to}
                    aria-label={t.label}
                    title={t.label}
                    aria-current={t.active ? 'page' : undefined}
                    className={cn(
                      'inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-md text-sm font-medium whitespace-nowrap transition-colors outline-none hover:bg-accent hover:text-accent-foreground focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50',
                      t.active ? 'bg-accent text-primary' : 'text-muted-foreground',
                    )}
                  >
                    <t.icon className="size-4" />
                  </NavLink>
                ))}
              </nav>
            )}
            {project && (
              <Button
                variant="ghost"
                size="sm"
                className="cursor-pointer"
                onClick={() => dispatchNavAction('open-terminal')}
                aria-label="Open terminal"
                title="Open terminal"
              >
                <TerminalSquare />
              </Button>
            )}
            {project && (
              <Button
                variant={agentSidebar.open ? 'secondary' : 'ghost'}
                size="sm"
                className="cursor-pointer"
                onClick={() => agentSidebar.setOpen(!agentSidebar.open)}
                aria-label={isMobile
                  ? `${agentStatus.label}; ${agentSidebar.open ? 'Close' : 'Open'} agent sidebar`
                  : agentSidebar.open ? 'Close agent sidebar' : 'Open agent sidebar'}
                title={isMobile
                  ? `${agentStatus.label}; ${agentSidebar.open ? 'Close' : 'Open'} agent sidebar`
                  : agentSidebar.open ? 'Close agent sidebar' : 'Open agent sidebar'}
                data-testid="agent-sidebar-toggle"
              >
                {isMobile
                  ? <StatusDot status={agentStatus.status} className="size-3.5 ring-2 ring-background" />
                  : agentSidebar.open ? <PanelRightClose /> : <PanelRightOpen />}
              </Button>
            )}
          </div>
        </header>
        <div className="flex min-h-0 flex-col" style={{ height: 'calc(100svh - 3.5rem)' }}>
          {error && (
            <div className="error-banner m-2 flex items-center justify-between rounded-md border border-red-200 bg-red-50 px-3.5 py-2.5 text-sm text-red-700">
              {error}
              <Button
                variant="ghost"
                size="sm"
                onClick={() => void refreshMeta()}
                className={cn('text-red-700 hover:bg-red-100 hover:text-red-800')}
              >
                retry
              </Button>
            </div>
          )}
          <div className="min-h-0 flex-1">
            <Suspense fallback={<div className="page p-4 text-muted-foreground md:p-6">Loading…</div>}>
              <Routes>
              <Route path="/projects" element={<ProjectsPage onChanged={refreshMeta} />} />
              <Route path="/stats" element={<StatsPage />} />
              {project ? (
                <>
                  <Route
                    path="/projects/:project/dashboard"
                    element={
                      <Dashboard
                        key={project}
                        refreshMeta={refreshMeta}
                        onRecentChanged={handleRecentChanged}
                        favorites={meta?.favorites ?? []}
                        recentFiles={meta?.recent_files ?? []}
                        herdrOverview={herdr.overview}
                        refreshHerdr={herdr.refresh}
                        agentSidebarOpen={agentSidebar.open}
                        onWebuiFocusChange={handleEmbeddedFocusChange}
                      />
                    }
                  />
                  <Route
                    path="/projects/:project/dashboard/files/*"
                    element={
                      <Dashboard
                        key={project}
                        refreshMeta={refreshMeta}
                        onRecentChanged={handleRecentChanged}
                        favorites={meta?.favorites ?? []}
                        recentFiles={meta?.recent_files ?? []}
                        herdrOverview={herdr.overview}
                        refreshHerdr={herdr.refresh}
                        agentSidebarOpen={agentSidebar.open}
                        onWebuiFocusChange={handleEmbeddedFocusChange}
                      />
                    }
                  />
                  <Route
                    path="/projects/:project/board"
                    element={<KanbanBoard key={project} herdrOverview={herdr.overview} />}
                  />
                  <Route path="/projects/:project/graph" element={<KnowledgeGraphPage key={project} />} />
                  <Route
                    path="/projects/:project/git"
                    element={<GitPage key={project} refreshMeta={refreshMeta} />}
                  />
                  <Route
                    path="/projects/:project/herdr"
                    element={
                      <HerdrPage
                        key={project}
                        overview={herdr.overview}
                        error={herdr.error}
                        loading={herdr.loading}
                        refresh={herdr.refresh}
                      />
                    }
                  />
                  <Route
                    path="/projects/:project"
                    element={<Navigate to={projectUrl('/dashboard')} replace />}
                  />
                  <Route path="*" element={<Navigate to={projectUrl('/dashboard')} replace />} />
                </>
              ) : (
                <>
                  <Route path="/board" element={<KanbanBoard />} />
                  <Route path="*" element={<Navigate to="/projects" replace />} />
                </>
              )}
              </Routes>
            </Suspense>
          </div>
          {project && <TerminalPanel />}
        </div>
      </SidebarInset>
      {project && (
        <AgentSidebar
          project={project}
          overview={herdr.overview}
          error={herdr.error}
          loading={herdr.loading}
          open={agentSidebar.open}
          onOpenChange={agentSidebar.setOpen}
          displayMode={agentSidebar.displayMode}
          onDisplayModeChange={agentSidebar.setDisplayMode}
          onWidthChange={setAgentSidebarWidth}
          openAgentPaneId={agentSidebar.paneId}
          onOpenAgentChange={agentSidebar.setPaneId}
          refresh={herdr.refresh}
          onFocusChange={handleAgentSidebarFocusChange}
          onFilePathClick={(filePath) => navigate(projectUrl(`/dashboard/files/${encodePath(filePath)}`))}
        />
      )}
      </SidebarProvider>
    </DialogsProvider>
  )
}
