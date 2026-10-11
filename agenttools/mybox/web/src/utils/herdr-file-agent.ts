// herdr agent names must match [a-z][a-z0-9_-]{0,31} (lowercase letters,
// digits, hyphens, underscores). ASCII task directories keep their existing
// folded names; directories containing non-ASCII characters get a deterministic
// hash suffix. This mirrors herdrTaskAgentName in internal/entrypoint/herdr.go.

const TASK_DIR_MAX_LENGTH = 80

function validTaskDirectoryName(name: string): boolean {
  if (!name || name === '.' || name === '..' || Array.from(name).length > TASK_DIR_MAX_LENGTH) {
    return false
  }
  return !/[\s/\\]/u.test(name) && !Array.from(name).some((ch) => {
    const code = ch.codePointAt(0) ?? 0
    return code <= 0x1f || code === 0x7f
  })
}

// taskDirFromPath returns the task directory name for a project-relative path
// inside _tasks/<dir>/..., or null when the path is not in a task directory.
// Only such files can start a herdr agent.
export function taskDirFromPath(path: string): string | null {
  const parts = path.trim().split('/').filter(Boolean)
	if (parts.length < 2 || parts[0] !== '_tasks') return null
  if (!validTaskDirectoryName(parts[1])) return null
  if (parts.slice(2).some((p) => p === '' || p === '.' || p === '..')) return null
  return parts[1]
}

