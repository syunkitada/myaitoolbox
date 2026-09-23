import { describe, expect, it } from 'vitest'
import {
  edgeNeighborsForSplit,
  layoutBoxForTab,
  paneColumnWidth,
  resizeRecipeForPane,
  splitTreeForLayout,
} from './herdr-layout'
import type { HerdrLayout } from '../api/client'

const splitLayout: HerdrLayout = {
  tab_id: 'w9:t1',
  workspace_id: 'w9',
  focused_pane_id: 'w9:p1',
  zoomed: false,
  area: { height: 47, width: 235, x: 32, y: 1 },
  panes: [
    { pane_id: 'w9:p1', focused: true, rect: { height: 47, width: 118, x: 32, y: 1 } },
    { pane_id: 'w9:p9', focused: false, rect: { height: 47, width: 117, x: 150, y: 1 } },
  ],
  splits: [
    {
      id: 'split_0_root',
      direction: 'right',
      ratio: 0.5,
      rect: { height: 47, width: 235, x: 32, y: 1 },
    },
  ],
}

// Real herdr snapshot geometry (nested: right split with a down split inside).
const nestedLayout: HerdrLayout = {
  tab_id: 'w9:t6',
  workspace_id: 'w9',
  focused_pane_id: 'w9:p6',
  zoomed: false,
  area: { height: 47, width: 235, x: 32, y: 1 },
  panes: [
    { pane_id: 'w9:p6', focused: true, rect: { height: 47, width: 97, x: 32, y: 1 } },
    { pane_id: 'w9:pB', focused: false, rect: { height: 24, width: 138, x: 129, y: 1 } },
    { pane_id: 'w9:pC', focused: false, rect: { height: 23, width: 138, x: 129, y: 25 } },
  ],
  splits: [
    {
      id: 'split_0_root',
      direction: 'right',
      ratio: 0.41276595,
      rect: { height: 47, width: 235, x: 32, y: 1 },
    },
    {
      id: 'split_1_1',
      direction: 'down',
      ratio: 0.5,
      rect: { height: 47, width: 138, x: 129, y: 1 },
    },
  ],
}

describe('layoutBoxForTab', () => {
  it('returns undefined without a layout', () => {
    expect(layoutBoxForTab(undefined)).toBeUndefined()
  })

  it('returns undefined for a degenerate area', () => {
    expect(
      layoutBoxForTab({ ...splitLayout, area: { height: 0, width: 0, x: 0, y: 0 } }),
    ).toBeUndefined()
  })

  it('converts split spans to percentages relative to the area origin', () => {
    const box = layoutBoxForTab(splitLayout)
    expect(box).toBeDefined()
    expect(box!.zoomed).toBe(false)
    expect(box!.aspectRatio).toBeCloseTo((235 * 0.5) / 47, 5)

    expect(box!.panes).toHaveLength(2)
    const [left, right] = box!.panes
    expect(left.paneId).toBe('w9:p1')
    expect(left.left).toBe('0.000%')
    expect(left.top).toBe('0.000%')
    expect(left.width).toBe('50.213%')
    expect(left.height).toBe('100.000%')
    expect(right.paneId).toBe('w9:p9')
    expect(right.left).toBe('50.213%')
    expect(right.width).toBe('49.787%')

    expect(box!.dividers).toHaveLength(1)
    expect(box!.dividers[0].splitId).toBe('split_0_root')
    expect(box!.dividers[0].axis).toBe('vertical')
    expect(box!.dividers[0].pos).toBe('50.213%')
  })

  it('only exposes the focused pane when a tab is zoomed', () => {
    const zoomed: HerdrLayout = { ...splitLayout, zoomed: true, focused_pane_id: 'w9:p9' }
    const box = layoutBoxForTab(zoomed)!
    expect(box.zoomed).toBe(true)
    expect(box.focusedPaneId).toBe('w9:p9')
    expect(box.dividers).toHaveLength(0)
    expect(box.panes).toEqual([
      { paneId: 'w9:p9', left: '0%', top: '0%', width: '100%', height: '100%' },
    ])
  })

  it('skips panes without a usable rect', () => {
    const partial: HerdrLayout = {
      ...splitLayout,
      panes: [{ pane_id: 'w9:p1', focused: true, rect: { height: 47, width: 0, x: 32, y: 1 } }],
    }
    expect(layoutBoxForTab(partial)).toBeUndefined()
  })
})

