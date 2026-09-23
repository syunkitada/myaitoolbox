import { MouseEvent, useCallback, useEffect, useMemo, useState } from 'react'
import { createPortal } from 'react-dom'
import { useNavigate } from 'react-router-dom'
import MarkdownIt from 'markdown-it'
import DOMPurify from 'dompurify'
import { Check, Copy, X } from 'lucide-react'
import { Prism, hasGrammar } from '../utils/prism-langs'
import { formatMarkdownForCopy, resolveMarkdownLink, type MarkdownCopyFormat } from '../utils/markdown'
import { filesUrl, getBasePath } from '../utils/routes'
import { copyToClipboard } from '../utils/clipboard'
import { Mermaid } from './Mermaid'
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
  const rendered = defaultFence
    ? defaultFence(tokens, idx, options, env, self)
    : `<pre><code>${md.utils.escapeHtml(tokens[idx].content)}</code></pre>\n`
  return `<div class="markdown-code-block" data-code-block><button type="button" class="markdown-code-copy" data-copy-code aria-label="Copy code" title="Copy code">Copy</button>${rendered}</div>\n`
}

md.renderer.rules.heading_open = (tokens, idx, options, _env, self) => {
  const token = tokens[idx]
  const inline = tokens[idx + 1]
  const text = inline ? md.renderer.renderInlineAsText(inline.children ?? [], options, undefined) : ''
  const plain = text.replace(/[*_`#]/g, '')
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

interface MarkdownRenderEnv {
  relativeTo?: string
  linkUrl?: (resolved: string) => string
  imageUrl?: (resolved: string) => string
  preserveExtension?: boolean
  taskSource?: string
  taskLineOffset?: number
  taskInteractive?: boolean
}

interface TaskListItem {
  line: number
  checked: boolean
}

const taskListItemPattern = /^\s*(?:(?:[-+*])|(?:\d+[.)]))\s+\[([ xX])\](?=\s|$)/

function findTaskListItem(tokens: Parameters<NonNullable<MarkdownIt['renderer']['rules']['list_item_open']>>[0], idx: number, env?: MarkdownRenderEnv): TaskListItem | null {
  const token = tokens[idx]
  const line = token.map?.[0]
  if (line === undefined || !env?.taskSource) return null
  const sourceLine = env.taskSource.split(/\r?\n/)[line]
  const sourceMatch = sourceLine ? taskListItemPattern.exec(sourceLine) : null
  if (!sourceMatch) return null

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
    checked: sourceMatch[1].toLowerCase() === 'x',
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
    const plain = raw.replace(/\[\[([^\]|]+)(?:\|[^\]]*)?\]\]/g, '$1').replace(/[*_`#]/g, '')
    outline.push({ level: m[1].length, id: slugify(plain), text: plain.trim() })
  }
  return outline
}

export interface RichMarkdownProps {
  text: string
  relativeTo?: string
  linkUrl?: (resolved: string) => string
  imageUrl?: (resolved: string) => string
  preserveExtension?: boolean
  onTaskToggle?: (line: number, checked: boolean) => boolean | Promise<boolean>
  copyDialogOpen?: boolean
  onCopyDialogClose?: () => void
}

interface Segment {
  kind: 'md' | 'mermaid'
  content: string
  lineOffset: number
}

function splitSegments(text: string): Segment[] {
  const segments: Segment[] = []
  const pattern = /^```\s*mermaid\s*\n([\s\S]*?)^```\s*$/gm
  let last = 0
  for (const m of text.matchAll(pattern)) {
    if (m.index! > last) {
      segments.push({
        kind: 'md',
        content: text.slice(last, m.index),
        lineOffset: text.slice(0, last).split(/\r?\n/).length - 1,
      })
    }
    segments.push({
      kind: 'mermaid',
      content: m[1].trim(),
      lineOffset: text.slice(0, m.index!).split(/\r?\n/).length - 1,
    })
    last = m.index! + m[0].length
  }
  if (last < text.length) {
    segments.push({
      kind: 'md',
      content: text.slice(last),
      lineOffset: text.slice(0, last).split(/\r?\n/).length - 1,
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
  onTaskToggle,
  copyDialogOpen = false,
  onCopyDialogClose,
}: RichMarkdownProps) {
  const segments = useMemo(() => splitSegments(text), [text])
  const navigate = useNavigate()

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
    <div className="markdown-viewer">
      <div className="markdown-body" onClick={handleClick}>
        {segments.map((seg, i) => {
          if (seg.kind === 'mermaid') {
            return <Mermaid key={i} code={seg.content} />
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
