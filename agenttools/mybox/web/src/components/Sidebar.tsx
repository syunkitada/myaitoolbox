import { useState } from 'react'
import { NavLink, useNavigate } from 'react-router-dom'
import { Meta, setProject } from '../api/client'
import { SearchBar } from './SearchBar'

interface SidebarProps {
  meta: Meta | null
  navigate: ReturnType<typeof useNavigate>
}

export function Sidebar({ meta, navigate }: SidebarProps) {
  const [q, setQ] = useState('')

  return (
    <aside className="sidebar">
      <div className="sidebar-brand">
        mybox
        {meta && meta.projects && meta.projects.length > 0 && (
          <select
            value={meta.project}
            onChange={(e) => {
              setProject(e.target.value)
              window.location.reload()
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
            {meta.projects.map((p) => (
              <option key={p} value={p}>
                {p}
              </option>
            ))}
          </select>
        )}
      </div>
      <SearchBar
        value={q}
        onChange={setQ}
        onSubmit={(query) => {
          navigate(`/search?q=${encodeURIComponent(query)}`)
        }}
      />
      <nav className="sidebar-nav">
        <NavLink to="/" end>
          Dashboard
        </NavLink>
        <NavLink to="/tasks">Tasks</NavLink>
        <NavLink to="/board">Board</NavLink>
        <NavLink to="/knowledge">Knowledge</NavLink>
        <NavLink to="/graph">Graph</NavLink>
      </nav>
      <div className="sidebar-section">
        <div className="sidebar-section-title">Favorites</div>
        <ul className="sidebar-list">
          {(meta?.favorites ?? []).map((p) => (
            <li key={p}>
              <button
                onClick={() => navigate(`/knowledge/${encodeURIComponent(p)}`)}
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
                onClick={() => navigate(`/knowledge/${encodeURIComponent(p)}`)}
                className="link-btn"
              >
                {p}
              </button>
            </li>
          ))}
        </ul>
      </div>
    </aside>
  )
}
