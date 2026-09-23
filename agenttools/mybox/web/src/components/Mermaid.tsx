import { useEffect, useRef, useState } from 'react'

interface MermaidProps {
  code: string
}

const CACHE_LIMIT = 100

// Per-render cache. Each entry is the rendered SVG string; whenever the cache
// grows beyond the limit the oldest entry is evicted so a long-lived session
// leaking many distinct diagrams can't grow memory without bound.
const cache = new Map<string, string>()
let counter = 0

function remember(code: string, svg: string): void {
  cache.set(code, svg)
  if (cache.size > CACHE_LIMIT) {
    const oldest = cache.keys().next()
    if (!oldest.done) cache.delete(oldest.value)
  }
}

export function Mermaid({ code }: MermaidProps) {
  const hostRef = useRef<HTMLDivElement>(null)
  const [error, setError] = useState(false)

  useEffect(() => {
    const host = hostRef.current
    if (!host) return
    setError(false)
    if (cache.has(code)) {
      host.innerHTML = cache.get(code)!
      return
    }
    let cancelled = false
    import('mermaid')
      .then(async ({ default: mermaid }) => {
        // 'strict' (rather than 'loose') lets mermaid sanitize and re-encode
        // any HTML/script injected through user-written markdown so diagrams
        // can't smuggle executable HTML into the document via innerHTML.
        mermaid.initialize({ startOnLoad: false, securityLevel: 'strict' })
        const id = `mmd-${(counter++).toString(36)}`
        const { svg } = await mermaid.render(id, code)
        remember(code, svg)
        if (!cancelled && hostRef.current) hostRef.current.innerHTML = svg
      })
      .catch((err) => {
        console.error('[Mermaid] render failed:', err)
        if (!cancelled) setError(true)
      })
    return () => {
      cancelled = true
    }
  }, [code])

  if (error) return <pre className="mermaid-error my-3 font-mono text-red-600 whitespace-pre-wrap">{code}</pre>
  return <div ref={hostRef} className="mermaid my-3" />
}
