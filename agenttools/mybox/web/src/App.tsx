import { useCallback, useEffect, useState } from 'react'
import { Routes, Route, useNavigate, Navigate } from 'react-router-dom'
import { api, Meta } from './api/client'
import { getProject, projectUrl } from './utils/routes'
import { Sidebar } from './components/Sidebar'
import { Dashboard } from './pages/Dashboard'
import { GraphPage } from './pages/GraphPage'
import { SearchPage } from './pages/SearchPage'
import { KanbanBoard } from './pages/KanbanBoard'
import { ProjectsPage } from './pages/ProjectsPage'
import { Button } from './components/ui/button'
import { cn } from '@/lib/utils'

export default function App() {
  const [meta, setMeta] = useState<Meta | null>(null)
  const [error, setError] = useState<string | null>(null)
  const navigate = useNavigate()
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

  const [sidebarOpen, setSidebarOpen] = useState(false)

  const closeSidebar = useCallback(() => setSidebarOpen(false), [])

  return (
    <div className="app flex min-h-screen">
      {/* Mobile overlay */}
      {sidebarOpen && (
        <div className="sidebar-overlay fixed inset-0 z-[199] bg-black/40 max-md:block" onClick={closeSidebar} />
      )}

      {/* Hamburger button (mobile only) */}
      <button
        className="hamburger fixed top-3 left-3 z-[300] hidden h-10 w-10 cursor-pointer flex-col items-center justify-between gap-0 rounded-lg border bg-card p-2 max-md:flex"
        aria-label="Toggle menu"
        onClick={() => setSidebarOpen((o) => !o)}
      >
        <span className="block h-0.5 w-5 rounded bg-foreground transition-transform" />
        <span className="block h-0.5 w-5 rounded bg-foreground transition-transform" />
        <span className="block h-0.5 w-5 rounded bg-foreground transition-transform" />
      </button>

      <Sidebar meta={meta} navigate={navigate} open={sidebarOpen} onClose={closeSidebar} project={project} />
      <main className="content flex-1 max-md:pt-16">
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
      </main>
    </div>
  )
}
