'use client'

import { useState, useCallback } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  Plus,
  Target,
  ChevronDown,
  ChevronRight,
  User,
  Calendar,
  Trash2,
  FolderKanban,
  AlertTriangle,
  X,
  Zap,
  DollarSign,
  CheckCircle2,
  ListTodo,
  Check,
  Bot,
  ChevronLeft,
} from 'lucide-react'
import {
  fetchGoals,
  fetchGoalProjects,
  fetchGoalStats,
  createGoal,
  deleteGoal,
  createProject,
  deleteProject,
  fetchAgents,
  applyAgent,
} from '@/lib/api'
import type { Goal, Project, HierarchyStats, Agent } from '@/types'
import { useTranslation } from '@/lib/i18n'

const STATUS_STYLES: Record<string, { border: string; badge: string; label: string }> = {
  active: {
    border: 'border-l-blue-500',
    badge: 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400',
    label: 'goals.active',
  },
  completed: {
    border: 'border-l-green-500',
    badge: 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400',
    label: 'goals.completed',
  },
  archived: {
    border: 'border-l-gray-400',
    badge: 'bg-gray-100 text-gray-600 dark:bg-gray-800 dark:text-gray-400',
    label: 'goals.archived',
  },
}

function formatCompactNumber(num: number): string {
  if (num >= 1_000_000) return `${(num / 1_000_000).toFixed(1)}M`
  if (num >= 1_000) return `${(num / 1_000).toFixed(1)}K`
  return String(num)
}

function formatCost(cost: number): string {
  return `$${cost.toFixed(4)}`
}

function StatsBar({
  stats,
  t,
}: {
  stats: HierarchyStats | undefined
  t: (key: string) => string
}) {
  if (!stats) {
    return (
      <div className="flex gap-4 text-xs text-gray-400 animate-pulse">
        <span>---</span>
      </div>
    )
  }

  return (
    <div className="flex flex-wrap gap-x-5 gap-y-1 text-xs text-gray-500 dark:text-gray-400">
      <span className="flex items-center gap-1">
        <ListTodo className="w-3.5 h-3.5" />
        {stats.taskCount} {t('goals.tasks')}
      </span>
      <span className="flex items-center gap-1">
        <CheckCircle2 className="w-3.5 h-3.5 text-green-500" />
        {stats.completedTasks} {t('goals.completed').toLowerCase()}
      </span>
      <span className="flex items-center gap-1">
        <Zap className="w-3.5 h-3.5" />
        {formatCompactNumber(stats.totalTokensIn + stats.totalTokensOut)} {t('goals.tokens')}
      </span>
      <span className="flex items-center gap-1">
        <DollarSign className="w-3.5 h-3.5" />
        {formatCost(stats.totalCost)}
      </span>
    </div>
  )
}

function ProjectCard({
  project,
  onDelete,
  t,
}: {
  project: Project
  onDelete: (id: string) => void
  t: (key: string) => string
}) {
  const [confirmDelete, setConfirmDelete] = useState(false)
  const statusStyle = STATUS_STYLES[project.status] || STATUS_STYLES.active

  return (
    <div className="bg-gray-50 dark:bg-gray-800/50 rounded-lg p-4 border border-gray-100 dark:border-gray-700">
      <div className="flex items-start justify-between">
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2">
            <FolderKanban className="w-4 h-4 text-gray-400 flex-shrink-0" />
            <h4 className="text-sm font-medium text-gray-900 dark:text-white truncate">
              {project.name}
            </h4>
            <span className={`px-2 py-0.5 rounded-full text-xs font-medium ${statusStyle.badge}`}>
              {t(statusStyle.label)}
            </span>
          </div>
          {project.description && (
            <p className="mt-1 text-xs text-gray-500 dark:text-gray-400 line-clamp-2 ml-6">
              {project.description}
            </p>
          )}
        </div>
        {confirmDelete ? (
          <div className="flex items-center gap-1 ml-2 flex-shrink-0">
            <button
              onClick={() => onDelete(project.id)}
              className="px-2 py-1 text-xs font-medium text-white bg-red-600 hover:bg-red-700 rounded transition-colors"
            >
              {t('common.delete')}
            </button>
            <button
              onClick={() => setConfirmDelete(false)}
              className="px-2 py-1 text-xs font-medium text-gray-500 hover:bg-gray-200 dark:hover:bg-gray-700 rounded transition-colors"
            >
              {t('common.cancel')}
            </button>
          </div>
        ) : (
          <button
            onClick={() => setConfirmDelete(true)}
            className="p-1 text-gray-400 hover:text-red-500 rounded transition-colors flex-shrink-0"
          >
            <Trash2 className="w-3.5 h-3.5" />
          </button>
        )}
      </div>
    </div>
  )
}

