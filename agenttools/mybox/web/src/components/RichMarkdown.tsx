import { MouseEvent, useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { useNavigate } from 'react-router-dom'
import MarkdownIt from 'markdown-it'
import DOMPurify from 'dompurify'
import { Check, Copy, X } from 'lucide-react'
import { Prism, hasGrammar } from '../utils/prism-langs'
import { formatMarkdownForCopy, parseMarkdownTaskItem, resolveMarkdownLink, type MarkdownCopyFormat } from '../utils/markdown'
import { filesUrl, getBasePath } from '../utils/routes'
import { copyToClipboard } from '../utils/clipboard'
import { Mermaid } from './Mermaid'
import { VegaLite } from './VegaLite'
import { Tabs, TabsContent, TabsList, TabsTrigger } from './ui/tabs'

const md: MarkdownIt = new MarkdownIt({
  html: true,
  linkify: true,
  breaks: true,
  highlight: (code: string, lang: string): string => {
    if (lang && hasGrammar(lang)) {
      try {
        const highlighted = Prism.highlight(code, Prism.languages[lang], lang)
        return `<pre class="language-${md.utils.escapeHtml(lang)}"><code>${highlighted}</code></pre>`
      } catch {
        // fall through to default
      }
    }
    return ''
  },
})

const defaultFence = md.renderer.rules.fence

md.renderer.rules.fence = (tokens, idx, options, env, self) => {
  const token = tokens[idx]
  const rendered = defaultFence
    ? defaultFence(tokens, idx, options, env, self)
    : `<pre><code>${md.utils.escapeHtml(tokens[idx].content)}</code></pre>\n`
  const lineOffset = (env as MarkdownRenderEnv | undefined)?.lineOffset ?? 0
  const sourceLine = token.map?.[0] === undefined ? '' : ` data-source-line="${lineOffset + token.map[0] + 1}"`
  return `<div class="markdown-code-block" data-code-block${sourceLine}><button type="button" class="markdown-code-copy" data-copy-code aria-label="Copy code" title="Copy code">Copy</button>${rendered}</div>\n`
}

interface MarkdownSourceLineEnv {
  lineOffset?: number
}

function setSourceLine(token: { map?: [number, number] | null; attrSet: (name: string, value: string) => void }, env?: MarkdownSourceLineEnv) {
  if (token.map?.[0] === undefined) return
  token.attrSet('data-source-line', String((env?.lineOffset ?? 0) + token.map[0] + 1))
}

md.renderer.rules.heading_open = (tokens, idx, options, _env, self) => {
  const token = tokens[idx]
  const inline = tokens[idx + 1]
  const text = (inline?.children ?? [])
    .map((child) => (
      child.type === 'code_inline'
        ? child.content
        : md.renderer.renderInlineAsText([child], options, undefined)
    ))
    .join('')
  const slug = slugify(text)
  token.attrSet('id', slug)
  setSourceLine(token, _env as MarkdownSourceLineEnv | undefined)
  return self.renderToken(tokens, idx, options)
}

const defaultParagraphOpen = md.renderer.rules.paragraph_open
md.renderer.rules.paragraph_open = (tokens, idx, options, env, self) => {
  setSourceLine(tokens[idx], env as MarkdownSourceLineEnv | undefined)
  return defaultParagraphOpen
    ? defaultParagraphOpen(tokens, idx, options, env, self)
    : self.renderToken(tokens, idx, options)
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

interface MarkdownRenderEnv {
  relativeTo?: string
  linkUrl?: (resolved: string) => string
  imageUrl?: (resolved: string) => string
  preserveExtension?: boolean
  taskSource?: string
  taskLineOffset?: number
  lineOffset?: number
  taskInteractive?: boolean
}

interface TaskListItem {
  line: number
  checked: boolean
}

function findTaskListItem(tokens: Parameters<NonNullable<MarkdownIt['renderer']['rules']['list_item_open']>>[0], idx: number, env?: MarkdownRenderEnv): TaskListItem | null {
  const token = tokens[idx]
  const line = token.map?.[0]
  if (line === undefined || !env?.taskSource) return null
  const sourceLine = env.taskSource.split(/\r?\n/)[line]
  const sourceItem = sourceLine ? parseMarkdownTaskItem(sourceLine) : null
  if (!sourceItem) return null

  const inline = tokens
    .slice(idx + 1)
    .find((candidate) => candidate.type === 'inline' && candidate.map?.[0] === line)
  if (!inline) return null
  const marker = /^\[([ xX])\](?:[ \t]+|$)/.exec(inline.content)
  if (!marker) return null

  inline.content = inline.content.slice(marker[0].length)
  if (inline.children?.[0]?.type === 'text') {
    inline.children[0].content = inline.children[0].content.slice(marker[0].length)
  }

  const task = {
    line: (env.taskLineOffset ?? 0) + line,
    checked: sourceItem.checked,
  }
  inline.meta = { ...(inline.meta ?? {}), task }
  if (inline.children?.[0]) {
    inline.children[0].meta = { ...(inline.children[0].meta ?? {}), task }
  }
  return task
}

const defaultListItemOpen = md.renderer.rules.list_item_open
const defaultRenderInline = md.renderer.renderInline.bind(md.renderer)

md.renderer.rules.list_item_open = (tokens, idx, options, env, self) => {
  const task = findTaskListItem(tokens, idx, env as MarkdownRenderEnv | undefined)
  if (task) tokens[idx].attrJoin('class', 'task-list-item')
  const rendered = defaultListItemOpen
    ? defaultListItemOpen(tokens, idx, options, env, self)
    : self.renderToken(tokens, idx, options)
  return rendered
}

md.renderer.renderInline = (tokens, options, env) => {
  const rendered = defaultRenderInline(tokens, options, env)
  const task = tokens[0]?.meta?.task as TaskListItem | undefined
  if (!task) return rendered

  const interactive = Boolean((env as MarkdownRenderEnv | undefined)?.taskInteractive)
  const checked = task.checked ? ' checked' : ''
  const disabled = interactive ? '' : ' disabled'
  const label = task.checked ? 'Mark task incomplete' : 'Mark task complete'
  return `<input class="task-list-checkbox" type="checkbox" data-task-checkbox="true" data-task-line="${task.line}" aria-label="${label}"${checked}${disabled}>${rendered}`
}

const anchorRe = /^#(.*)$/

function markDeadAnchors(html: string): string {
  const doc = new DOMParser().parseFromString(html, 'text/html')
  const targets = new Set<string>()
  for (const el of doc.querySelectorAll('h1,h2,h3,h4,h5,h6')) {
    const id = el.getAttribute('id')
    if (id) targets.add(id)
  }
  let changed = false
  for (const a of Array.from(doc.querySelectorAll('a[href^="#"]'))) {
    const raw = (a.getAttribute('href') ?? '').match(anchorRe)?.[1]
    if (!raw) continue
    let id: string
    try { id = decodeURIComponent(raw) } catch { id = raw }
    if (targets.has(id)) continue
    a.classList.add('dead-anchor')
    if (!a.querySelector('.dead-link-mark')) {
      const mark = doc.createElement('span')
      mark.className = 'dead-link-mark'
      mark.textContent = 'リンク切れ'
      a.appendChild(mark)
    }
    changed = true
  }
  return changed ? doc.body.innerHTML : html
}

export function extractOutline(text: string): Array<{ level: number; id: string; text: string }> {
  const outline: Array<{ level: number; id: string; text: string }> = []
  let fence: string | null = null
  // Normalize CRLF/LF line endings so heading lines never carry a trailing \r.
  for (const line of text.split(/\r?\n/)) {
    // Track fenced code blocks (``` or ~~~) so lines inside them are skipped.
    const fenceMatch = /^\s*(`{3,}|~{3,})/.exec(line)
    if (fenceMatch) {
      const marker: string = fenceMatch[1][0]
      if (fence === marker) {
        fence = null
      } else if (fence === null) {
        fence = marker
      }
      continue
    }
    if (fence) continue
    const m = /^ {0,3}(#{1,4})[ \t]+(.+)$/.exec(line)
    if (!m) continue
    const raw = m[2]
    const plain = raw.replace(/\[\[([^\]|]+)(?:\|[^\]]*)?\]\]/g, '$1').replace(/[*`#]/g, '')
    outline.push({ level: m[1].length, id: slugify(plain), text: plain.trim() })
  }
  return outline
}

export interface RichMarkdownProps {
  text: string
  relativeTo?: string
  linkUrl?: (resolved: string) => string
  imageUrl?: (resolved: string) => string
  dataUrl?: (resolved: string) => string
  preserveExtension?: boolean
  onTaskToggle?: (line: number, checked: boolean) => boolean | Promise<boolean>
  copyDialogOpen?: boolean
  onCopyDialogClose?: () => void
  focusLine?: number
  searchQuery?: string
  sourceLineOffset?: number
}

interface Segment {
  kind: 'md' | 'mermaid' | 'vega-lite'
  content: string
  lineOffset: number
  sourceLineOffset: number
}

function splitSegments(text: string, sourceLineOffset = 0): Segment[] {
  const segments: Segment[] = []
  const pattern = /^```\s*(mermaid|vega-lite)\s*\r?\n([\s\S]*?)^```\s*$/gm
  let last = 0
  for (const m of text.matchAll(pattern)) {
    if (m.index! > last) {
      segments.push({
        kind: 'md',
        content: text.slice(last, m.index),
        lineOffset: text.slice(0, last).split(/\r?\n/).length - 1,
        sourceLineOffset: sourceLineOffset + text.slice(0, last).split(/\r?\n/).length - 1,
      })
    }
    segments.push({
      kind: m[1] as 'mermaid' | 'vega-lite',
      content: m[2].trim(),
      lineOffset: text.slice(0, m.index!).split(/\r?\n/).length - 1,
      sourceLineOffset: sourceLineOffset + text.slice(0, m.index!).split(/\r?\n/).length - 1,
    })
    last = m.index! + m[0].length
  }
  if (last < text.length) {
    segments.push({
      kind: 'md',
      content: text.slice(last),
      lineOffset: text.slice(0, last).split(/\r?\n/).length - 1,
      sourceLineOffset: sourceLineOffset + text.slice(0, last).split(/\r?\n/).length - 1,
    })
  }
  return segments
}

function MarkdownCopyDialog({ text, onClose }: { text: string; onClose: () => void }) {
  const [copyDialogFormat, setCopyDialogFormat] = useState<MarkdownCopyFormat>('text')
  const [copyDialogCopied, setCopyDialogCopied] = useState(false)
  const [copyDialogError, setCopyDialogError] = useState(false)
  const copyDialogText = formatMarkdownForCopy(text, copyDialogFormat)

  const closeCopyDialog = useCallback(() => {
    onClose()
  }, [onClose])

  const changeCopyDialogFormat = useCallback((value: string) => {
    if (value !== 'text' && value !== 'jira') return
    setCopyDialogFormat(value)
    setCopyDialogCopied(false)
    setCopyDialogError(false)
  }, [])

  const handleCopyDialog = useCallback(() => {
    void copyToClipboard(copyDialogText)
      .then(() => {
        setCopyDialogCopied(true)
        setCopyDialogError(false)
      })
      .catch(() => {
        setCopyDialogCopied(false)
        setCopyDialogError(true)
      })
  }, [copyDialogText])

  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') closeCopyDialog()
    }
    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [closeCopyDialog])

  return createPortal(
    <div
      className="markdown-copy-dialog-overlay"
      onMouseDown={(event) => {
        if (event.target === event.currentTarget) closeCopyDialog()
      }}
    >
      <div className="markdown-copy-dialog" role="dialog" aria-modal="true" aria-label="Copy file contents">
        <div className="markdown-copy-dialog-header">
          <h2 className="text-base font-semibold">Copy file contents</h2>
          <button
            type="button"
            className="markdown-copy-dialog-close"
            onClick={closeCopyDialog}
            aria-label="Close copy preview"
            title="Close copy preview"
          >
            <X className="size-4" />
          </button>
        </div>
        <Tabs value={copyDialogFormat} onValueChange={changeCopyDialogFormat} className="gap-3">
          <TabsList aria-label="Copy format">
            <TabsTrigger value="text">Text</TabsTrigger>
            <TabsTrigger value="jira">Jira</TabsTrigger>
          </TabsList>
          <TabsContent value="text">
            <textarea
              className="markdown-copy-dialog-preview"
              value={formatMarkdownForCopy(text, 'text')}
              readOnly
              aria-label="Text copy preview"
            />
          </TabsContent>
          <TabsContent value="jira">
            <textarea
              className="markdown-copy-dialog-preview"
              value={formatMarkdownForCopy(text, 'jira')}
              readOnly
              aria-label="Jira copy preview"
            />
          </TabsContent>
        </Tabs>
        {copyDialogError && (
          <p className="mt-2 text-sm text-destructive">Unable to copy to clipboard.</p>
        )}
        <div className="markdown-copy-dialog-actions">
          <button type="button" className="markdown-text-copy" onClick={handleCopyDialog}>
            {copyDialogCopied ? <Check className="size-3.5" /> : <Copy className="size-3.5" />}
            {copyDialogCopied ? 'Copied' : 'Copy'}
          </button>
          <button type="button" className="markdown-copy-dialog-cancel" onClick={closeCopyDialog}>
            Close
          </button>
        </div>
      </div>
    </div>,
    document.body,
  )
}

export function RichMarkdown({
  text,
  relativeTo,
  linkUrl,
  imageUrl,
  preserveExtension,
  dataUrl,
  onTaskToggle,
  copyDialogOpen = false,
  onCopyDialogClose,
  focusLine,
  searchQuery,
  sourceLineOffset = 0,
}: RichMarkdownProps) {
  const segments = useMemo(() => splitSegments(text, sourceLineOffset), [sourceLineOffset, text])
  const navigate = useNavigate()
  const viewerRef = useRef<HTMLDivElement | null>(null)

  useEffect(() => {
    if (!focusLine || focusLine < 1) return
    const target = viewerRef.current?.querySelector<HTMLElement>(`[data-source-line="${focusLine}"]`)
    if (!target) return
    target.scrollIntoView({ behavior: 'smooth', block: 'center' })
    target.classList.add('bg-yellow-100', 'dark:bg-yellow-900/50', 'rounded')
    const timer = window.setTimeout(() => {
      target.classList.remove('bg-yellow-100', 'dark:bg-yellow-900/50')
    }, 2500)
    return () => window.clearTimeout(timer)
  }, [focusLine, searchQuery, sourceLineOffset, text])

  const handleTaskToggle = useCallback((input: HTMLInputElement) => {
    if (!onTaskToggle || input.disabled) return
    const line = Number(input.dataset.taskLine)
    if (!Number.isInteger(line) || line < 0) return

    const checked = input.checked
    input.disabled = true
    try {
      const result = onTaskToggle(line, checked)
      void Promise.resolve(result)
        .then((ok) => {
          if (ok !== false || !input.isConnected) return
          input.checked = !checked
          input.setAttribute('aria-label', checked ? 'Mark task complete' : 'Mark task incomplete')
        })
        .catch(() => {
          if (!input.isConnected) return
          input.checked = !checked
          input.setAttribute('aria-label', checked ? 'Mark task complete' : 'Mark task incomplete')
        })
        .finally(() => {
          if (input.isConnected) input.disabled = false
        })
    } catch {
      input.checked = !checked
      input.disabled = false
      input.setAttribute('aria-label', checked ? 'Mark task complete' : 'Mark task incomplete')
    }
  }, [onTaskToggle])

  const handleClick = useCallback(
    (e: MouseEvent<HTMLDivElement>) => {
      const taskCheckbox = (e.target as HTMLElement).closest<HTMLInputElement>('input[data-task-checkbox]')
      if (taskCheckbox && e.currentTarget.contains(taskCheckbox)) {
        handleTaskToggle(taskCheckbox)
        return
      }

      const copyButton = (e.target as HTMLElement).closest<HTMLButtonElement>('[data-copy-code]')
      if (copyButton && e.currentTarget.contains(copyButton)) {
        e.preventDefault()
        const code = copyButton.closest<HTMLElement>('[data-code-block]')?.querySelector('pre code')?.textContent ?? ''
        void copyToClipboard(code).then(() => {
          if (!copyButton.isConnected) return
          copyButton.textContent = 'Copied'
          copyButton.setAttribute('aria-label', 'Copied')
          copyButton.setAttribute('title', 'Copied')
          copyButton.classList.add('is-copied')
          window.setTimeout(() => {
            if (!copyButton.isConnected) return
            copyButton.textContent = 'Copy'
            copyButton.setAttribute('aria-label', 'Copy code')
            copyButton.setAttribute('title', 'Copy code')
            copyButton.classList.remove('is-copied')
          }, 1500)
        }).catch(() => undefined)
        return
      }

      const anchor = (e.target as HTMLElement).closest<HTMLAnchorElement>('a[href]')
      if (!anchor) return
      const href = anchor.getAttribute('href') ?? ''

      if (href.startsWith('#')) {
        const raw = href.slice(1)
        if (!raw) return
        let id: string
        try { id = decodeURIComponent(raw) } catch { id = raw }
        const target = document.getElementById(id)
        if (!target) return
        e.preventDefault()
        target.scrollIntoView({ behavior: 'smooth', block: 'start' })
        return
      }

      let url: URL
      try {
        url = new URL(href, window.location.origin)
      } catch {
        return
      }
      if (url.origin !== window.location.origin) return
      const base = getBasePath()
      const pathname = base && url.pathname.startsWith(base) ? url.pathname.slice(base.length) : url.pathname
      if (!pathname.startsWith('/projects/')) return

      e.preventDefault()
      navigate(pathname + url.search + url.hash)
    },
    [handleTaskToggle, navigate],
  )

  return (
    <div ref={viewerRef} className="markdown-viewer">
      <div className="markdown-body" onClick={handleClick}>
        {segments.map((seg, i) => {
          if (seg.kind === 'mermaid') {
            return <Mermaid key={i} code={seg.content} />
          }
          if (seg.kind === 'vega-lite') {
            return <VegaLite key={i} code={seg.content} relativeTo={relativeTo} dataUrl={dataUrl} />
          }
          const html = markDeadAnchors(
            DOMPurify.sanitize(
              md.render(seg.content, {
                relativeTo,
                linkUrl,
                imageUrl,
                preserveExtension,
                taskSource: seg.content,
                taskLineOffset: seg.lineOffset,
                lineOffset: seg.sourceLineOffset,
                taskInteractive: Boolean(onTaskToggle),
              } satisfies MarkdownRenderEnv),
            ),
          )
          return <div key={i} dangerouslySetInnerHTML={{ __html: html }} />
        })}
      </div>
      {copyDialogOpen && onCopyDialogClose && (
        <MarkdownCopyDialog text={text} onClose={onCopyDialogClose} />
      )}
    </div>
  )
}
