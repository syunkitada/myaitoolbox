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
import { encodePath, getBasePath, getProject, projectUrl } from '../utils/routes'
import { Button } from '../components/ui/button'
import { cn } from '@/lib/utils'
import { DueBadge, PendingBadge, PriorityBadge, ProjectBadge, TagBadge } from '../components/badges'

const COLUMNS: TaskStatus[] = ['todo', 'doing', 'blocked', 'review', 'done']

function dueClass(due: string): string {
  const d = new Date(due)
  if (Number.isNaN(d.getTime())) return ''
  const today = new Date()
  today.setHours(0, 0, 0, 0)
  const dueDay = new Date(d)
  dueDay.setHours(0, 0, 0, 0)
  const days = Math.round((dueDay.getTime() - today.getTime()) / 86400000)
  if (days < 0) return 'overdue'
  if (days <= 7) return 'soon'
  return 'far'
}

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
  const isPending = Boolean(task.pending_until || task.pending_reason)
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
      className={cn(
        'board-card cursor-grab rounded-md border bg-card p-2.5 shadow-sm',
        isDragging && 'dragging opacity-60 shadow-lg',
        isPending && 'pending border-dashed opacity-55',
      )}
    >
      <Button variant="link" size="xs" className="h-auto p-0 text-left" onClick={() => onOpen(task.id, task.project)}>
        {task.title}
      </Button>
      <div className="board-card-meta mt-1.5 flex flex-wrap gap-1">
        {showProject && task.project && <ProjectBadge>{task.project}</ProjectBadge>}
        <PriorityBadge priority={task.priority} />
        {task.due && <DueBadge due={task.due} dueClass={dueClass(task.due)} />}
        {(task.tags ?? []).slice(0, 3).map((t) => (
          <TagBadge key={t}>{t}</TagBadge>
        ))}
      </div>
      {isPending && (
        <div className="board-card-pending mt-1.5 flex flex-wrap items-center gap-1.5 text-xs">
          {task.pending_until && <PendingBadge>{task.pending_until}</PendingBadge>}
          {task.pending_reason && <span className="muted text-muted-foreground">{task.pending_reason}</span>}
        </div>
      )}
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
    <div
      ref={setNodeRef}
      className={cn(
        'board-column flex min-h-[60vh] flex-col rounded-lg border bg-card p-2',
        isOver && 'over border-primary bg-blue-50',
        'max-md:min-h-[40vh] max-md:min-w-[220px] max-md:shrink-0 max-sm:min-w-[180px]',
      )}
      data-status={status}
      data-testid={`column-${status}`}
    >
      <h2 className="board-column-title mb-2.5 flex items-center justify-between border-b pb-2 text-sm font-semibold tracking-wider uppercase">
        {status}
        <span className="muted text-muted-foreground">{tasks.length}</span>
      </h2>
      <div className="board-column-body flex flex-col gap-2">
        {tasks.map((t) => (
          <TaskCard key={`${t.project ?? ''}/${t.id}`} task={t} onOpen={onOpen} showProject={showProject} readonly={readonly} />
        ))}
        {tasks.length === 0 && (
          <div className="board-empty rounded-md border border-dashed py-4 text-center text-xs text-muted-foreground">
            {readonly ? 'no tasks' : 'drop here'}
          </div>
        )}
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
      const target = project || currentProject
      if (!target) return
      const filePath = `/dashboard/files/tasks/${encodePath(id)}/task.md`
      if (target === currentProject) {
        navigate(projectUrl(filePath))
      } else {
        window.location.href = `${getBasePath()}/projects/${encodeURIComponent(target)}${filePath}`
      }
    },
    [navigate, currentProject],
  )

  const byStatus = (s: TaskStatus) =>
    tasks.filter((t) => t.status === s && !t.archived)

  return (
    <div className="page p-4 md:p-6">
      <div className="page-header mb-3 flex items-center justify-between gap-3">
        <div>
          <h1 className="text-2xl font-bold">Board</h1>
          {isGlobal && (
            <p className="page-subtitle mt-0.5 text-sm text-muted-foreground">
              全プロジェクト横断ビュー（読み取り専用）
            </p>
          )}
        </div>
      </div>
      {error && (
        <div className="error-banner my-2 flex items-center justify-between rounded-md border border-red-200 bg-red-50 px-3.5 py-2.5 text-sm text-red-700">
          {error}
        </div>
      )}
      <DndContext sensors={sensors} onDragEnd={onDragEnd}>
        <div className="board grid grid-cols-5 items-start gap-2.5 max-md:flex max-md:gap-3 max-md:overflow-x-auto max-md:pb-2">
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
