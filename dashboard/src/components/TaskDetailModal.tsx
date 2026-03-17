'use client'

import { clsx } from 'clsx'
import { format, formatDistanceStrict } from 'date-fns'
import { useQuery } from '@tanstack/react-query'
import {
  X,
  ArrowUpRight,
  ArrowDownRight,
  DollarSign,
  ChevronRight,
  Clock,
  AlertCircle,
  CheckCircle2,
  ScrollText,
} from 'lucide-react'
import { useState } from 'react'
import type { Task, Goal, Project, Issue, LogEntry } from '@/types'
import { fetchTaskLogs } from '@/lib/api'
import { useTranslation } from '@/lib/i18n'

interface TaskDetailModalProps {
  task: Task | null
  goals: Goal[]
  projects: Project[]
  issues: Issue[]
  onClose: () => void
  onFilterByGoal?: (goalId: string) => void
  onFilterByProject?: (projectId: string) => void
  onFilterByIssue?: (issueId: string) => void
}

const statusColors: Record<string, string> = {
  Running: 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-400',
  Pending: 'bg-gray-100 text-gray-800 dark:bg-gray-800 dark:text-gray-400',
  Completed: 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400',
  Failed: 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-400',
  Cancelled: 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-400',
}

const levelBadgeColors: Record<string, string> = {
  debug: 'bg-gray-100 text-gray-600 dark:bg-gray-800 dark:text-gray-400',
  info: 'bg-blue-100 text-blue-600 dark:bg-blue-900/30 dark:text-blue-400',
  warn: 'bg-yellow-100 text-yellow-600 dark:bg-yellow-900/30 dark:text-yellow-400',
  error: 'bg-red-100 text-red-600 dark:bg-red-900/30 dark:text-red-400',
}

