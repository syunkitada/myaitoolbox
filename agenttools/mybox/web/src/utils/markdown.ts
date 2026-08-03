const wikiLinkPattern = /\[\[([^\]|]+)(?:\|([^\]]*))?\]\]/g

export interface WikiLink {
  target: string
  alias: string | null
  raw: string
}

export function extractWikiLinks(text: string): WikiLink[] {
  const links: WikiLink[] = []
  for (const m of text.matchAll(wikiLinkPattern)) {
    const target = m[1].trim()
    links.push({
      target,
      alias: m[2] ? m[2].trim() : null,
      raw: m[0],
    })
  }
  return links
}

export function resolveWikiPath(target: string): string {
  const cleaned = target.endsWith('.md') ? target.slice(0, -3) : target
  return cleaned.replace(/\.md$/, '')
}

export function normalizePath(path: string): string {
  return path.toLowerCase().replace(/\.md$/, '')
}

export function renderWikiLinks(text: string, pathOf: (target: string) => string | null): string {
  return text.replace(wikiLinkPattern, (_raw, targetRaw: string, aliasRaw?: string) => {
    const target = targetRaw.trim()
    const alias = aliasRaw ? aliasRaw.trim() : null
    const resolved = pathOf(resolveWikiPath(target))
    const label = alias ?? target
    if (resolved) {
      return `[${label}](#/knowledge/${encodeURIComponent(resolved)})`
    }
    return label
  })
}

export function resolveMarkdownLink(target: string, relativeTo?: string | null): string | null {
  if (!target || target.startsWith('#') || target.startsWith('/')) return null
  if (/^[a-z][a-z0-9+.-]*:/i.test(target)) return null
  const clean = target.split(/[?#]/)[0]
  if (!/\.md$/i.test(clean)) return null
  const stack = (relativeTo ?? '').split('/').filter(Boolean)
  stack.pop()
  for (const part of clean.split('/')) {
    if (part === '' || part === '.') continue
    if (part === '..') {
      stack.pop()
    } else {
      stack.push(part)
    }
  }
  const resolved = stack.join('/').replace(/\.md$/i, '')
  return resolved || null
}
