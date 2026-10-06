import { describe, it, expect, beforeEach } from 'vitest'
import { fireEvent, render, screen } from '@testing-library/react'
import { useAgentSidebarState } from './use-agent-sidebar'

function Probe({ project, isMobile = false }: { project: string; isMobile?: boolean }) {
  const state = useAgentSidebarState(project, isMobile)
  return (
    <div>
      <output data-testid="state">{`${state.open}:${state.paneId ?? ''}:${state.displayMode}`}</output>
      <output data-testid="draft-w1-p1">{state.drafts['w1:p1'] ?? ''}</output>
      <output data-testid="draft-w1-p2">{state.drafts['w1:p2'] ?? ''}</output>
      <button type="button" onClick={() => state.setOpen(true)}>open</button>
      <button type="button" onClick={() => state.setOpen(false)}>close</button>
      <button type="button" onClick={() => state.setPaneId('w1:p1')}>select</button>
      <button type="button" onClick={() => state.setDraft('w1:p1', 'draft for first agent')}>draft first</button>
      <button type="button" onClick={() => state.setDraft('w1:p2', 'draft for second agent')}>draft second</button>
      <button type="button" onClick={() => state.setDraft('w1:p1', '')}>clear first draft</button>
      <button type="button" onClick={() => state.setDisplayMode('herdr')}>herdr mode</button>
    </div>
  )
}

describe('useAgentSidebarState', () => {
  beforeEach(() => localStorage.clear())

  it('persists open state and selected agent separately for each project', () => {
    const view = render(<Probe project="demo" />)
    expect(screen.getByTestId('state')).toHaveTextContent('true::auto')

    fireEvent.click(screen.getByRole('button', { name: 'close' }))
    fireEvent.click(screen.getByRole('button', { name: 'select' }))
    expect(screen.getByTestId('state')).toHaveTextContent('false:w1:p1:auto')
    view.unmount()

    render(<Probe project="other" />)
    expect(screen.getByTestId('state')).toHaveTextContent('true::auto')
  })

  it('persists the selected output display mode per project', () => {
    const view = render(<Probe project="demo" />)
    fireEvent.click(screen.getByRole('button', { name: 'herdr mode' }))
    expect(screen.getByTestId('state')).toHaveTextContent('true::herdr')
    view.unmount()

    render(<Probe project="demo" />)
    expect(screen.getByTestId('state')).toHaveTextContent('true::herdr')
  })

  it('persists prompt drafts per project and agent pane', () => {
    const view = render(<Probe project="demo" />)
    fireEvent.click(screen.getByRole('button', { name: 'draft first' }))
    fireEvent.click(screen.getByRole('button', { name: 'draft second' }))

    expect(screen.getByTestId('draft-w1-p1')).toHaveTextContent('draft for first agent')
    expect(screen.getByTestId('draft-w1-p2')).toHaveTextContent('draft for second agent')
    view.unmount()

    const otherView = render(<Probe project="other" />)
    expect(screen.getByTestId('draft-w1-p1')).toBeEmptyDOMElement()
    expect(screen.getByTestId('draft-w1-p2')).toBeEmptyDOMElement()

    otherView.unmount()
    const restored = render(<Probe project="demo" />)
    expect(screen.getByTestId('draft-w1-p1')).toHaveTextContent('draft for first agent')
    expect(screen.getByTestId('draft-w1-p2')).toHaveTextContent('draft for second agent')

    fireEvent.click(screen.getByRole('button', { name: 'clear first draft' }))
    expect(screen.getByTestId('draft-w1-p1')).toBeEmptyDOMElement()
    restored.unmount()
    render(<Probe project="demo" />)
    expect(screen.getByTestId('draft-w1-p1')).toBeEmptyDOMElement()
    expect(screen.getByTestId('draft-w1-p2')).toHaveTextContent('draft for second agent')
  })

  it('migrates the legacy Herdr selected-agent state', () => {
    localStorage.setItem('mybox:herdr-open-agent', JSON.stringify({ demo: 'w1:p1' }))
    render(<Probe project="demo" />)

    expect(screen.getByTestId('state')).toHaveTextContent('true:w1:p1:auto')
  })

  it('keeps mobile sidebar closed independently of desktop open state', () => {
    localStorage.setItem('mybox:agent-sidebar-state', JSON.stringify({
      demo: { open: true, paneId: null, displayMode: 'auto' },
    }))

    const mobile = render(<Probe project="demo" isMobile />)
    expect(screen.getByTestId('state')).toHaveTextContent('false::auto')

    fireEvent.click(screen.getByRole('button', { name: 'open' }))
    expect(screen.getByTestId('state')).toHaveTextContent('true::auto')
    mobile.unmount()

    render(<Probe project="demo" />)
    expect(screen.getByTestId('state')).toHaveTextContent('true::auto')
  })
})
