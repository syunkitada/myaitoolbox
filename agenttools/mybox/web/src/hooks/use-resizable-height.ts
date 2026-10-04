import { useCallback, useEffect, useRef, useState } from 'react'

export const MIN_RESIZABLE_HEIGHT = 40
export const HERDR_AGENT_OUTPUT_HEIGHT_STORAGE_KEY = 'mybox:herdr-agent-output-height'
export const HERDR_AGENT_PROMPT_HEIGHT_STORAGE_KEY = 'mybox:herdr-agent-prompt-height'
export const AGENT_SIDEBAR_PROMPT_HEIGHT_STORAGE_KEY = 'mybox:agent-sidebar-prompt-height'

interface UseResizableHeightOptions {
  storageKey: string
  defaultHeight: number
  minHeight?: number
  maxHeight?: number
}

function clampHeight(height: number, minHeight: number, maxHeight: number): number {
  return Math.min(maxHeight, Math.max(minHeight, height))
}

function readHeight({ storageKey, defaultHeight, minHeight, maxHeight }: UseResizableHeightOptions): number {
  const min = minHeight ?? MIN_RESIZABLE_HEIGHT
  const max = maxHeight ?? Number.POSITIVE_INFINITY
  const fallback = clampHeight(defaultHeight, min, max)
  if (typeof window === 'undefined') return fallback

  try {
    const raw = window.localStorage.getItem(storageKey)
    if (raw === null) return fallback
    const saved = Number(raw)
    return Number.isFinite(saved) ? clampHeight(saved, min, max) : fallback
  } catch {
    return fallback
  }
}

function measuredHeight(entry: ResizeObserverEntry): number {
  const borderBoxSize = Array.isArray(entry.borderBoxSize) ? entry.borderBoxSize[0] : entry.borderBoxSize
  if (borderBoxSize && borderBoxSize.blockSize > 0) return borderBoxSize.blockSize

  const rectHeight = entry.target.getBoundingClientRect().height
  return rectHeight > 0 ? rectHeight : entry.contentRect.height
}

export function useResizableHeight<T extends HTMLElement>({
  storageKey,
  defaultHeight,
  minHeight = MIN_RESIZABLE_HEIGHT,
  maxHeight = Number.POSITIVE_INFINITY,
}: UseResizableHeightOptions) {
  const heightRef = useRef(0)
  const [element, setElement] = useState<T | null>(null)
  const [height, setHeight] = useState(() => {
    const initial = readHeight({ storageKey, defaultHeight, minHeight, maxHeight })
    heightRef.current = initial
    return initial
  })

  useEffect(() => {
    heightRef.current = height
    try {
      window.localStorage.setItem(storageKey, String(height))
    } catch {
      // Ignore unavailable or full local storage.
    }
  }, [height, storageKey])

  useEffect(() => {
    if (!element || typeof ResizeObserver === 'undefined') return

    const observer = new ResizeObserver((entries) => {
      const entry = entries[0]
      if (!entry) return
      const measured = measuredHeight(entry)
      if (!Number.isFinite(measured) || measured <= 0) return
      const next = clampHeight(Math.round(measured), minHeight, maxHeight)
      if (next === heightRef.current) return
      heightRef.current = next
      setHeight(next)
    })
    observer.observe(element)
    return () => observer.disconnect()
  }, [element, maxHeight, minHeight])

  const ref = useCallback((nextElement: T | null) => setElement(nextElement), [])
  return { height, ref }
}
