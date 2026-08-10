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
  const suffix = path.startsWith('/') ? path : `/${path}`
  return `/projects/${encodeURIComponent(project)}${suffix}`
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

export function knowledgeUrl(resolved: string): string {
  return appUrl(projectUrl(`/knowledge/${encodePath(resolved)}`))
}

export function setProject(project: string) {
  window.location.href =
    getBasePath() + '/projects/' + encodeURIComponent(project) + '/dashboard'
}

export function clearProject() {
  window.location.href = getBasePath() + '/projects'
}