function GoalCard({
  goal,
  onDelete,
  onAddProject,
  t,
}: {
  goal: Goal
  onDelete: (id: string) => void
  onAddProject: (goalId: string) => void
  t: (key: string) => string
}) {
  const [expanded, setExpanded] = useState(false)
  const [confirmDelete, setConfirmDelete] = useState(false)
  const queryClient = useQueryClient()
  const statusStyle = STATUS_STYLES[goal.status] || STATUS_STYLES.active

  const { data: projects = [] } = useQuery({
    queryKey: ['goal-projects', goal.id],
    queryFn: () => fetchGoalProjects(goal.id),
    enabled: expanded,
  })

  const { data: stats } = useQuery({
    queryKey: ['goal-stats', goal.id],
    queryFn: () => fetchGoalStats(goal.id),
  })

  const deleteProjectMutation = useMutation({
    mutationFn: deleteProject,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['goal-projects', goal.id] })
      queryClient.invalidateQueries({ queryKey: ['goal-stats', goal.id] })
    },
  })

  return (
    <div
      className={`bg-white dark:bg-gray-900 rounded-xl shadow-sm border border-gray-200 dark:border-gray-800 border-l-4 ${statusStyle.border} overflow-hidden transition-shadow hover:shadow-md`}
    >
      <div className="p-5">
        {/* Header row */}
        <div className="flex items-start justify-between gap-3">
          <div className="flex-1 min-w-0">
            <div className="flex items-center gap-2 flex-wrap">
              <h3 className="text-lg font-semibold text-gray-900 dark:text-white truncate">
                {goal.name}
              </h3>
              <span className={`px-2.5 py-0.5 rounded-full text-xs font-medium ${statusStyle.badge}`}>
                {t(statusStyle.label)}
              </span>
            </div>
            {goal.description && (
              <p className="mt-1 text-sm text-gray-500 dark:text-gray-400 line-clamp-2">
                {goal.description}
              </p>
            )}
          </div>

          {/* Delete */}
          {confirmDelete ? (
            <div className="flex items-center gap-2 bg-red-50 dark:bg-red-900/20 rounded-lg p-2 flex-shrink-0">
              <AlertTriangle className="w-4 h-4 text-red-500" />
              <span className="text-xs text-red-600 dark:text-red-400">
                {t('goals.confirmDelete')}
              </span>
              <button
                onClick={() => onDelete(goal.id)}
                className="px-2 py-1 text-xs font-medium text-white bg-red-600 hover:bg-red-700 rounded transition-colors"
              >
                {t('common.delete')}
              </button>
              <button
                onClick={() => setConfirmDelete(false)}
                className="px-2 py-1 text-xs font-medium text-gray-600 dark:text-gray-400 hover:bg-gray-200 dark:hover:bg-gray-700 rounded transition-colors"
              >
                {t('common.cancel')}
              </button>
            </div>
          ) : (
            <button
              onClick={() => setConfirmDelete(true)}
              className="p-1.5 text-gray-400 hover:text-red-500 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-800 transition-colors flex-shrink-0"
            >
              <Trash2 className="w-4 h-4" />
            </button>
          )}
        </div>

        {/* Meta row */}
        <div className="mt-3 flex flex-wrap gap-x-4 gap-y-1 text-sm text-gray-500 dark:text-gray-400">
          {goal.owner && (
            <span className="flex items-center gap-1">
              <User className="w-3.5 h-3.5" />
              {goal.owner}
            </span>
          )}
          {goal.deadline && (
            <span className="flex items-center gap-1">
              <Calendar className="w-3.5 h-3.5" />
              {new Date(goal.deadline).toLocaleDateString()}
            </span>
          )}
        </div>

        {/* Stats */}
        <div className="mt-3">
          <StatsBar stats={stats} t={t} />
        </div>

        {/* Expand toggle */}
        <button
          onClick={() => setExpanded(!expanded)}
          className="mt-3 flex items-center gap-1 text-sm font-medium text-primary-600 dark:text-primary-400 hover:text-primary-700 dark:hover:text-primary-300 transition-colors"
        >
          {expanded ? (
            <ChevronDown className="w-4 h-4" />
          ) : (
            <ChevronRight className="w-4 h-4" />
          )}
          {t('goals.projects')} ({projects.length})
        </button>
      </div>

      {/* Expanded projects section */}
      {expanded && (
        <div className="border-t border-gray-100 dark:border-gray-800 bg-gray-50/50 dark:bg-gray-800/20 p-5 space-y-3">
          {projects.length === 0 ? (
            <p className="text-sm text-gray-400 dark:text-gray-500 text-center py-2">
              {t('tasks.noProjects')}
            </p>
          ) : (
            projects.map((project) => (
              <ProjectCard
                key={project.id}
                project={project}
                onDelete={(id) => deleteProjectMutation.mutate(id)}
                t={t}
              />
            ))
          )}
          <button
            onClick={() => onAddProject(goal.id)}
            className="flex items-center gap-2 w-full justify-center px-3 py-2 text-sm font-medium text-primary-600 dark:text-primary-400 hover:bg-primary-50 dark:hover:bg-primary-900/20 rounded-lg border border-dashed border-primary-300 dark:border-primary-700 transition-colors"
          >
            <Plus className="w-4 h-4" />
            {t('goals.addProject')}
          </button>
        </div>
      )}
    </div>
  )
}

// --- Wizard types ---

