import { useMemo } from 'react'
import MarkdownIt from 'markdown-it'
import DOMPurify from 'dompurify'
import { renderWikiLinks, resolveMarkdownLink } from '../utils/markdown'
import { filesUrl } from '../utils/routes'
import { Mermaid } from './Mermaid'

const md = new MarkdownIt({
  html: false,
  linkify: true,
  breaks: true,
})

md.renderer.rules.heading_open = (tokens, idx, options, _env, self) => {
  const token = tokens[idx]
  const inline = tokens[idx + 1]
  const text = inline ? md.renderer.renderInlineAsText(inline.children ?? [], options, undefined) : ''
  const plain = text.replace(/\[\[([^\]|]+)(?:\|[^\]]*)?\]\]/g, '$1').replace(/[*_`#]/g, '')
  const slug = slugify(plain)
  token.attrSet('id', slug)
  return self.renderToken(tokens, idx, options)
}

md.renderer.rules.link_open = (tokens, idx, options, env, self) => {
  const href = tokens[idx].attrGet('href') ?? ''
  const files = Boolean(env?.linkUrl)
  const resolved = resolveMarkdownLink(href, env?.relativeTo, env?.preserveExtension, {
    resolveDirectories: files,
    resolveAnyFile: files,
  })
  if (resolved) {
    tokens[idx].attrSet('href', env?.linkUrl ? env.linkUrl(resolved) : filesUrl(resolved))
  }
  return self.renderToken(tokens, idx, options)
}

md.renderer.rules.image = (tokens, idx, options, env, self) => {
  const src = tokens[idx].attrGet('src') ?? ''
  const resolved = env?.imageUrl
    ? resolveMarkdownLink(src, env?.relativeTo, env?.preserveExtension, {
        resolveDirectories: false,
        resolveAnyFile: true,
      })
    : null
  if (resolved && env?.imageUrl) {
    tokens[idx].attrSet('src', env.imageUrl(resolved))
  }
  return self.renderToken(tokens, idx, options)
}

export function slugify(text: string): string {
  return text
    .toLowerCase()
    .replace(/[^\w\u3040-\u30ff\u3400-\u9fff\s-]/g, '')
    .replace(/\s+/g, '-')
    .replace(/-+/g, '-')
    .replace(/^-|-$/g, '')
}

export function extractOutline(text: string): Array<{ level: number; id: string; text: string }> {
  const outline: Array<{ level: number; id: string; text: string }> = []
  for (const line of text.split('\n')) {
    const m = /^(#{1,4})\s+(.+)$/.exec(line)
    if (!m) continue
    const raw = m[2]
    const plain = raw.replace(/\[\[([^\]|]+)(?:\|[^\]]*)?\]\]/g, '$1').replace(/[*_`#]/g, '')
    outline.push({ level: m[1].length, id: slugify(plain), text: plain.trim() })
  }
  return outline
}

export interface RichMarkdownProps {
  text: string
  pathOf?: (target: string) => string | null
  relativeTo?: string
  linkUrl?: (resolved: string) => string
  imageUrl?: (resolved: string) => string
  preserveExtension?: boolean
}

interface Segment {
  kind: 'md' | 'mermaid'
  content: string
}

function splitSegments(text: string): Segment[] {
  const segments: Segment[] = []
  const pattern = /^```mermaid\s*\n([\s\S]*?)^```\s*$/gm
  let last = 0
  for (const m of text.matchAll(pattern)) {
    if (m.index! > last) {
      segments.push({ kind: 'md', content: text.slice(last, m.index) })
    }
    segments.push({ kind: 'mermaid', content: m[1].trim() })
    last = m.index! + m[0].length
  }
  if (last < text.length) {
    segments.push({ kind: 'md', content: text.slice(last) })
  }
  return segments
}

export function RichMarkdown({ text, pathOf, relativeTo, linkUrl, imageUrl, preserveExtension }: RichMarkdownProps) {
  const segments = useMemo(() => splitSegments(text), [text])

  return (
    <div className="markdown-body">
      {segments.map((seg, i) => {
        if (seg.kind === 'mermaid') {
          return <Mermaid key={i} code={seg.content} />
        }
        const withLinks = pathOf ? renderWikiLinks(seg.content, pathOf) : seg.content
        const html = DOMPurify.sanitize(
          md.render(withLinks, { relativeTo, linkUrl, imageUrl, preserveExtension }),
        )
        return <div key={i} dangerouslySetInnerHTML={{ __html: html }} />
      })}
    </div>
  )
}
