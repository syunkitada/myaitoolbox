import { FormEvent } from 'react'
import { Input } from './ui/input'

interface SearchBarProps {
  value: string
  onChange: (v: string) => void
  onSubmit: (q: string) => void
  placeholder?: string
  autoFocus?: boolean
}

export function SearchBar({ value, onChange, onSubmit, placeholder, autoFocus }: SearchBarProps) {
  const submit = (e: FormEvent) => {
    e.preventDefault()
    onSubmit(value.trim())
  }
  return (
    <form className="search-bar flex" onSubmit={submit}>
      <Input
        type="search"
        value={value}
        placeholder={placeholder ?? 'Search…'}
        onChange={(e) => onChange(e.target.value)}
        autoFocus={autoFocus}
        aria-label="Search"
        className="min-w-[220px] max-md:min-w-0"
      />
    </form>
  )
}
