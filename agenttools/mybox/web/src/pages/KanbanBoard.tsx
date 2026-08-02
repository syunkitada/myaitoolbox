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

const COLUMNS: TaskStatus[] = ['todo', 'doing', 'blocked', 'review', 'done']

interface TaskCardProps {
  task: Task
  onOpen: (id: string) => void
}

function TaskCard({ task, onOpen }: TaskCardProps) {
  const { attributes, listeners, setNodeRef, transform, isDragging } = useDraggable({
    id: task.id,
    data: { task },
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
      <button className="link-btn" onClick={() => onOpen(task.id)}>
        {task.title}
      </button>
      <div className="board-card-meta">
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
  onOpen: (id: string) => void
}

function Column({ status, tasks, onOpen }: ColumnProps) {
  const { setNodeRef, isOver } = useDroppable({ id: status })
  return (
    <div ref={setNodeRef} className={`board-column ${isOver ? 'over' : ''}`} data-status={status} data-testid={`column-${status}`}>
      <h2 className="board-column-title">
        {status}
        <span className="muted">{tasks.length}</span>
      </h2>
      <div className="board-column-body">
        {tasks.map((t) => (
          <TaskCard key={t.id} task={t} onOpen={onOpen} />
        ))}
        {tasks.length === 0 && <div className="muted board-empty">drop here</div>}
      </div>
    </div>
  )
}

export function KanbanBoard() {
  const [tasks, setTasks] = useState<Task[]>([])
  const [error, setError] = useState<string | null>(null)
  const navigate = useNavigate()

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
      const task = (event.active.data.current as { task?: Task } | undefined)?.task
      const status = event.over?.id as TaskStatus | undefined
      if (!task || !status || !COLUMNS.includes(status) || status === task.status) return
      void api
        .updateTask(task.id, { status })
        .then(() => load())
        .catch((e) => setError(e instanceof Error ? e.message : String(e)))
    },
    [load],
  )

  const byStatus = (s: TaskStatus) =>
    tasks.filter((t) => t.status === s && !t.archived)

  return (
    <div className="page">
      <div className="page-header">
        <h1>Board</h1>
        <button className="ghost" onClick={() => navigate('/tasks')}>
          List view →
        </button>
      </div>
      {error && <div className="error-banner">{error}</div>}
      <DndContext sensors={sensors} onDragEnd={onDragEnd}>
        <div className="board">
          {COLUMNS.map((s) => (
            <Column key={s} status={s} tasks={byStatus(s)} onOpen={(id) => navigate(`/tasks/${id}`)} />
          ))}
        </div>
      </DndContext>
    </div>
  )
}
