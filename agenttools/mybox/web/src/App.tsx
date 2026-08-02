import { useCallback, useEffect, useState } from 'react'
import { Routes, Route, useNavigate } from 'react-router-dom'
import { api, Meta } from './api/client'
import { Sidebar } from './components/Sidebar'
import { Dashboard } from './pages/Dashboard'
import { TaskList } from './pages/TaskList'
import { TaskDetail } from './pages/TaskDetail'
import { KnowledgeExplorer } from './pages/KnowledgeExplorer'
import { KnowledgeView } from './pages/KnowledgeView'
import { GraphPage } from './pages/GraphPage'
import { SearchPage } from './pages/SearchPage'
import { KanbanBoard } from './pages/KanbanBoard'

export default function App() {
  const [meta, setMeta] = useState<Meta | null>(null)
  const [error, setError] = useState<string | null>(null)
  const navigate = useNavigate()

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

      <Sidebar meta={meta} navigate={navigate} open={sidebarOpen} onClose={closeSidebar} />
      <main className="content">
        {error && (
          <div className="error-banner">
            {error}
            <button onClick={() => void refreshMeta()}>retry</button>
          </div>
        )}
        <Routes>
          <Route path="/" element={<Dashboard meta={meta} navigate={navigate} />} />
          <Route path="/tasks" element={<TaskList />} />
          <Route path="/tasks/:id" element={<TaskDetail navigate={navigate} />} />
          <Route path="/board" element={<KanbanBoard />} />
          <Route path="/knowledge" element={<KnowledgeExplorer />} />
          <Route
            path="/knowledge/*"
            element={
              <KnowledgeView
                refreshMeta={refreshMeta}
                navigate={navigate}
                favorites={meta?.favorites ?? []}
              />
            }
          />
          <Route path="/graph" element={<GraphPage />} />
          <Route path="/search" element={<SearchPage navigate={navigate} />} />
        </Routes>
      </main>
    </div>
  )
}
