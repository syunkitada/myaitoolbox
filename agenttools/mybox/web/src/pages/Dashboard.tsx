import { useEffect } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { encodePath, projectUrl, getProject } from '../utils/routes'
import { BrowserPage } from './BrowserPage'
import { api } from '../api/client'
import { subscribeNavActions } from '../lib/nav-actions'

const LAST_FILE_KEY = 'mybox_last_selected_file'

interface DashboardProps {
  refreshMeta: () => Promise<void>
  favorites: string[]
  recentFiles: string[]
}

export function Dashboard({ refreshMeta, favorites, recentFiles }: DashboardProps) {
  const params = useParams()
  const selected = (params['*'] ?? '').trim()
  const navigate = useNavigate()

  useEffect(() => {
    if (!selected) {
      const project = getProject()
      const stored = localStorage.getItem(LAST_FILE_KEY)
      if (stored) {
        try {
          const map = JSON.parse(stored) as Record<string, string>
          if (project && map[project]) {
            navigate(projectUrl(`/dashboard/files/${encodePath(map[project])}`), { replace: true })
          }
        } catch {
          // ignore malformed storage
        }
      }
    }
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  const persistSelected = (path: string) => {
    const project = getProject()
    if (!project) return
    try {
      const stored = localStorage.getItem(LAST_FILE_KEY)
      const map = stored ? (JSON.parse(stored) as Record<string, string>) : {}
      map[project] = path
      localStorage.setItem(LAST_FILE_KEY, JSON.stringify(map))
    } catch {
      // ignore
    }
    navigate(projectUrl(`/dashboard/files/${encodePath(path)}`))
  }

  const handleNewTask = () => {
    const name = window.prompt('New task name')
    if (name && name.trim()) {
      void api
        .createTask({ name: name.trim() })
        .then((t) => navigate(projectUrl(`/dashboard/files/tasks/${encodePath(t.id)}/task.md`)))
        .catch(() => undefined)
    }
  }

  const handleNewAdhoc = () => {
    const name = window.prompt('Adhoc task name')
    if (name && name.trim()) {
      void api
        .createTask({ name: name.trim(), type: 'adhoc' })
        .then((t) => navigate(projectUrl(`/dashboard/files/tasks/adhoc/${encodePath(t.id)}.md`)))
        .catch(() => undefined)
    }
  }

  const handleNewFile = (dir: string) => {
    const prefix = dir ? `${dir}/` : ''
    const name = window.prompt('New file path', prefix)
    if (!name || !name.trim()) return
    const path = name.trim()
    void api
      .createFile(path)
      .then(() => navigate(projectUrl(`/dashboard/files/${encodePath(path)}`)))
      .catch((e) => window.alert(e instanceof Error ? e.message : String(e)))
  }

  useEffect(
    () =>
      subscribeNavActions((action) => {
        if (action === 'new-task') handleNewTask()
        else if (action === 'new-adhoc') handleNewAdhoc()
        else if (action === 'new-file') handleNewFile('')
      }),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [],
  )

  return (
    <BrowserPage
      mode="files"
      title="Files"
      selected={selected}
      onSelect={persistSelected}
      onBack={() => navigate(projectUrl('/dashboard'))}
      favorites={favorites}
      recentFiles={recentFiles}
      refreshMeta={refreshMeta}
      defaultSelect={(entries) =>
        entries.some((e) => e.kind === 'file' && e.path === 'README.md') ? 'README.md' : undefined
      }
    />
  )
}
