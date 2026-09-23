import type { NodeId } from '../graph/types'

const GRAPH_CAMERA_KEY = 'mybox_graph_camera'
const GRAPH_LAYOUT_KEY = 'mybox_graph_layout'

export interface PersistedPosition {
  x: number
  y: number
  fixed?: boolean
}

export interface CameraView {
  x: number
  y: number
  angle: number
  ratio: number
}

type CameraStore = Record<string, CameraView>
type LayoutStore = Record<string, Record<NodeId, PersistedPosition>>

function read<T>(key: string): T | null {
  try {
    const raw = window.localStorage.getItem(key)
    if (!raw) return null
    return JSON.parse(raw) as T
  } catch {
    return null
  }
}

function write(key: string, value: unknown): void {
  try {
    window.localStorage.setItem(key, JSON.stringify(value))
  } catch {
    // ignore quota / privacy errors
  }
}

export function loadCamera(project: string): CameraView | null {
  const store = read<CameraStore>(GRAPH_CAMERA_KEY)
  const state = store?.[project]
  if (!state) return null
  if (
    typeof state.x !== 'number' ||
    typeof state.y !== 'number' ||
    typeof state.angle !== 'number' ||
    typeof state.ratio !== 'number'
  ) {
    return null
  }
  return { x: state.x, y: state.y, angle: state.angle, ratio: state.ratio }
}

export function saveCamera(project: string, state: CameraView): void {
  const store = read<CameraStore>(GRAPH_CAMERA_KEY) ?? {}
  store[project] = state
  write(GRAPH_CAMERA_KEY, store)
}

export function loadLayout(project: string): Record<NodeId, PersistedPosition> | null {
  const store = read<LayoutStore>(GRAPH_LAYOUT_KEY)
  return store?.[project] ?? null
}

export function saveLayout(
  project: string,
  layout: Record<NodeId, PersistedPosition>,
): void {
  const store = read<LayoutStore>(GRAPH_LAYOUT_KEY) ?? {}
  store[project] = layout
  write(GRAPH_LAYOUT_KEY, store)
}