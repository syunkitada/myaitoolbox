import { getBasePath, getProject } from '../utils/routes'
export { getBasePath, getProject, setProject, clearProject } from '../utils/routes'

export type TaskStatus = 'todo' | 'doing' | 'blocked' | 'review' | 'done'
export type TaskPriority = 'low' | 'medium' | 'high' | 'urgent'

export interface Task {
  id: string
  title: string
  status: TaskStatus
  priority: TaskPriority
  assignee?: string | null
  due?: string | null
  tags?: string[] | null
  project?: string | null
  created?: string | null
  body?: string | null
  archived?: boolean | null
}

export interface CreateTaskRequest {
  name: string
  status?: TaskStatus
  priority?: TaskPriority
  assignee?: string
  due?: string
  tags?: string[]
}

export type UpdateTaskRequest = Partial<CreateTaskRequest>

export interface Knowledge {
  path: string
  title: string
  tags?: string[] | null
  aliases?: string[] | null
  type?: string | null
  created?: string | null
  lastmod?: string | null
  wiki_links?: string[] | null
  body?: string | null
}

export interface KnowledgeContent {
  path: string
  content: string
}

export type FileKind = 'file' | 'dir'

export interface FileEntry {
  path: string
  name: string
  kind: FileKind
  status?: string
}

export interface FileContent {
  path: string
  content: string
}

export interface SearchResult {
  type: 'task' | 'knowledge'
  id?: string | null
  path: string
  title: string
  snippet?: string | null
}

export interface GraphNode {
  id: string
  label: string
  type?: string | null
}

export interface GraphLink {
  source: string
  target: string
}

export interface GraphData {
  nodes: GraphNode[]
  links: GraphLink[]
}

export interface Meta {
  project: string
  projects: string[]
  default_project: string
  tags: string[]
  favorites: string[]
  recent_files: string[]
}

export interface Project {
  name: string
  path: string
}

export class ApiError extends Error {
  status: number

  constructor(status: number, message: string) {
    super(message)
    this.status = status
  }
}

async function request<T>(method: string, url: string, body?: unknown): Promise<T> {
  const init: RequestInit = { method, headers: {} }
  const currentProject = getProject()
  if (currentProject) {
    (init.headers as Record<string, string>)['X-Project'] = currentProject
  }
  if (body !== undefined) {
    (init.headers as Record<string, string>)['Content-Type'] = 'application/json'
    init.body = JSON.stringify(body)
  }
  const res = await fetch(getBasePath() + url, init)
  if (!res.ok) {
    let message = res.statusText
    try {
      const data = (await res.json()) as { error?: string }
      if (data.error) message = data.error
    } catch {
      /* ignore non-json errors */
    }
    throw new ApiError(res.status, message)
  }
  if (res.status === 204) return undefined as T
  return (await res.json()) as T
}

function qs(params: Record<string, string | number | boolean | undefined | null>): string {
  const entries = Object.entries(params).filter(
    ([, v]) => v !== undefined && v !== null && v !== '',
  )
  if (entries.length === 0) return ''
  return '?' + new URLSearchParams(entries.map(([k, v]) => [k, String(v)])).toString()
}

export const api = {
  getMeta: () => request<Meta>('GET', '/api/meta'),

  listProjects: () => request<Project[]>('GET', '/api/projects'),

  createProject: (path: string) => request<Project>('POST', '/api/projects', { path }),

  deleteProject: (name: string) =>
    request<void>('DELETE', `/api/projects/${encodeURIComponent(name)}`),

  getProjectPaths: (prefix: string) =>
    request<string[]>('GET', '/api/projects/paths' + qs({ prefix })),

  setFavorite: (path: string, enabled: boolean) =>
    request<void>('PUT', '/api/meta/favorites', { path, enabled }),

  recordRecent: (path: string) => request<void>('POST', '/api/meta/recent', { path }),

  search: (q: string, type?: 'task' | 'knowledge') =>
    request<SearchResult[]>('GET', '/api/search' + qs({ q, type })),

  listTasks: (params: { status?: TaskStatus; tag?: string; all?: boolean } = {}) =>
    request<Task[]>('GET', '/api/tasks' + qs(params)),

  getTask: (id: string) => request<Task>('GET', `/api/tasks/${encodeURIComponent(id)}`),

  createTask: (req: CreateTaskRequest) =>
    request<Task>('POST', '/api/tasks', req),

  updateTask: (id: string, req: UpdateTaskRequest) =>
    request<Task>('PATCH', `/api/tasks/${encodeURIComponent(id)}`, req),

  archiveTask: (id: string) =>
    request<void>('POST', `/api/tasks/${encodeURIComponent(id)}/archive`),

  listKnowledge: (params: { path?: string; tag?: string } = {}) =>
    request<Knowledge[]>('GET', '/api/knowledge' + qs(params)),

  createKnowledge: (path: string) =>
    request<Knowledge>('POST', '/api/knowledge', { path }),

  getKnowledgeContent: (path: string) =>
    request<KnowledgeContent>('GET', '/api/knowledge/content' + qs({ path })),

  saveKnowledgeContent: (path: string, content: string) =>
    request<void>('PUT', '/api/knowledge/content', { path, content }),

  moveKnowledge: (oldPath: string, newPath: string) =>
    request<void>('POST', '/api/knowledge/move', { old_path: oldPath, new_path: newPath }),

  renameKnowledge: (oldPath: string, newName: string) =>
    request<void>('POST', '/api/knowledge/rename', { old_path: oldPath, new_name: newName }),

  listFiles: () => request<FileEntry[]>('GET', '/api/files'),

  getFileContent: (path: string) =>
    request<FileContent>('GET', '/api/files/content' + qs({ path })),

  saveFileContent: (path: string, content: string) =>
    request<void>('PUT', '/api/files/content', { path, content }),

  moveFile: (oldPath: string, newPath: string) =>
    request<void>('POST', '/api/files/move', { old_path: oldPath, new_path: newPath }),

  copyFile: (oldPath: string, newPath: string) =>
    request<void>('POST', '/api/files/copy', { old_path: oldPath, new_path: newPath }),

  deleteFile: (path: string) => request<void>('POST', '/api/files/delete', { path }),

  getGraph: (path?: string) =>
    request<GraphData>('GET', '/api/graph' + qs({ path })),
}
