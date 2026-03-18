'use client'

import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  Plus, Target, ChevronDown, ChevronRight, Trash2,
  FolderKanban, ListTodo, Bot, Loader2, Zap, CheckCircle2,
  XCircle, Clock, AlertCircle,
} from 'lucide-react'
import {
  fetchGoals, createGoal, deleteGoal,
  fetchGoalProjects, fetchGoalStats,
} from '@/lib/api'
import type { Goal, Project, HierarchyStats } from '@/types'

const phaseConfig: Record<string, { icon: typeof Loader2; color: string; label: string }> = {
  decomposing: { icon: Loader2, color: 'text-blue-500', label: 'AI Decomposing...' },
  planned: { icon: AlertCircle, color: 'text-yellow-500', label: 'Awaiting Approval' },
  approved: { icon: CheckCircle2, color: 'text-green-500', label: 'Approved' },
  in_progress: { icon: Zap, color: 'text-blue-500', label: 'In Progress' },
  completed: { icon: CheckCircle2, color: 'text-green-600', label: 'Completed' },
  failed: { icon: XCircle, color: 'text-red-500', label: 'Failed' },
  active: { icon: Target, color: 'text-gray-500', label: 'Active' },
}

function GoalCard({ goal, onDelete }: { goal: Goal; onDelete: () => void }) {
  const [expanded, setExpanded] = useState(false)

  const { data: projects = [] } = useQuery({
    queryKey: ['goal-projects', goal.id],
    queryFn: () => fetchGoalProjects(goal.id),
    enabled: expanded,
  })

  const { data: stats } = useQuery<HierarchyStats>({
    queryKey: ['goal-stats', goal.id],
    queryFn: () => fetchGoalStats(goal.id),
    enabled: expanded,
  })

  const phase = phaseConfig[goal.phase || goal.status] || phaseConfig.active
  const PhaseIcon = phase.icon
  const isDecomposing = goal.phase === 'decomposing'

  return (
    <div className="bg-white dark:bg-gray-900 rounded-xl shadow-sm border border-gray-200 dark:border-gray-800">
      {/* Header */}
      <div
        className="flex items-start justify-between p-5 cursor-pointer hover:bg-gray-50 dark:hover:bg-gray-800/50 rounded-t-xl"
        onClick={() => setExpanded(prev => !prev)}
      >
        <div className="flex items-start gap-3 flex-1">
          <div className="mt-0.5">
            {expanded ? <ChevronDown className="w-5 h-5 text-gray-400" /> : <ChevronRight className="w-5 h-5 text-gray-400" />}
          </div>
          <div className="flex-1 min-w-0">
            <div className="flex items-center gap-2 flex-wrap">
              <h3 className="text-lg font-semibold text-gray-900 dark:text-white">{goal.name}</h3>
              <span className={`inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium ${phase.color} bg-opacity-10`}>
                <PhaseIcon className={`w-3 h-3 ${isDecomposing ? 'animate-spin' : ''}`} />
                {phase.label}
              </span>
            </div>
            {goal.description && (
              <p className="text-sm text-gray-500 dark:text-gray-400 mt-1 line-clamp-2">{goal.description}</p>
            )}
            <div className="flex gap-4 mt-2 text-xs text-gray-400">
              {goal.owner && <span>Owner: {goal.owner}</span>}
              {goal.deadline && <span><Clock className="w-3 h-3 inline" /> {goal.deadline}</span>}
              {goal.cost > 0 && <span>${goal.cost.toFixed(4)}</span>}
              {(goal.tokensIn > 0 || goal.tokensOut > 0) && <span>Tokens: {goal.tokensIn + goal.tokensOut}</span>}
            </div>
          </div>
        </div>
        <button
          onClick={(e) => { e.stopPropagation(); onDelete() }}
          className="p-1.5 text-gray-400 hover:text-red-500 hover:bg-red-50 dark:hover:bg-red-900/20 rounded-lg"
        >
          <Trash2 className="w-4 h-4" />
        </button>
      </div>

      {/* Expanded: Auto-decomposed hierarchy (read-only) */}
      {expanded && (
        <div className="border-t border-gray-200 dark:border-gray-800 px-5 py-4">
          {/* Stats summary */}
          {stats && (
            <div className="flex gap-6 mb-4 text-sm">
              <span className="text-gray-500">{stats.taskCount} tasks</span>
              <span className="text-green-600">{stats.completedTasks} success</span>
              <span className="text-red-600">{stats.failedTasks} fail</span>
              {stats.totalCost > 0 && <span className="text-gray-400">${stats.totalCost.toFixed(4)}</span>}
              {stats.taskCount > 0 && (
                <div className="flex-1 flex items-center gap-2">
                  <div className="flex-1 h-1.5 bg-gray-200 dark:bg-gray-700 rounded-full overflow-hidden">
                    <div className="h-full flex">
                      <div className="bg-green-500" style={{ width: `${(stats.completedTasks / stats.taskCount) * 100}%` }} />
                      <div className="bg-red-500" style={{ width: `${(stats.failedTasks / stats.taskCount) * 100}%` }} />
                    </div>
                  </div>
                  <span className="text-xs text-gray-400">{Math.round(((stats.completedTasks + stats.failedTasks) / stats.taskCount) * 100)}%</span>
                </div>
              )}
            </div>
          )}

          {/* Decomposition plan */}
          {goal.decompositionPlan && (
            <div className="mb-3">
              <span className="text-xs font-medium text-gray-500 uppercase tracking-wide">AI Decomposition Plan</span>
            </div>
          )}

          {/* Projects tree */}
          {projects.length > 0 ? (
            <div className="space-y-2">
              {projects.map((p) => (
                <ProjectNode key={p.id} project={p} />
              ))}
            </div>
          ) : isDecomposing ? (
            <div className="flex items-center gap-2 text-sm text-blue-500 py-4">
              <Loader2 className="w-4 h-4 animate-spin" />
              AI is decomposing this goal into projects and tasks...
            </div>
          ) : (
            <p className="text-sm text-gray-400 italic py-2">No projects yet. Enable auto-decompose when creating goals.</p>
          )}
        </div>
      )}
    </div>
  )
}

