export interface LineDiff {
  added: Set<number>
  changed: Set<number>
}

// The dynamic-programming table uses O(originalLines * modifiedLines) memory
// and time. A live editor must remain responsive even when a large file is
// opened, so omit inline decorations beyond this bounded workload.
const MAX_DIFF_CELLS = 1_000_000

export function computeLineDiff(original: string, modified: string): LineDiff | null {
  const a = original.split('\n')
  const b = modified.split('\n')
  const n = a.length
  const m = b.length
  if ((n + 1) * (m + 1) > MAX_DIFF_CELLS) return null

  const dp: number[][] = Array.from({ length: n + 1 }, () => new Array(m + 1).fill(0))
  for (let i = n - 1; i >= 0; i--) {
    for (let j = m - 1; j >= 0; j--) {
      dp[i][j] = a[i] === b[j] ? dp[i + 1][j + 1] + 1 : Math.max(dp[i + 1][j], dp[i][j + 1])
    }
  }

  const matchedB = new Set<number>()
  const addedB = new Set<number>()
  const changedB = new Set<number>()
  let i = 0
  let j = 0
  while (i < n && j < m) {
    if (a[i] === b[j]) {
      matchedB.add(j)
      i++
      j++
    } else if (dp[i + 1][j] >= dp[i][j + 1]) {
      changedB.add(j)
      i++
    } else {
      addedB.add(j)
      j++
    }
  }
  while (j < m) {
    addedB.add(j)
    j++
  }
  for (let k = 0; k < m; k++) {
    if (!matchedB.has(k) && !addedB.has(k)) changedB.add(k)
  }
  return { added: addedB, changed: changedB }
}
