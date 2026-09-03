export function getBasePath(): string {
  return (window as unknown as { __MYBOX_BASE__?: string }).__MYBOX_BASE__ ?? ''
}

export function getProject(): string {
  const base = getBasePath()
  const rest = base ? window.location.pathname.slice(base.length) : window.location.pathname
  const parts = rest.split('/')
  if (parts.length > 2 && parts[1] === 'projects' && parts[2] !== '') {
    return decodeURIComponent(parts[2])
  }
  return ''
}

export function projectUrl(path: string): string {
  const project = getProject()
  if (!project) return '/projects'
  return projectUrlFor(project, path)
}

// projectUrlFor builds a project-scoped URL for an explicit project, unlike
// projectUrl which derives the project from the current location.
export function projectUrlFor(project: string, path: string): string {
  const suffix = path.startsWith('/') ? path : `/${path}`
  return `/projects/${encodeURIComponent(project)}${suffix}`
}

// dirName returns the last path segment, or '' when unknown.
export function dirName(path?: string | null): string {
  if (!path) return ''
  const parts = path.split('/').filter(Boolean)
  return parts[parts.length - 1] ?? ''
}

export function appUrl(path: string): string {
  return getBasePath() + path
}

export function encodePath(path: string): string {
  return path
    .split('/')
    .map((seg) => encodeURIComponent(seg))
    .join('/')
}

export function filesUrl(resolved: string): string {
  return appUrl(projectUrl(`/dashboard/files/${encodePath(resolved)}`))
}

// rawFileUrl points at the API endpoint that streams a project file as raw
// bytes, so <img> tags in markdown can load it without the X-Project header.
export function rawFileUrl(resolved: string): string {
  const params = new URLSearchParams({ path: resolved })
  const project = getProject()
  if (project) params.set('project', project)
  return appUrl(`/api/files/raw?${params.toString()}`)
}

// terminalWsUrl points at the WebSocket endpoint that hosts a shell for the
// current project. The project is passed as a query parameter because the
// X-Project header cannot be set on a WebSocket handshake. A persistent
// session id lets a reconnect resume the same server-side shell.
export function terminalWsUrl(command?: string, session?: string): string {
  const base = getBasePath()
  const project = getProject()
  const protocol = window.location.protocol === 'https:' ? 'wss' : 'ws'
  const params = new URLSearchParams()
  if (project) params.set('project', project)
  if (command) params.set('command', command)
  if (session) params.set('session', session)
  const query = params.toString()
  return `${protocol}://${window.location.host}${base}/api/terminal${query ? `?${query}` : ''}`
}

// taskIdOf extracts the task id from a task graph node id, which is the task
// file path without the extension (e.g. "tasks/20260811_x/task" -> "20260811_x"
// and "tasks/adhoc/20260902_review-pr" -> "20260902_review-pr").
export function taskIdOf(nodeId: string): string {
  const parts = nodeId.split('/')
  if (parts[parts.length - 2] === 'adhoc') {
    return parts[parts.length - 1] ?? nodeId
  }
  return parts[parts.length - 2] ?? nodeId
}

const TAB_STORAGE_KEY = 'mybox:project-tabs'

function getProjectTabs(): Record<string, string> {
  try {
    const raw = localStorage.getItem(TAB_STORAGE_KEY)
    return raw ? JSON.parse(raw) : {}
  } catch {
    return {}
  }
}

function saveProjectTab(project: string, tabPath: string) {
  const tabs = getProjectTabs()
  tabs[project] = tabPath
  localStorage.setItem(TAB_STORAGE_KEY, JSON.stringify(tabs))
}

// rememberCurrentTab saves the current tab path for the current project.
export function rememberCurrentTab() {
  const currentProject = getProject()
  if (!currentProject) return
  const base = getBasePath()
  const currentPath = base ? window.location.pathname.slice(base.length) : window.location.pathname
  const projectPrefix = `/projects/${encodeURIComponent(currentProject)}`
  if (currentPath.startsWith(projectPrefix)) {
    const tabPath = currentPath.slice(projectPrefix.length) || '/dashboard'
    saveProjectTab(currentProject, tabPath)
  }
}

export function setProject(project: string, navigate: (to: string) => void) {
  const currentProject = getProject()
  // Save the current project's tab before switching
  if (currentProject) rememberCurrentTab()

  // Restore the saved tab for the target project, or fall back to /dashboard
  const tabs = getProjectTabs()
  const tabPath = tabs[project] || '/dashboard'
  navigate(`/projects/${encodeURIComponent(project)}${tabPath}`)
}

export function clearProject(navigate: (to: string) => void) {
  navigate('/projects')
}
