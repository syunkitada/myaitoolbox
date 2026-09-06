import { extractWikiLinks, resolveMarkdownLink } from '../utils/markdown'
import type { LinkReference } from './types'
import { isMarkdownFile } from './tree'

const markdownLinkPattern = /\[[^\]\n]*\]\(([^)\s]+)\)/g

export interface LinkResolveContext {
  knownMarkdown: Set<string>
  aliases?: Map<string, string>
}

function normalize(name: string): string {
  return name.toLowerCase().replace(/\.md$/, '')
}

/**
 * Normalizes a title/alias into the key used for wiki-link lookup.
 */
export function normalizeAlias(name: string): string {
  return normalize(name)
}

function basenameResolution(knownMarkdown: Set<string>, target: string): string | null {
  const needle = normalize(target.split('/').pop() ?? target)
  let match: string | null = null
  for (const path of knownMarkdown) {
    const base = normalize(path.split('/').pop() ?? path)
    if (base === needle) {
      if (match && match !== path) return null
      match = path
    }
  }
  return match
}

export function resolveLinkTarget(
  target: string,
  ctx: LinkResolveContext,
  relativeTo?: string | null,
  preserveExtension = true,
): string | null {
  const cleaned = target.split(/[#?]/)[0].trim()
  if (!cleaned) return null

  if (ctx.aliases?.has(normalize(target))) {
    const viaAlias = ctx.aliases.get(normalize(target))
    if (viaAlias) return viaAlias
  }

  const knownPath = (p: string) => p && ctx.knownMarkdown.has(p)

  if (knownPath(cleaned)) return cleaned
  if (knownPath(`${cleaned}.md`)) return `${cleaned}.md`

  if (/^[a-z][a-z0-9+.-]*:/i.test(cleaned) || cleaned.startsWith('/')) return null

  const rel = resolveMarkdownLink(cleaned, relativeTo ?? '', preserveExtension)
  if (rel) {
    if (knownPath(rel)) return rel
    if (knownPath(`${rel}.md`)) return `${rel}.md`
    if (isMarkdownFile(rel)) return rel
  }

  return basenameResolution(ctx.knownMarkdown, target)
}

export function parseMarkdownLinks(content: string, fromPath: string, ctx: LinkResolveContext): LinkReference[] {
  const refs: LinkReference[] = []
  const seen = new Set<string>()

  const push = (target: string) => {
    if (!target || target === fromPath) return
    const key = `${fromPath}\u0000${target}`
    if (seen.has(key)) return
    seen.add(key)
    refs.push({ sourcePath: fromPath, targetPath: target })
  }

  for (const m of content.matchAll(markdownLinkPattern)) {
    const resolved = resolveLinkTarget(m[1], ctx, fromPath)
    if (resolved) push(resolved)
  }

  for (const link of extractWikiLinks(content)) {
    const resolved = resolveLinkTarget(link.target, ctx)
    if (resolved) push(resolved)
  }

  return refs
}