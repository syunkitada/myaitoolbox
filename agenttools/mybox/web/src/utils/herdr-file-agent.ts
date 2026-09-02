// herdr agent names must match [a-z][a-z0-9_-]{0,31} while filenames also
// carry dots, uppercase, and spaces, so the basename is lowercased and every
// other character is folded into a single hyphen. Mirrors the Go helper
// herdrFileAgentName in internal/entrypoint/herdr.go.

export function fileAgentName(filename: string): string {
  let slug = filename.trim().toLowerCase()
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