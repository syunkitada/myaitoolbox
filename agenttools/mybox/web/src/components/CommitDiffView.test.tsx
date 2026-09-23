import { render, screen, fireEvent } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { CommitDiffView, splitCommitDiff } from './CommitDiffView'

const multiFileDiff = `diff --git a/a.md b/a.md
index ce01362..3b18e51 100644
--- a/a.md
+++ b/a.md
@@ -1 +1 @@
-hello
+hello world
diff --git a/src/b.ts b/src/b.ts
index 0000000..e69de29
new file mode 100644
--- /dev/null
+++ b/src/b.ts
@@ -0,0 +1 @@
+const b = 1
diff --git a/old.txt b/new.txt
similarity index 100%
rename from old.txt
rename to new.txt`

describe('splitCommitDiff', () => {
  it('splits a multi-file diff into per-file segments', () => {
    const segments = splitCommitDiff(multiFileDiff)
    expect(segments).toHaveLength(3)
    expect(segments[0].path).toBe('a.md')
    expect(segments[0].name).toBe('a.md')
    expect(segments[0].dir).toBe('')
    expect(segments[1].path).toBe('src/b.ts')
    expect(segments[1].name).toBe('b.ts')
    expect(segments[1].dir).toBe('src')
    expect(segments[1].content).toContain('+const b = 1')
  })

  it('keeps the split boundary at diff --git headers', () => {
    const segments = splitCommitDiff(multiFileDiff)
    expect(segments[1].content.startsWith('diff --git a/src/b.ts b/src/b.ts')).toBe(true)
    expect(segments[1].content).not.toContain('diff --git a/a.md b/a.md')
  })

  it('uses the destination path for renames', () => {
    const segments = splitCommitDiff(multiFileDiff)
    expect(segments[2].path).toBe('new.txt')
  })

  it('drops a git show commit header before the first diff', () => {
    const withHeader = `commit abc123
Author: Test <t@example.com>
Date:   Mon Sep 15 00:00:00 2026 +0000

    message

${multiFileDiff}`
    const segments = splitCommitDiff(withHeader)
    expect(segments).toHaveLength(3)
    expect(segments[0].content.startsWith('diff --git')).toBe(true)
  })

  it('returns no segments for an unrelated diff', () => {
    expect(splitCommitDiff('diff --notag header\ncontext')).toHaveLength(0)
    expect(splitCommitDiff('')).toHaveLength(0)
  })
})

describe('CommitDiffView', () => {
  it('renders each file as a collapsible section', () => {
    render(<CommitDiffView diff={multiFileDiff} />)
    expect(screen.getAllByTestId('commit-diff-file')).toHaveLength(3)
    expect(screen.getByTestId('commit-diff-view').textContent).toContain('+hello world')
  })

  it('collapses a section on click', () => {
    render(<CommitDiffView diff={multiFileDiff} />)
    const view = screen.getByTestId('commit-diff-view')
    expect(view.textContent).toContain('+hello world')
    fireEvent.click(screen.getAllByTestId('commit-diff-file')[0])
    expect(view.textContent).not.toContain('+hello world')
    fireEvent.click(screen.getAllByTestId('commit-diff-file')[0])
    expect(view.textContent).toContain('+hello world')
  })

  it('supports collapse all and expand all', () => {
    render(<CommitDiffView diff={multiFileDiff} />)
    const view = screen.getByTestId('commit-diff-view')
    fireEvent.click(screen.getByTitle('Collapse all'))
    expect(view.textContent).not.toContain('+hello world')
    expect(view.textContent).not.toContain('+const b = 1')
    fireEvent.click(screen.getByTitle('Expand all'))
    expect(view.textContent).toContain('+hello world')
    expect(view.textContent).toContain('+const b = 1')
  })

  it('falls back to a plain diff when no file header is found', () => {
    render(<CommitDiffView diff="+line\n-context" />)
    expect(screen.getByTestId('diff-view').textContent).toContain('+line')
  })
})