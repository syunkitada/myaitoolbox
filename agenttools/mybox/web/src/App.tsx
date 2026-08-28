import { useCallback, useEffect, useState } from 'react'
import { Routes, Route, useLocation, useNavigate, Navigate, NavLink } from 'react-router-dom'
import { api, Meta, ProjectGitStatus } from './api/client'
import { getProject, projectUrl } from './utils/routes'
import { AppSidebar } from './components/Sidebar'
import { Dashboard } from './pages/Dashboard'
import { GraphPage } from './pages/GraphPage'
import { SearchPage } from './pages/SearchPage'
import { KanbanBoard } from './pages/KanbanBoard'
import { ProjectsPage } from './pages/ProjectsPage'
import { HerdrPage } from './pages/HerdrPage'
import { GitPage } from './pages/GitPage'
import { useHerdrOverview } from './hooks/use-herdr'
import { useAgentFavicon } from './hooks/use-agent-favicon'
import { Button } from './components/ui/button'

import { Separator } from './components/ui/separator'
import { SidebarInset, SidebarProvider, SidebarTrigger } from './components/ui/sidebar'
import { TerminalPanel } from './components/TerminalPanel'
import { Folder, GitBranch, Network, PanelsTopLeft, SquareKanban, TerminalSquare } from 'lucide-react'

import { dispatchNavAction } from './lib/nav-actions'
import { cn } from '@/lib/utils'

export default function App() {
  const [meta, setMeta] = useState<Meta | null>(null)
  const [gitStatus, setGitStatus] = useState<Record<string, ProjectGitStatus>>({})
  const [error, setError] = useState<string | null>(null)
  const navigate = useNavigate()
  const { pathname } = useLocation()
  const project = getProject()
  const herdr = useHerdrOverview()
  useAgentFavicon(herdr.overview)

  const refreshMeta = useCallback(async () => {
    try {
      const [m, gs] = await Promise.all([api.getMeta(), api.getProjectGitStatus()])
      setMeta(m)
      setGitStatus(gs)
      setError(null)
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    }
  }, [])

  useEffect(() => {
    void refreshMeta()
  }, [refreshMeta, project])

  const projectTabs = [
    { to: projectUrl('/dashboard'), label: 'Files', icon: Folder, active: pathname.includes('/dashboard') },
    { to: projectUrl('/board'), label: 'Board', icon: SquareKanban, active: pathname === projectUrl('/board') },
    { to: projectUrl('/graph'), label: 'Graph', icon: Network, active: pathname === projectUrl('/graph') },
    { to: projectUrl('/git'), label: 'Git', icon: GitBranch, active: pathname === projectUrl('/git') },
    { to: projectUrl('/herdr'), label: 'Herdr', icon: PanelsTopLeft, active: pathname === projectUrl('/herdr') },
  ]

  return (
    <SidebarProvider style={{ '--sidebar-width': '20rem' } as React.CSSProperties}>
      <AppSidebar meta={meta} project={project} herdr={herdr.overview} gitStatus={gitStatus} />
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
          </div>
        </header>
        <div className="flex min-h-0 flex-col" style={{ maxHeight: 'calc(100svh - 3.5rem)' }}>
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
            <Routes>
              <Route path="/projects" element={<ProjectsPage onChanged={refreshMeta} />} />
              {project ? (
                <>
                  <Route
                    path="/projects/:project/dashboard"
                    element={
                      <Dashboard
                        key={project}
                        refreshMeta={refreshMeta}
                        favorites={meta?.favorites ?? []}
                        recentFiles={meta?.recent_files ?? []}
                      />
                    }
                  />
                  <Route
                    path="/projects/:project/dashboard/files/*"
                    element={
                      <Dashboard
                        key={project}
                        refreshMeta={refreshMeta}
                        favorites={meta?.favorites ?? []}
                        recentFiles={meta?.recent_files ?? []}
                      />
                    }
                  />
                  <Route path="/projects/:project/board" element={<KanbanBoard key={project} />} />
                  <Route path="/projects/:project/graph" element={<GraphPage key={project} />} />
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
                        refresh={() => herdr.refresh()}
                      />
                    }
                  />
                  <Route path="/projects/:project/search" element={<SearchPage key={project} navigate={navigate} />} />
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
          </div>
          {project && <TerminalPanel />}
        </div>
      </SidebarInset>
    </SidebarProvider>
  )
}