function slugAgentName(name: string): string {
  const normalized = name.trim().toLowerCase()
  let out = ''
  let prevDash = false
  let nonASCII = false
  for (const ch of normalized) {
    if ((ch.codePointAt(0) ?? 0) > 0x7f) nonASCII = true
    if ((ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9')) {
      out += ch
      prevDash = false
    } else if (ch === '_') {
      out += ch
      prevDash = false
    } else if (!prevDash && out.length > 0) {
      out += '-'
      prevDash = true
    }
  }
  let slug = out.replace(/^-+|-+$/g, '')
  if (slug === '') {
    if (!nonASCII) return ''
    slug = 'task'
  }
  if (slug.charCodeAt(0) < 97 || slug.charCodeAt(0) > 122) slug = 'f' + slug
  if (nonASCII) {
    const suffix = fnv1a64(normalized)
    const maxBaseLength = 32 - suffix.length - 1
    if (slug.length > maxBaseLength) slug = slug.slice(0, maxBaseLength).replace(/-+$/, '')
    slug += `-${suffix}`
  } else if (slug.length > 32) {
    slug = slug.slice(0, 32).replace(/-+$/, '')
  }
  return slug
}

function fnv1a64(value: string): string {
  let hash = 0xcbf29ce484222325n
  for (const byte of new TextEncoder().encode(value)) {
    hash ^= BigInt(byte)
    hash = BigInt.asUintN(64, hash * 0x100000001b3n)
  }
  return hash.toString(16).padStart(16, '0')
}

// taskAgentName derives the herdr agent name for a task file path. The task
// directory name (_tasks/<dir>/...) is slugified as in herdrTaskAgentName, so
// every file in the same task directory shares one agent. Returns '' when the
// path is not inside a task directory.
export function taskAgentName(path: string): string {
  const dir = taskDirFromPath(path)
  if (dir === null) return ''
  return slugAgentName(dir)
}

// filePathForAgent finds the path whose derived agent name matches the given
// herdr agent name. Each agent belongs to a task directory (_tasks/<dir>/...);
// the canonical task.md under that directory is preferred, otherwise the
// shallowest file in the directory is the link target.
export function filePathForAgent(
  files: ReadonlyArray<{ path: string }>,
  agentName: string,
): string | null {
  let best: string | null = null
  let bestScore = Infinity
  for (const f of files) {
    if (taskAgentName(f.path) !== agentName) continue
    const depth = f.path.split('/').filter(Boolean).length
    const score = f.path.endsWith('/task.md') ? depth - 1 : depth
    if (score < bestScore) {
      best = f.path
      bestScore = score
    }
  }
  return best
}

/**
 * Resolves the canonical task file for a file agent. Newer API responses carry
 * the stable task ID explicitly because the tab label is the user-facing task
 * title and is no longer the task directory name. The label fallback keeps
 * older herdr responses linkable during an upgrade.
 */
export function taskFilePathForAgent(
  agent: Readonly<{ name: string; pane_id: string; task_id?: string | null }>,
  panes: ReadonlyArray<Readonly<{ pane_id: string; tab_id: string }>>,
  tabs: ReadonlyArray<Readonly<{ tab_id: string; label: string }>>,
): string | null {
  const pane = panes.find((candidate) => candidate.pane_id === agent.pane_id)
  if (!pane) return null
  const tab = tabs.find((candidate) => candidate.tab_id === pane.tab_id)

  const taskID = agent.task_id?.trim()
  if (taskID) {
    const path = `_tasks/${taskID}/task.md`
    return taskDirFromPath(path) === taskID && taskAgentName(path) === agent.name ? path : null
  }

  const label = tab?.label?.trim()
  if (!label) return null
  const path = `_tasks/${label}/task.md`
  return taskAgentName(path) === agent.name ? path : null
}

export function taskDirectoryForFilePath(filePath: string): string {
  const slash = filePath.lastIndexOf('/')
  return slash < 0 ? '' : filePath.slice(0, slash)
}

export function hasTaskFile(
  entries: ReadonlyArray<Readonly<{ path: string; kind: 'file' | 'dir' }>>,
  filePath: string,
): boolean {
  return entries.some((entry) => entry.path === filePath && entry.kind === 'file')
}

export interface LinkedTaskAgent {
  agent: Readonly<{
    name: string
    pane_id: string
    custom_name?: string | null
    task_id?: string | null
    task_title?: string | null
  }>
  pane: Readonly<{ pane_id: string; tab_id: string }>
  tab: Readonly<{ tab_id: string; label: string }>
  taskDirectory: string
  taskFile: string
}

/** Returns the project-relative task directory for a valid file-agent label. */
export function taskDirectoryPathForLabel(label: string): string | null {
  const trimmed = label.trim()
  if (!trimmed) return null
  const taskFile = `_tasks/${trimmed}/task.md`
  return taskDirFromPath(taskFile) === trimmed ? `_tasks/${trimmed}` : null
}

export function linkedTaskAgentForTab(
  tabId: string,
  agents: ReadonlyArray<Readonly<{
    name: string
    pane_id: string
    custom_name?: string | null
    task_id?: string | null
    task_title?: string | null
  }>>,
  panes: ReadonlyArray<Readonly<{ pane_id: string; tab_id: string }>>,
  tabs: ReadonlyArray<Readonly<{ tab_id: string; label: string }>>,
): LinkedTaskAgent | null {
  const tab = tabs.find((candidate) => candidate.tab_id === tabId)
  const tabPanes = panes.filter((candidate) => candidate.tab_id === tabId)
  if (!tab) return null
  for (const pane of tabPanes) {
    const agent = agents.find((candidate) => candidate.pane_id === pane.pane_id && (candidate.custom_name || candidate.task_id))
    if (!agent) continue
    const taskFile = taskFilePathForAgent(agent, panes, tabs)
    if (!taskFile) continue
    return {
      agent,
      pane,
      tab,
      taskDirectory: taskFile.slice(0, -'/task.md'.length),
      taskFile,
    }
  }
  return null
}

export function linkedTaskAgentForPath(
  path: string,
  agents: ReadonlyArray<Readonly<{
    name: string
    pane_id: string
    custom_name?: string | null
    task_id?: string | null
    task_title?: string | null
  }>>,
  panes: ReadonlyArray<Readonly<{ pane_id: string; tab_id: string }>>,
  tabs: ReadonlyArray<Readonly<{ tab_id: string; label: string }>>,
): LinkedTaskAgent | null {
  for (const agent of agents) {
    const link = agent.custom_name || agent.task_id ? linkedTaskAgentForTabForPane(agent, panes, tabs) : null
    if (link && (path === link.taskDirectory || path.startsWith(`${link.taskDirectory}/`))) return link
  }
  return null
}

export interface LinkedTaskRename {
  taskId: string
  oldTaskDirectory: string
  newTaskDirectory: string
  oldAgentName: string
  newAgentName: string
  label: string
}

function validTaskTabLabel(label: string): boolean {
  const trimmed = label.trim()
  if (!trimmed || Array.from(trimmed).length > TASK_DIR_MAX_LENGTH) {
    return false
  }
  return !Array.from(trimmed).some((ch) => {
    const code = ch.codePointAt(0) ?? 0
    return code <= 0x1f || code === 0x7f
  })
}

export function linkedTaskRenameForLabel(link: LinkedTaskAgent, label: string): LinkedTaskRename | null {
  const normalizedLabel = label.trim()
  if (!validTaskTabLabel(normalizedLabel)) return null
  const taskId = link.agent.task_id?.trim() || link.taskDirectory.slice('_tasks/'.length)
  if (!taskId || taskDirFromPath(`_tasks/${taskId}/task.md`) !== taskId) return null
  return {
    taskId,
    oldTaskDirectory: link.taskDirectory,
    newTaskDirectory: link.taskDirectory,
    oldAgentName: link.agent.name,
    newAgentName: link.agent.name,
    label: normalizedLabel,
  }
}

export function linkedTaskRenameForPathMove(link: LinkedTaskAgent, newPath: string): LinkedTaskRename | null {
  const dir = taskDirFromPath(newPath)
  if (!dir) return null
  const newTaskDirectory = `_tasks/${dir}`
  const newAgentName = taskAgentName(`${newTaskDirectory}/task.md`)
  const label = link.tab.label.trim()
  if (!newAgentName || !validTaskTabLabel(label)) return null
  return {
    taskId: dir,
    oldTaskDirectory: link.taskDirectory,
    newTaskDirectory,
    oldAgentName: link.agent.name,
    newAgentName,
    label,
  }
}

function linkedTaskAgentForTabForPane(
  agent: Readonly<{
    name: string
    pane_id: string
    custom_name?: string | null
    task_id?: string | null
    task_title?: string | null
  }>,
  panes: ReadonlyArray<Readonly<{ pane_id: string; tab_id: string }>>,
  tabs: ReadonlyArray<Readonly<{ tab_id: string; label: string }>>,
): LinkedTaskAgent | null {
  const pane = panes.find((candidate) => candidate.pane_id === agent.pane_id)
  if (!pane) return null
  return linkedTaskAgentForTab(pane.tab_id, [agent], panes, tabs)
}
