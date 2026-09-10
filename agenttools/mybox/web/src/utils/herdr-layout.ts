import type { HerdrLayout, HerdrPaneRect } from '../api/client'

// A pane positioned with CSS percentages relative to the tab's layout area.
export interface PaneBox {
  paneId: string
  left: string
  top: string
  width: string
  height: string
}

// A terminal-cell rectangle.
export type Rect = HerdrPaneRect

export type SplitAxis = 'vertical' | 'horizontal'

// One split divider, positioned as a percentage of the layout area so it can
// be rendered as a draggable handle. `rect` is the split's own region (in
// terminal cells) which ratios are relative to.
export interface Divider extends Rect {
  splitId: string
  axis: SplitAxis
  ratio: number
  pos: string
}

// Structural split tree derived from herdr's flat split/pane description.
export type LayoutTree =
  | { kind: 'pane'; paneId: string; rect: Rect }
  | { kind: 'split'; id: string; axis: SplitAxis; ratio: number; rect: Rect; children: LayoutTree[] }

export interface LayoutBox {
  zoomed: boolean
  focusedPaneId?: string
  aspectRatio: number
  area: Rect
  panes: PaneBox[]
  dividers: Divider[]
}

const CELL_ASPECT = 0.5
export const MIN_RATIO = 0.05
export const MAX_RATIO = 0.95

export interface ResizeRecipe {
  paneId: string
  direction: 'left' | 'up' | 'down' | 'right'
  amount: number
}

export type ResizeDirection = ResizeRecipe['direction']

function pct(value: number): string {
  return `${(value * 100).toFixed(3)}%`
}

function aspectRatioFor(area: Rect): number {
  return (area.width * CELL_ASPECT) / area.height
}

function clampRatio(v: number): number {
  return Math.min(MAX_RATIO, Math.max(MIN_RATIO, v))
}

function rectEquals(a: Rect, b: Rect): boolean {
  return a.x === b.x && a.y === b.y && a.width === b.width && a.height === b.height
}

function rectArea(r: Rect): number {
  return r.width * r.height
}

function rectInside(outer: Rect, inner: Rect): boolean {
  return (
    inner.x >= outer.x &&
    inner.y >= outer.y &&
    inner.x + inner.width <= outer.x + outer.width &&
    inner.y + inner.height <= outer.y + outer.height
  )
}

function intersectionArea(a: Rect, b: Rect): number {
  const w = Math.max(0, Math.min(a.x + a.width, b.x + b.width) - Math.max(a.x, b.x))
  const h = Math.max(0, Math.min(a.y + a.height, b.y + b.height) - Math.max(a.y, b.y))
  return w * h
}

function toRect(r: HerdrPaneRect): Rect {
  return { x: r.x, y: r.y, width: r.width, height: r.height }
}

function childRegions(region: Rect, axis: SplitAxis, ratio: number): [Rect, Rect] {
  if (axis === 'vertical') {
    const leftWidth = Math.round(region.width * ratio)
    const rightWidth = region.width - leftWidth
    return [
      { x: region.x, y: region.y, width: leftWidth, height: region.height },
      { x: region.x + leftWidth, y: region.y, width: rightWidth, height: region.height },
    ]
  }
  const topHeight = Math.round(region.height * ratio)
  const bottomHeight = region.height - topHeight
  return [
    { x: region.x, y: region.y, width: region.width, height: topHeight },
    { x: region.x, y: region.y + topHeight, width: region.width, height: bottomHeight },
  ]
}

// The split governing a region: an exact match first, then the largest split
// rectangle fitting inside the region (covers off-by-one cell rounding).
// Structural node used while building the tree: children are attached purely
// by stored-rect containment and their side is derived from position, so an
// overridden ratio can move/shrink regions without losing nested subdivisions.
interface StructSplit {
  id: string
  axis: SplitAxis
  ratio: number
  rect: Rect
  first: StructSplit | null
  second: StructSplit | null
}

function buildStructTree(layout: HerdrLayout): { root: StructSplit | undefined; rootRect: Rect } {
  const area = toRect(layout.area)
  const splits = layout.splits
  if (splits.length === 0) return { root: undefined, rootRect: area }

  const rootRaw =
    splits.find((s) => rectEquals(toRect(s.rect), area)) ??
    [...splits].sort((a, b) => rectArea(toRect(b.rect)) - rectArea(toRect(a.rect)))[0]

  const nodes = new Map<string, StructSplit>()
  for (const s of splits) {
    nodes.set(s.id, {
      id: s.id,
      axis: s.direction === 'right' ? 'vertical' : 'horizontal',
      ratio: s.ratio,
      rect: toRect(s.rect),
      first: null,
      second: null,
    })
  }

  for (const s of splits) {
    const parent = nodes.get(s.id)!
    const srect = toRect(s.rect)
    // Splits strictly inside this one, not contained in another contained split.
    const contained = splits.filter((o) => o.id !== s.id && rectInside(srect, toRect(o.rect)))
    const direct = contained.filter(
      (o) =>
        !contained.some((p) => p.id !== o.id && rectInside(toRect(o.rect), toRect(p.rect))),
    )
    const div =
      parent.axis === 'vertical'
        ? parent.rect.x + Math.round(parent.rect.width * parent.ratio)
        : parent.rect.y + Math.round(parent.rect.height * parent.ratio)
    for (const d of direct) {
      const child = nodes.get(d.id)!
      const center =
        parent.axis === 'vertical'
          ? child.rect.x + child.rect.width / 2
          : child.rect.y + child.rect.height / 2
      if (center <= div) {
        if (!parent.first) parent.first = child
      } else if (!parent.second) {
        parent.second = child
      }
    }
  }

  return { root: nodes.get(rootRaw.id), rootRect: toRect(rootRaw.rect) }
}

