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
// X-Project header cannot be set on a WebSocket handshake.
export function terminalWsUrl(command?: string): string {
  const base = getBasePath()
  const project = getProject()
  const protocol = window.location.protocol === 'https:' ? 'wss' : 'ws'
  const params = new URLSearchParams()
  if (project) params.set('project', project)
  if (command) params.set('command', command)
  const query = params.toString()
  return `${protocol}://${window.location.host}${base}/api/terminal${query ? `?${query}` : ''}`
}

// taskIdOf extracts the task id from a task graph node id, which is the task
// file path without the extension (e.g. "tasks/20260811_x/task" -> "20260811_x").
export function taskIdOf(nodeId: string): string {
  const parts = nodeId.split('/')
  return parts[parts.length - 2] ?? nodeId
}

export function setProject(project: string) {
  window.location.href =
    getBasePath() + '/projects/' + encodeURIComponent(project) + '/dashboard'
}

export function clearProject() {
  window.location.href = getBasePath() + '/projects'
}
