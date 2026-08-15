import { useEffect, useRef, useState } from 'react'

interface MermaidProps {
  code: string
}

const cache = new Map<string, string>()
let counter = 0

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
        mermaid.initialize({ startOnLoad: false, securityLevel: 'loose' })
        const id = `mmd-${(counter++).toString(36)}`
        const { svg } = await mermaid.render(id, code)
        cache.set(code, svg)
        if (!cancelled && hostRef.current) hostRef.current.innerHTML = svg
      })
      .catch(() => {
        if (!cancelled) setError(true)
      })
    return () => {
      cancelled = true
    }
  }, [code])

  if (error) return <pre className="mermaid-error my-3 font-mono text-red-600 whitespace-pre-wrap">{code}</pre>
  return <div ref={hostRef} className="mermaid my-3" />
}
