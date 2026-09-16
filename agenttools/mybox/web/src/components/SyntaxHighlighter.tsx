import { useMemo } from 'react'
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
}

export function SyntaxHighlighter({ text, language, className, cols }: SyntaxHighlighterProps) {
  const html = useMemo(() => highlightText(text, language), [text, language])
  const fixedWidth =
    cols != null && cols > 0 ? { width: `${cols}ch`, minWidth: `${cols}ch` } : undefined

  return (
    <pre
      className={`overflow-x-auto font-mono text-[13px] leading-6 whitespace-pre-wrap break-words ${className ?? ''}`}
      style={fixedWidth}
    >
      <code dangerouslySetInnerHTML={{ __html: html }} />
    </pre>
  )
}
