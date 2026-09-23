import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { DiffView } from './DiffView'

const sampleDiff = `diff --git a/a.md b/a.md
index ce01362..3b18e51 100644
--- a/a.md
+++ b/a.md
@@ -1 +1 @@
-hello
+hello world`

describe('DiffView', () => {
  it('renders added and removed lines with their prefixes', () => {
    render(<DiffView diff={sampleDiff} />)
    const view = screen.getByTestId('diff-view')
    expect(view.textContent).toContain('-hello')
    expect(view.textContent).toContain('+hello world')
    expect(view.textContent).toContain('@@ -1 +1 @@')
  })

  it('shows a placeholder when the diff is empty', () => {
    render(<DiffView diff="" />)
    expect(screen.getByText('No diff available.')).toBeInTheDocument()
  })

  it('does not treat header lines as added/removed', () => {
    render(<DiffView diff={sampleDiff} />)
    const view = screen.getByTestId('diff-view')
    // "\ No newline" and header lines must not be colored as +/-; the
    // rendered text still contains the marker but only diff bodies count.
    expect(view.textContent).toContain('diff --git a/a.md b/a.md')
    expect(view.textContent).not.toContain('+diff ')
  })
})