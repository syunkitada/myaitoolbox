import { getBasePath, getProject } from '../utils/routes'
export { getBasePath, getProject, setProject, clearProject } from '../utils/routes'

export type TaskStatus = 'todo' | 'doing' | 'blocked' | 'review' | 'done'
export type TaskPriority = 'low' | 'medium' | 'high' | 'urgent'
export type TaskType = 'regular' | 'adhoc'

export interface Task {
  id: string
  title: string
  status: TaskStatus
  priority: TaskPriority
  type?: TaskType | null
  assignee?: string | null
  due?: string | null
  pending_until?: string | null
  pending_reason?: string | null
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
  type?: TaskType
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

export interface ProjectGitStatus {
  dirty: boolean
  modified: number
  staged: number
  untracked: number
}

export type GitFileStatus = 'staged' | 'unstaged' | 'untracked'

export interface GitFile {
  path: string
  status: GitFileStatus
  code: string
  diff: string
}

export interface GitDetail {
  is_repo: boolean
  branch: string
  remote: string
  ahead: number
  behind: number
  last_commit_message: string
  staged: GitFile[]
  unstaged: GitFile[]
  untracked: GitFile[]
}

export interface GitResult {
  ok: boolean
  output?: string
}

export interface HerdrWorkspace {
  workspace_id: string
  label: string
  number?: number | null
  agent_status: string
  focused?: boolean | null
  tab_count?: number | null
  pane_count?: number | null
}

export interface HerdrAgent {
  name: string
  custom_name?: string | null
  status: string
  workspace_id: string
  cwd?: string | null
  title?: string | null
  focused?: boolean | null
  pane_id: string
}

export interface HerdrTab {
  tab_id: string
  workspace_id: string
  label: string
  number?: number | null
  agent_status?: string | null
  focused?: boolean | null
  pane_count?: number | null
}

export interface HerdrPane {
  pane_id: string
  tab_id: string
  workspace_id: string
  cwd?: string | null
  agent_status?: string | null
  title?: string | null
  focused?: boolean | null
}

export interface HerdrOverview {
  available: boolean
  workspaces: HerdrWorkspace[]
  agents: HerdrAgent[]
  tabs: HerdrTab[]
  panes: HerdrPane[]
}

export interface StatsCPU {
  id: number
  model: string
  usage_percent: number
}

export interface StatsMemory {
  total: number
  used: number
  available: number
  usage_percent: number
}

export interface StatsDisk {
  device: string
  mount_point: string
  fs_type: string
  total: number
  used: number
  available: number
  usage_percent: number
}

export interface StatsNet {
  name: string
  rx_bytes: number
  tx_bytes: number
  rx_packets: number
  tx_packets: number
  state: string
}

export interface StatsProcess {
  pid: number
  user: string
  cpu_percent: number
  mem_percent: number
  rss_bytes: number
  vms_bytes: number
  command: string
}

export interface Stats {
  hostname: string
  os: string
  uptime_seconds: number
  load_avg: [number, number, number]
  cpu_cores: number
  cpu: StatsCPU[]
  memory: StatsMemory
  swap: StatsMemory
  disks: StatsDisk[]
  network: StatsNet[]
  processes: StatsProcess[]
  processes_by_cpu: StatsProcess[]
  collected_at: string
}

export class ApiError extends Error {
  status: number