function formatTokens(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`
  return String(n)
}

function formatTimestamp(ts?: string): string {
  if (!ts) return '-'
  return format(new Date(ts), 'yyyy-MM-dd HH:mm:ss')
}

function computeDuration(start?: string, end?: string): string {
  if (!start) return '-'
  const s = new Date(start)
  const e = end ? new Date(end) : new Date()
  return formatDistanceStrict(s, e)
}

type Tab = 'details' | 'logs'

function TaskLogsTab({ taskId, t }: { taskId: string; t: (key: string) => string }) {
  const { data: logs = '', isLoading } = useQuery<string>({
    queryKey: ['task-logs', taskId],
    queryFn: () => fetchTaskLogs(taskId),
    refetchInterval: 5000,
  })

  if (isLoading) {
    return (
      <div className="py-8 text-center text-gray-500 dark:text-gray-400 text-sm">
        {t('taskDetail.loadingLogs')}
      </div>
    )
  }

  if (!logs || logs.length === 0) {
    return (
      <div className="py-8 text-center text-gray-500 dark:text-gray-400 text-sm">
        {t('taskDetail.noLogs')}
      </div>
    )
  }

  return (
    <div className="space-y-2">
      <pre className="text-xs p-3 bg-gray-50 dark:bg-gray-800/50 rounded-lg whitespace-pre-wrap font-mono text-gray-700 dark:text-gray-300 overflow-auto max-h-96">
        {logs}
      </pre>
    </div>
  )
}

export function TaskDetailModal({
  task,
  goals,
  projects,
  issues,
  onClose,
  onFilterByGoal,
  onFilterByProject,
  onFilterByIssue,
}: TaskDetailModalProps) {
  const { t } = useTranslation()
  const [activeTab, setActiveTab] = useState<Tab>('details')

  if (!task) return null

  const issue = task.issueId
    ? issues.find((i) => i.id === task.issueId)
    : undefined
  const project = task.projectId
    ? projects.find((p) => p.id === task.projectId)
    : issue
      ? projects.find((p) => p.id === issue.projectId)
      : undefined
  const goal = task.goalId
    ? goals.find((g) => g.id === task.goalId)
    : project
      ? goals.find((g) => g.id === project.goalId)
      : undefined

  const breadcrumb = [
    goal ? { label: goal.name, cost: goal.cost, onClick: () => onFilterByGoal?.(goal.id) } : null,
    project ? { label: project.name, cost: project.cost, onClick: () => onFilterByProject?.(project.id) } : null,
    issue ? { label: issue.name, cost: issue.cost, onClick: () => onFilterByIssue?.(issue.id) } : null,
  ].filter(Boolean) as { label: string; cost: number; onClick: () => void }[]

  return (
    <div className="fixed inset-0 z-50 flex justify-end">
      {/* Backdrop */}
      <div
        className="absolute inset-0 bg-black/40 backdrop-blur-sm"
        onClick={onClose}
        onKeyDown={(e) => { if (e.key === 'Escape') onClose() }}
        role="button"
        tabIndex={0}
        aria-label="Close modal"
      />

      {/* Slide-over panel */}
      <div className="relative w-full max-w-lg bg-white dark:bg-gray-900 shadow-2xl overflow-y-auto animate-in slide-in-from-right">
        {/* Header */}
        <div className="sticky top-0 bg-white dark:bg-gray-900 border-b border-gray-200 dark:border-gray-700 px-6 py-4 z-10">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              <h2 className="text-lg font-semibold text-gray-900 dark:text-white">
                {t('taskDetail.title')}
              </h2>
              <span
                className={clsx(
                  'px-2 py-0.5 rounded-full text-xs font-medium',
                  statusColors[task.status] || statusColors.Pending
                )}
              >
                {task.status}
              </span>
            </div>
            <button
              type="button"
              onClick={onClose}
              className="p-1.5 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-800 text-gray-500 dark:text-gray-400 transition-colors"
            >
              <X className="w-5 h-5" />
            </button>
          </div>

          {/* Tabs */}
          <div className="flex gap-4 mt-3 border-b border-gray-200 dark:border-gray-700 -mx-6 px-6">
            <button
              type="button"
              onClick={() => setActiveTab('details')}
              className={clsx(
                'pb-2 text-sm font-medium border-b-2 transition-colors',
                activeTab === 'details'
                  ? 'border-primary-500 text-primary-600 dark:text-primary-400'
                  : 'border-transparent text-gray-500 dark:text-gray-400 hover:text-gray-700'
              )}
            >
              {t('taskDetail.details')}
            </button>
            <button
              type="button"
              onClick={() => setActiveTab('logs')}
              className={clsx(
                'pb-2 text-sm font-medium border-b-2 transition-colors flex items-center gap-1.5',
                activeTab === 'logs'
                  ? 'border-primary-500 text-primary-600 dark:text-primary-400'
                  : 'border-transparent text-gray-500 dark:text-gray-400 hover:text-gray-700'
              )}
            >
              <ScrollText className="w-3.5 h-3.5" />
              {t('taskDetail.logs')}
            </button>
          </div>
        </div>

        <div className="px-6 py-5 space-y-6">
          {activeTab === 'logs' ? (
            <TaskLogsTab taskId={task.id} t={t} />
          ) : (
            <>
              {/* Hierarchy breadcrumb */}
              {breadcrumb.length > 0 && (
                <div className="space-y-2">
                  <h3 className="text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-gray-400">
                    {t('taskDetail.hierarchy')}
                  </h3>
                  <div className="flex items-center flex-wrap gap-1 text-sm">
                    {breadcrumb.map((item, idx) => (
                      <span key={item.label} className="flex items-center gap-1">
                        {idx > 0 && (
                          <ChevronRight className="w-3.5 h-3.5 text-gray-400" />
                        )}
                        <button
                          type="button"
                          onClick={item.onClick}
                          className="text-primary-600 dark:text-primary-400 hover:underline"
                        >
                          {item.label}
                        </button>
                        <span className="text-gray-400 text-xs">
                          (${item.cost.toFixed(2)})
                        </span>
                      </span>
                    ))}
                    <ChevronRight className="w-3.5 h-3.5 text-gray-400" />
                    <span className="font-medium text-gray-700 dark:text-gray-300">
                      {t('taskDetail.task')}
                    </span>
                  </div>
                </div>
              )}

              {/* Basic info */}
              <div className="space-y-3">
                <h3 className="text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-gray-400">
                  {t('taskDetail.info')}
                </h3>
                <div className="grid grid-cols-2 gap-3 text-sm">
                  <div>
                    <span className="text-gray-500 dark:text-gray-400">ID</span>
                    <p className="font-mono text-xs text-gray-800 dark:text-gray-200 break-all">
                      {task.id}
                    </p>
                  </div>
                  <div>
                    <span className="text-gray-500 dark:text-gray-400">{t('taskDetail.agent')}</span>
                    <p className="text-gray-800 dark:text-gray-200">
                      <span className="inline-block px-1.5 py-0.5 text-xs font-medium rounded bg-indigo-100 text-indigo-700 dark:bg-indigo-900/30 dark:text-indigo-400">
                        {task.agentName}
                      </span>
                    </p>
                  </div>
                </div>
                <div>
                  <span className="text-sm text-gray-500 dark:text-gray-400">
                    {t('taskDetail.message')}
                  </span>
                  <p className="mt-1 text-sm text-gray-800 dark:text-gray-200 bg-gray-50 dark:bg-gray-800 p-3 rounded-lg">
                    {task.message}
                  </p>
                </div>
              </div>

              {/* Token consumption */}
              <div className="space-y-3">
                <h3 className="text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-gray-400">
                  {t('taskDetail.tokenConsumption')}
                </h3>
                <div className="grid grid-cols-3 gap-3">
                  <div className="bg-gray-50 dark:bg-gray-800 rounded-lg p-3 text-center">
                    <ArrowUpRight className="w-4 h-4 text-blue-500 mx-auto mb-1" />
                    <p className="text-lg font-semibold text-gray-900 dark:text-white">
                      {formatTokens(task.tokensIn || 0)}
                    </p>
                    <p className="text-xs text-gray-500 dark:text-gray-400">
                      {t('taskDetail.tokensIn')}
                    </p>
                  </div>
                  <div className="bg-gray-50 dark:bg-gray-800 rounded-lg p-3 text-center">
                    <ArrowDownRight className="w-4 h-4 text-emerald-500 mx-auto mb-1" />
                    <p className="text-lg font-semibold text-gray-900 dark:text-white">
                      {formatTokens(task.tokensOut || 0)}
                    </p>
                    <p className="text-xs text-gray-500 dark:text-gray-400">
                      {t('taskDetail.tokensOut')}
                    </p>
                  </div>
                  <div className="bg-gray-50 dark:bg-gray-800 rounded-lg p-3 text-center">
                    <DollarSign className="w-4 h-4 text-amber-500 mx-auto mb-1" />
                    <p className="text-lg font-semibold text-gray-900 dark:text-white">
                      ${(task.cost || 0).toFixed(4)}
                    </p>
                    <p className="text-xs text-gray-500 dark:text-gray-400">
                      {t('taskDetail.cost')}
                    </p>
                  </div>
                </div>
              </div>

              {/* Timestamps */}
              <div className="space-y-3">
                <h3 className="text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-gray-400">
                  {t('taskDetail.timestamps')}
                </h3>
                <div className="grid grid-cols-2 gap-3 text-sm">
                  <div>
                    <span className="text-gray-500 dark:text-gray-400">{t('taskDetail.created')}</span>
                    <p className="text-gray-800 dark:text-gray-200 text-xs">
                      {formatTimestamp(task.createdAt)}
                    </p>
                  </div>
                  <div>
                    <span className="text-gray-500 dark:text-gray-400">{t('taskDetail.started')}</span>
                    <p className="text-gray-800 dark:text-gray-200 text-xs">
                      {formatTimestamp(task.startedAt)}
                    </p>
                  </div>
                  <div>
                    <span className="text-gray-500 dark:text-gray-400">{t('taskDetail.ended')}</span>
                    <p className="text-gray-800 dark:text-gray-200 text-xs">
                      {formatTimestamp(task.endedAt)}
                    </p>
                  </div>
                  <div className="flex items-start gap-1.5">
                    <div>
                      <span className="text-gray-500 dark:text-gray-400">{t('taskDetail.duration')}</span>
                      <p className="text-gray-800 dark:text-gray-200 text-xs flex items-center gap-1">
                        <Clock className="w-3 h-3" />
                        {computeDuration(task.startedAt, task.endedAt)}
                      </p>
                    </div>
                  </div>
                </div>
              </div>

              {/* Error */}
              {task.error && (
                <div className="space-y-2">
                  <h3 className="text-xs font-semibold uppercase tracking-wider text-red-500">
                    {t('taskDetail.error')}
                  </h3>
                  <div className="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg p-3 flex gap-2">
                    <AlertCircle className="w-4 h-4 text-red-500 flex-shrink-0 mt-0.5" />
                    <pre className="text-sm text-red-700 dark:text-red-400 whitespace-pre-wrap break-all font-mono">
                      {task.error}
                    </pre>
                  </div>
                </div>
              )}

              {/* Result */}
              {task.result && (
                <div className="space-y-2">
                  <h3 className="text-xs font-semibold uppercase tracking-wider text-green-600 dark:text-green-400 flex items-center gap-1">
                    <CheckCircle2 className="w-3.5 h-3.5" />
                    {t('taskDetail.result')}
                  </h3>
                  <pre className="text-sm text-gray-800 dark:text-gray-200 bg-gray-50 dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg p-3 whitespace-pre-wrap break-all font-mono max-h-64 overflow-y-auto">
                    {task.result}
                  </pre>
                </div>
              )}
            </>
          )}
        </div>
      </div>
    </div>
  )
}
