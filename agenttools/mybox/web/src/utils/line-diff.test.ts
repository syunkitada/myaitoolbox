import { describe, expect, it } from 'vitest'
import { computeLineDiff } from './line-diff'

describe('computeLineDiff', () => {
  it('returns added lines for a small edit', () => {
    expect(computeLineDiff('one', 'one\nnew')).toEqual({
      added: new Set([1]),
      changed: new Set(),
    })
  })

  it('skips the quadratic calculation for oversized inputs', () => {
    const original = Array.from({ length: 2_000 }, (_, i) => `original-${i}`).join('\n')
    const modified = Array.from({ length: 2_000 }, (_, i) => `modified-${i}`).join('\n')

    expect(computeLineDiff(original, modified)).toBeNull()
  })
})