function ProjectNode({ project }: { project: Project }) {
  const [open, setOpen] = useState(false)
  return (
    <div className="border-l-2 border-blue-200 dark:border-blue-800 pl-4">
      <button
        onClick={() => setOpen(prev => !prev)}
        className="flex items-center gap-2 text-sm font-medium text-gray-800 dark:text-gray-200 hover:text-blue-600 w-full text-left py-1"
      >
        {open ? <ChevronDown className="w-3.5 h-3.5" /> : <ChevronRight className="w-3.5 h-3.5" />}
        <FolderKanban className="w-4 h-4 text-blue-500" />
        {project.name}
        <span className="text-xs text-gray-400 ml-auto">{project.status}</span>
      </button>
      {open && project.description && (
        <p className="text-xs text-gray-500 pl-7 pb-2">{project.description}</p>
      )}
    </div>
  )
}

function CreateGoalModal({ isOpen, onClose, onSubmit }: {
  isOpen: boolean
  onClose: () => void
  onSubmit: (data: { name: string; description: string; owner?: string; deadline?: string; autoDecompose: boolean; approval: string }) => void
}) {
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [owner, setOwner] = useState('')
  const [deadline, setDeadline] = useState('')
  const [autoDecompose, setAutoDecompose] = useState(true)
  const [approval, setApproval] = useState<'auto' | 'required'>('auto')

  if (!isOpen) return null

  const handleSubmit = () => {
    if (!name.trim()) return
    onSubmit({
      name: name.trim(),
      description: description.trim(),
      owner: owner.trim() || undefined,
      deadline: deadline || undefined,
      autoDecompose,
      approval,
    })
    setName(''); setDescription(''); setOwner(''); setDeadline('')
    onClose()
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50" onClick={onClose}>
      <div className="bg-white dark:bg-gray-900 rounded-xl shadow-xl w-full max-w-lg mx-4" onClick={(e) => e.stopPropagation()}>
        <div className="flex items-center justify-between p-5 border-b border-gray-200 dark:border-gray-800">
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white">Create Goal</h2>
          <button onClick={onClose} className="text-gray-400 hover:text-gray-600">
            <XCircle className="w-5 h-5" />
          </button>
        </div>
        <div className="p-5 space-y-4">
          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Goal Name *</label>
            <input
              value={name} onChange={(e) => setName(e.target.value)}
              placeholder="e.g. Build messaging system"
              className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-800 text-gray-900 dark:text-white text-sm focus:ring-2 focus:ring-primary-500 focus:border-transparent"
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Description</label>
            <textarea
              value={description} onChange={(e) => setDescription(e.target.value)}
              placeholder="Describe what you want to achieve. AI will decompose this into projects, tasks, and issues."
              rows={3}
              className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-800 text-gray-900 dark:text-white text-sm focus:ring-2 focus:ring-primary-500 focus:border-transparent"
            />
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Owner</label>
              <input
                value={owner} onChange={(e) => setOwner(e.target.value)}
                placeholder="Optional"
                className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-800 text-gray-900 dark:text-white text-sm"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Deadline</label>
              <input
                type="date" value={deadline} onChange={(e) => setDeadline(e.target.value)}
                className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-800 text-gray-900 dark:text-white text-sm"
              />
            </div>
          </div>

          {/* Auto Decompose */}
          <div className="bg-purple-50 dark:bg-purple-900/10 rounded-lg p-4 space-y-3">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <Zap className="w-4 h-4 text-purple-500" />
                <span className="text-sm font-medium text-gray-800 dark:text-gray-200">AI Auto-Decompose</span>
              </div>
              <button
                onClick={() => setAutoDecompose(!autoDecompose)}
                className={`relative w-10 h-5 rounded-full transition-colors ${autoDecompose ? 'bg-purple-500' : 'bg-gray-300 dark:bg-gray-600'}`}
              >
                <span className={`absolute top-0.5 w-4 h-4 bg-white rounded-full shadow transition-transform ${autoDecompose ? 'translate-x-5' : 'translate-x-0.5'}`} />
              </button>
            </div>
            {autoDecompose && (
              <>
                <p className="text-xs text-gray-500">AI will automatically decompose your goal into Projects → Tasks → Issues and assign agents.</p>
                <div className="flex gap-3">
                  <label className="flex items-center gap-1.5 text-sm cursor-pointer">
                    <input type="radio" checked={approval === 'auto'} onChange={() => setApproval('auto')} className="text-purple-500" />
                    <span className="text-gray-700 dark:text-gray-300">Auto-execute</span>
                  </label>
                  <label className="flex items-center gap-1.5 text-sm cursor-pointer">
                    <input type="radio" checked={approval === 'required'} onChange={() => setApproval('required')} className="text-purple-500" />
                    <span className="text-gray-700 dark:text-gray-300">Review first</span>
                  </label>
                </div>
              </>
            )}
          </div>
        </div>
        <div className="flex justify-end gap-3 p-5 border-t border-gray-200 dark:border-gray-800">
          <button onClick={onClose} className="px-4 py-2 text-sm text-gray-600 hover:bg-gray-100 dark:hover:bg-gray-800 rounded-lg">Cancel</button>
          <button
            onClick={handleSubmit}
            disabled={!name.trim()}
            className="px-4 py-2 text-sm text-white bg-purple-600 hover:bg-purple-700 rounded-lg disabled:opacity-50 flex items-center gap-2"
          >
            {autoDecompose && <Zap className="w-4 h-4" />}
            {autoDecompose ? 'Create & Decompose' : 'Create Goal'}
          </button>
        </div>
      </div>
    </div>
  )
}