interface WizardTask {
  name: string
  description: string
  assignAgent: string
  useCustomAgent: boolean
}

interface WizardProject {
  name: string
  description: string
  tasks: WizardTask[]
  expanded: boolean
}

interface WizardGoalInfo {
  name: string
  description: string
  owner: string
  deadline: string
}

// --- Agent Selector component ---

function AgentSelector({
  value,
  useCustom,
  agents,
  onSelect,
  onToggleCustom,
  onCustomChange,
  t,
}: {
  value: string
  useCustom: boolean
  agents: Agent[]
  onSelect: (agentName: string) => void
  onToggleCustom: (custom: boolean) => void
  onCustomChange: (name: string) => void
  t: (key: string) => string
}) {
  const [isOpen, setIsOpen] = useState(false)

  if (useCustom) {
    return (
      <div className="flex gap-2">
        <input
          type="text"
          value={value}
          onChange={(e) => onCustomChange(e.target.value)}
          placeholder={t('goals.wizard.customAgent')}
          className="flex-1 px-3 py-1.5 rounded-lg border border-gray-300 dark:border-gray-700 bg-white dark:bg-gray-800 text-gray-900 dark:text-white text-sm focus:ring-2 focus:ring-primary-500 focus:border-transparent"
        />
        <button
          type="button"
          onClick={() => onToggleCustom(false)}
          className="px-2 py-1.5 text-xs text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-800 rounded-lg transition-colors"
        >
          {t('goals.wizard.selectAgent')}
        </button>
      </div>
    )
  }

  const runningAgents = agents.filter((a) => a.phase === 'Running')
  const otherAgents = agents.filter((a) => a.phase !== 'Running')
  const sortedAgents = [...runningAgents, ...otherAgents]

  return (
    <div className="relative">
      <button
        type="button"
        onClick={() => setIsOpen(!isOpen)}
        className="w-full flex items-center justify-between px-3 py-1.5 rounded-lg border border-gray-300 dark:border-gray-700 bg-white dark:bg-gray-800 text-sm text-left focus:ring-2 focus:ring-primary-500 focus:border-transparent"
      >
        {value ? (
          <span className="text-gray-900 dark:text-white">{value}</span>
        ) : (
          <span className="text-gray-400">{t('goals.wizard.selectAgent')}</span>
        )}
        <ChevronDown className="w-4 h-4 text-gray-400 flex-shrink-0" />
      </button>

      {isOpen && (
        <div className="absolute z-20 mt-1 w-full bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg shadow-lg max-h-52 overflow-y-auto">
          {sortedAgents.map((agent) => {
            const isRunning = agent.phase === 'Running'
            return (
              <button
                key={agent.name}
                type="button"
                onClick={() => {
                  onSelect(agent.name)
                  setIsOpen(false)
                }}
                className={`w-full flex items-center gap-2 px-3 py-2 text-sm text-left hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors ${
                  isRunning ? 'bg-green-50/50 dark:bg-green-900/10' : ''
                }`}
              >
                <span
                  className={`w-2 h-2 rounded-full flex-shrink-0 ${
                    isRunning ? 'bg-green-500' : 'bg-gray-400'
                  }`}
                />
                <span className="flex-1 text-gray-900 dark:text-white truncate">
                  {agent.name}
                </span>
                <span className="px-1.5 py-0.5 rounded text-[10px] font-medium bg-gray-100 dark:bg-gray-700 text-gray-500 dark:text-gray-400">
                  {agent.type}
                </span>
                {isRunning && (
                  <span className="px-1.5 py-0.5 rounded text-[10px] font-medium bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-400">
                    {t('goals.wizard.runningAgent')}
                  </span>
                )}
              </button>
            )
          })}

          {/* Custom agent option */}
          <button
            type="button"
            onClick={() => {
              onToggleCustom(true)
              setIsOpen(false)
            }}
            className="w-full flex items-center gap-2 px-3 py-2 text-sm text-left border-t border-gray-100 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors text-primary-600 dark:text-primary-400"
          >
            <Bot className="w-3.5 h-3.5" />
            {t('goals.wizard.customAgent')}
          </button>
        </div>
      )}
    </div>
  )
}

// --- Wizard Steps ---

function WizardStepIndicator({
  currentStep,
  t,
}: {
  currentStep: number
  t: (key: string) => string
}) {
  const steps = [
    { num: 1, label: t('goals.wizard.step1') },
    { num: 2, label: t('goals.wizard.step2') },
    { num: 3, label: t('goals.wizard.step3') },
  ]

  return (
    <div className="flex items-center justify-center gap-2 px-6 py-4 border-b border-gray-200 dark:border-gray-800">
      {steps.map((step, idx) => (
        <div key={step.num} className="flex items-center gap-2">
          <div
            className={`flex items-center justify-center w-7 h-7 rounded-full text-xs font-semibold transition-colors ${
              currentStep === step.num
                ? 'bg-primary-600 text-white'
                : currentStep > step.num
                  ? 'bg-green-500 text-white'
                  : 'bg-gray-200 dark:bg-gray-700 text-gray-500 dark:text-gray-400'
            }`}
          >
            {currentStep > step.num ? <Check className="w-3.5 h-3.5" /> : step.num}
          </div>
          <span
            className={`text-xs font-medium hidden sm:inline ${
              currentStep === step.num
                ? 'text-gray-900 dark:text-white'
                : 'text-gray-400 dark:text-gray-500'
            }`}
          >
            {step.label}
          </span>
          {idx < steps.length - 1 && (
            <div
              className={`w-8 h-0.5 ${
                currentStep > step.num
                  ? 'bg-green-500'
                  : 'bg-gray-200 dark:bg-gray-700'
              }`}
            />
          )}
        </div>
      ))}
    </div>
  )
}

