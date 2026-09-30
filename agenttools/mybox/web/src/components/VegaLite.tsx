import { useEffect, useRef, useState } from 'react'
import { resolveMarkdownLink } from '../utils/markdown'

export interface VegaLiteProps {
  code: string
  relativeTo?: string
  dataUrl?: (resolved: string) => string
}

type VegaLiteObject = Record<string, unknown>

interface VegaView {
  finalize: () => void
}

interface VegaEmbedResult {
  view: VegaView
}

function isObject(value: unknown): value is VegaLiteObject {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

function inferDataFormat(url: string): string | null {
  const path = url.split(/[?#]/, 1)[0].toLowerCase()
  const extension = path.slice(path.lastIndexOf('.') + 1)
  if (extension === 'csv' || extension === 'tsv' || extension === 'dsv' || extension === 'json' || extension === 'topojson') {
    return extension
  }
  return null
}

function resolveDataUrls(value: unknown, relativeTo: string | undefined, dataUrl?: (resolved: string) => string): unknown {
  if (Array.isArray(value)) {
    return value.map((item) => resolveDataUrls(item, relativeTo, dataUrl))
  }
  if (!isObject(value)) return value

  const resolved: VegaLiteObject = {}
  for (const [key, child] of Object.entries(value)) {
    if (key === 'data' && isObject(child) && typeof child.url === 'string' && dataUrl) {
      const localPath = resolveMarkdownLink(child.url, relativeTo, true, { resolveAnyFile: true })
      if (localPath) {
        const data = resolveDataUrls(child, relativeTo, dataUrl) as VegaLiteObject
        data.url = dataUrl(localPath)
        if (!Object.prototype.hasOwnProperty.call(child, 'format')) {
          const format = inferDataFormat(child.url)
          if (format) data.format = { type: format }
        }
        resolved[key] = data
        continue
      }
    }
    resolved[key] = resolveDataUrls(child, relativeTo, dataUrl)
  }
  return resolved
}

export function resolveVegaLiteSpec(code: string, relativeTo?: string, dataUrl?: (resolved: string) => string): VegaLiteObject {
  const parsed: unknown = JSON.parse(code)
  if (!isObject(parsed)) throw new Error('A Vega-Lite specification must be a JSON object.')
  return resolveDataUrls(parsed, relativeTo, dataUrl) as VegaLiteObject
}

export function VegaLite({ code, relativeTo, dataUrl }: VegaLiteProps) {
  const hostRef = useRef<HTMLDivElement>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    const host = hostRef.current
    if (!host) return

    let cancelled = false
    let view: VegaView | null = null
    host.replaceChildren()
    setError(null)

    let spec: VegaLiteObject
    try {
      spec = resolveVegaLiteSpec(code, relativeTo, dataUrl)
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
      return () => {
        cancelled = true
      }
    }

    import('vega-embed')
      .then(async ({ default: embed }) => {
        const result = await embed(host, spec, { actions: false, renderer: 'svg' }) as VegaEmbedResult
        if (cancelled) {
          result.view.finalize()
          return
        }
        view = result.view
      })
      .catch((err: unknown) => {
        if (!cancelled) setError(err instanceof Error ? err.message : String(err))
      })

    return () => {
      cancelled = true
      view?.finalize()
      host.replaceChildren()
    }
  }, [code, dataUrl, relativeTo])

  if (error) {
    return (
      <div className="vega-lite-error my-3" role="alert">
        <p>Unable to render Vega-Lite chart: {error}</p>
        <pre>{code}</pre>
      </div>
    )
  }
  return <div ref={hostRef} className="vega-lite my-3" />
}
