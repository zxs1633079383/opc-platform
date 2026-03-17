'use client'

import { useState } from 'react'
import { clsx } from 'clsx'
import { formatDistanceToNow } from 'date-fns'
import { ArrowUpRight, ArrowDownRight, DollarSign, Clock, GripVertical } from 'lucide-react'
import {
  DndContext,
  DragOverlay,
  closestCorners,
  KeyboardSensor,
  PointerSensor,
  useSensor,
  useSensors,
  type DragStartEvent,
  type DragEndEvent,
  type DragOverEvent,
} from '@dnd-kit/core'
import {
  SortableContext,
  verticalListSortingStrategy,
  useSortable,
} from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import type { Task } from '@/types'
import { useTranslation } from '@/lib/i18n'

interface TaskKanbanProps {
  tasks: Task[]
  onTaskClick: (task: Task) => void
}

const statusColors: Record<string, string> = {
  Running: 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-400',
  Pending: 'bg-gray-100 text-gray-800 dark:bg-gray-800 dark:text-gray-400',
  Completed: 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400',
  Failed: 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-400',
  Cancelled: 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-400',
}

function formatTokens(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`
  return String(n)
}

interface ColumnDef {
  id: string
  titleKey: string
  statuses: string[]
  headerColor: string
  dotColor: string
}

const columns: ColumnDef[] = [
  {
    id: 'todo',
    titleKey: 'tasks.todo',
    statuses: ['Pending'],
    headerColor: 'border-gray-400 dark:border-gray-500',
    dotColor: 'bg-gray-400',
  },
  {
    id: 'inprogress',
    titleKey: 'tasks.inProgress',
    statuses: ['Running'],
    headerColor: 'border-blue-500',
    dotColor: 'bg-blue-500 animate-pulse',
  },
  {
    id: 'done',
    titleKey: 'tasks.done',
    statuses: ['Completed', 'Failed', 'Cancelled'],
    headerColor: 'border-green-500',
    dotColor: 'bg-green-500',
  },
]

function getColumnForStatus(status: string): string {
  for (const col of columns) {
    if (col.statuses.includes(status)) return col.id
  }
  return 'todo'
}

function TaskCardContent({
  task,
  isDragging,
}: {
  task: Task
  isDragging?: boolean
}) {
  return (
    <div
      className={clsx(
        'w-full text-left p-3 bg-white dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-lg transition-all',
        isDragging
          ? 'shadow-xl rotate-2 opacity-90 ring-2 ring-primary-500'
          : 'hover:shadow-md hover:border-gray-300 dark:hover:border-gray-600'
      )}
    >
      <div className="flex items-center justify-between mb-2">
        <div className="flex items-center gap-1.5">
          <GripVertical className="w-3.5 h-3.5 text-gray-300 dark:text-gray-600 cursor-grab" />
          <span className="font-mono text-xs text-gray-400 dark:text-gray-500">
            {task.id.slice(0, 8)}
          </span>
        </div>
        <span
          className={clsx(
            'px-1.5 py-0.5 rounded-full text-[10px] font-medium',
            statusColors[task.status] || statusColors.Pending
          )}
        >
          {task.status}
        </span>
      </div>

      <div className="mb-2">
        <span className="inline-block px-1.5 py-0.5 text-xs font-medium rounded bg-indigo-100 text-indigo-700 dark:bg-indigo-900/30 dark:text-indigo-400">
          {task.agentName}
        </span>
      </div>

      <p className="text-sm text-gray-700 dark:text-gray-300 line-clamp-2 mb-3">
        {task.message}
      </p>

      <div className="flex items-center justify-between text-xs text-gray-500 dark:text-gray-400">
        <div className="flex items-center gap-2">
          <span className="flex items-center gap-0.5" title="Tokens in">
            <ArrowUpRight className="w-3 h-3" />
            {formatTokens(task.tokensIn || 0)}
          </span>
          <span className="flex items-center gap-0.5" title="Tokens out">
            <ArrowDownRight className="w-3 h-3" />
            {formatTokens(task.tokensOut || 0)}
          </span>
          <span className="flex items-center gap-0.5" title="Cost">
            <DollarSign className="w-3 h-3" />
            {(task.cost || 0).toFixed(2)}
          </span>
        </div>
        <span className="flex items-center gap-0.5" title="Time">
          <Clock className="w-3 h-3" />
          {formatDistanceToNow(new Date(task.createdAt), { addSuffix: false })}
        </span>
      </div>
    </div>
  )
}

function SortableTaskCard({
  task,
  onClick,
}: {
  task: Task
  onClick: () => void
}) {
  const {
    attributes,
    listeners,
    setNodeRef,
    transform,
    transition,
    isDragging,
  } = useSortable({ id: task.id })

  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.4 : 1,
  }

  return (
    <div ref={setNodeRef} style={style} {...attributes} {...listeners}>
      <button
        type="button"
        onClick={onClick}
        className="w-full focus:outline-none focus:ring-2 focus:ring-primary-500 rounded-lg"
      >
        <TaskCardContent task={task} />
      </button>
    </div>
  )
}

export function TaskKanban({ tasks, onTaskClick }: TaskKanbanProps) {
  const { t } = useTranslation()
  const [activeTask, setActiveTask] = useState<Task | null>(null)

  const sensors = useSensors(
    useSensor(PointerSensor, {
      activationConstraint: {
        distance: 8,
      },
    }),
    useSensor(KeyboardSensor)
  )

  const handleDragStart = (event: DragStartEvent) => {
    const task = tasks.find((t) => t.id === event.active.id)
    if (task) setActiveTask(task)
  }

  const handleDragEnd = (event: DragEndEvent) => {
    setActiveTask(null)

    const { active, over } = event
    if (!over) return

    const taskId = active.id as string
    const task = tasks.find((t) => t.id === taskId)
    if (!task) return

    // Determine target column
    let targetColumnId: string | null = null

    // Check if dropped over a column directly
    for (const col of columns) {
      if (over.id === col.id) {
        targetColumnId = col.id
        break
      }
    }

    // Check if dropped over another task -- find that task's column
    if (!targetColumnId) {
      const overTask = tasks.find((t) => t.id === over.id)
      if (overTask) {
        targetColumnId = getColumnForStatus(overTask.status)
      }
    }

    if (!targetColumnId) return

    const sourceColumnId = getColumnForStatus(task.status)
    if (sourceColumnId !== targetColumnId) {
      const targetColumn = columns.find((c) => c.id === targetColumnId)
      if (targetColumn) {
        const newStatus = targetColumn.statuses[0]
        console.log(
          `[Kanban] Task ${task.id.slice(0, 8)} moved: ${task.status} -> ${newStatus} (column: ${targetColumnId})`
        )
      }
    }
  }

  const handleDragOver = (_event: DragOverEvent) => {
    // Could add visual feedback here in the future
  }

  return (
    <DndContext
      sensors={sensors}
      collisionDetection={closestCorners}
      onDragStart={handleDragStart}
      onDragEnd={handleDragEnd}
      onDragOver={handleDragOver}
    >
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4 min-h-[400px]">
        {columns.map((col) => {
          const columnTasks = tasks.filter((t) =>
            col.statuses.includes(t.status)
          )
          const taskIds = columnTasks.map((t) => t.id)

          return (
            <div
              key={col.id}
              className="flex flex-col bg-gray-50 dark:bg-gray-800/50 rounded-xl"
            >
              <div
                className={clsx(
                  'flex items-center gap-2 px-4 py-3 border-t-2 rounded-t-xl',
                  col.headerColor
                )}
              >
                <span className={clsx('w-2 h-2 rounded-full', col.dotColor)} />
                <h3 className="text-sm font-semibold text-gray-700 dark:text-gray-200">
                  {t(col.titleKey)}
                </h3>
                <span className="ml-auto text-xs text-gray-400 dark:text-gray-500 bg-gray-200 dark:bg-gray-700 px-1.5 py-0.5 rounded-full">
                  {columnTasks.length}
                </span>
              </div>
              <SortableContext
                id={col.id}
                items={taskIds}
                strategy={verticalListSortingStrategy}
              >
                <div className="flex-1 p-3 space-y-2 overflow-y-auto max-h-[600px]">
                  {columnTasks.length === 0 ? (
                    <div className="text-center py-8 text-sm text-gray-400 dark:text-gray-500">
                      {t('tasks.noTasks')}
                    </div>
                  ) : (
                    columnTasks.map((task) => (
                      <SortableTaskCard
                        key={task.id}
                        task={task}
                        onClick={() => onTaskClick(task)}
                      />
                    ))
                  )}
                </div>
              </SortableContext>
            </div>
          )
        })}
      </div>

      <DragOverlay>
        {activeTask ? (
          <div className="w-80">
            <TaskCardContent task={activeTask} isDragging />
          </div>
        ) : null}
      </DragOverlay>
    </DndContext>
  )
}
