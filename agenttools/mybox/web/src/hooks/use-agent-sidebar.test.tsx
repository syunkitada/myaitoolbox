import { describe, it, expect, beforeEach } from 'vitest'
import { fireEvent, render, screen } from '@testing-library/react'
import { useAgentSidebarState } from './use-agent-sidebar'

function Probe({ project, isMobile = false }: { project: string; isMobile?: boolean }) {
  const state = useAgentSidebarState(project, isMobile)
  return (
    <div>
      <output data-testid="state">{`${state.open}:${state.paneId ?? ''}:${state.displayMode}`}</output>
      <button type="button" onClick={() => state.setOpen(true)}>open</button>
      <button type="button" onClick={() => state.setOpen(false)}>close</button>
      <button type="button" onClick={() => state.setPaneId('w1:p1')}>select</button>
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
