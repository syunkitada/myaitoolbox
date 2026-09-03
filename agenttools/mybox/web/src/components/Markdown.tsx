import { useMemo } from 'react'
import DOMPurify from 'dompurify'
import MarkdownIt from 'markdown-it'
import { renderWikiLinks } from '../utils/markdown'

const md = new MarkdownIt({
  html: true,
  linkify: true,
  breaks: true,
})

export interface MarkdownProps {
  text: string
  pathOf?: (target: string) => string | null
  className?: string
}

export function Markdown({ text, pathOf, className }: MarkdownProps) {
  const html = useMemo(() => {
    const withLinks = pathOf ? renderWikiLinks(text, pathOf) : text
    return DOMPurify.sanitize(md.render(withLinks))
  }, [text, pathOf])
  return <div className={className ?? 'markdown-body'} dangerouslySetInnerHTML={{ __html: html }} />
}
