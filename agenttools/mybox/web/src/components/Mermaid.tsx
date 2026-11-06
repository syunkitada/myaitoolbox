import { useEffect, useRef, useState, type PointerEvent as ReactPointerEvent } from 'react'
import { createPortal } from 'react-dom'
import { Maximize2, X } from 'lucide-react'
import { useEscapeKey } from '../hooks/use-escape-key'
import { Button } from './ui/button'

interface MermaidProps {
  code: string
}

const CACHE_LIMIT = 100
const MIN_ZOOM = 0.25
const MAX_ZOOM = 6

interface DiagramView {
  scale: number
  x: number
  y: number
}

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
  const expandedHostRef = useRef<HTMLDivElement>(null)
  const viewportRef = useRef<HTMLDivElement>(null)
  const dragRef = useRef<{ clientX: number; clientY: number; x: number; y: number } | null>(null)
  const [error, setError] = useState(false)
  const [svg, setSvg] = useState<string | null>(null)
  const [expanded, setExpanded] = useState(false)
  const [diagramView, setDiagramView] = useState<DiagramView>({ scale: 1, x: 0, y: 0 })
  const [dragging, setDragging] = useState(false)

  useEscapeKey(() => setExpanded(false), expanded)

  useEffect(() => {
    const host = hostRef.current
    if (!host) return
    setError(false)
    setSvg(null)
    if (cache.has(code)) {
      const cachedSvg = cache.get(code)!
      host.innerHTML = cachedSvg
      setSvg(cachedSvg)
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
        if (!cancelled && hostRef.current) {
          hostRef.current.innerHTML = svg
          setSvg(svg)
        }
      })
      .catch((err) => {
        console.error('[Mermaid] render failed:', err)
        if (!cancelled) setError(true)
      })
    return () => {
      cancelled = true
    }
  }, [code])

  useEffect(() => {
    const diagram = expandedHostRef.current?.querySelector('svg')
    if (!diagram) return
    diagram.style.width = '100%'
    diagram.style.height = '100%'
    diagram.style.maxWidth = 'none'
    diagram.style.maxHeight = 'none'
  }, [expanded, svg])

  useEffect(() => {
    const viewport = viewportRef.current
    if (!expanded || !viewport) return

    const handleWheel = (event: WheelEvent) => {
      event.preventDefault()
      setDiagramView((current) => {
        const bounds = viewport.getBoundingClientRect()
        const pointerX = event.clientX - bounds.left - bounds.width / 2
        const pointerY = event.clientY - bounds.top - bounds.height / 2
        const nextScale = Math.min(MAX_ZOOM, Math.max(MIN_ZOOM, current.scale * Math.exp(-event.deltaY * 0.001)))
        const scaleRatio = nextScale / current.scale

        return {
          scale: nextScale,
          x: pointerX + (current.x - pointerX) * scaleRatio,
          y: pointerY + (current.y - pointerY) * scaleRatio,
        }
      })
    }

    viewport.addEventListener('wheel', handleWheel, { passive: false })
    return () => viewport.removeEventListener('wheel', handleWheel)
  }, [expanded])

  const resetDiagramView = () => {
    dragRef.current = null
    setDragging(false)
    setDiagramView({ scale: 1, x: 0, y: 0 })
  }

  const handlePointerDown = (event: ReactPointerEvent<HTMLDivElement>) => {
    if (event.button !== undefined && event.button !== 0) return
    event.preventDefault()
    dragRef.current = {
      clientX: event.clientX,
      clientY: event.clientY,
      x: diagramView.x,
      y: diagramView.y,
    }
    setDragging(true)
    event.currentTarget.setPointerCapture?.(event.pointerId)
  }

  const handlePointerMove = (event: ReactPointerEvent<HTMLDivElement>) => {
    const drag = dragRef.current
    if (!drag) return
    setDiagramView((current) => ({
      ...current,
      x: drag.x + event.clientX - drag.clientX,
      y: drag.y + event.clientY - drag.clientY,
    }))
  }

  const handlePointerEnd = () => {
    dragRef.current = null
    setDragging(false)
  }

  const openExpanded = () => {
    resetDiagramView()
    setExpanded(true)
  }

  if (error) return <pre className="mermaid-error my-3 font-mono text-red-600 whitespace-pre-wrap">{code}</pre>

  const closeExpanded = () => setExpanded(false)

  return (
    <>
      <div className="group relative my-3">
        <div ref={hostRef} className="mermaid" />
        {svg && (
          <Button
            type="button"
            variant="outline"
            size="icon-sm"
            className="absolute right-2 top-2 z-10 bg-background/90 text-muted-foreground opacity-80 shadow-sm hover:opacity-100"
            aria-label="Expand Mermaid diagram"
            title="Expand diagram"
            onClick={openExpanded}
          >
            <Maximize2 className="size-4" />
          </Button>
        )}
      </div>
      {expanded && svg && createPortal(
        <div
          className="fixed inset-0 z-[80] flex items-center justify-center bg-black/60 p-2 sm:p-4"
          onMouseDown={(event) => {
            if (event.target === event.currentTarget) closeExpanded()
          }}
        >
          <section
            className="flex h-[92vh] w-[96vw] max-w-[1400px] flex-col overflow-hidden rounded-lg border bg-card shadow-2xl"
            role="dialog"
            aria-modal="true"
            aria-label="Expanded Mermaid diagram"
          >
            <div className="flex shrink-0 items-center gap-2 border-b border-border px-2 py-1.5">
              <span className="mr-auto px-2 text-xs text-muted-foreground">Scroll to zoom · drag to pan</span>
              <Button
                type="button"
                variant="ghost"
                size="sm"
                aria-label="Reset Mermaid view"
                onClick={resetDiagramView}
              >
                Reset view
              </Button>
              <Button
                type="button"
                variant="ghost"
                size="icon-sm"
                aria-label="Close expanded diagram"
                title="Close diagram"
                onClick={closeExpanded}
              >
                <X className="size-4" />
              </Button>
            </div>
            <div className="min-h-0 flex-1 p-3 sm:p-5">
              <div
                ref={viewportRef}
                className={`h-full w-full overflow-hidden touch-none select-none ${dragging ? 'cursor-grabbing' : 'cursor-grab'}`}
                role="region"
                aria-label="Mermaid diagram viewport"
                onPointerDown={handlePointerDown}
                onPointerMove={handlePointerMove}
                onPointerUp={handlePointerEnd}
                onPointerCancel={handlePointerEnd}
                onLostPointerCapture={handlePointerEnd}
              >
                <div
                  ref={expandedHostRef}
                  className="flex h-full w-full items-center justify-center [&>svg]:max-h-full [&>svg]:max-w-full"
                  data-testid="mermaid-zoom-surface"
                  style={{
                    transform: `translate(${diagramView.x}px, ${diagramView.y}px) scale(${diagramView.scale})`,
                    transformOrigin: 'center',
                  }}
                  dangerouslySetInnerHTML={{ __html: svg }}
                />
              </div>
            </div>
          </section>
        </div>,
        document.body,
      )}
    </>
  )
}
