import { useEffect, useMemo, useRef } from 'react'
import { Prism, hasGrammar } from '../utils/prism-langs'

const URL_RE = /https?:\/\/[^\s<>"')\]]+/g

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

function highlightText(text: string, language?: string): string {
  if (language && hasGrammar(language)) {
    try {
      return linkifyUrls(Prism.highlight(text, Prism.languages[language], language))
    } catch {
      // fall through to plaintext
    }
  }
  return linkifyUrls(escapeHtml(text))
}

export interface SyntaxHighlighterProps {
  text: string
  language?: string
  className?: string
  // Terminal column width the pre is forced to render at so long lines wrap at
  // (and only at) the same columns as the herdr pane they came from.
  cols?: number
  focusLine?: number
  searchQuery?: string
}

export function SyntaxHighlighter({ text, language, className, cols, focusLine, searchQuery }: SyntaxHighlighterProps) {
  const lines = useMemo(() => text.split(/\r?\n/), [text])
  const lineRef = useRef<HTMLPreElement | null>(null)
  const fixedWidth =
    cols != null && cols > 0 ? { width: `${cols}ch`, minWidth: `${cols}ch` } : undefined

  useEffect(() => {
    if (!focusLine || focusLine < 1) return
    const target = lineRef.current?.querySelector<HTMLElement>(`[data-search-line="${focusLine}"]`)
    target?.scrollIntoView({ behavior: 'smooth', block: 'center' })
  }, [focusLine, searchQuery, text])

  return (
    <pre
      ref={lineRef}
      className={`overflow-x-auto font-mono text-[13px] leading-6 whitespace-pre-wrap break-words ${className ?? ''}`}
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
              dangerouslySetInnerHTML={{ __html: highlightText(line, language) || '\u200b' }}
            />
          )
        })}
      </code>
    </pre>
  )
}