function Step1GoalInfo({
  info,
  onChange,
  t,
}: {
  info: WizardGoalInfo
  onChange: (info: WizardGoalInfo) => void
  t: (key: string) => string
}) {
  return (
    <div className="space-y-4">
      <div>
        <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
          {t('goals.name')} *
        </label>
        <input
          type="text"
          value={info.name}
          onChange={(e) => onChange({ ...info, name: e.target.value })}
          required
          placeholder="e.g. Q1 Product Launch"
          className="w-full px-3 py-2 rounded-lg border border-gray-300 dark:border-gray-700 bg-white dark:bg-gray-800 text-gray-900 dark:text-white focus:ring-2 focus:ring-primary-500 focus:border-transparent text-sm"
        />
      </div>
      <div>
        <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
          {t('goals.description')}
        </label>
        <textarea
          value={info.description}
          onChange={(e) => onChange({ ...info, description: e.target.value })}
          rows={3}
          placeholder="Describe the goal..."
          className="w-full px-3 py-2 rounded-lg border border-gray-300 dark:border-gray-700 bg-white dark:bg-gray-800 text-gray-900 dark:text-white focus:ring-2 focus:ring-primary-500 focus:border-transparent text-sm resize-none"
        />
      </div>
      <div>
        <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
          {t('goals.owner')}
        </label>
        <input
          type="text"
          value={info.owner}
          onChange={(e) => onChange({ ...info, owner: e.target.value })}
          placeholder="e.g. Alice"
          className="w-full px-3 py-2 rounded-lg border border-gray-300 dark:border-gray-700 bg-white dark:bg-gray-800 text-gray-900 dark:text-white focus:ring-2 focus:ring-primary-500 focus:border-transparent text-sm"
        />
      </div>
      <div>
        <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
          {t('goals.deadline')}
        </label>
        <input
          type="date"
          value={info.deadline}
          onChange={(e) => onChange({ ...info, deadline: e.target.value })}
          className="w-full px-3 py-2 rounded-lg border border-gray-300 dark:border-gray-700 bg-white dark:bg-gray-800 text-gray-900 dark:text-white focus:ring-2 focus:ring-primary-500 focus:border-transparent text-sm"
        />
      </div>
    </div>
  )
}

