import { useCallback, useEffect, useRef, useState, type KeyboardEvent as ReactKeyboardEvent, type PointerEvent as ReactPointerEvent } from 'react'

export const MIN_RESIZABLE_WIDTH = 180
export const MAX_RESIZABLE_WIDTH = 480

interface UseResizableWidthOptions {
  storageKey: string
  defaultWidth: number
  minWidth?: number
  maxWidth?: number
  handleSide?: 'left' | 'right'
}

interface UseResizableWidthResult {
  width: number
  resizing: boolean
  handlePointerDown: (event: ReactPointerEvent<HTMLDivElement>) => void
  handleKeyDown: (event: ReactKeyboardEvent<HTMLDivElement>) => void
}

function clampWidth(width: number, minWidth: number, maxWidth: number): number {
  return Math.min(maxWidth, Math.max(minWidth, width))
}

function readWidth({ storageKey, defaultWidth, minWidth, maxWidth }: UseResizableWidthOptions): number {
  if (typeof window === 'undefined') return clampWidth(defaultWidth, minWidth ?? MIN_RESIZABLE_WIDTH, maxWidth ?? MAX_RESIZABLE_WIDTH)
  const raw = window.localStorage.getItem(storageKey)
  if (raw === null) return clampWidth(defaultWidth, minWidth ?? MIN_RESIZABLE_WIDTH, maxWidth ?? MAX_RESIZABLE_WIDTH)
  const saved = Number(raw)
  return Number.isFinite(saved)
    ? clampWidth(saved, minWidth ?? MIN_RESIZABLE_WIDTH, maxWidth ?? MAX_RESIZABLE_WIDTH)
    : clampWidth(defaultWidth, minWidth ?? MIN_RESIZABLE_WIDTH, maxWidth ?? MAX_RESIZABLE_WIDTH)
}

export function useResizableWidth({
  storageKey,
  defaultWidth,
  minWidth = MIN_RESIZABLE_WIDTH,
  maxWidth = MAX_RESIZABLE_WIDTH,
  handleSide = 'right',
}: UseResizableWidthOptions): UseResizableWidthResult {
  const [width, setWidth] = useState(() => readWidth({ storageKey, defaultWidth, minWidth, maxWidth }))
  const [resizing, setResizing] = useState(false)
  const resizeStart = useRef<{ clientX: number; width: number } | null>(null)

  useEffect(() => {
    window.localStorage.setItem(storageKey, String(width))
  }, [storageKey, width])

  useEffect(() => {
    if (!resizing) return
    const previousUserSelect = document.body.style.userSelect
    const previousCursor = document.body.style.cursor
    document.body.style.userSelect = 'none'
    document.body.style.cursor = 'col-resize'
    return () => {
      document.body.style.userSelect = previousUserSelect
      document.body.style.cursor = previousCursor
    }
  }, [resizing])

  const handlePointerDown = useCallback(
    (event: ReactPointerEvent<HTMLDivElement>) => {
      if (event.button !== 0) return
      event.preventDefault()
      resizeStart.current = { clientX: event.clientX, width }
      setResizing(true)
    },
    [width],
  )

  useEffect(() => {
    if (!resizing) return
    const handlePointerMove = (event: PointerEvent) => {
      const start = resizeStart.current
      if (!start) return
      const delta = handleSide === 'left'
        ? start.clientX - event.clientX
        : event.clientX - start.clientX
      setWidth(clampWidth(start.width + delta, minWidth, maxWidth))
    }
    const finishResize = () => {
      resizeStart.current = null
      setResizing(false)
    }
    window.addEventListener('pointermove', handlePointerMove)
    window.addEventListener('pointerup', finishResize)
    window.addEventListener('pointercancel', finishResize)
    return () => {
      window.removeEventListener('pointermove', handlePointerMove)
      window.removeEventListener('pointerup', finishResize)
      window.removeEventListener('pointercancel', finishResize)
    }
  }, [handleSide, maxWidth, minWidth, resizing])

  const handleKeyDown = useCallback(
    (event: ReactKeyboardEvent<HTMLDivElement>) => {
      let nextWidth: number | null = null
      if (event.key === 'ArrowLeft') nextWidth = width + (handleSide === 'left' ? 10 : -10)
      if (event.key === 'ArrowRight') nextWidth = width + (handleSide === 'left' ? -10 : 10)
      if (event.key === 'Home') nextWidth = minWidth
      if (event.key === 'End') nextWidth = maxWidth
      if (nextWidth === null) return
      event.preventDefault()
      setWidth(clampWidth(nextWidth, minWidth, maxWidth))
    },
    [handleSide, maxWidth, minWidth, width],
  )

  return { width, resizing, handlePointerDown, handleKeyDown }
}
