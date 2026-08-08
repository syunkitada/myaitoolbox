import { useCallback, useEffect, useState } from 'react'
import { api, Project, setProject } from '../api/client'

interface ProjectsPageProps {
  onChanged?: () => void
}

export function ProjectsPage({ onChanged }: ProjectsPageProps) {
  const [projects, setProjects] = useState<Project[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [path, setPath] = useState('')
  const [candidates, setCandidates] = useState<string[]>([])
  const [showCandidates, setShowCandidates] = useState(false)
  const [busy, setBusy] = useState(false)
  const [feedback, setFeedback] = useState<string | null>(null)

  const refresh = useCallback(async () => {
    setLoading(true)
    try {
      setProjects(await api.listProjects())
      setError(null)
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void refresh()
  }, [refresh])

  useEffect(() => {
    const t = setTimeout(() => {
      void (async () => {
        try {
          setCandidates(await api.getProjectPaths(path))
        } catch {
          setCandidates([])
        }
      })()
    }, 120)
    return () => clearTimeout(t)
  }, [path])

  const handleCreate = async () => {
    if (!path.trim()) return
    setBusy(true)
    setFeedback(null)
    try {
      const created = await api.createProject(path.trim())
      setFeedback(`Created project "${created.name}".`)
      setPath('')
      setCandidates([])
      await refresh()
      onChanged?.()
    } catch (e) {
      setFeedback(e instanceof Error ? e.message : String(e))
    } finally {
      setBusy(false)
    }
  }

  const openProject = (name: string) => {
    window.location.hash = '#/'
    setProject(name)
  }

  const handleDelete = async (name: string) => {
    if (!window.confirm(`Delete project "${name}"?`)) return
    setBusy(true)
    try {
      await api.deleteProject(name)
      await refresh()
      onChanged?.()
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="projects-page">
      <h1>Projects</h1>

      <section className="card projects-create">
        <h2>New project</h2>
        <p className="projects-create-hint">
          Register an existing directory as a project. Candidates are real paths on this machine.
        </p>
        <form
          onSubmit={(e) => {
            e.preventDefault()
            void handleCreate()
          }}
        >
          <div className="path-picker">
            <input
              aria-label="Project path"
              placeholder="/path/to/project"
              value={path}
              onChange={(e) => setPath(e.target.value)}
              onFocus={() => setShowCandidates(true)}
              onBlur={() => setTimeout(() => setShowCandidates(false), 150)}
            />
            {showCandidates && candidates.length > 0 && (
              <ul className="path-candidates">
                {candidates.map((c) => (
                  <li key={c}>
                    <button
                      type="button"
                      onMouseDown={(e) => {
                        e.preventDefault()
                        setPath(c.endsWith('/') ? c : c + '/')
                      }}
                    >
                      {c}
                    </button>
                  </li>
                ))}
              </ul>
            )}
          </div>
          <button type="submit" className="btn-primary" disabled={busy || !path.trim()}>
            {busy ? 'Creating…' : 'Create project'}
          </button>
        </form>
        {feedback && <p className="projects-feedback">{feedback}</p>}
      </section>

      {error && <p className="error-text">{error}</p>}

      <section className="projects-list">
        {loading ? (
          <p className="muted">Loading projects…</p>
        ) : projects.length === 0 ? (
          <p className="muted">No projects yet. Register a directory above to get started.</p>
        ) : (
          <table className="projects-table">
            <thead>
              <tr>
                <th>Project</th>
                <th>Path</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {projects.map((p) => (
                <tr key={p.name}>
                  <td>
                    <button
                      className="link-btn project-name"
                      onClick={() => openProject(p.name)}
                      title={`Open ${p.name}`}
                    >
                      {p.name}
                    </button>
                  </td>
                  <td className="project-path">{p.path}</td>
                  <td className="projects-actions">
                    <button
                      className="btn-secondary"
                      onClick={() => openProject(p.name)}
                    >
                      Open
                    </button>
                    <button
                      className="btn-danger"
                      disabled={busy}
                      onClick={() => void handleDelete(p.name)}
                    >
                      Delete
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </section>
    </div>
  )
}