  constructor(status: number, message: string) {
    super(message)
    this.status = status
  }
}

async function request<T>(method: string, url: string, body?: unknown): Promise<T> {
  const init: RequestInit = { method, headers: {}, cache: 'no-store' }
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

  getProjectGitStatus: () =>
    request<Record<string, ProjectGitStatus>>('GET', '/api/projects/git-status'),

  setFavorite: (path: string, enabled: boolean) =>
    request<void>('PUT', '/api/meta/favorites', { path, enabled }),

  recordRecent: (path: string) => request<void>('POST', '/api/meta/recent', { path }),

  search: (q: string, type?: 'task' | 'knowledge') =>
    request<SearchResult[]>('GET', '/api/search' + qs({ q, type })),

  listTasks: (params: { status?: TaskStatus; tag?: string; type?: TaskType; all?: boolean } = {}) =>
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

  listFiles: () => request<FileEntry[]>('GET', '/api/files'),
  getFileGitStatus: () => request<Record<string, string>>('GET', '/api/files/git-status'),
  createFile: (path: string) => request<void>('POST', '/api/files', { path }),
  getFileContent: (path: string) =>
    request<FileContent>('GET', '/api/files/content' + qs({ path })),

  getHerdrOverview: () => request<HerdrOverview>('GET', '/api/herdr/overview'),

  readHerdrAgent: (target: string) =>
    request<{ output: string }>('POST', '/api/herdr/agents/read', { target }),

  promptHerdrAgent: (target: string, text: string) =>
    request<{ ok: boolean }>('POST', '/api/herdr/agents/prompt', { target, text }),

  sendKeysHerdrAgent: (target: string, keys: string[]) =>
    request<{ ok: boolean }>('POST', '/api/herdr/agents/send-keys', { target, keys }),

  renameHerdrAgent: (target: string, name?: string, clear?: boolean) =>
    request<{ ok: boolean }>('POST', '/api/herdr/agents/rename', {
      target,
      name: name || undefined,
      clear: clear || undefined,
    }),

  createHerdrTab: (workspaceId?: string, label?: string, cwd?: string, project?: string) =>
    request<{ ok: boolean }>('POST', '/api/herdr/tabs/create', {
      workspace_id: workspaceId,
      label,
      cwd,
      project,
    }),

  renameHerdrTab: (tabId: string, label: string) =>
    request<{ ok: boolean }>('POST', '/api/herdr/tabs/rename', { tab_id: tabId, label }),

  closeHerdrTab: (tabId: string) =>
    request<{ ok: boolean }>('POST', '/api/herdr/tabs/close', { tab_id: tabId }),

  splitHerdrPane: (paneId: string, direction: 'right' | 'down') =>
    request<{ ok: boolean }>('POST', '/api/herdr/panes/split', {
      pane_id: paneId,
      direction,
    }),

  renameHerdrPane: (paneId: string, label: string) =>
    request<{ ok: boolean }>('POST', '/api/herdr/panes/rename', { pane_id: paneId, label }),

  readHerdrPane: (paneId: string) =>
    request<{ output: string }>('POST', '/api/herdr/panes/read', { target: paneId }),

  closeHerdrPane: (paneId: string) =>
    request<{ ok: boolean }>('POST', '/api/herdr/panes/close', { pane_id: paneId }),

  sendTextHerdrPane: (paneId: string, text: string) =>
    request<{ ok: boolean }>('POST', '/api/herdr/panes/send-text', { pane_id: paneId, text }),

  sendKeysHerdrPane: (paneId: string, keys: string[]) =>
    request<{ ok: boolean }>('POST', '/api/herdr/panes/send-keys', { pane_id: paneId, keys }),

  saveFileContent: (path: string, content: string) =>
    request<void>('PUT', '/api/files/content', { path, content }),

  moveFile: (oldPath: string, newPath: string) =>
    request<void>('POST', '/api/files/move', { old_path: oldPath, new_path: newPath }),

  copyFile: (oldPath: string, newPath: string) =>
    request<void>('POST', '/api/files/copy', { old_path: oldPath, new_path: newPath }),

  deleteFile: (path: string) => request<void>('POST', '/api/files/delete', { path }),
  destroyTerminal: (session: string) =>
    request<void>('DELETE', '/api/terminal/destroy' + qs({ session })),

  getGitStatus: () => request<GitDetail>('GET', '/api/git/status'),
  gitInit: () => request<GitResult>('POST', '/api/git/init'),
  gitCommit: (message: string, stagedOnly?: boolean, amend?: boolean) =>
    request<GitResult>('POST', '/api/git/commit', {
      message,
      staged_only: stagedOnly ?? false,
      amend: amend ?? false,
    }),
  gitPull: () => request<GitResult>('POST', '/api/git/pull'),
  gitPush: () => request<GitResult>('POST', '/api/git/push'),
  gitStage: (paths: string[]) => request<GitResult>('POST', '/api/git/stage', { paths }),
  gitUnstage: (paths: string[]) => request<GitResult>('POST', '/api/git/unstage', { paths }),
  gitDiscard: (paths: string[]) => request<GitResult>('POST', '/api/git/discard', { paths }),

  getGraph: (path?: string) =>
    request<GraphData>('GET', '/api/graph' + qs({ path })),

  getStats: () => request<Stats>('GET', '/api/stats'),
}
