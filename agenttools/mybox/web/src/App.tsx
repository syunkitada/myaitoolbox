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
    <div className="app">
      {/* Mobile overlay */}
      {sidebarOpen && <div className="sidebar-overlay" onClick={closeSidebar} />}

      {/* Hamburger button (mobile only) */}
      <button
        className="hamburger"
        aria-label="Toggle menu"
        onClick={() => setSidebarOpen((o) => !o)}
      >
        <span />
        <span />
        <span />
      </button>

      <Sidebar meta={meta} navigate={navigate} open={sidebarOpen} onClose={closeSidebar} project={project} />
      <main className="content">
        {error && (
          <div className="error-banner">
            {error}
            <button onClick={() => void refreshMeta()}>retry</button>
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
