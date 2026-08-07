import { useState } from 'react'
import { BrowserPage } from './BrowserPage'

interface DashboardProps {
  refreshMeta: () => Promise<void>
  favorites: string[]
}

export function Dashboard({ refreshMeta, favorites }: DashboardProps) {
  const [selected, setSelected] = useState('')

  return (
    <BrowserPage
      mode="files"
      root=""
      title="Files"
      selected={selected}
      onSelect={setSelected}
      onBack={() => setSelected('')}
      favorites={favorites}
      refreshMeta={refreshMeta}
      defaultSelect={(entries) =>
        entries.some((e) => e.kind === 'file' && e.path === 'README.md') ? 'README.md' : undefined
      }
    />
  )
}
