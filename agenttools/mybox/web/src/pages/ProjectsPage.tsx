import { useCallback, useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import {
  DndContext,
  closestCenter,
  PointerSensor,
  useSensor,
  useSensors,
  DragEndEvent,
} from '@dnd-kit/core'
import {
  SortableContext,
  useSortable,
  verticalListSortingStrategy,
  arrayMove,
} from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import { api, Project, setProject } from '../api/client'
import { Button } from '../components/ui/button'
import { Input } from '../components/ui/input'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/card'
import { GripVertical } from 'lucide-react'

interface SortableRowProps {
  project: Project
  busy: boolean
  onOpen: (name: string) => void
  onDelete: (name: string) => void
}

function SortableRow({ project, busy, onOpen, onDelete }: SortableRowProps) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({
    id: project.name,
  })

  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.5 : 1,
  }

  return (
    <tr ref={setNodeRef} style={style}>
      <td className="border-b p-2 w-8">
        <button
          type="button"
          className="cursor-grab touch-none text-muted-foreground hover:text-foreground"
          {...attributes}
          {...listeners}
        >
          <GripVertical className="size-4" />
        </button>
      </td>
      <td className="border-b p-2">
        <Button
          variant="link"
          size="xs"
          className="project-name h-auto p-0 font-semibold"
          onClick={() => onOpen(project.name)}
          title={`Open ${project.name}`}
        >
          {project.name}
        </Button>
      </td>
      <td className="project-path border-b p-2 text-[13px] break-all text-muted-foreground">{project.path}</td>
      <td className="projects-actions border-b p-2">
        <div className="flex justify-end gap-1.5">
          <Button size="sm" onClick={() => onOpen(project.name)}>
            Open
          </Button>
          <Button
            variant="destructive"
            size="sm"
            disabled={busy}
            onClick={() => void onDelete(project.name)}
          >
            Delete
          </Button>
        </div>
      </td>
    </tr>
  )
}

interface ProjectsPageProps {
  onChanged?: () => void
}

export function ProjectsPage({ onChanged }: ProjectsPageProps) {
  const navigate = useNavigate()
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
    setProject(name, navigate)
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

  const handleDragEnd = (event: DragEndEvent) => {
    const { active, over } = event
    if (!over || active.id === over.id) return
    const oldIndex = projects.findIndex((p) => p.name === active.id)
    const newIndex = projects.findIndex((p) => p.name === over.id)
    if (oldIndex === -1 || newIndex === -1) return
    const newOrder = arrayMove(projects, oldIndex, newIndex)
    setProjects(newOrder)
    void api.reorderProjects(newOrder.map((p) => p.name)).catch((e) => {
      setError(e instanceof Error ? e.message : String(e))
      void refresh()
    })
  }

  const sensors = useSensors(
    useSensor(PointerSensor, {
      activationConstraint: { distance: 5 },
    }),
  )

  return (
    <div className="projects-page mx-auto max-w-[900px] p-6">
      <h1 className="text-2xl font-bold">Workspaces</h1>

      <Card className="projects-create mt-4 gap-3">
        <CardHeader className="px-6 py-0">
          <CardTitle className="text-lg">New project</CardTitle>
          <CardDescription>
            Register an existing directory as a project. Candidates are real paths on this machine.
          </CardDescription>
        </CardHeader>
        <CardContent className="px-6 pb-0 pt-0">
          <form
            className="flex items-start gap-2 max-sm:flex-col"
            onSubmit={(e) => {
              e.preventDefault()
              void handleCreate()
            }}
          >
            <div className="path-picker relative flex-1 max-sm:w-full">
              <Input
                aria-label="Project path"
                placeholder="/path/to/project"
                value={path}
                onChange={(e) => setPath(e.target.value)}
                onFocus={() => setShowCandidates(true)}
                onBlur={() => setTimeout(() => setShowCandidates(false), 150)}
              />
              {showCandidates && candidates.length > 0 && (
                <ul className="path-candidates absolute top-[calc(100%+4px)] right-0 left-0 z-30 m-0 max-h-60 list-none overflow-y-auto rounded-md border bg-card p-1 shadow-lg">
                  {candidates.map((c) => (
                    <li key={c}>
                      <Button
                        type="button"
                        variant="ghost"
                        size="sm"
                        className="w-full justify-start"
                        onMouseDown={(e) => {
                          e.preventDefault()
                          setPath(c.endsWith('/') ? c : c + '/')
                        }}
                      >
                        {c}
                      </Button>
                    </li>
                  ))}
                </ul>
              )}
            </div>
            <Button type="submit" disabled={busy || !path.trim()}>
              {busy ? 'Creating…' : 'Create project'}
            </Button>
          </form>
          {feedback && (
            <p className={feedback.includes('Created') ? 'mt-2 text-sm text-muted-foreground' : 'mt-2 text-sm text-red-700'}>
              {feedback}
            </p>
          )}
        </CardContent>
      </Card>

      {error && <p className="mt-3 text-sm text-red-700">{error}</p>}

      <section className="projects-list mt-4">
        {loading ? (
          <p className="muted text-sm text-muted-foreground">Loading projects…</p>
        ) : projects.length === 0 ? (
          <p className="muted text-sm text-muted-foreground">No workspaces yet. Register a directory above to get started.</p>
        ) : (
          <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={handleDragEnd}>
            <table className="projects-table w-full border-collapse">
              <thead>
                <tr>
                  <th className="border-b p-2 w-8" />
                  <th className="border-b p-2 text-left text-xs font-semibold tracking-wider text-muted-foreground uppercase">
                    Project
                  </th>
                  <th className="border-b p-2 text-left text-xs font-semibold tracking-wider text-muted-foreground uppercase">
                    Path
                  </th>
                  <th className="border-b p-2" />
                </tr>
              </thead>
              <SortableContext items={projects.map((p) => p.name)} strategy={verticalListSortingStrategy}>
                <tbody>
                  {projects.map((p) => (
                    <SortableRow
                      key={p.name}
                      project={p}
                      busy={busy}
                      onOpen={openProject}
                      onDelete={handleDelete}
                    />
                  ))}
                </tbody>
              </SortableContext>
            </table>
          </DndContext>
        )}
      </section>
    </div>
  )
}
