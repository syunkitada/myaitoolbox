import { useCallback, useEffect, useState } from 'react'
import { Routes, Route, useLocation, useNavigate, Navigate } from 'react-router-dom'
import { api, Meta } from './api/client'
import { getProject, projectUrl } from './utils/routes'
import { AppSidebar } from './components/Sidebar'
import { Dashboard } from './pages/Dashboard'
import { GraphPage } from './pages/GraphPage'
import { SearchPage } from './pages/SearchPage'
import { KanbanBoard } from './pages/KanbanBoard'
import { ProjectsPage } from './pages/ProjectsPage'
import { Button } from './components/ui/button'
import { Breadcrumb, BreadcrumbItem, BreadcrumbList, BreadcrumbPage } from './components/ui/breadcrumb'
import { Separator } from './components/ui/separator'
import { SidebarInset, SidebarProvider, SidebarTrigger } from './components/ui/sidebar'
import { FilePlus, ListPlus, MessageSquare, TerminalSquare } from 'lucide-react'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from './components/ui/dropdown-menu'
import { dispatchNavAction } from './lib/nav-actions'
import { cn } from '@/lib/utils'

export default function App() {
  const [meta, setMeta] = useState<Meta | null>(null)
  const [error, setError] = useState<string | null>(null)
  const navigate = useNavigate()
  const { pathname } = useLocation()
  const project = getProject()

  const refreshMeta = useCallback(async () => {
    try {
      setMeta(await api.getMeta())
      setError(null)
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    }
  }, [])

  useEffect(() => {
    void refreshMeta()
  }, [refreshMeta])

  return (
    <SidebarProvider>
      <AppSidebar meta={meta} project={project} />
      <SidebarInset>
        <header className="sticky top-0 z-30 flex h-14 shrink-0 items-center gap-2 border-b bg-background px-4">
          <div className="flex flex-1 items-center gap-2">
            <SidebarTrigger />
            <Separator orientation="vertical" className="mr-2 data-[orientation=vertical]:h-4" />
            <Breadcrumb>
              <BreadcrumbList>
                <BreadcrumbItem>
                  <BreadcrumbPage className="line-clamp-1 max-w-[70vw] truncate">
                    {project ? project : 'Projects'}
                  </BreadcrumbPage>
                </BreadcrumbItem>
              </BreadcrumbList>
            </Breadcrumb>
          </div>
          {project && pathname.includes('/dashboard') && (
            <div className="nav-actions flex items-center gap-1">
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
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <button
                    className="inline-flex h-8 items-center justify-center gap-1.5 rounded-md px-3 text-sm font-medium whitespace-nowrap transition-all outline-none hover:bg-accent hover:text-accent-foreground cursor-pointer has-[>svg]:px-2.5 [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4"
                    aria-label="Open chat"
                    title="Open chat"
                  >
                    <MessageSquare />
                  </button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end">
                  <DropdownMenuItem onClick={() => dispatchNavAction('open-chat-opencode')}>
                    OpenCode
                  </DropdownMenuItem>
                  <DropdownMenuItem onClick={() => dispatchNavAction('open-chat-codex')}>
                    Codex
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
              <Button
                variant="ghost"
                size="sm"
                className="cursor-pointer"
                onClick={() => dispatchNavAction('new-file')}
                aria-label="New file"
                title="New file"
              >
                <FilePlus />
                <span className="hidden sm:inline">New file</span>
                <span className="sm:hidden">File</span>
              </Button>
              <Button
                variant="ghost"
                size="sm"
                className="cursor-pointer"
                onClick={() => dispatchNavAction('new-task')}
                aria-label="New task"
                title="New task"
              >
                <ListPlus />
                <span className="hidden sm:inline">New task</span>
                <span className="sm:hidden">Task</span>
              </Button>
            </div>
          )}
        </header>
        <div className="flex-1">
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
          <Routes>
            <Route path="/projects" element={<ProjectsPage onChanged={refreshMeta} />} />
            {project ? (
              <>
                <Route
                  path="/projects/:project/dashboard"
                  element={<Dashboard refreshMeta={refreshMeta} favorites={meta?.favorites ?? []} />}
                />
                <Route
                  path="/projects/:project/dashboard/files/*"
                  element={<Dashboard refreshMeta={refreshMeta} favorites={meta?.favorites ?? []} />}
                />
                <Route path="/projects/:project/board" element={<KanbanBoard />} />
                <Route path="/projects/:project/graph" element={<GraphPage />} />
                <Route path="/projects/:project/search" element={<SearchPage navigate={navigate} />} />
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
      </SidebarInset>
    </SidebarProvider>
  )
}
