import { useCallback, useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import {
  DndContext,
  DragEndEvent,
  PointerSensor,
  useDroppable,
  useDraggable,
  useSensor,
  useSensors,
} from '@dnd-kit/core'
import { Task, TaskStatus, api } from '../api/client'
import { getProject, projectUrl } from '../utils/routes'

const COLUMNS: TaskStatus[] = ['todo', 'doing', 'blocked', 'review', 'done']

interface TaskCardProps {
  task: Task
  onOpen: (id: string, project?: string | null) => void
  showProject?: boolean
  readonly?: boolean
}

function TaskCard({ task, onOpen, showProject, readonly }: TaskCardProps) {
  const { attributes, listeners, setNodeRef, transform, isDragging } = useDraggable({
    id: task.id,
    data: { task },
    disabled: readonly,
  })
  const style = transform
    ? {
        transform: `translate3d(${transform.x}px, ${transform.y}px, 0)`,
      }
    : undefined
  return (
    <div
      ref={setNodeRef}
      style={style}
      {...listeners}
      {...attributes}
      className={`board-card ${isDragging ? 'dragging' : ''}`}
    >
      <button className="link-btn" onClick={() => onOpen(task.id, task.project)}>
        {task.title}
      </button>
      <div className="board-card-meta">
        {showProject && task.project && (
          <span className="badge project-badge">{task.project}</span>
        )}
        <span className={`badge priority-${task.priority}`}>{task.priority}</span>
        {(task.tags ?? []).slice(0, 3).map((t) => (
          <span key={t} className="badge tag">
            {t}
          </span>
        ))}
      </div>
    </div>
  )
}

interface ColumnProps {
  status: TaskStatus
  tasks: Task[]
  onOpen: (id: string, project?: string | null) => void
  showProject?: boolean
  readonly?: boolean
}

function Column({ status, tasks, onOpen, showProject, readonly }: ColumnProps) {
  const { setNodeRef, isOver } = useDroppable({ id: status })
  return (
    <div ref={setNodeRef} className={`board-column ${isOver ? 'over' : ''}`} data-status={status} data-testid={`column-${status}`}>
      <h2 className="board-column-title">
        {status}
        <span className="muted">{tasks.length}</span>
      </h2>
      <div className="board-column-body">
        {tasks.map((t) => (
          <TaskCard key={`${t.project ?? ''}/${t.id}`} task={t} onOpen={onOpen} showProject={showProject} readonly={readonly} />
        ))}
        {tasks.length === 0 && <div className="muted board-empty">{readonly ? 'no tasks' : 'drop here'}</div>}
      </div>
    </div>
  )
}

export function KanbanBoard() {
  const [tasks, setTasks] = useState<Task[]>([])
  const [error, setError] = useState<string | null>(null)
  const navigate = useNavigate()
  const currentProject = getProject()
  const isGlobal = !currentProject  // プロジェクト未選択 = 全プロジェクト横断モード

  const sensors = useSensors(useSensor(PointerSensor, { activationConstraint: { distance: 4 } }))

  const load = useCallback(() => {
    void api
      .listTasks({ all: true })
      .then(setTasks)
      .catch((e) => setError(e instanceof Error ? e.message : String(e)))
  }, [])

  useEffect(load, [load])

  const onDragEnd = useCallback(
    (event: DragEndEvent) => {
      if (isGlobal) return  // 横断モードはドラッグ無効
      const task = (event.active.data.current as { task?: Task } | undefined)?.task
      const status = event.over?.id as TaskStatus | undefined
      if (!task || !status || !COLUMNS.includes(status) || status === task.status) return
      void api
        .updateTask(task.id, { status })
        .then(() => load())
        .catch((e) => setError(e instanceof Error ? e.message : String(e)))
    },
    [load, isGlobal],
  )

  const handleOpen = useCallback(
    (id: string, project?: string | null) => {
      if (project) {
        navigate(`/projects/${encodeURIComponent(project)}/tasks/${id}`)
      } else {
        navigate(projectUrl(`/tasks/${id}`))
      }
    },
    [navigate],
  )

  const byStatus = (s: TaskStatus) =>
    tasks.filter((t) => t.status === s && !t.archived)

  return (
    <div className="page">
      <div className="page-header">
        <div>
          <h1>Board</h1>
          {isGlobal && (
            <p className="page-subtitle muted">全プロジェクト横断ビュー（読み取り専用）</p>
          )}
        </div>
        {!isGlobal && (
          <button className="ghost" onClick={() => navigate(projectUrl('/tasks'))}>
            List view →
          </button>
        )}
      </div>
      {error && <div className="error-banner">{error}</div>}
      <DndContext sensors={sensors} onDragEnd={onDragEnd}>
        <div className="board">
          {COLUMNS.map((s) => (
            <Column
              key={s}
              status={s}
              tasks={byStatus(s)}
              onOpen={handleOpen}
              showProject={isGlobal}
              readonly={isGlobal}
            />
          ))}
        </div>
      </DndContext>
    </div>
  )
}
