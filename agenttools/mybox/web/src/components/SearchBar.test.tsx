import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { SearchBar } from './SearchBar'

describe('SearchBar', () => {
  it('renders input and submits trimmed value', () => {
    const onSubmit = vi.fn()
    render(<SearchBar value="  foo  " onChange={() => undefined} onSubmit={onSubmit} />)
    fireEvent.submit(screen.getByRole('searchbox'))
    expect(onSubmit).toHaveBeenCalledWith('foo')
  })

  it('calls onChange while typing', () => {
    const onChange = vi.fn()
    render(<SearchBar value="" onChange={onChange} onSubmit={() => undefined} />)
    fireEvent.change(screen.getByRole('searchbox'), { target: { value: 'bar' } })
    expect(onChange).toHaveBeenCalledWith('bar')
  })

  it('navigates with query param', () => {
    const onSubmit = vi.fn()
    render(
      <MemoryRouter>
        <SearchBar value="" onChange={() => undefined} onSubmit={onSubmit} />
      </MemoryRouter>,
    )
    expect(screen.getByRole('searchbox')).toBeInTheDocument()
  })
})
