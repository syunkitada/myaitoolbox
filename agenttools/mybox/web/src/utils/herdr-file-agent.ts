// herdr agent names must match [a-z][a-z0-9_-]{0,31} (lowercase letters,
// digits, hyphens, underscores) while task directory names can carry digits,
// uppercase, and spaces, so the task directory name is lowercased and every
// other character is folded into a single hyphen. Mirrors the Go helper
// herdrTaskAgentName in internal/entrypoint/herdr.go.

// taskDirFromPath returns the task directory name for a project-relative path
// inside _tasks/<dir>/..., or null when the path is not in a task directory.
// Only such files can start a herdr agent.
export function taskDirFromPath(path: string): string | null {
  const parts = path.trim().split('/').filter(Boolean)
	if (parts.length < 2 || parts[0] !== '_tasks') return null
  if (!/^[A-Za-z0-9][A-Za-z0-9_.:-]{0,63}$/.test(parts[1])) return null
  if (parts.slice(2).some((p) => p === '' || p === '.' || p === '..')) return null
  return parts[1]
}

function slugAgentName(name: string): string {
  let out = ''
  let prevDash = false
  for (const ch of name.trim().toLowerCase()) {
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
  if (slug === '') return ''
  if (slug.charCodeAt(0) < 97 || slug.charCodeAt(0) > 122) slug = 'f' + slug
  if (slug.length > 32) slug = slug.slice(0, 32).replace(/-+$/, '')
  return slug
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
 * Resolves the canonical task file from the tab label used by a file agent.
 * File-agent tabs retain the original task directory name, while the agent
 * name is the slug derived from that directory. This avoids scanning files to
 * reverse the slug.
 */
export function taskFilePathForAgent(
  agent: Readonly<{ name: string; pane_id: string }>,
  panes: ReadonlyArray<Readonly<{ pane_id: string; tab_id: string }>>,
  tabs: ReadonlyArray<Readonly<{ tab_id: string; label: string }>>,
): string | null {
  const pane = panes.find((candidate) => candidate.pane_id === agent.pane_id)
  if (!pane) return null
  const tab = tabs.find((candidate) => candidate.tab_id === pane.tab_id)
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
  agent: Readonly<{ name: string; pane_id: string; custom_name?: string | null }>
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
  agents: ReadonlyArray<Readonly<{ name: string; pane_id: string; custom_name?: string | null }>>,
  panes: ReadonlyArray<Readonly<{ pane_id: string; tab_id: string }>>,
  tabs: ReadonlyArray<Readonly<{ tab_id: string; label: string }>>,
): LinkedTaskAgent | null {
  const tab = tabs.find((candidate) => candidate.tab_id === tabId)
  const tabPanes = panes.filter((candidate) => candidate.tab_id === tabId)
  if (!tab) return null
  for (const pane of tabPanes) {
    const agent = agents.find((candidate) => candidate.pane_id === pane.pane_id && candidate.custom_name)
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
  agents: ReadonlyArray<Readonly<{ name: string; pane_id: string; custom_name?: string | null }>>,
  panes: ReadonlyArray<Readonly<{ pane_id: string; tab_id: string }>>,
  tabs: ReadonlyArray<Readonly<{ tab_id: string; label: string }>>,
): LinkedTaskAgent | null {
  for (const agent of agents) {
    const link = agent.custom_name ? linkedTaskAgentForTabForPane(agent, panes, tabs) : null
    if (link && (path === link.taskDirectory || path.startsWith(`${link.taskDirectory}/`))) return link
  }
  return null
}

export interface LinkedTaskRename {
  oldTaskDirectory: string
  newTaskDirectory: string
  oldAgentName: string
  newAgentName: string
  label: string
}

export function linkedTaskRenameForLabel(link: LinkedTaskAgent, label: string): LinkedTaskRename | null {
  const newTaskDirectory = taskDirectoryPathForLabel(label)
  if (!newTaskDirectory) return null
  const normalizedLabel = newTaskDirectory.slice('_tasks/'.length)
  const newAgentName = taskAgentName(`${newTaskDirectory}/task.md`)
  if (!newAgentName) return null
  return {
    oldTaskDirectory: link.taskDirectory,
    newTaskDirectory,
    oldAgentName: link.agent.name,
    newAgentName,
    label: normalizedLabel,
  }
}

export function linkedTaskRenameForPathMove(link: LinkedTaskAgent, newPath: string): LinkedTaskRename | null {
  const dir = taskDirFromPath(newPath)
  if (!dir) return null
  return linkedTaskRenameForLabel(link, dir)
}

function linkedTaskAgentForTabForPane(
  agent: Readonly<{ name: string; pane_id: string; custom_name?: string | null }>,
  panes: ReadonlyArray<Readonly<{ pane_id: string; tab_id: string }>>,
  tabs: ReadonlyArray<Readonly<{ tab_id: string; label: string }>>,
): LinkedTaskAgent | null {
  const pane = panes.find((candidate) => candidate.pane_id === agent.pane_id)
  if (!pane) return null
  return linkedTaskAgentForTab(pane.tab_id, [agent], panes, tabs)
}