// The pane occupying a region: the one with the largest overlap (regions can
// be a cell off from the real pane rects after a ratio override).
function bestPaneForRegion(panes: HerdrLayout['panes'], region: Rect) {
  let best = panes[0]
  let bestArea = -1
  for (const p of panes) {
    const a = intersectionArea(toRect(p.rect), region)
    if (a > bestArea) {
      best = p
      bestArea = a
    }
  }
  return bestArea > 0 ? best : undefined
}

// splitTreeForLayout builds a binary split tree from herdr's flat description.
// `overrides` lets one or more splits render at an alternative ratio (used to
// preview a drag without waiting for herdr to apply it).
export function splitTreeForLayout(
  layout: HerdrLayout,
  overrides: Record<string, number> = {},
): LayoutTree {
  const { root, rootRect } = buildStructTree(layout)

  const build = (node: StructSplit | null, region: Rect): LayoutTree => {
    if (!node) {
      const pane = bestPaneForRegion(layout.panes, region)
      if (pane) return { kind: 'pane', paneId: pane.pane_id, rect: region }
      // No pane claimable: collapse the region to a void leaf.
      return { kind: 'pane', paneId: '', rect: region }
    }
    const ratio = clampRatio(overrides[node.id] ?? node.ratio)
    const [a, b] = childRegions(region, node.axis, ratio)
    return {
      kind: 'split',
      id: node.id,
      axis: node.axis,
      ratio,
      rect: region,
      children: [build(node.first, a), build(node.second, b)],
    }
  }
  return build(root ?? null, rootRect)
}

function collectPanes(tree: LayoutTree, area: Rect, out: PaneBox[]): void {
  if (tree.kind === 'pane') {
    if (!tree.paneId) return
    out.push({
      paneId: tree.paneId,
      left: pct((tree.rect.x - area.x) / area.width),
      top: pct((tree.rect.y - area.y) / area.height),
      width: pct(tree.rect.width / area.width),
      height: pct(tree.rect.height / area.height),
    })
    return
  }
  collectPanes(tree.children[0], area, out)
  collectPanes(tree.children[1], area, out)
}

function collectDividers(tree: LayoutTree, area: Rect, out: Divider[]): void {
  if (tree.kind === 'pane') return
  const { rect, axis, ratio, id } = tree
  const pos =
    axis === 'vertical'
      ? (rect.x - area.x + Math.round(rect.width * ratio)) / area.width
      : (rect.y - area.y + Math.round(rect.height * ratio)) / area.height
  out.push({ splitId: id, axis, ratio, x: rect.x, y: rect.y, width: rect.width, height: rect.height, pos: pct(pos) })
  collectDividers(tree.children[0], area, out)
  collectDividers(tree.children[1], area, out)
}

// layoutBoxForTab converts one herdr tab layout into absolutely-positioned CSS
// boxes plus draggable divider geometry. `overrides` previews a drag by
// rendering one or more splits at an alternative ratio. Returns undefined when
// the layout carries no usable geometry (callers fall back to a plain grid).
export function layoutBoxForTab(
  layout: HerdrLayout | undefined,
  overrides: Record<string, number> = {},
): LayoutBox | undefined {
  if (!layout) return undefined
  const area = toRect(layout.area)
  if (area.width <= 0 || area.height <= 0) return undefined
  const aspectRatio = aspectRatioFor(area)

  // A zoomed tab exposes only its focused pane filling the whole area.
  if (layout.zoomed) {
    const focused = layout.panes.find((p) => p.pane_id === layout.focused_pane_id) ?? layout.panes[0]
    if (!focused) return undefined
    return {
      zoomed: true,
      focusedPaneId: focused.pane_id,
      aspectRatio,
      area,
      panes: [{ paneId: focused.pane_id, left: '0%', top: '0%', width: '100%', height: '100%' }],
      dividers: [],
    }
  }

  const panes: PaneBox[] = []
  const dividers: Divider[] = []
  collectPanes(splitTreeForLayout(layout, overrides), area, panes)
  if (panes.length === 0) return undefined
  // A pane can only be claimed by its own leaf; tolerate transient ambiguity
  // mid-drag by keeping the first placement.
  const seen = new Set<string>()
  const unique = panes.filter((p) => (seen.has(p.paneId) ? false : (seen.add(p.paneId), true)))
  collectDividers(splitTreeForLayout(layout, overrides), area, dividers)
  return { zoomed: false, aspectRatio, area, panes: unique, dividers }
}