function Step2Projects({
  projects,
  agents,
  onProjectsChange,
  t,
}: {
  projects: WizardProject[]
  agents: Agent[]
  onProjectsChange: (projects: WizardProject[]) => void
  t: (key: string) => string
}) {
  const addProject = useCallback(() => {
    onProjectsChange([
      ...projects,
      { name: '', description: '', tasks: [], expanded: true },
    ])
  }, [projects, onProjectsChange])

  const removeProject = useCallback(
    (idx: number) => {
      onProjectsChange(projects.filter((_, i) => i !== idx))
    },
    [projects, onProjectsChange]
  )

  const updateProject = useCallback(
    (idx: number, updates: Partial<WizardProject>) => {
      onProjectsChange(
        projects.map((p, i) => (i === idx ? { ...p, ...updates } : p))
      )
    },
    [projects, onProjectsChange]
  )

  const addTask = useCallback(
    (projectIdx: number) => {
      const updated = projects.map((p, i) =>
        i === projectIdx
          ? {
              ...p,
              tasks: [
                ...p.tasks,
                { name: '', description: '', assignAgent: '', useCustomAgent: false },
              ],
            }
          : p
      )
      onProjectsChange(updated)
    },
    [projects, onProjectsChange]
  )

  const removeTask = useCallback(
    (projectIdx: number, taskIdx: number) => {
      const updated = projects.map((p, i) =>
        i === projectIdx
          ? { ...p, tasks: p.tasks.filter((_, ti) => ti !== taskIdx) }
          : p
      )
      onProjectsChange(updated)
    },
    [projects, onProjectsChange]
  )

  const updateTask = useCallback(
    (projectIdx: number, taskIdx: number, updates: Partial<WizardTask>) => {
      const updated = projects.map((p, i) =>
        i === projectIdx
          ? {
              ...p,
              tasks: p.tasks.map((task, ti) =>
                ti === taskIdx ? { ...task, ...updates } : task
              ),
            }
          : p
      )
      onProjectsChange(updated)
    },
    [projects, onProjectsChange]
  )

  return (
    <div className="space-y-4">
      {projects.length === 0 && (
        <p className="text-sm text-gray-400 dark:text-gray-500 text-center py-4">
          {t('goals.wizard.noProjects')}
        </p>
      )}

      {projects.map((project, pIdx) => (
        <div
          key={pIdx}
          className="border border-gray-200 dark:border-gray-700 rounded-lg overflow-hidden"
        >
          {/* Project header */}
          <div className="flex items-start gap-3 p-4 bg-gray-50 dark:bg-gray-800/50">
            <button
              type="button"
              onClick={() => updateProject(pIdx, { expanded: !project.expanded })}
              className="mt-1 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
            >
              {project.expanded ? (
                <ChevronDown className="w-4 h-4" />
              ) : (
                <ChevronRight className="w-4 h-4" />
              )}
            </button>
            <div className="flex-1 space-y-2">
              <input
                type="text"
                value={project.name}
                onChange={(e) => updateProject(pIdx, { name: e.target.value })}
                placeholder={t('goals.projectName')}
                className="w-full px-3 py-1.5 rounded-lg border border-gray-300 dark:border-gray-700 bg-white dark:bg-gray-800 text-gray-900 dark:text-white text-sm focus:ring-2 focus:ring-primary-500 focus:border-transparent"
              />
              {project.expanded && (
                <input
                  type="text"
                  value={project.description}
                  onChange={(e) => updateProject(pIdx, { description: e.target.value })}
                  placeholder={t('goals.projectDesc')}
                  className="w-full px-3 py-1.5 rounded-lg border border-gray-300 dark:border-gray-700 bg-white dark:bg-gray-800 text-gray-900 dark:text-white text-sm focus:ring-2 focus:ring-primary-500 focus:border-transparent"
                />
              )}
            </div>
            <button
              type="button"
              onClick={() => removeProject(pIdx)}
              className="mt-1 p-1 text-gray-400 hover:text-red-500 rounded transition-colors"
            >
              <Trash2 className="w-4 h-4" />
            </button>
          </div>

          {/* Tasks */}
          {project.expanded && (
            <div className="p-4 space-y-3">
              {project.tasks.map((task, tIdx) => (
                <div
                  key={tIdx}
                  className="flex items-start gap-2 p-3 bg-gray-50 dark:bg-gray-800/30 rounded-lg border border-gray-100 dark:border-gray-700"
                >
                  <div className="flex-1 space-y-2">
                    <input
                      type="text"
                      value={task.name}
                      onChange={(e) =>
                        updateTask(pIdx, tIdx, { name: e.target.value })
                      }
                      placeholder="Task name"
                      className="w-full px-3 py-1.5 rounded-lg border border-gray-300 dark:border-gray-700 bg-white dark:bg-gray-800 text-gray-900 dark:text-white text-sm focus:ring-2 focus:ring-primary-500 focus:border-transparent"
                    />
                    <input
                      type="text"
                      value={task.description}
                      onChange={(e) =>
                        updateTask(pIdx, tIdx, { description: e.target.value })
                      }
                      placeholder="Task description"
                      className="w-full px-3 py-1.5 rounded-lg border border-gray-300 dark:border-gray-700 bg-white dark:bg-gray-800 text-gray-900 dark:text-white text-sm focus:ring-2 focus:ring-primary-500 focus:border-transparent"
                    />
                    <div>
                      <label className="block text-xs text-gray-500 dark:text-gray-400 mb-1">
                        {t('goals.wizard.assignAgent')}
                      </label>
                      <AgentSelector
                        value={task.assignAgent}
                        useCustom={task.useCustomAgent}
                        agents={agents}
                        onSelect={(name) =>
                          updateTask(pIdx, tIdx, { assignAgent: name, useCustomAgent: false })
                        }
                        onToggleCustom={(custom) =>
                          updateTask(pIdx, tIdx, { useCustomAgent: custom, assignAgent: '' })
                        }
                        onCustomChange={(name) =>
                          updateTask(pIdx, tIdx, { assignAgent: name })
                        }
                        t={t}
                      />
                    </div>
                  </div>
                  <button
                    type="button"
                    onClick={() => removeTask(pIdx, tIdx)}
                    className="mt-1 p-1 text-gray-400 hover:text-red-500 rounded transition-colors"
                  >
                    <Trash2 className="w-3.5 h-3.5" />
                  </button>
                </div>
              ))}

              <button
                type="button"
                onClick={() => addTask(pIdx)}
                className="flex items-center gap-1.5 w-full justify-center px-3 py-2 text-xs font-medium text-primary-600 dark:text-primary-400 hover:bg-primary-50 dark:hover:bg-primary-900/20 rounded-lg border border-dashed border-primary-300 dark:border-primary-700 transition-colors"
              >
                <Plus className="w-3.5 h-3.5" />
                {t('goals.wizard.addTask')}
              </button>
            </div>
          )}
        </div>
      ))}

      <button
        type="button"
        onClick={addProject}
        className="flex items-center gap-2 w-full justify-center px-3 py-2.5 text-sm font-medium text-primary-600 dark:text-primary-400 hover:bg-primary-50 dark:hover:bg-primary-900/20 rounded-lg border border-dashed border-primary-300 dark:border-primary-700 transition-colors"
      >
        <Plus className="w-4 h-4" />
        {t('goals.wizard.addProject')}
      </button>
    </div>
  )
}

