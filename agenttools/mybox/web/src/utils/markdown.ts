import { parse as parseYaml, stringify as stringifyYaml } from 'yaml'
import { filesUrl } from './routes'

const wikiLinkPattern = /\[\[([^\]|]+)(?:\|([^\]]*))?\]\]/g

export interface WikiLink {
  target: string
  alias: string | null
  raw: string
}

export interface FrontmatterSplit {
  frontmatter: string
  body: string
  has: boolean
}

export interface FrontmatterParse {
  ok: boolean
  data: Record<string, unknown>
}

export function splitFrontmatter(text: string): FrontmatterSplit {
  const lines = text.split('\n')
  if (lines.length < 3 || lines[0].trim() !== '---') {
    return { frontmatter: '', body: text, has: false }
  }
  let end = -1
  for (let i = 1; i < lines.length; i++) {
    if (lines[i].trim() === '---') {
      end = i
      break
    }
  }
  if (end === -1) {
    return { frontmatter: '', body: text, has: false }
  }
  const frontmatter = lines.slice(1, end).join('\n')
  const body = lines.slice(end + 1).join('\n').replace(/^\n+/, '')
  return { frontmatter, body, has: true }
}

export function parseFrontmatter(frontmatter: string): FrontmatterParse {
  if (frontmatter.trim() === '') {
    return { ok: true, data: {} }
  }
  try {
    const data = parseYaml(frontmatter)
    if (typeof data !== 'object' || data === null || Array.isArray(data)) {
      return { ok: false, data: {} }
    }
    return { ok: true, data: data as Record<string, unknown> }
  } catch {
    return { ok: false, data: {} }
  }
}

export function serializeFrontmatter(fields: Record<string, unknown>): string {
  const cleaned: Record<string, unknown> = {}
  for (const [key, value] of Object.entries(fields)) {
    if (value === undefined || value === null) continue
    if (typeof value === 'string' && value.trim() === '') continue
    if (Array.isArray(value) && value.length === 0) continue
    cleaned[key] = value
  }
  if (Object.keys(cleaned).length === 0) return ''
  return stringifyYaml(cleaned).replace(/\n+$/, '')
}

export function buildMarkdown(frontmatter: string, body: string): string {
  const trimmed = frontmatter.trim()
  if (trimmed === '') return body
  return `---\n${trimmed}\n---\n\n${body}`
}

export function extractFrontmatterTags(text: string): string[] {
  const m = /^---\r?\n([\s\S]*?)\r?\n---/.exec(text)
  if (!m) return []
  const body = m[1]
  const inline = /(?:^|\n)tags:\s*\[([^\]]*)\]/.exec(body)
  if (inline) {
    return inline[1]
      .split(',')
      .map((s) => s.trim().replace(/^["']|["']$/g, ''))
      .filter(Boolean)
  }
  const block = /(?:^|\n)tags:\s*\n((?:\s*-\s*.+\n?)+)/.exec(body)
  if (block) {
    return block[1]
      .split('\n')
      .map((s) => s.replace(/^[\s-]*/, '').trim().replace(/^["']|["']$/g, ''))
      .filter(Boolean)
  }
  return []
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

export function renderWikiLinks(
  text: string,
  pathOf: (target: string) => string | null,
  hrefOf?: (resolved: string) => string,
): string {
  return text.replace(wikiLinkPattern, (_raw, targetRaw: string, aliasRaw?: string) => {
    const target = targetRaw.trim()
    const alias = aliasRaw ? aliasRaw.trim() : null
    const resolved = pathOf(resolveWikiPath(target))
    const label = alias ?? target
    if (resolved) {
      const href = hrefOf ? hrefOf(resolved) : filesUrl(resolved)
      return `[${label}](${href})`
    }
    return label
  })
}

export interface ResolveLinkOptions {
  resolveDirectories?: boolean
  resolveAnyFile?: boolean
}

export function resolveMarkdownLink(
  target: string,
  relativeTo?: string | null,
  preserveExtension = false,
  opts: ResolveLinkOptions = {},
): string | null {
  if (!target || target.startsWith('#') || target.startsWith('/')) return null
  if (/^[a-z][a-z0-9+.-]*:/i.test(target)) return null
  const clean = target.split(/[?#]/)[0]
  const dirTarget = opts.resolveDirectories && clean.endsWith('/')
  const fileTarget = /\.md$/i.test(clean) || (opts.resolveAnyFile && /\.[a-z0-9]+$/i.test(clean))
  if (!dirTarget && !fileTarget) return null
  const isDir = (relativeTo ?? '').endsWith('/')
  const stack = (relativeTo ?? '').split('/').filter(Boolean)
  if (!isDir) stack.pop()
  for (const part of clean.split('/')) {
    if (part === '' || part === '.') continue
    if (part === '..') {
      stack.pop()
    } else {
      stack.push(part)
    }
  }
  const joined = stack.join('/')
  if (!joined) return null
  if (dirTarget) return joined
  return preserveExtension ? joined : joined.replace(/\.md$/i, '')
}

export function buildDirListing(
  dir: string,
  entries: Array<{ path: string; name: string; kind: 'file' | 'dir' }>,
): string {
  const parent = (p: string) => {
    const i = p.lastIndexOf('/')
    return i >= 0 ? p.slice(0, i) : ''
  }
  const children = entries
    .filter((e) => parent(e.path) === dir)
    .sort((a, b) => {
      if (a.kind !== b.kind) return a.kind === 'dir' ? -1 : 1
      return a.name.localeCompare(b.name)
    })
  const linkable = (name: string) => /^[^()<>\s]+$/.test(name)
  const link = (e: { name: string; kind: 'file' | 'dir' }) => {
    const label = e.kind === 'dir' ? `${e.name}/` : e.name
    if (!linkable(label)) return `- ${label}`
    return `- [${label}](${label})`
  }
  const title = dir ? dir.split('/').pop() ?? dir : 'Files'
  const lines = [`# ${title}`, '']
  const dirs = children.filter((c) => c.kind === 'dir')
  const files = children.filter((c) => c.kind === 'file')
  if (dirs.length > 0) {
    lines.push('## Directories', '')
    lines.push(...dirs.map((d) => link(d)))
    lines.push('')
  }
  if (files.length > 0) {
    lines.push('## Files', '')
    lines.push(...files.map((f) => link(f)))
    lines.push('')
  }
  if (children.length === 0) lines.push('(empty)', '')
  return lines.join('\n')
}
