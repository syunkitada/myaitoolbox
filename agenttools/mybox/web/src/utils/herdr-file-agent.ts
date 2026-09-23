// herdr agent names must match [a-z][a-z0-9_-]{0,31} (lowercase letters,
// digits, hyphens, underscores) while task directory names can carry digits,
// uppercase, and spaces, so the task directory name is lowercased and every
// other character is folded into a single hyphen. Mirrors the Go helper
// herdrTaskAgentName in internal/entrypoint/herdr.go.

// taskDirFromPath returns the task directory name for a project-relative path
// inside tasks/<dir>/..., or null when the path is not in a task directory.
// Only such files can start a herdr agent.
export function taskDirFromPath(path: string): string | null {
  const parts = path.trim().split('/').filter(Boolean)
  if (parts.length < 2 || parts[0] !== 'tasks') return null
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
// directory name (tasks/<dir>/...) is slugified as in herdrTaskAgentName, so
// every file in the same task directory shares one agent. Returns '' when the
// path is not inside a task directory.
export function taskAgentName(path: string): string {
  const dir = taskDirFromPath(path)
  if (dir === null) return ''
  return slugAgentName(dir)
}

// filePathForAgent finds the path whose derived agent name matches the given
// herdr agent name. Each agent belongs to a task directory (tasks/<dir>/...);
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