function Step3Review({
  goalInfo,
  projects,
  t,
}: {
  goalInfo: WizardGoalInfo
  projects: WizardProject[]
  t: (key: string) => string
}) {
  const totalTasks = projects.reduce((sum, p) => sum + p.tasks.length, 0)

  return (
    <div className="space-y-4">
      {/* Summary counts */}
      <div className="p-4 bg-primary-50 dark:bg-primary-900/20 rounded-lg border border-primary-200 dark:border-primary-800">
        <p className="text-sm font-medium text-primary-700 dark:text-primary-300 mb-1">
          {t('goals.wizard.summary')}
        </p>
        <div className="flex gap-4 text-sm text-primary-600 dark:text-primary-400">
          <span>
            {t('goals.wizard.projectCount').replace('{count}', String(projects.length))}
          </span>
          <span>
            {t('goals.wizard.taskCount').replace('{count}', String(totalTasks))}
          </span>
        </div>
      </div>

      {/* Tree view */}
      <div className="border border-gray-200 dark:border-gray-700 rounded-lg p-4 space-y-2">
        {/* Goal root */}
        <div className="flex items-center gap-2">
          <Target className="w-4 h-4 text-primary-500" />
          <span className="text-sm font-semibold text-gray-900 dark:text-white">
            {goalInfo.name || '(unnamed)'}
          </span>
        </div>
        {goalInfo.description && (
          <p className="ml-6 text-xs text-gray-500 dark:text-gray-400">
            {goalInfo.description}
          </p>
        )}
        <div className="ml-6 flex flex-wrap gap-3 text-xs text-gray-500 dark:text-gray-400">
          {goalInfo.owner && (
            <span className="flex items-center gap-1">
              <User className="w-3 h-3" />
              {goalInfo.owner}
            </span>
          )}
          {goalInfo.deadline && (
            <span className="flex items-center gap-1">
              <Calendar className="w-3 h-3" />
              {goalInfo.deadline}
            </span>
          )}
        </div>

        {/* Projects */}
        {projects.map((project, pIdx) => (
          <div key={pIdx} className="ml-4 mt-2">
            <div className="flex items-center gap-2">
              <FolderKanban className="w-3.5 h-3.5 text-blue-500" />
              <span className="text-sm font-medium text-gray-800 dark:text-gray-200">
                {project.name || '(unnamed project)'}
              </span>
            </div>
            {project.description && (
              <p className="ml-5.5 text-xs text-gray-500 dark:text-gray-400 ml-6">
                {project.description}
              </p>
            )}

            {/* Tasks */}
            {project.tasks.map((task, tIdx) => (
              <div key={tIdx} className="ml-8 mt-1 flex items-center gap-2">
                <ListTodo className="w-3 h-3 text-gray-400" />
                <span className="text-xs text-gray-700 dark:text-gray-300">
                  {task.name || '(unnamed task)'}
                </span>
                {task.assignAgent && (
                  <span className="px-1.5 py-0.5 rounded text-[10px] font-medium bg-gray-100 dark:bg-gray-700 text-gray-500 dark:text-gray-400 flex items-center gap-1">
                    <Bot className="w-2.5 h-2.5" />
                    {task.assignAgent}
                  </span>
                )}
              </div>
            ))}
          </div>
        ))}
      </div>
    </div>
  )
}

// --- Main Wizard Modal ---

