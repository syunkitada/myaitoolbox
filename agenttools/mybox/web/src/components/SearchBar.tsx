import { FormEvent } from 'react'

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
    <form className="search-bar" onSubmit={submit}>
      <input
        type="search"
        value={value}
        placeholder={placeholder ?? 'Search…'}
        onChange={(e) => onChange(e.target.value)}
        autoFocus={autoFocus}
        aria-label="Search"
      />
    </form>
  )
}
