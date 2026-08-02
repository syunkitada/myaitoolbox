import { useState } from 'react'
import { NavLink, useNavigate } from 'react-router-dom'
import { Meta, setProject } from '../api/client'
import { SearchBar } from './SearchBar'

interface SidebarProps {
  meta: Meta | null
  navigate: ReturnType<typeof useNavigate>
  open?: boolean
  onClose?: () => void
}

export function Sidebar({ meta, navigate, open, onClose }: SidebarProps) {
  const [q, setQ] = useState('')

  const handleNav = () => {
    onClose?.()
  }

  return (
    <aside className={`sidebar${open ? ' sidebar--open' : ''}`}>
      <div className="sidebar-brand">
        mybox
        {meta && meta.projects && meta.projects.length > 0 && (
          <select
            value={meta.project}
            onChange={(e) => {
              setProject(e.target.value)
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
          handleNav()
        }}
      />
      <nav className="sidebar-nav">
        <NavLink to="/" end onClick={handleNav}>
          Dashboard
        </NavLink>
        <NavLink to="/tasks" onClick={handleNav}>Tasks</NavLink>
        <NavLink to="/board" onClick={handleNav}>Board</NavLink>
        <NavLink to="/knowledge" onClick={handleNav}>Knowledge</NavLink>
        <NavLink to="/graph" onClick={handleNav}>Graph</NavLink>
      </nav>
      <div className="sidebar-section">
        <div className="sidebar-section-title">Favorites</div>
        <ul className="sidebar-list">
          {(meta?.favorites ?? []).map((p) => (
            <li key={p}>
              <button
                onClick={() => {
                  navigate(`/knowledge/${encodeURIComponent(p)}`)
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
                  navigate(`/knowledge/${encodeURIComponent(p)}`)
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
    </aside>
  )
}
