import { useEffect, useRef } from 'react'

type EscapeHandler = () => void

interface EscapeEntry {
  id: number
  handler: EscapeHandler
}

const escapeEntries: EscapeEntry[] = []
let nextEscapeEntryId = 0

function handleDocumentKeyDown(event: KeyboardEvent) {
  if (event.key !== 'Escape') return

  const entry = escapeEntries[escapeEntries.length - 1]
  if (!entry) return

  event.preventDefault()
  event.stopPropagation()
  entry.handler()
}

function registerEscapeHandler(handler: EscapeHandler) {
  const entry = { id: nextEscapeEntryId++, handler }
  if (escapeEntries.length === 0) {
    document.addEventListener('keydown', handleDocumentKeyDown, true)
  }
  escapeEntries.push(entry)

  return () => {
    const index = escapeEntries.findIndex((candidate) => candidate.id === entry.id)
    if (index < 0) return
    escapeEntries.splice(index, 1)
    if (escapeEntries.length === 0) {
      document.removeEventListener('keydown', handleDocumentKeyDown, true)
    }
  }
}

/** Registers an ESC handler while the associated modal or overlay is open. */
export function useEscapeKey(handler: EscapeHandler, enabled = true) {
  const handlerRef = useRef(handler)
  handlerRef.current = handler

  useEffect(() => {
    if (!enabled) return
    return registerEscapeHandler(() => handlerRef.current())
  }, [enabled])
}
