import { useMemo } from 'react'
import DOMPurify from 'dompurify'
import MarkdownIt from 'markdown-it'
import { openExternalLinksInNewTabs } from '../utils/markdown'

const md = new MarkdownIt({
  html: true,
  linkify: true,
  breaks: true,
})

interface MarkdownRenderEnv {
  transformHref?: (href: string) => string
}

md.renderer.rules.link_open = (tokens, idx, options, env, self) => {
  const token = tokens[idx]
  const transformHref = (env as MarkdownRenderEnv | undefined)?.transformHref
  const href = token.attrGet('href')
  if (href && transformHref) token.attrSet('href', transformHref(href))
  return self.renderToken(tokens, idx, options)
}

export interface MarkdownProps {
  text: string
  className?: string
  transformHref?: (href: string) => string
}

export function Markdown({ text, className, transformHref }: MarkdownProps) {
  const html = useMemo(() => {
    return openExternalLinksInNewTabs(DOMPurify.sanitize(md.render(text, { transformHref })))
  }, [text, transformHref])
  return <div className={className ?? 'markdown-body'} dangerouslySetInnerHTML={{ __html: html }} />
}
