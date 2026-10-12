import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { FrontmatterForm } from './FrontmatterForm'

const metadata = {
  title: 'Example',
  status: 'doing',
  due: '2026-10-12',
  pending_until: '20261013',
  pending_reason: 'Waiting for review',
  tags: ['work'],
}

function fieldLabels(container: HTMLElement): string[] {
  return Array.from(container.querySelectorAll('.frontmatter-form label > span')).map((span) => span.textContent ?? '')
}

describe('FrontmatterForm', () => {
  it('shows task metadata in the requested order', () => {
    const { container } = render(<FrontmatterForm value={metadata} onChange={vi.fn()} taskMode />)

    expect(fieldLabels(container)).toEqual(['Status', 'Due', 'Pending until', 'Pending reason', 'Tags'])
  })

  it('shows only Tags for non-task Markdown', () => {
    const { container } = render(
      <FrontmatterForm value={{ ...metadata, aliases: ['example'] }} onChange={vi.fn()} />,
    )

    expect(fieldLabels(container)).toEqual(['Tags'])
    expect(screen.queryByText('Other metadata')).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '+ Add metadata' })).not.toBeInTheDocument()
  })
})
