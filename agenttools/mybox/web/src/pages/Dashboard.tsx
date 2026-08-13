import { useNavigate, useParams } from 'react-router-dom'
import { encodePath, projectUrl } from '../utils/routes'
import { BrowserPage } from './BrowserPage'
import { api } from '../api/client'

interface DashboardProps {
  refreshMeta: () => Promise<void>
  favorites: string[]
}

export function Dashboard({ refreshMeta, favorites }: DashboardProps) {
  const params = useParams()
  const selected = (params['*'] ?? '').trim()
  const navigate = useNavigate()

  const handleNewTask = () => {
    const name = window.prompt('New task name')
    if (name && name.trim()) {
      void api
        .createTask({ name: name.trim() })
        .then((t) => navigate(projectUrl(`/dashboard/files/tasks/${encodePath(t.id)}/task.md`)))
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

  return (
    <BrowserPage
      mode="files"
      root=""
      title="Files"
      selected={selected}
      onSelect={(path) => navigate(projectUrl(`/dashboard/files/${encodePath(path)}`))}
      onBack={() => navigate(projectUrl('/dashboard'))}
      favorites={favorites}
      refreshMeta={refreshMeta}
      onNew={handleNewTask}
      newLabel="New task"
      onNewFile={handleNewFile}
      defaultSelect={(entries) =>
        entries.some((e) => e.kind === 'file' && e.path === 'README.md') ? 'README.md' : undefined
      }
    />
  )
}
