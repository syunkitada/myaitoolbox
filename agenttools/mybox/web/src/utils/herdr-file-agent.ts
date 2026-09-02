// herdr agent names must match [a-z][a-z0-9_-]{0,31} while filenames also
// carry dots, uppercase, and spaces, so the path is lowercased and every
// other character is folded into a single hyphen. Mirrors the Go helper
// herdrFileAgentName in internal/entrypoint/herdr.go.

export function fileAgentName(path: string): string {
  const parts = path.trim().split('/').filter(Boolean)
  const suffix = parts.slice(-2).join('-')
  let slug = (suffix || path).trim().toLowerCase()
  let out = ''
  let prevDash = false
  for (const ch of slug) {
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
  slug = out.replace(/^-+|-+$/g, '')
  if (slug === '') return ''
  if (slug.charCodeAt(0) < 97 || slug.charCodeAt(0) > 122) slug = 'f' + slug
  if (slug.length > 32) slug = slug.slice(0, 32).replace(/-+$/, '')
  return slug
}

export function fileLabel(filename: string): string {
  return filename.trim()
}

// filePathForAgent finds the file path whose derived agent name matches the
// given herdr agent name. This inverts fileAgentName() so the Agents panel can
// link an agent back to the file it was started for. The mapping is lossy (the
// agent name only encodes the last two path segments), so when several files
// map to the same name the closest (shortest, most specific) match wins.
export function filePathForAgent(
  files: ReadonlyArray<{ path: string }>,
  agentName: string,
): string | null {
  let best: string | null = null
  let bestSegments = Infinity
  for (const f of files) {
    if (fileAgentName(f.path) !== agentName) continue
    const depth = f.path.split('/').filter(Boolean).length
    if (depth < bestSegments) {
      best = f.path
      bestSegments = depth
    }
  }
  return best
}
