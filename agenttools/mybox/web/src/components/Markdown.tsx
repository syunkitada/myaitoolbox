import { useMemo } from 'react'
import DOMPurify from 'dompurify'
import MarkdownIt from 'markdown-it'

const md = new MarkdownIt({
  html: true,
  linkify: true,
  breaks: true,
})

export interface MarkdownProps {
  text: string
  className?: string
}

export function Markdown({ text, className }: MarkdownProps) {
  const html = useMemo(() => {
    return DOMPurify.sanitize(md.render(text))
  }, [text])
  return <div className={className ?? 'markdown-body'} dangerouslySetInnerHTML={{ __html: html }} />
}
