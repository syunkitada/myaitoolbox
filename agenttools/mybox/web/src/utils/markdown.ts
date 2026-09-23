import { parse as parseYaml, stringify as stringifyYaml } from 'yaml'
import MarkdownIt from 'markdown-it'

export interface FrontmatterSplit {
  frontmatter: string
  body: string
  has: boolean
}

export interface FrontmatterParse {
  ok: boolean
  data: Record<string, unknown>
  error?: string
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
  } catch (err) {
    return { ok: false, data: {}, error: err instanceof Error ? err.message : String(err) }
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

/** Replaces only the body of a frontmatter document, preserving its original formatting. */
export function replaceMarkdownBody(text: string, body: string): string {
  const prefix = /^[ \t]*---[ \t]*\r?\n[\s\S]*?\r?\n[ \t]*---[ \t]*(?:\r?\n)+/.exec(text)?.[0]
  return prefix ? prefix + body : body
}

const taskListItemPattern = /^(\s*(?:(?:[-+*])|(?:\d+[.)]))\s+)\[([ xX])\](?=\s|$)/

/**
 * Updates the task marker on a Markdown list item without changing the rest of
 * the document. The line number is relative to the supplied Markdown body.
 */
export function setMarkdownTaskChecked(text: string, line: number, checked: boolean): string | null {
  const lines = text.split(/\r?\n/)
  if (!Number.isInteger(line) || line < 0 || line >= lines.length) return null

  const match = taskListItemPattern.exec(lines[line])
  if (!match) return null

  const eol = text.includes('\r\n') ? '\r\n' : '\n'
  const markerStart = match[1].length
  const markerEnd = markerStart + 3
  lines[line] = lines[line].slice(0, markerStart) + `[${checked ? 'x' : ' '}]` + lines[line].slice(markerEnd)
  return lines.join(eol)
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

export function normalizePath(path: string): string {
  return path.toLowerCase().replace(/\.md$/, '')
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
  const fileTarget = /\.md$/i.test(clean) || (opts.resolveAnyFile && /\.[a-z0-9]+$/i.test(clean))
  const dirTarget = opts.resolveDirectories && (clean.endsWith('/') || !fileTarget)
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

export type MarkdownCopyFormat = 'text' | 'jira'

type MarkdownToken = ReturnType<MarkdownIt['parse']>[number]

const jiraMarkdown = new MarkdownIt({
  html: true,
  linkify: true,
  breaks: true,
})

function jiraAttr(token: MarkdownToken, name: string): string {
  return token.attrGet(name) ?? ''
}

function escapeJiraText(value: string, escapePipe = false): string {
  const escaped = value.replace(/[\\*_+^~{}!-]/g, '\\$&')
  return escapePipe ? escaped.replace(/\|/g, '\\|') : escaped
}

function findClosingToken(
  tokens: MarkdownToken[],
  start: number,
  openType: string,
  closeType: string,
  end = tokens.length,
): number {
  let depth = 0
  for (let i = start; i < end; i++) {
    if (tokens[i].type === openType) depth++
    if (tokens[i].type === closeType) {
      depth--
      if (depth === 0) return i
    }
  }
  return end
}

function renderJiraInline(tokens: MarkdownToken[], start = 0, end = tokens.length, escapePipe = false): string {
  let result = ''
  for (let i = start; i < end;) {
    const token = tokens[i]
    switch (token.type) {
      case 'text':
      case 'entity':
        result += escapeJiraText(token.content, escapePipe)
        i++
        continue
      case 'code_inline':
        result += `{{${token.content.replace(/\r?\n/g, ' ')}}}`
        i++
        continue
      case 'softbreak':
        result += '\n'
        i++
        continue
      case 'hardbreak':
        result += '\\\\\n'
        i++
        continue
      case 'html_inline':
        if (/^<br\s*\/?\s*>$/i.test(token.content)) {
          result += '\\\\'
        } else {
          result += escapeJiraText(token.content.replace(/<[^>]*>/g, ''), escapePipe)
        }
        i++
        continue
      case 'image': {
        const src = jiraAttr(token, 'src').replace(/\|/g, '\\|')
        const alt = escapeJiraText(token.content, true)
        result += alt ? `!${src}|alt=${alt}!` : `!${src}!`
        i++
        continue
      }
      case 'strong_open':
      case 'em_open':
      case 's_open':
      case 'link_open': {
        const closeType = token.type.replace('_open', '_close')
        const close = findClosingToken(tokens, i, token.type, closeType, end)
        const content = renderJiraInline(tokens, i + 1, close, escapePipe)
        if (token.type === 'strong_open') result += `*${content}*`
        else if (token.type === 'em_open') result += `_${content}_`
        else if (token.type === 's_open') result += `-${content}-`
        else {
          const href = jiraAttr(token, 'href').replace(/\|/g, '\\|')
          result += href ? `[${content}|${href}]` : content
        }
        i = close < end ? close + 1 : end
        continue
      }
      default:
        if (token.nesting === 0 && token.content) {
          result += escapeJiraText(token.content, escapePipe)
        }
        i++
    }
  }
  return result
}

function renderJiraInlineToken(token: MarkdownToken, escapePipe = false): string {
  return renderJiraInline(token.children ?? [], 0, token.children?.length ?? 0, escapePipe)
}

function renderJiraTable(tokens: MarkdownToken[], start: number, end: number): string {
  const rows: string[][] = []
  for (let i = start + 1; i < end;) {
    if (tokens[i].type !== 'tr_open') {
      i++
      continue
    }
    const rowEnd = findClosingToken(tokens, i, 'tr_open', 'tr_close', end)
    const cells: string[] = []
    for (let j = i + 1; j < rowEnd;) {
      if (tokens[j].type !== 'th_open' && tokens[j].type !== 'td_open') {
        j++
        continue
      }
      const cellEnd = findClosingToken(
        tokens,
        j,
        tokens[j].type,
        tokens[j].type === 'th_open' ? 'th_close' : 'td_close',
        rowEnd,
      )
      const inline = tokens.slice(j + 1, cellEnd).find((token) => token.type === 'inline')
      cells.push(inline ? renderJiraInlineToken(inline, true) : '')
      j = cellEnd < rowEnd ? cellEnd + 1 : rowEnd
    }
    rows.push(cells)
    i = rowEnd < end ? rowEnd + 1 : end
  }
  if (rows.length === 0) return ''
  const [header, ...body] = rows
  return [
    `||${header.join('||')}||`,
    ...body.map((row) => `|${row.join('|')}|`),
  ].join('\n')
}

function renderJiraList(tokens: MarkdownToken[], start: number, end: number, depth: number): string {
  const marker = tokens[start].type === 'ordered_list_open' ? '#' : '*'
  const lines: string[] = []
  for (let i = start + 1; i < end;) {
    if (tokens[i].type !== 'list_item_open') {
      i++
      continue
    }
    const itemEnd = findClosingToken(tokens, i, 'list_item_open', 'list_item_close', end)
    const blocks = renderJiraBlocks(tokens, i + 1, itemEnd, depth + 1)
    const prefix = `${marker.repeat(depth)} `
    if (blocks.length === 0) {
      lines.push(prefix.trimEnd())
    } else {
      lines.push(prefix + blocks[0])
      lines.push(...blocks.slice(1))
    }
    i = itemEnd < end ? itemEnd + 1 : end
  }
  return lines.join('\n')
}

function renderJiraBlocks(tokens: MarkdownToken[], start = 0, end = tokens.length, listDepth = 1): string[] {
  const blocks: string[] = []
  for (let i = start; i < end;) {
    const token = tokens[i]
    switch (token.type) {
      case 'heading_open': {
        const close = findClosingToken(tokens, i, 'heading_open', 'heading_close', end)
        const inline = tokens.slice(i + 1, close).find((candidate) => candidate.type === 'inline')
        blocks.push(`${token.tag}. ${inline ? renderJiraInlineToken(inline) : ''}`)
        i = close < end ? close + 1 : end
        continue
      }
      case 'paragraph_open': {
        const close = findClosingToken(tokens, i, 'paragraph_open', 'paragraph_close', end)
        const inline = tokens.slice(i + 1, close).find((candidate) => candidate.type === 'inline')
        blocks.push(inline ? renderJiraInlineToken(inline) : '')
        i = close < end ? close + 1 : end
        continue
      }
      case 'bullet_list_open':
      case 'ordered_list_open': {
        const closeType = token.type === 'bullet_list_open' ? 'bullet_list_close' : 'ordered_list_close'
        const close = findClosingToken(tokens, i, token.type, closeType, end)
        blocks.push(renderJiraList(tokens, i, close, listDepth))
        i = close < end ? close + 1 : end
        continue
      }
      case 'blockquote_open': {
        const close = findClosingToken(tokens, i, 'blockquote_open', 'blockquote_close', end)
        const content = renderJiraBlocks(tokens, i + 1, close, listDepth)
        blocks.push(content.length > 0 ? `{quote}\n${content.join('\n\n')}\n{quote}` : '{quote}{quote}')
        i = close < end ? close + 1 : end
        continue
      }
      case 'table_open': {
        const close = findClosingToken(tokens, i, 'table_open', 'table_close', end)
        blocks.push(renderJiraTable(tokens, i, close))
        i = close < end ? close + 1 : end
        continue
      }
      case 'fence': {
        const info = token.info.trim().split(/\s+/, 1)[0]
        const language = info ? `:${info.replace(/[{}\s]/g, '')}` : ''
        const content = token.content.replace(/\r?\n$/, '')
        blocks.push(`{code${language}}\n${content}\n{code}`)
        i++
        continue
      }
      case 'code_block': {
        const content = token.content.replace(/\r?\n$/, '')
        blocks.push(`{code}\n${content}\n{code}`)
        i++
        continue
      }
      case 'hr':
        blocks.push('----')
        i++
        continue
      case 'html_block': {
        const content = token.content.replace(/<[^>]*>/g, '').trim()
        if (content) blocks.push(escapeJiraText(content))
        i++
        continue
      }
      case 'inline':
        blocks.push(renderJiraInlineToken(token))
        i++
        continue
      default:
        i++
    }
  }
  return blocks.filter((block, index) => block !== '' || index === 0)
}

/** Converts Markdown to Jira's legacy wiki markup for pasting into wiki-rendered fields. */
export function markdownToJira(text: string): string {
  const blocks = renderJiraBlocks(jiraMarkdown.parse(text, {}))
  const result = blocks.join('\n\n')
  return /\r?\n$/.test(text) && result !== '' ? `${result}\n` : result
}

export function formatMarkdownForCopy(text: string, format: MarkdownCopyFormat = 'text'): string {
  return format === 'jira' ? markdownToJira(text) : text
}
