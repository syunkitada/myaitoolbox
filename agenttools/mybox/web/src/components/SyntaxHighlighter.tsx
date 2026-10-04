import { MouseEvent as ReactMouseEvent, useEffect, useMemo, useRef } from 'react'
import { Prism, hasGrammar } from '../utils/prism-langs'
import { filesUrl, getProject } from '../utils/routes'
import { normalizeAgentOutput, type AgentOutputDisplayMode } from '../utils/agent-output-display'

const URL_RE = /https?:\/\/[^\s<>"')\]]+/g
const FILE_PATH_RE = /(?<![A-Za-z0-9_./-])((?:[A-Za-z0-9_.-]+\/)+[A-Za-z0-9_.-]+\.[A-Za-z0-9]+)(?![A-Za-z0-9_.-])/g

function escapeHtml(str: string): string {
  return str
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
}

function linkifyUrls(html: string): string {
  return html.replace(URL_RE, (url) => {
    const safe = url.replace(/&amp;/g, '&')
    return `<a href="${safe}" target="_blank" rel="noopener noreferrer" class="syntax-link">${escapeHtml(url)}</a>`
  })
}

function linkifyFilePaths(html: string): string {
  if (!getProject()) return html
  return html.replace(FILE_PATH_RE, (path) => {
    const href = escapeHtml(filesUrl(path))
    return `<a href="${href}" data-file-path="${escapeHtml(path)}" class="syntax-link syntax-file-link">${path}</a>`
  })
}

function highlightText(text: string, language?: string, linkFilePaths = false): string {
  const decorate = (html: string) => {
    const linked = linkifyUrls(html)
    return linkFilePaths ? linkifyFilePaths(linked) : linked
  }
  if (language && hasGrammar(language)) {
    try {
      return decorate(Prism.highlight(text, Prism.languages[language], language))
    } catch {
      // fall through to plaintext
    }
  }
  return decorate(escapeHtml(text))
}

export interface SyntaxHighlighterProps {
  text: string
  language?: string
  className?: string
  // Terminal column width the pre is forced to render at so long lines wrap at
  // (and only at) the same columns as the herdr pane they came from.
  cols?: number
  // Agent sidebar display mode. Omitted callers retain the existing behavior.
  displayMode?: AgentOutputDisplayMode
  linkFilePaths?: boolean
  focusLine?: number
  searchQuery?: string
  onFilePathClick?: (path: string) => void
}

export function SyntaxHighlighter({
  text,
  language,
  className,
  cols,
  displayMode,
  linkFilePaths = false,
  focusLine,
  searchQuery,
  onFilePathClick,
}: SyntaxHighlighterProps) {
  const displayText = displayMode === 'auto' ? normalizeAgentOutput(text) : text
  const lines = useMemo(() => displayText.split(/\r?\n/), [displayText])
  const lineRef = useRef<HTMLPreElement | null>(null)
  const layoutClass = displayMode === 'herdr'
    ? 'w-full min-w-0 max-w-full overflow-x-auto'
    : 'overflow-x-auto'
  const fixedWidth =
    cols != null && cols > 0 ? { width: `${cols}ch`, minWidth: `${cols}ch` } : undefined

  const handleClick = (e: ReactMouseEvent<HTMLPreElement>) => {
    if (!onFilePathClick || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return
    const anchor = (e.target as HTMLElement).closest<HTMLAnchorElement>('a.syntax-file-link[data-file-path]')
    if (!anchor || !e.currentTarget.contains(anchor)) return
    const path = anchor.dataset.filePath
    if (!path) return
    e.preventDefault()
    onFilePathClick(path)
  }

  useEffect(() => {
    if (!focusLine || focusLine < 1) return
    const target = lineRef.current?.querySelector<HTMLElement>(`[data-search-line="${focusLine}"]`)
    target?.scrollIntoView({ behavior: 'smooth', block: 'center' })
  }, [focusLine, searchQuery, text])

  return (
    <pre
      ref={lineRef}
      onClick={handleClick}
      className={`${layoutClass} font-mono text-[13px] leading-6 ${displayMode === 'herdr' ? 'whitespace-pre' : 'whitespace-pre-wrap break-words'} ${className ?? ''}`}
      style={fixedWidth}
    >
      <code>
        {lines.map((line, index) => {
          const lineNumber = index + 1
          return (
            <span
              key={lineNumber}
              data-search-line={lineNumber}
              className={lineNumber === focusLine ? 'block rounded bg-yellow-100 px-1 dark:bg-yellow-900/50' : 'block'}
              dangerouslySetInnerHTML={{ __html: highlightText(line, language, linkFilePaths) || '\u200b' }}
            />
          )
        })}
      </code>
    </pre>
  )
}
