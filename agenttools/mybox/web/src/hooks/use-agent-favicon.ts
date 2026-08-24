import { useEffect } from 'react'
import type { HerdrOverview } from '../api/client'

// Hex values mirror AGENT_STATUS_CLASSES in components/herdr-status.tsx.
const STATUS_COLORS: Record<string, string> = {
  working: '#3b82f6',
  blocked: '#ef4444',
  idle: '#10b981',
  done: '#8b5cf6',
  unknown: '#a1a1aa',
}

// Attention order: the favicon shows the most demanding status first.
const STATUS_PRIORITY = ['blocked', 'working', 'idle', 'done', 'unknown']

const FAVICON_SELECTOR = 'link[rel="icon"]'

// aggregateAgentStatus reduces the overview's agent statuses to a single
// favicon-worthy status; unknown covers "no data" and unrecognized values.
export function aggregateAgentStatus(overview: HerdrOverview | null | undefined): string {
  const statuses = (overview?.available ? overview.agents : []).map((a) => a.status || 'unknown')
  for (const status of STATUS_PRIORITY) {
    if (statuses.includes(status)) return status
  }
  return 'unknown'
}

// renderStatusIcon draws the status dot as a PNG data URL. Returns null when
// canvas is unavailable (e.g. jsdom), leaving any current favicon untouched.
export function renderStatusIcon(status: string): string | null {
  const size = 64
  const canvas = document.createElement('canvas')
  canvas.width = size
  canvas.height = size
  const ctx = canvas.getContext('2d')
  if (!ctx) return null
  ctx.beginPath()
  ctx.arc(size / 2, size / 2, size * 0.42, 0, Math.PI * 2)
  ctx.fillStyle = STATUS_COLORS[status] ?? STATUS_COLORS.unknown
  ctx.fill()
  // Subtle ring keeps the dot visible on light and dark tab strips.
  ctx.lineWidth = size * 0.05
  ctx.strokeStyle = 'rgba(255,255,255,0.45)'
  ctx.stroke()
  return canvas.toDataURL('image/png')
}

function setFavicon(href: string) {
  let link = document.head.querySelector<HTMLLinkElement>(FAVICON_SELECTOR)
  if (!link) {
    link = document.createElement('link')
    link.rel = 'icon'
    document.head.appendChild(link)
  }
  if (link.getAttribute('href') === href) return
  link.setAttribute('href', href)
}

// useAgentFavicon mirrors the herdr agents' aggregated status in the browser
// tab icon. It re-renders whenever useHerdrOverview polls fresh data.
export function useAgentFavicon(overview: HerdrOverview | null | undefined) {
  const status = aggregateAgentStatus(overview)
  useEffect(() => {
    const href = renderStatusIcon(status)
    if (href) setFavicon(href)
  }, [status])
}
