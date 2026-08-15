import { useState } from 'react'
import { NavLink, useNavigate } from 'react-router-dom'
import { Meta } from '../api/client'
import { clearProject, encodePath, projectUrl, setProject } from '../utils/routes'
import { Button } from './ui/button'
import { cn } from '@/lib/utils'
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

  const navLinkClass = ({ isActive }: { isActive: boolean }) =>
    cn(
      'rounded-md px-2.5 py-2 text-sm transition-colors',
      isActive ? 'bg-primary font-medium text-primary-foreground' : 'text-foreground hover:bg-muted',
    )

  return (
    <aside
      className={cn(
        'sidebar sticky top-0 flex h-screen max-h-screen w-60 shrink-0 flex-col gap-4 overflow-y-auto border-r bg-card p-3',
        'max-md:fixed max-md:left-0 max-md:top-0 max-md:z-[200] max-md:h-full max-md:transition-transform max-md:duration-200 max-md:ease-in-out max-md:shadow-xl',
        open ? 'max-md:translate-x-0' : 'max-md:-translate-x-full',
      )}
    >
      <div className="sidebar-brand flex items-center">
        <Button
          variant="ghost"
          className="px-1 text-xl font-bold hover:bg-transparent"
          onClick={clearProject}
          aria-label="Go to top"
        >
          mybox
        </Button>
        {meta && meta.projects.length > 0 && (
          <select
            value={project}
            onChange={(e) => {
              if (e.target.value) setProject(e.target.value)
            }}
            aria-label="Switch project"
            className="ml-2 max-w-[120px] rounded border border-input bg-card px-1 py-0.5 text-sm text-foreground"
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
          <nav className="sidebar-nav flex flex-col gap-1">
            <NavLink to={projectUrl('/dashboard')} onClick={handleNav} className={navLinkClass}>
              Dashboard
            </NavLink>
            <NavLink to={projectUrl('/board')} onClick={handleNav} className={navLinkClass}>
              Board
            </NavLink>
            <NavLink to={projectUrl('/graph')} onClick={handleNav} className={navLinkClass}>
              Graph
            </NavLink>
          </nav>
          <div className="sidebar-section">
            <div className="sidebar-section-title mb-1 text-xs font-semibold tracking-wider text-muted-foreground uppercase">
              Favorites
            </div>
            <ul className="sidebar-list m-0 flex list-none flex-col gap-0.5 p-0">
              {(meta?.favorites ?? []).map((p) => (
                <li key={p}>
                  <Button
                    variant="link"
                    size="xs"
                    className="justify-start px-1.5"
                    onClick={() => {
                      navigate(projectUrl(`/dashboard/files/${encodePath(p)}`))
                      handleNav()
                    }}
                  >
                    {p}
                  </Button>
                </li>
              ))}
            </ul>
          </div>
          <div className="sidebar-section">
            <div className="sidebar-section-title mb-1 text-xs font-semibold tracking-wider text-muted-foreground uppercase">
              Recent
            </div>
            <ul className="sidebar-list m-0 flex list-none flex-col gap-0.5 p-0">
              {(meta?.recent_files ?? []).map((p) => (
                <li key={p}>
                  <Button
                    variant="link"
                    size="xs"
                    className="justify-start px-1.5"
                    onClick={() => {
                      navigate(projectUrl(`/dashboard/files/${encodePath(p)}`))
                      handleNav()
                    }}
                  >
                    {p}
                  </Button>
                </li>
              ))}
            </ul>
          </div>
        </>
      ) : (
        <nav className="sidebar-nav flex flex-col gap-1">
          <NavLink to="/projects" end onClick={handleNav} className={navLinkClass}>
            Projects
          </NavLink>
          <NavLink to="/board" onClick={handleNav} className={navLinkClass}>
            Board
          </NavLink>
        </nav>
      )}
    </aside>
  )
}
