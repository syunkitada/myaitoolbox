import { useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { api } from '../api/client'
import { encodePath, projectUrl } from '../utils/routes'
import { BrowserPage } from './BrowserPage'

interface KnowledgePageProps {
  refreshMeta: () => Promise<void>
  favorites: string[]
}

export function KnowledgePage({ refreshMeta, favorites }: KnowledgePageProps) {
  const params = useParams()
  const selected = (params['*'] ?? '').trim()
  const navigate = useNavigate()
  const [lastPath, setLastPath] = useState<string | null>(null)

  useEffect(() => {
    if (selected) setLastPath(selected)
  }, [selected])

  const handleNew = () => {
    const path = window.prompt('New knowledge path (e.g. notes/foo)')
    if (path && path.trim()) {
      void api
        .createKnowledge(path.trim())
        .then((k) => {
          setLastPath(k.path)
          navigate(projectUrl(`/knowledge/${encodePath(k.path)}`))
        })
        .catch(() => undefined)
    }
  }

  const handleClose = () => {
    if (lastPath) navigate(projectUrl(`/knowledge/${encodePath(lastPath)}`))
    else navigate(projectUrl('/knowledge'))
  }

  return (
    <BrowserPage
      mode="knowledge"
      root="knowledge"
      title="Knowledge"
      selected={selected}
      onSelect={(path) => navigate(projectUrl(`/knowledge/${encodePath(path)}`))}
      onBack={() => navigate(projectUrl('/knowledge'))}
      favorites={favorites}
      refreshMeta={refreshMeta}
      onNew={handleNew}
      onClose={handleClose}
    />
  )
}
