'use client'

import { useQuery } from '@tanstack/react-query'
import { Search, Filter, LayoutGrid, List, Target, FolderKanban, CircleDot, AlertTriangle, ArrowUpRight, ArrowDownRight, DollarSign } from 'lucide-react'
import { useState, useMemo, useCallback } from 'react'
import { clsx } from 'clsx'
import { TaskList } from '@/components/TaskList'
import { TaskKanban } from '@/components/TaskKanban'
import { TaskDetailModal } from '@/components/TaskDetailModal'
import { fetchTasks, fetchGoals, fetchProjects, fetchIssues } from '@/lib/api'
import type { Task, Goal, Project, Issue } from '@/types'
import { useTranslation } from '@/lib/i18n'

type ViewMode = 'list' | 'kanban'
type DisplayLevel = 'task' | 'goal' | 'project' | 'issue'

function formatTokens(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`
  return String(n)
}

const levelStatusColors: Record<string, string> = {
  active: 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400',
  completed: 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-400',
  planned: 'bg-gray-100 text-gray-800 dark:bg-gray-800 dark:text-gray-400',
  open: 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-400',
  closed: 'bg-gray-100 text-gray-500 dark:bg-gray-800 dark:text-gray-500',
}

interface HierarchyCardProps {
  name: string
  description: string
  status: string
  tokensIn: number
  tokensOut: number
  cost: number
  taskCount: number
  completedCount: number
  failedCount: number
  taskLabel: string
  failedLabel: string
  onClick?: () => void
}

function HierarchyCard({
  name,
  description,
  status,
  tokensIn,
  tokensOut,
  cost,
  taskCount,
  completedCount,
  failedCount,
  taskLabel,
  failedLabel,
  onClick,
}: HierarchyCardProps) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="w-full text-left p-4 bg-white dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-xl hover:shadow-md transition-all hover:border-gray-300 dark:hover:border-gray-600 focus:outline-none focus:ring-2 focus:ring-primary-500"
    >
      <div className="flex items-start justify-between mb-2">
        <h3 className="font-semibold text-gray-900 dark:text-white truncate pr-2">
          {name}
        </h3>
        <span
          className={clsx(
            'px-2 py-0.5 rounded-full text-xs font-medium flex-shrink-0',
            levelStatusColors[status.toLowerCase()] || levelStatusColors.planned
          )}
        >
          {status}
        </span>
      </div>

      {description && (
        <p className="text-sm text-gray-500 dark:text-gray-400 line-clamp-2 mb-3">
          {description}
        </p>
      )}

      <div className="flex items-center gap-4 text-xs text-gray-500 dark:text-gray-400">
        <span className="flex items-center gap-1">
          <ArrowUpRight className="w-3 h-3" />
          {formatTokens(tokensIn)}
        </span>
        <span className="flex items-center gap-1">
          <ArrowDownRight className="w-3 h-3" />
          {formatTokens(tokensOut)}
        </span>
        <span className="flex items-center gap-1">
          <DollarSign className="w-3 h-3" />
          {cost.toFixed(2)}
        </span>
        <span className="ml-auto">
          {completedCount}/{taskCount} {taskLabel}
          {failedCount > 0 && (
            <span className="text-red-500 ml-1">({failedCount} {failedLabel})</span>
          )}
        </span>
      </div>
    </button>
  )
}

export default function TasksPage() {
  const { t } = useTranslation()
  const [search, setSearch] = useState('')
  const [statusFilter, setStatusFilter] = useState<string>('all')
  const [viewMode, setViewMode] = useState<ViewMode>('kanban')
  const [displayLevel, setDisplayLevel] = useState<DisplayLevel>('task')
  const [selectedTask, setSelectedTask] = useState<Task | null>(null)

  const { data: tasks = [], isLoading: tasksLoading } = useQuery({
    queryKey: ['tasks'],
    queryFn: fetchTasks,
  })

  const { data: goals = [] } = useQuery({
    queryKey: ['goals'],
    queryFn: fetchGoals,
  })

  const { data: projects = [] } = useQuery({
    queryKey: ['projects'],
    queryFn: fetchProjects,
  })

  const { data: issues = [] } = useQuery({
    queryKey: ['issues'],
    queryFn: fetchIssues,
  })

  const filteredTasks = useMemo(() => {
    return tasks.filter((task) => {
      const matchesSearch =
        search === '' ||
        task.message.toLowerCase().includes(search.toLowerCase()) ||
        task.agentName.toLowerCase().includes(search.toLowerCase()) ||
        task.id.toLowerCase().includes(search.toLowerCase())

      const matchesStatus =
        statusFilter === 'all' || task.status === statusFilter

      return matchesSearch && matchesStatus
    })
  }, [tasks, search, statusFilter])

  const getTasksForGoal = useCallback(
    (goalId: string): Task[] =>
      tasks.filter((t) => t.goalId === goalId),
    [tasks]
  )

  const getTasksForProject = useCallback(
    (projectId: string): Task[] =>
      tasks.filter((t) => t.projectId === projectId),
    [tasks]
  )

  const getTasksForIssue = useCallback(
    (issueId: string): Task[] =>
      tasks.filter((t) => t.issueId === issueId),
    [tasks]
  )

  const handleTaskClick = useCallback((task: Task) => {
    setSelectedTask(task)
  }, [])

  const handleCloseModal = useCallback(() => {
    setSelectedTask(null)
  }, [])

  const totalTokensIn = tasks.reduce((sum, t) => sum + (t.tokensIn || 0), 0)
  const totalTokensOut = tasks.reduce((sum, t) => sum + (t.tokensOut || 0), 0)
  const totalCost = tasks.reduce((sum, t) => sum + (t.cost || 0), 0)

  const isLoading = tasksLoading

  return (
    <div className="p-6 space-y-6">
      {/* Header */}
      <div className="flex justify-between items-center">
        <div>
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white">
            {t('tasks.title')}
          </h1>
          <p className="text-gray-500 dark:text-gray-400">
            {t('tasks.subtitle')}
          </p>
        </div>

        {/* View toggle */}
        <div className="flex items-center gap-1 bg-gray-100 dark:bg-gray-800 rounded-lg p-1">
          <button
            type="button"
            onClick={() => setViewMode('list')}
            className={clsx(
              'flex items-center gap-1.5 px-3 py-1.5 rounded-md text-sm font-medium transition-colors',
              viewMode === 'list'
                ? 'bg-white dark:bg-gray-700 text-gray-900 dark:text-white shadow-sm'
                : 'text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300'
            )}
          >
            <List className="w-4 h-4" />
            {t('tasks.list')}
          </button>
          <button
            type="button"
            onClick={() => setViewMode('kanban')}
            className={clsx(
              'flex items-center gap-1.5 px-3 py-1.5 rounded-md text-sm font-medium transition-colors',
              viewMode === 'kanban'
                ? 'bg-white dark:bg-gray-700 text-gray-900 dark:text-white shadow-sm'
                : 'text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300'
            )}
          >
            <LayoutGrid className="w-4 h-4" />
            {t('tasks.kanban')}
          </button>
        </div>
      </div>

      {/* Filters */}
      <div className="flex flex-wrap gap-4">
        <div className="flex-1 min-w-[200px] relative">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-5 h-5 text-gray-400" />
          <input
            type="text"
            placeholder={t('tasks.search')}
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="w-full pl-10 pr-4 py-2 border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900 text-gray-900 dark:text-white focus:ring-2 focus:ring-primary-500 focus:border-transparent"
          />
        </div>

        <div className="relative">
          <Filter className="absolute left-3 top-1/2 -translate-y-1/2 w-5 h-5 text-gray-400" />
          <select
            value={statusFilter}
            onChange={(e) => setStatusFilter(e.target.value)}
            className="pl-10 pr-8 py-2 border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900 text-gray-900 dark:text-white focus:ring-2 focus:ring-primary-500 focus:border-transparent appearance-none"
          >
            <option value="all">{t('tasks.allStatus')}</option>
            <option value="Pending">{t('tasks.pending')}</option>
            <option value="Running">{t('tasks.running')}</option>
            <option value="Completed">{t('tasks.completed')}</option>
            <option value="Failed">{t('tasks.failed')}</option>
            <option value="Cancelled">{t('tasks.cancelled')}</option>
          </select>
        </div>

        {/* Level filter */}
        <div className="flex items-center gap-1 bg-gray-100 dark:bg-gray-800 rounded-lg p-1">
          {([
            { key: 'task' as const, icon: CircleDot, label: 'Task' },
            { key: 'goal' as const, icon: Target, label: 'Goal' },
            { key: 'project' as const, icon: FolderKanban, label: 'Project' },
            { key: 'issue' as const, icon: AlertTriangle, label: 'Issue' },
          ]).map(({ key, icon: Icon, label }) => (
            <button
              key={key}
              type="button"
              onClick={() => setDisplayLevel(key)}
              className={clsx(
                'flex items-center gap-1.5 px-3 py-1.5 rounded-md text-sm font-medium transition-colors',
                displayLevel === key
                  ? 'bg-white dark:bg-gray-700 text-gray-900 dark:text-white shadow-sm'
                  : 'text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300'
              )}
            >
              <Icon className="w-4 h-4" />
              {label}
            </button>
          ))}
        </div>
      </div>

      {/* Content */}
      <div className="min-h-[400px]">
        {isLoading ? (
          <div className="flex items-center justify-center h-48">
            <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary-500" />
          </div>
        ) : displayLevel === 'task' ? (
          viewMode === 'kanban' ? (
            <TaskKanban tasks={filteredTasks} onTaskClick={handleTaskClick} />
          ) : (
            <div className="bg-white dark:bg-gray-900 rounded-xl shadow-sm border border-gray-200 dark:border-gray-800 p-6">
              <TaskList tasks={filteredTasks} />
            </div>
          )
        ) : displayLevel === 'goal' ? (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            {goals.length === 0 ? (
              <div className="col-span-full text-center py-12 text-gray-500 dark:text-gray-400">
                {t('tasks.noGoals')}
              </div>
            ) : (
              goals.map((goal) => {
                const goalTasks = getTasksForGoal(goal.id)
                return (
                  <HierarchyCard
                    key={goal.id}
                    name={goal.name}
                    description={goal.description}
                    status={goal.status}
                    tokensIn={goal.tokensIn}
                    tokensOut={goal.tokensOut}
                    cost={goal.cost}
                    taskCount={goalTasks.length}
                    completedCount={goalTasks.filter((t) => t.status === 'Completed').length}
                    failedCount={goalTasks.filter((t) => t.status === 'Failed').length}
                    taskLabel={t('tasks.tasks')}
                    failedLabel={t('tasks.failed')}
                  />
                )
              })
            )}
          </div>
        ) : displayLevel === 'project' ? (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            {projects.length === 0 ? (
              <div className="col-span-full text-center py-12 text-gray-500 dark:text-gray-400">
                {t('tasks.noProjects')}
              </div>
            ) : (
              projects.map((project) => {
                const projectTasks = getTasksForProject(project.id)
                return (
                  <HierarchyCard
                    key={project.id}
                    name={project.name}
                    description={project.description}
                    status={project.status}
                    tokensIn={project.tokensIn}
                    tokensOut={project.tokensOut}
                    cost={project.cost}
                    taskCount={projectTasks.length}
                    completedCount={projectTasks.filter((t) => t.status === 'Completed').length}
                    failedCount={projectTasks.filter((t) => t.status === 'Failed').length}
                    taskLabel={t('tasks.tasks')}
                    failedLabel={t('tasks.failed')}
                  />
                )
              })
            )}
          </div>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            {issues.length === 0 ? (
              <div className="col-span-full text-center py-12 text-gray-500 dark:text-gray-400">
                {t('tasks.noIssues')}
              </div>
            ) : (
              issues.map((issue) => {
                const issueTasks = getTasksForIssue(issue.id)
                return (
                  <HierarchyCard
                    key={issue.id}
                    name={issue.name}
                    description={issue.description}
                    status={issue.status}
                    tokensIn={issue.tokensIn}
                    tokensOut={issue.tokensOut}
                    cost={issue.cost}
                    taskCount={issueTasks.length}
                    completedCount={issueTasks.filter((t) => t.status === 'Completed').length}
                    failedCount={issueTasks.filter((t) => t.status === 'Failed').length}
                    taskLabel={t('tasks.tasks')}
                    failedLabel={t('tasks.failed')}
                  />
                )
              })
            )}
          </div>
        )}
      </div>

      {/* Summary */}
      <div className="flex flex-wrap gap-4 text-sm text-gray-500 dark:text-gray-400 bg-white dark:bg-gray-900 rounded-xl border border-gray-200 dark:border-gray-800 p-4">
        <span>{t('tasks.total')}: {tasks.length}</span>
        <span className="text-gray-300 dark:text-gray-600">|</span>
        <span>{t('tasks.running')}: {tasks.filter((t) => t.status === 'Running').length}</span>
        <span>{t('tasks.completed')}: {tasks.filter((t) => t.status === 'Completed').length}</span>
        <span>{t('tasks.failed')}: {tasks.filter((t) => t.status === 'Failed').length}</span>
        <span className="text-gray-300 dark:text-gray-600">|</span>
        <span className="flex items-center gap-1">
          <ArrowUpRight className="w-3.5 h-3.5" />
          {formatTokens(totalTokensIn)}
        </span>
        <span className="flex items-center gap-1">
          <ArrowDownRight className="w-3.5 h-3.5" />
          {formatTokens(totalTokensOut)}
        </span>
        <span className="flex items-center gap-1">
          <DollarSign className="w-3.5 h-3.5" />
          ${totalCost.toFixed(2)}
        </span>
      </div>

      {/* Task detail modal */}
      <TaskDetailModal
        task={selectedTask}
        goals={goals}
        projects={projects}
        issues={issues}
        onClose={handleCloseModal}
      />
    </div>
  )
}
