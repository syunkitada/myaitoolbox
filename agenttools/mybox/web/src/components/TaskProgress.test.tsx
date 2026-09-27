import { describe, expect, it } from 'vitest'
import { render, screen } from '@testing-library/react'
import { TaskProgress } from './TaskProgress'

describe('TaskProgress', () => {
  it('shows the completed count and progress bar value', () => {
    render(<TaskProgress completed={2} total={5} />)

    expect(screen.getByTestId('task-progress')).toHaveTextContent('2/5')
    expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '2')
    expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuemax', '5')
    expect(screen.getByTestId('task-progress').querySelector('.task-progress-bar')).toHaveStyle({ width: '40%' })
  })

  it('shows an empty bar for a task without checklist items', () => {
    render(<TaskProgress completed={0} total={0} />)

    expect(screen.getByTestId('task-progress')).toHaveTextContent('0/0')
    expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '0')
    expect(screen.getByTestId('task-progress').querySelector('.task-progress-bar')).toHaveStyle({ width: '0%' })
  })
})