export default function GoalsPage() {
  const [showCreateModal, setShowCreateModal] = useState(false)
  const queryClient = useQueryClient()

  const { data: goals = [], isLoading } = useQuery({
    queryKey: ['goals'],
    queryFn: fetchGoals,
    refetchInterval: 5000, // Poll for decomposition status updates
  })

  const createMutation = useMutation({
    mutationFn: createGoal,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['goals'] }),
  })

  const deleteMutation = useMutation({
    mutationFn: deleteGoal,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['goals'] }),
  })

  return (
    <div className="p-6 space-y-6">
      <div className="flex justify-between items-center">
        <div>
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white">Goals</h1>
          <p className="text-gray-500 dark:text-gray-400">
            Create strategic goals — AI auto-decomposes into Projects → Tasks → Issues
          </p>
        </div>
        <button
          onClick={() => setShowCreateModal(true)}
          className="flex items-center gap-2 px-4 py-2 text-sm text-white bg-purple-600 hover:bg-purple-700 rounded-lg transition-colors"
        >
          <Plus className="w-4 h-4" />
          Create Goal
        </button>
      </div>

      {isLoading ? (
        <div className="space-y-4">
          {[1, 2, 3].map((i) => (
            <div key={i} className="bg-white dark:bg-gray-900 rounded-xl h-24 animate-pulse border border-gray-200 dark:border-gray-800" />
          ))}
        </div>
      ) : goals.length === 0 ? (
        <div className="bg-white dark:bg-gray-900 rounded-xl shadow-sm border border-gray-200 dark:border-gray-800 p-12">
          <div className="max-w-md mx-auto text-center">
            <div className="w-16 h-16 bg-purple-100 dark:bg-purple-900/20 rounded-full flex items-center justify-center mx-auto mb-4">
              <Target className="w-8 h-8 text-purple-500" />
            </div>
            <h3 className="text-lg font-medium text-gray-900 dark:text-white mb-2">No goals yet</h3>
            <p className="text-gray-500 dark:text-gray-400 mb-6">
              Create your first goal. Just describe what you want — AI handles the rest.
            </p>
            <button
              onClick={() => setShowCreateModal(true)}
              className="inline-flex items-center gap-2 px-4 py-2 text-sm text-white bg-purple-600 hover:bg-purple-700 rounded-lg"
            >
              <Zap className="w-4 h-4" />
              Create Goal
            </button>
          </div>
        </div>
      ) : (
        <div className="space-y-4">
          {goals.map((goal) => (
            <GoalCard
              key={goal.id}
              goal={goal}
              onDelete={() => { if (confirm(`Delete goal "${goal.name}"?`)) deleteMutation.mutate(goal.id) }}
            />
          ))}
        </div>
      )}

      <CreateGoalModal
        isOpen={showCreateModal}
        onClose={() => setShowCreateModal(false)}
        onSubmit={(data) => createMutation.mutate(data)}
      />
    </div>
  )
}
