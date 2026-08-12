import { useState } from 'react'
import { NavLink, useNavigate } from 'react-router-dom'
import { Meta } from '../api/client'
import { clearProject, encodePath, projectUrl, setProject } from '../utils/routes'
import { SearchBar } from './SearchBar'

interface SidebarProps {
  meta: Meta | null
  navigate: ReturnType<typeof useNavigate>
  open?: boolean
  onClose?: () => void
  project: string
}

export function Sidebar({ meta, navigate, open, onClose, project }: SidebarProps) {
  const [q, setQ] = useState('')

  const handleNav = () => {
    onClose?.()
  }

  return (
    <aside className={`sidebar${open ? ' sidebar--open' : ''}`}>
      <div className="sidebar-brand">
        <button
          className="sidebar-brand-link"
          onClick={clearProject}
          aria-label="Go to top"
        >
          mybox
        </button>
        {meta && meta.projects.length > 0 && (
          <select
            value={project}
            onChange={(e) => {
              if (e.target.value) setProject(e.target.value)
            }}
            style={{
              marginLeft: '0.5rem',
              maxWidth: '120px',
              fontSize: '0.85rem',
              padding: '2px 4px',
              borderRadius: '4px',
              border: '1px solid var(--border-color)',
              background: 'var(--bg-card)',
              color: 'var(--text-main)',
            }}
          >
            {!project && <option value="">未選択</option>}
            {meta.projects.map((p) => (
              <option key={p} value={p}>
                {p}
              </option>
            ))}
          </select>
        )}
      </div>
      {project ? (
        <>
          <SearchBar
            value={q}
            onChange={setQ}
            onSubmit={(query) => {
              navigate(projectUrl(`/search?q=${encodeURIComponent(query)}`))
              handleNav()
            }}
          />
          <nav className="sidebar-nav">
            <NavLink to={projectUrl('/dashboard')} onClick={handleNav}>
              Dashboard
            </NavLink>
            <NavLink to={projectUrl('/board')} onClick={handleNav}>Board</NavLink>
            <NavLink to={projectUrl('/graph')} onClick={handleNav}>Graph</NavLink>
          </nav>
          <div className="sidebar-section">
            <div className="sidebar-section-title">Favorites</div>
            <ul className="sidebar-list">
              {(meta?.favorites ?? []).map((p) => (
                <li key={p}>
                  <button
                    onClick={() => {
                      navigate(projectUrl(`/dashboard/files/${encodePath(p)}`))
                      handleNav()
                    }}
                    className="link-btn"
                  >
                    {p}
                  </button>
                </li>
              ))}
            </ul>
          </div>
          <div className="sidebar-section">
            <div className="sidebar-section-title">Recent</div>
            <ul className="sidebar-list">
              {(meta?.recent_files ?? []).map((p) => (
                <li key={p}>
                  <button
                    onClick={() => {
                      navigate(projectUrl(`/dashboard/files/${encodePath(p)}`))
                      handleNav()
                    }}
                    className="link-btn"
                  >
                    {p}
                  </button>
                </li>
              ))}
            </ul>
          </div>
        </>
      ) : (
        <nav className="sidebar-nav">
          <NavLink to="/projects" end onClick={handleNav}>
            Projects
          </NavLink>
          <NavLink to="/board" onClick={handleNav}>
            Board
          </NavLink>
        </nav>
      )}
    </aside>
  )
}