function CreateGoalWizard({
  isOpen,
  onClose,
  onCreated,
  t,
}: {
  isOpen: boolean
  onClose: () => void
  onCreated: () => void
  t: (key: string) => string
}) {
  const [step, setStep] = useState(1)
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [goalInfo, setGoalInfo] = useState<WizardGoalInfo>({
    name: '',
    description: '',
    owner: '',
    deadline: '',
  })
  const [projects, setProjects] = useState<WizardProject[]>([])

  const { data: agents = [] } = useQuery({
    queryKey: ['agents'],
    queryFn: fetchAgents,
    enabled: isOpen,
  })

  const resetWizard = useCallback(() => {
    setStep(1)
    setIsSubmitting(false)
    setGoalInfo({ name: '', description: '', owner: '', deadline: '' })
    setProjects([])
  }, [])

  const handleClose = useCallback(() => {
    resetWizard()
    onClose()
  }, [resetWizard, onClose])

  const canProceedStep1 = goalInfo.name.trim().length > 0
  const canProceedStep2 = projects.length > 0 && projects.every((p) => p.name.trim().length > 0)

  const buildYaml = useCallback((): string => {
    const projectsYaml = projects
      .map((p) => {
        const tasksYaml = p.tasks
          .filter((task) => task.name.trim())
          .map((task) => {
            const lines = [`          - name: ${task.name.trim()}`]
            if (task.description.trim()) {
              lines.push(`            description: ${task.description.trim()}`)
            }
            if (task.assignAgent.trim()) {
              lines.push(`            assignAgent: ${task.assignAgent.trim()}`)
            }
            return lines.join('\n')
          })
          .join('\n')

        const lines = [`      - name: ${p.name.trim()}`]
        if (p.description.trim()) {
          lines.push(`        description: ${p.description.trim()}`)
        }
        if (tasksYaml) {
          lines.push(`        tasks:`)
          lines.push(tasksYaml)
        }
        return lines.join('\n')
      })
      .join('\n')

    const lines = [
      'apiVersion: opc/v1',
      'kind: Goal',
      'metadata:',
      `  name: ${goalInfo.name.trim()}`,
      'spec:',
    ]
    if (goalInfo.description.trim()) {
      lines.push(`  description: ${goalInfo.description.trim()}`)
    }
    if (goalInfo.owner.trim()) {
      lines.push(`  owner: ${goalInfo.owner.trim()}`)
    }
    if (goalInfo.deadline.trim()) {
      lines.push(`  deadline: ${goalInfo.deadline.trim()}`)
    }
    if (projectsYaml) {
      lines.push('  decomposition:')
      lines.push('    projects:')
      lines.push(projectsYaml)
    }

    return lines.join('\n')
  }, [goalInfo, projects])

  const handleSubmit = useCallback(async () => {
    setIsSubmitting(true)
    try {
      const yaml = buildYaml()
      await applyAgent(yaml)
      resetWizard()
      onCreated()
    } catch {
      // error handling could be enhanced
      setIsSubmitting(false)
    }
  }, [buildYaml, resetWizard, onCreated])

  if (!isOpen) return null

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
      <div className="bg-white dark:bg-gray-900 rounded-xl shadow-xl border border-gray-200 dark:border-gray-800 w-full max-w-2xl mx-4 max-h-[90vh] flex flex-col">
        {/* Title bar */}
        <div className="flex items-center justify-between px-6 pt-5 pb-0">
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white">
            {t('goals.create')}
          </h2>
          <button
            onClick={handleClose}
            className="p-1 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-800 transition-colors"
          >
            <X className="w-5 h-5 text-gray-400" />
          </button>
        </div>

        {/* Step indicator */}
        <WizardStepIndicator currentStep={step} t={t} />

        {/* Content area */}
        <div className="flex-1 overflow-y-auto p-6">
          {step === 1 && (
            <Step1GoalInfo info={goalInfo} onChange={setGoalInfo} t={t} />
          )}
          {step === 2 && (
            <Step2Projects
              projects={projects}
              agents={agents}
              onProjectsChange={setProjects}
              t={t}
            />
          )}
          {step === 3 && (
            <Step3Review goalInfo={goalInfo} projects={projects} t={t} />
          )}
        </div>

        {/* Footer buttons */}
        <div className="flex items-center justify-between px-6 py-4 border-t border-gray-200 dark:border-gray-800">
          <div>
            {step > 1 && (
              <button
                type="button"
                onClick={() => setStep(step - 1)}
                disabled={isSubmitting}
                className="flex items-center gap-1 px-4 py-2 text-sm text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800 rounded-lg transition-colors disabled:opacity-50"
              >
                <ChevronLeft className="w-4 h-4" />
                {t('goals.wizard.back')}
              </button>
            )}
          </div>
          <div className="flex items-center gap-3">
            <button
              type="button"
              onClick={handleClose}
              disabled={isSubmitting}
              className="px-4 py-2 text-sm text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800 rounded-lg transition-colors disabled:opacity-50"
            >
              {t('common.cancel')}
            </button>
            {step < 3 ? (
              <button
                type="button"
                onClick={() => setStep(step + 1)}
                disabled={step === 1 ? !canProceedStep1 : !canProceedStep2}
                className="flex items-center gap-1 px-4 py-2 text-sm text-white bg-primary-600 hover:bg-primary-700 rounded-lg transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
              >
                {t('goals.wizard.next')}
                <ChevronRight className="w-4 h-4" />
              </button>
            ) : (
              <button
                type="button"
                onClick={handleSubmit}
                disabled={isSubmitting}
                className="px-4 py-2 text-sm text-white bg-primary-600 hover:bg-primary-700 rounded-lg transition-colors disabled:opacity-50"
              >
                {isSubmitting ? t('goals.wizard.creating') : t('goals.create')}
              </button>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}

function CreateProjectModal({
  isOpen,
  goalId,
  onClose,
  onSubmit,
  t,
}: {
  isOpen: boolean
  goalId: string
  onClose: () => void
  onSubmit: (data: Partial<Project>) => void
  t: (key: string) => string
}) {
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')

  if (!isOpen) return null

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    onSubmit({
      name,
      goalId,
      description,
      status: 'active',
    })
    setName('')
    setDescription('')
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
      <div className="bg-white dark:bg-gray-900 rounded-xl shadow-xl border border-gray-200 dark:border-gray-800 w-full max-w-md mx-4">
        <div className="flex items-center justify-between p-6 border-b border-gray-200 dark:border-gray-800">
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white">
            {t('goals.addProject')}
          </h2>
          <button
            onClick={onClose}
            className="p-1 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-800 transition-colors"
          >
            <X className="w-5 h-5 text-gray-400" />
          </button>
        </div>
        <form onSubmit={handleSubmit} className="p-6 space-y-4">
          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
              {t('goals.projectName')} *
            </label>
            <input
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              required
              placeholder="e.g. Frontend Redesign"
              className="w-full px-3 py-2 rounded-lg border border-gray-300 dark:border-gray-700 bg-white dark:bg-gray-800 text-gray-900 dark:text-white focus:ring-2 focus:ring-primary-500 focus:border-transparent text-sm"
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
              {t('goals.projectDesc')}
            </label>
            <textarea
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              rows={3}
              placeholder="Describe the project..."
              className="w-full px-3 py-2 rounded-lg border border-gray-300 dark:border-gray-700 bg-white dark:bg-gray-800 text-gray-900 dark:text-white focus:ring-2 focus:ring-primary-500 focus:border-transparent text-sm resize-none"
            />
          </div>
          <div className="flex justify-end gap-3 pt-2">
            <button
              type="button"
              onClick={onClose}
              className="px-4 py-2 text-sm text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800 rounded-lg transition-colors"
            >
              {t('common.cancel')}
            </button>
            <button
              type="submit"
              className="px-4 py-2 text-sm text-white bg-primary-600 hover:bg-primary-700 rounded-lg transition-colors"
            >
              {t('goals.addProject')}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}

export default function GoalsPage() {
  const { t } = useTranslation()
  const [isCreateGoalOpen, setIsCreateGoalOpen] = useState(false)
  const [addProjectGoalId, setAddProjectGoalId] = useState<string | null>(null)
  const queryClient = useQueryClient()

  const { data: goals = [], isLoading } = useQuery({
    queryKey: ['goals'],
    queryFn: fetchGoals,
  })

  const deleteGoalMutation = useMutation({
    mutationFn: deleteGoal,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['goals'] })
    },
  })

  const createProjectMutation = useMutation({
    mutationFn: createProject,
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: ['goal-projects', variables.goalId] })
      queryClient.invalidateQueries({ queryKey: ['goal-stats', variables.goalId] })
      setAddProjectGoalId(null)
    },
  })

  const handleGoalCreated = useCallback(() => {
    queryClient.invalidateQueries({ queryKey: ['goals'] })
    setIsCreateGoalOpen(false)
  }, [queryClient])

  return (
    <div className="p-6 space-y-6">
      {/* Header */}
      <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4">
        <div>
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white">
            {t('goals.title')}
          </h1>
          <p className="text-gray-500 dark:text-gray-400 mt-1">
            {t('goals.subtitle')}
          </p>
        </div>
        <button
          onClick={() => setIsCreateGoalOpen(true)}
          className="flex items-center gap-2 px-4 py-2 text-sm text-white bg-primary-600 hover:bg-primary-700 rounded-lg transition-colors"
        >
          <Plus className="w-4 h-4" />
          {t('goals.create')}
        </button>
      </div>

      {/* Content */}
      {isLoading ? (
        <div className="flex items-center justify-center h-48">
          <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary-500" />
        </div>
      ) : goals.length === 0 ? (
        <div className="bg-white dark:bg-gray-900 rounded-xl shadow-sm border border-gray-200 dark:border-gray-800 p-16 text-center">
          <div className="mx-auto w-16 h-16 rounded-full bg-gray-100 dark:bg-gray-800 flex items-center justify-center mb-5">
            <Target className="w-8 h-8 text-gray-400 dark:text-gray-500" />
          </div>
          <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-2">
            {t('goals.noGoals')}
          </h3>
          <p className="text-gray-500 dark:text-gray-400 mb-6 max-w-sm mx-auto">
            {t('goals.noGoalsDesc')}
          </p>
          <button
            onClick={() => setIsCreateGoalOpen(true)}
            className="inline-flex items-center gap-2 px-4 py-2 text-sm text-white bg-primary-600 hover:bg-primary-700 rounded-lg transition-colors"
          >
            <Plus className="w-4 h-4" />
            {t('goals.create')}
          </button>
        </div>
      ) : (
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
          {goals.map((goal) => (
            <GoalCard
              key={goal.id}
              goal={goal}
              onDelete={(id) => deleteGoalMutation.mutate(id)}
              onAddProject={(goalId) => setAddProjectGoalId(goalId)}
              t={t}
            />
          ))}
        </div>
      )}

      {/* Create Goal Wizard */}
      <CreateGoalWizard
        isOpen={isCreateGoalOpen}
        onClose={() => setIsCreateGoalOpen(false)}
        onCreated={handleGoalCreated}
        t={t}
      />

      {/* Create Project Modal */}
      <CreateProjectModal
        isOpen={addProjectGoalId !== null}
        goalId={addProjectGoalId || ''}
        onClose={() => setAddProjectGoalId(null)}
        onSubmit={(data) => createProjectMutation.mutate(data)}
        t={t}
      />
    </div>
  )
}
