'use client'

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useParams } from 'next/navigation'
import Link from 'next/link'
import {
  ArrowLeft, CheckCircle2, Clock, Loader2, AlertCircle,
} from 'lucide-react'
import { fetchGoalDetail, fetchGoalProjects, fetchGoalStats, fetchGoalIssues, approveGoal } from '@/lib/api'
import { GoalTree } from '@/components/GoalTree'
import { Skeleton } from '@/components/Skeleton'
import type { Project, Issue } from '@/types'

const phaseConfig: Record<string, { color: string; label: string }> = {
  decomposing: { color: 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400', label: 'Decomposing' },
  planned: { color: 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-400', label: 'Awaiting Approval' },
  approved: { color: 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400', label: 'Approved' },
  in_progress: { color: 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400', label: 'In Progress' },
  completed: { color: 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400', label: 'Completed' },
  failed: { color: 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400', label: 'Failed' },
}

function buildTreeNodes(projects: Project[], issues: Issue[]) {
  return projects.map((p) => ({
    name: p.name,
    status: p.status,
    cost: p.cost,
    children: issues
      .filter((iss) => iss.projectId === p.id)
      .map((iss) => ({
        name: iss.name,
        status: iss.status,
        cost: iss.cost,
        agent: iss.agentRef,
      })),
  }))
}

export default function GoalDetailPage() {
  const params = useParams()
  const id = params.id as string
  const queryClient = useQueryClient()

  const { data: goal, isLoading } = useQuery({
    queryKey: ['goal-detail', id],
    queryFn: () => fetchGoalDetail(id),
  })

  const { data: projects = [] } = useQuery({
    queryKey: ['goal-projects', id],
    queryFn: () => fetchGoalProjects(id),
    enabled: !!goal,
  })

  const { data: issues = [] } = useQuery({
    queryKey: ['goal-issues', id],
    queryFn: () => fetchGoalIssues(id),
    enabled: !!goal,
  })

  const { data: stats } = useQuery({
    queryKey: ['goal-stats', id],
    queryFn: () => fetchGoalStats(id),
    enabled: !!goal,
  })

  const approveMutation = useMutation({
    mutationFn: () => approveGoal(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['goal-detail', id] })
      queryClient.invalidateQueries({ queryKey: ['goals'] })
    },
  })

  if (isLoading) {
    return (
      <div className="p-6 space-y-4">
        <Skeleton className="h-8 w-48" />
        <Skeleton className="h-4 w-96" />
        <Skeleton className="h-64 w-full" />
      </div>
    )
  }

  if (!goal) {
    return (
      <div className="p-6">
        <p className="text-gray-500">Goal not found</p>
        <Link href="/goals" className="text-blue-600 hover:underline text-sm mt-2 inline-block">Back to Goals</Link>
      </div>
    )
  }

  const phase = phaseConfig[goal.phase || goal.status] || { color: 'bg-gray-100 text-gray-700', label: goal.phase || goal.status }
  const treeNodes = buildTreeNodes(projects, issues)

  return (
    <div className="p-6 space-y-6">
      {/* Header */}
      <div className="flex items-start justify-between">
        <div className="flex items-start gap-3">
          <Link href="/goals" className="mt-1 p-1 hover:bg-gray-100 dark:hover:bg-gray-800 rounded">
            <ArrowLeft className="w-5 h-5 text-gray-400" />
          </Link>
          <div>
            <div className="flex items-center gap-3">
              <h1 className="text-2xl font-bold text-gray-900 dark:text-white">{goal.name}</h1>
              <span className={`px-2.5 py-0.5 rounded-full text-xs font-medium ${phase.color}`}>{phase.label}</span>
            </div>
            {goal.description && (
              <p className="text-gray-500 dark:text-gray-400 mt-1">{goal.description}</p>
            )}
            <div className="flex gap-4 mt-2 text-xs text-gray-400">
              {goal.owner && <span>Owner: {goal.owner}</span>}
              {goal.deadline && <span><Clock className="w-3 h-3 inline" /> {goal.deadline}</span>}
              <span>Created: {new Date(goal.createdAt).toLocaleDateString()}</span>
            </div>
          </div>
        </div>
        {(goal.phase === 'planned') && (
          <button
            onClick={() => approveMutation.mutate()}
            disabled={approveMutation.isPending}
            className="flex items-center gap-2 px-4 py-2 text-sm text-white bg-green-600 hover:bg-green-700 rounded-lg disabled:opacity-50"
          >
            {approveMutation.isPending ? <Loader2 className="w-4 h-4 animate-spin" /> : <CheckCircle2 className="w-4 h-4" />}
            Approve Plan
          </button>
        )}
      </div>

      {/* Stats */}
      {stats && (
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
          <div className="bg-white dark:bg-gray-900 rounded-xl border border-gray-200 dark:border-gray-800 p-4">
            <p className="text-sm text-gray-500">Total Tasks</p>
            <p className="text-2xl font-semibold text-gray-900 dark:text-white">{stats.taskCount}</p>
          </div>
          <div className="bg-white dark:bg-gray-900 rounded-xl border border-gray-200 dark:border-gray-800 p-4">
            <p className="text-sm text-gray-500">Completed</p>
            <p className="text-2xl font-semibold text-green-600">{stats.completedTasks}</p>
          </div>
          <div className="bg-white dark:bg-gray-900 rounded-xl border border-gray-200 dark:border-gray-800 p-4">
            <p className="text-sm text-gray-500">Failed</p>
            <p className="text-2xl font-semibold text-red-600">{stats.failedTasks}</p>
          </div>
          <div className="bg-white dark:bg-gray-900 rounded-xl border border-gray-200 dark:border-gray-800 p-4">
            <p className="text-sm text-gray-500">Total Cost</p>
            <p className="text-2xl font-semibold text-gray-900 dark:text-white">${stats.totalCost.toFixed(4)}</p>
          </div>
        </div>
      )}

      {/* Hierarchy Tree */}
      <div className="bg-white dark:bg-gray-900 rounded-xl shadow-sm border border-gray-200 dark:border-gray-800 p-6">
        <h2 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">Decomposition Hierarchy</h2>
        {goal.phase === 'decomposing' ? (
          <div className="flex items-center gap-2 text-sm text-blue-500 py-4">
            <Loader2 className="w-4 h-4 animate-spin" />
            AI is decomposing this goal...
          </div>
        ) : (
          <GoalTree nodes={treeNodes} />
        )}
      </div>

      {/* Decomposition Plan */}
      {goal.decompositionPlan && (
        <div className="bg-white dark:bg-gray-900 rounded-xl shadow-sm border border-gray-200 dark:border-gray-800 p-6">
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white mb-3">
            <AlertCircle className="w-5 h-5 inline mr-2 text-purple-500" />
            AI Decomposition Plan
          </h2>
          <pre className="text-sm text-gray-600 dark:text-gray-400 whitespace-pre-wrap">{goal.decompositionPlan}</pre>
        </div>
      )}
    </div>
  )
}
