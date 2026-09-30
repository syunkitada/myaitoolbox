import { useCallback, useEffect, useState } from 'react'
import { useLocation, useNavigate, useParams } from 'react-router-dom'
import { LAST_SELECTED_FILE_KEY, encodePath, projectUrl, getProject, rememberedFilesUrl } from '../utils/routes'
import { BrowserPage } from './BrowserPage'
import { api, HerdrOverview, Task, TaskTrigger } from '../api/client'
import { NewTaskDialog } from '../components/NewTaskDialog'
import { useDialogs } from '../components/AppDialogs'
import { subscribeNavActions } from '../lib/nav-actions'

interface DashboardProps {
  refreshMeta: () => Promise<void>
  onRecentChanged?: (path: string) => void
  favorites: string[]
  recentFiles: string[]
  herdrOverview?: HerdrOverview | null
  refreshHerdr?: () => void
  webuiFocusedPaneId?: string | null
  onWebuiFocusChange?: (paneId: string | null) => void
}

export function Dashboard({
  refreshMeta,
  onRecentChanged,
  favorites,
  recentFiles,
  herdrOverview,
  refreshHerdr,
  webuiFocusedPaneId,
  onWebuiFocusChange,
}: DashboardProps) {
  const params = useParams()
  const { pathname } = useLocation()
  // The app is mounted below a top-level `*` route. On the dashboard index
  // route React Router therefore exposes the whole URL as that splat, even
  // though no file is selected. Only the nested files route owns the splat.
  const selected = pathname.includes('/dashboard/files/') ? (params['*'] ?? '').trim() : ''
  const navigate = useNavigate()
  const [taskDialog, setTaskDialog] = useState(false)
  const [revealPath, setRevealPath] = useState<string | undefined>()
  const { prompt, alert } = useDialogs()

  useEffect(() => {
    if (!selected) {
      const target = rememberedFilesUrl()
      if (target !== projectUrl('/dashboard')) {
        navigate(target, { replace: true })
      }
    }
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  const persistSelected = (path: string) => {
    const project = getProject()
    if (!project) return
    try {
      const stored = localStorage.getItem(LAST_SELECTED_FILE_KEY)
      const map = stored ? (JSON.parse(stored) as Record<string, string>) : {}
      map[project] = path
      localStorage.setItem(LAST_SELECTED_FILE_KEY, JSON.stringify(map))
    } catch {
      // ignore
    }
    navigate(projectUrl(`/dashboard/files/${encodePath(path)}`))
  }

  const handleNewTask = () => {
    setTaskDialog(true)
  }

  const handleNewFile = async (dir: string) => {
    const prefix = dir ? `${dir}/` : ''
    const name = await prompt('New file path', prefix)
    if (!name || !name.trim()) return
    const path = name.trim()
    void api
      .createFile(path)
      .then(() => navigate(projectUrl(`/dashboard/files/${encodePath(path)}`)))
      .catch((e) => void alert(e instanceof Error ? e.message : String(e)))
  }

  const onTaskCreated = (task: Task) =>
		navigate(projectUrl(`/dashboard/files/_tasks/${encodePath(task.id)}/task.md`))

  const onTriggerCreated = useCallback((trigger: TaskTrigger) => {
    setRevealPath(trigger.task_path)
  }, [])

  const onRevealPathHandled = useCallback((path: string) => {
    setRevealPath((current) => (current === path ? undefined : current))
  }, [])

  useEffect(
    () =>
      subscribeNavActions((action) => {
        if (action === 'new-task') handleNewTask()
        else if (action === 'new-file') handleNewFile('')
      }),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [],
  )

  return (
    <>
      <BrowserPage
        title="Files"
        selected={selected}
        onSelect={persistSelected}
        onBack={() => navigate(projectUrl('/dashboard'))}
        favorites={favorites}
        recentFiles={recentFiles}
        refreshMeta={refreshMeta}
        onRecentChanged={onRecentChanged}
        revealPath={revealPath}
        onRevealPathHandled={onRevealPathHandled}
        herdrOverview={herdrOverview}
        refreshHerdr={refreshHerdr}
        webuiFocusedPaneId={webuiFocusedPaneId}
        onWebuiFocusChange={onWebuiFocusChange}
        defaultSelect={(entries) =>
          entries.some((e) => e.kind === 'file' && e.path === 'README.md') ? 'README.md' : undefined
        }
      />
      <NewTaskDialog
        open={taskDialog}
        onOpenChange={setTaskDialog}
        onCreated={onTaskCreated}
        onTriggerCreated={onTriggerCreated}
        onError={(message) => void alert(message)}
      />
    </>
  )
}
