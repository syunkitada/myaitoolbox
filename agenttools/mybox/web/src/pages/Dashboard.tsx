import { useNavigate, useParams } from 'react-router-dom'
import { BrowserPage } from './BrowserPage'

interface DashboardProps {
  refreshMeta: () => Promise<void>
  favorites: string[]
}

export function Dashboard({ refreshMeta, favorites }: DashboardProps) {
  const params = useParams()
  const selected = (params['*'] ?? '').trim()
  const navigate = useNavigate()

  return (
    <BrowserPage
      mode="files"
      root=""
      title="Files"
      selected={selected}
      onSelect={(path) => navigate(`/files/${encodeURIComponent(path)}`)}
      onBack={() => navigate('/')}
      favorites={favorites}
      refreshMeta={refreshMeta}
      defaultSelect={(entries) =>
        entries.some((e) => e.kind === 'file' && e.path === 'README.md') ? 'README.md' : undefined
      }
    />
  )
}