describe('splitTreeForLayout', () => {
  it('reproduces the nested split tree', () => {
    const tree = splitTreeForLayout(nestedLayout)
    expect(tree.kind).toBe('split')
    if (tree.kind !== 'split') return
    expect(tree.id).toBe('split_0_root')
    expect(tree.axis).toBe('vertical')
    expect(tree.ratio).toBeCloseTo(0.41276595, 5)

    const [left, right] = tree.children
    expect(left.kind).toBe('pane')
    if (right.kind !== 'split') throw new Error('right child should be a split')
    expect(right.id).toBe('split_1_1')
    expect(right.axis).toBe('horizontal')
    expect(right.ratio).toBe(0.5)

    const [top, bottom] = right.children
    expect(top.kind === 'pane' && top.paneId).toBe('w9:pB')
    expect(bottom.kind === 'pane' && bottom.paneId).toBe('w9:pC')
    expect(top.kind === 'pane' && top.rect.height).toBe(24)
  })

  it('lays out a nested tree at the real sizes', () => {
    const box = layoutBoxForTab(nestedLayout)!
    const p6 = box.panes.find((p) => p.paneId === 'w9:p6')!
    const pB = box.panes.find((p) => p.paneId === 'w9:pB')!
    const pC = box.panes.find((p) => p.paneId === 'w9:pC')!
    expect(p6.width).toBe('41.277%')
    expect(p6.height).toBe('100.000%')
    expect(pB.left).toBe('41.277%')
    expect(pB.width).toBe('58.723%')
    expect(pB.height).toBe('51.064%')
    expect(pC.top).toBe('51.064%')
    expect(pC.height).toBe('48.936%')

    const [rootDiv, downDiv] = box.dividers
    expect(rootDiv.pos).toBe('41.277%')
    expect(rootDiv.axis).toBe('vertical')
    expect(downDiv.pos).toBe('51.064%')
    expect(downDiv.axis).toBe('horizontal')
  })

  it('previews a split at an overridden ratio', () => {
    const box = layoutBoxForTab(nestedLayout, { split_0_root: 0.6 })!
    const p6 = box.panes.find((p) => p.paneId === 'w9:p6')!
    const pB = box.panes.find((p) => p.paneId === 'w9:pB')!
    const pC = box.panes.find((p) => p.paneId === 'w9:pC')!
    expect(p6.width).toBe('60.000%')
    expect(pB.left).toBe('60.000%')
    expect(pB.width).toBe('40.000%')
    expect(pC.left).toBe('60.000%')
    // inner down-split still rendered at its own ratio (24 of 47 cells tall)
    expect(pB.height).toBe('51.064%')
    expect(pC.top).toBe('51.064%')
    expect(pC.height).toBe('48.936%')
  })

  it('clamps an overridden ratio', () => {
    const box = layoutBoxForTab(splitLayout, { split_0_root: 1.5 })!
    const left = box.panes.find((p) => p.paneId === 'w9:p1')!
    // 0.95*235 rounds down to 223 of 235 cells.
    expect(left.width).toBe('94.894%')
  })
})

describe('edgeNeighborsForSplit', () => {
  it('maps a vertical split to its left/right edges', () => {
    const edges = edgeNeighborsForSplit(splitLayout, 'split_0_root')!
    expect(edges.positive).toMatchObject({ paneId: 'w9:p1', direction: 'right' })
    expect(edges.negative).toMatchObject({ paneId: 'w9:p9', direction: 'left' })
  })

  it('maps a nested down split to its top/bottom edges', () => {
    const edges = edgeNeighborsForSplit(nestedLayout, 'split_1_1')!
    expect(edges.positive).toMatchObject({ paneId: 'w9:pB', direction: 'down' })
    expect(edges.negative).toMatchObject({ paneId: 'w9:pC', direction: 'up' })
  })

  it('returns undefined for an unknown split', () => {
    expect(edgeNeighborsForSplit(nestedLayout, 'split_nope')).toBeUndefined()
  })
})

describe('paneColumnWidth', () => {
  it('returns the column width of a pane from a layout', () => {
    expect(paneColumnWidth([splitLayout, nestedLayout], 'w9:p1')).toBe(118)
    expect(paneColumnWidth([nestedLayout], 'w9:pC')).toBe(138)
  })

  it('returns undefined for a missing or degenerate width', () => {
    expect(paneColumnWidth(undefined, 'w9:p1')).toBeUndefined()
    expect(paneColumnWidth([], 'w9:p1')).toBeUndefined()
    expect(paneColumnWidth([nestedLayout], 'w9:missing')).toBeUndefined()
    expect(
      paneColumnWidth(
        [{ ...nestedLayout, panes: [{ pane_id: 'w9:pC', focused: false, rect: { height: 23, width: 0, x: 129, y: 25 } }] }],
        'w9:pC',
      ),
    ).toBeUndefined()
  })
})

describe('resizeRecipeForPane', () => {
  it('grows the left pane rightward', () => {
    const recipe = resizeRecipeForPane(splitLayout, 'w9:p1', 'right', 10)!
    expect(recipe).toMatchObject({ paneId: 'w9:p1', direction: 'right' })
    expect(recipe.amount).toBeCloseTo(10 / 235, 5)
  })

  it('grows the right pane leftward', () => {
    const recipe = resizeRecipeForPane(splitLayout, 'w9:p9', 'left', 5)!
    expect(recipe).toMatchObject({ paneId: 'w9:p9', direction: 'left' })
  })

  it('grows the top pane downward in a nested split', () => {
    const recipe = resizeRecipeForPane(nestedLayout, 'w9:pB', 'down', 3)!
    expect(recipe).toMatchObject({ paneId: 'w9:pB', direction: 'down' })
    expect(recipe.amount).toBeCloseTo(3 / 47, 5)
  })

  it('grows the bottom pane upward in a nested split', () => {
    const recipe = resizeRecipeForPane(nestedLayout, 'w9:pC', 'up', 3)!
    expect(recipe).toMatchObject({ paneId: 'w9:pC', direction: 'up' })
  })

  it('returns undefined when the pane edge is a window border', () => {
    expect(resizeRecipeForPane(splitLayout, 'w9:p1', 'left', 5)).toBeUndefined()
    expect(resizeRecipeForPane(splitLayout, 'w9:p1', 'up', 5)).toBeUndefined()
  })
})