// edgeNeighborsForSplit returns the panes directly on each side of a split's
// divider and the herdr edge direction that grows them across the divider.
// Positive delta ratio (divider moves right/down) grows the leading pane with
// `right`/`down`; negative delta grows the trailing pane with `left`/`up`.
export function edgeNeighborsForSplit(
  layout: HerdrLayout,
  splitId: string,
): { positive: ResizeRecipe; negative: ResizeRecipe } | undefined {
  const split = layout.splits.find((s) => s.id === splitId)
  if (!split) return undefined
  const axis: SplitAxis = split.direction === 'right' ? 'vertical' : 'horizontal'
  const region = toRect(split.rect)
  const div = axis === 'vertical' ? region.x + Math.round(region.width * split.ratio) : region.y + Math.round(region.height * split.ratio)
  const inRegion = layout.panes.filter((p) => {
    const r = toRect(p.rect)
    return r.x >= region.x && r.y >= region.y && r.x + r.width <= region.x + region.width && r.y + r.height <= region.y + region.height
  })
  const perpendicular = (r: Rect) =>
    axis === 'vertical'
      ? Math.max(0, Math.min(r.y + r.height, region.y + region.height) - Math.max(r.y, region.y))
      : Math.max(0, Math.min(r.x + r.width, region.x + region.width) - Math.max(r.x, region.x))
  const best = (target: (p: HerdrLayout['panes'][number], r: Rect) => boolean) =>
    inRegion
      .filter((p) => target(p, toRect(p.rect)))
      .sort((a, b) => perpendicular(toRect(b.rect)) - perpendicular(toRect(a.rect)))[0]

  let positive: { paneId: string; direction: ResizeDirection }
  let negative: { paneId: string; direction: ResizeDirection }
  if (axis === 'vertical') {
    const left = best((_p, r) => r.x + r.width === div)
    const right = best((_p, r) => r.x === div)
    if (!left || !right) return undefined
    positive = { paneId: left.pane_id, direction: 'right' }
    negative = { paneId: right.pane_id, direction: 'left' }
  } else {
    const top = best((_p, r) => r.y + r.height === div)
    const bottom = best((_p, r) => r.y === div)
    if (!top || !bottom) return undefined
    positive = { paneId: top.pane_id, direction: 'down' }
    negative = { paneId: bottom.pane_id, direction: 'up' }
  }
  return { positive: { ...positive, amount: 0 }, negative: { ...negative, amount: 0 } }
}

// resizeRecipeForPane converts an arrow-button resize (a pane edge + a number
// of cells) into the herdr (split, pane, direction, ratio-delta) that moves
// that edge, mirroring herdr's own walk up the split tree. Returns undefined
// when the pane's edge is a window border (nothing to resize).
export function resizeRecipeForPane(
  layout: HerdrLayout,
  paneId: string,
  direction: ResizeDirection,
  cells: number,
): ResizeRecipe | undefined {
  const pane = layout.panes.find((p) => p.pane_id === paneId)
  if (!pane) return undefined
  const r = toRect(pane.rect)

  // The edge being moved belongs to the innermost split whose divider sits at
  // that edge (mirroring herdr's walk up the split tree).
  type Candidate = ResizeRecipe & { regionArea: number }
  const candidates: Candidate[] = []
  for (const s of layout.splits) {
    const axis: SplitAxis = s.direction === 'right' ? 'vertical' : 'horizontal'
    const region = toRect(s.rect)
    const inside = r.x >= region.x && r.y >= region.y && r.x + r.width <= region.x + region.width && r.y + r.height <= region.y + region.height
    if (!inside) continue
    const div = axis === 'vertical' ? region.x + Math.round(region.width * s.ratio) : region.y + Math.round(region.height * s.ratio)
    const isVertical = axis === 'vertical'
    const base = (paneDir: ResizeDirection) => ({
      paneId,
      direction: paneDir,
      amount: cells / (isVertical ? region.width : region.height),
      regionArea: rectArea(region),
    })
    if (direction === 'right' && isVertical && r.x + r.width === div && r.x < div) {
      candidates.push(base('right'))
    } else if (direction === 'left' && isVertical && r.x === div && r.x + r.width > div) {
      candidates.push(base('left'))
    } else if (direction === 'down' && !isVertical && r.y + r.height === div && r.y < div) {
      candidates.push(base('down'))
    } else if (direction === 'up' && !isVertical && r.y === div && r.y + r.height > div) {
      candidates.push(base('up'))
    }
  }
  if (candidates.length === 0) return undefined
  candidates.sort((a, b) => a.regionArea - b.regionArea)
  const { regionArea: _ignore, ...recipe } = candidates[0]
  return recipe
}

export const RESIZE_DIRECTIONS = ['left', 'up', 'down', 'right'] as const

export const RESIZE_DIRECTION_LABELS: Record<ResizeDirection, string> = {
  left: '◀',
  up: '▲',
  down: '▼',
  right: '▶',
}