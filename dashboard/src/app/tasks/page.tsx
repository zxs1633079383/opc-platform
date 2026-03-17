'use client'

import { useQuery } from '@tanstack/react-query'
import { Search, ChevronDown, ChevronUp, ChevronRight, ListTodo, Bot, Target, FolderKanban, GitBranch } from 'lucide-react'
import { useState, useMemo } from 'react'
import { clsx } from 'clsx'
import { formatDistanceToNow } from 'date-fns'
import { fetchTasks, fetchGoals, fetchProjects, fetchIssues } from '@/lib/api'
import { useSearchParams } from 'next/navigation'
import type { Task, Goal, Project, Issue } from '@/types'

const statusTabs = [
  { value: 'all', label: 'All' },
  { value: 'todo', label: 'Todo' },
  { value: 'running', label: 'Running' },
  { value: 'complete', label: 'Complete' },
  { value: 'success', label: 'Success' },
  { value: 'fail', label: 'Fail' },
]

const statusColors: Record<string, string> = {
  Running: 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-400',
  Pending: 'bg-gray-100 text-gray-800 dark:bg-gray-800 dark:text-gray-400',
  Completed: 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400',
  Failed: 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-400',
  Cancelled: 'bg-gray-100 text-gray-500 dark:bg-gray-800 dark:text-gray-500',
}

// Map backend status to the new model: Todo / Running / Complete(Success|Fail)
function getStatusGroup(status: string): string {
  switch (status) {
    case 'Pending': case 'Cancelled': return 'todo'
    case 'Running': return 'running'
    case 'Completed': return 'success'
    case 'Failed': return 'fail'
    default: return 'todo'
  }
}

function getStatusLabel(status: string): string {
  const group = getStatusGroup(status)
  if (group === 'todo') return 'Todo'
  if (group === 'running') return 'Running'
  if (group === 'success') return 'Success'
  if (group === 'fail') return 'Fail'
  return status
}

export default function TasksPage() {
  const searchParams = useSearchParams()
  const initialStatus = searchParams.get('status') || 'all'

  const [search, setSearch] = useState('')
  const [statusFilter, setStatusFilter] = useState<string>(initialStatus)
  const [agentFilter, setAgentFilter] = useState<string>('all')
  const [expandedTask, setExpandedTask] = useState<string | null>(null)
  const [viewMode, setViewMode] = useState<'flat' | 'hierarchy' | 'goals' | 'projects' | 'tasks' | 'issues'>('flat')
  const [expandedGoals, setExpandedGoals] = useState<Set<string>>(new Set())
  const [expandedProjects, setExpandedProjects] = useState<Set<string>>(new Set())

  const { data: tasks = [], isLoading } = useQuery({
    queryKey: ['tasks'],
    queryFn: fetchTasks,
  })

  const needsHierarchy = viewMode !== 'flat'

  const { data: goals = [] } = useQuery({
    queryKey: ['goals'],
    queryFn: fetchGoals,
    enabled: needsHierarchy,
  })

  const { data: projects = [] } = useQuery({
    queryKey: ['projects'],
    queryFn: fetchProjects,
    enabled: needsHierarchy,
  })

  const { data: issues = [] } = useQuery({
    queryKey: ['issues'],
    queryFn: fetchIssues,
    enabled: needsHierarchy,
  })

  // Build hierarchy: Goal → Projects → Tasks → Issues
  const hierarchy = useMemo(() => {
    if (viewMode !== 'hierarchy') return []

    const goalMap = new Map(goals.map(g => [g.id, g]))
    const projectsByGoal = new Map<string, Project[]>()
    for (const p of projects) {
      const list = projectsByGoal.get(p.goalId) || []
      list.push(p)
      projectsByGoal.set(p.goalId, list)
    }
    const tasksByProject = new Map<string, Task[]>()
    const tasksByGoal = new Map<string, Task[]>()
    const orphanTasks: Task[] = []
    for (const t of tasks) {
      if (t.projectId) {
        const list = tasksByProject.get(t.projectId) || []
        list.push(t)
        tasksByProject.set(t.projectId, list)
      } else if (t.goalId) {
        const list = tasksByGoal.get(t.goalId) || []
        list.push(t)
        tasksByGoal.set(t.goalId, list)
      } else {
        orphanTasks.push(t)
      }
    }
    const issuesByProject = new Map<string, Issue[]>()
    for (const i of issues) {
      const list = issuesByProject.get(i.projectId) || []
      list.push(i)
      issuesByProject.set(i.projectId, list)
    }

    return { goalMap, projectsByGoal, tasksByProject, tasksByGoal, issuesByProject, orphanTasks }
  }, [viewMode, goals, projects, tasks, issues])

  const toggleGoal = (id: string) => {
    setExpandedGoals(prev => { const s = new Set(prev); s.has(id) ? s.delete(id) : s.add(id); return s })
  }
  const toggleProject = (id: string) => {
    setExpandedProjects(prev => { const s = new Set(prev); s.has(id) ? s.delete(id) : s.add(id); return s })
  }

  const agentNames = useMemo(() => {
    const names = new Set(tasks.map(t => t.agentName))
    return Array.from(names).sort()
  }, [tasks])

  const statusCounts = useMemo(() => {
    const counts: Record<string, number> = { all: tasks.length, todo: 0, running: 0, complete: 0, success: 0, fail: 0 }
    for (const task of tasks) {
      const group = getStatusGroup(task.status)
      counts[group] = (counts[group] || 0) + 1
      if (group === 'success' || group === 'fail') counts.complete++
    }
    return counts
  }, [tasks])

  const filteredTasks = tasks.filter((task) => {
    const matchesSearch =
      search === '' ||
      task.message.toLowerCase().includes(search.toLowerCase()) ||
      task.agentName.toLowerCase().includes(search.toLowerCase()) ||
      task.id.toLowerCase().includes(search.toLowerCase())

    const group = getStatusGroup(task.status)
    const matchesStatus =
      statusFilter === 'all' ||
      statusFilter === group ||
      (statusFilter === 'complete' && (group === 'success' || group === 'fail'))

    const matchesAgent =
      agentFilter === 'all' || task.agentName === agentFilter

    return matchesSearch && matchesStatus && matchesAgent
  })

  return (
    <div className="p-6 space-y-6">
      <div className="flex justify-between items-center">
        <div>
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white">
            Tasks
          </h1>
          <p className="text-gray-500 dark:text-gray-400">
            View and manage task executions
          </p>
        </div>
        <div className="flex gap-1 bg-gray-100 dark:bg-gray-800 rounded-lg p-1 overflow-x-auto">
          {([
            { value: 'flat', label: 'All Tasks', icon: ListTodo },
            { value: 'hierarchy', label: 'Hierarchy', icon: GitBranch },
            { value: 'goals', label: 'Goals', icon: Target },
            { value: 'projects', label: 'Projects', icon: FolderKanban },
            { value: 'issues', label: 'Issues', icon: Bot },
          ] as const).map((tab) => {
            const Icon = tab.icon
            return (
              <button
                key={tab.value}
                onClick={() => setViewMode(tab.value)}
                className={clsx('flex items-center gap-1 px-3 py-1.5 text-sm font-medium rounded-md transition-colors whitespace-nowrap',
                  viewMode === tab.value ? 'bg-white dark:bg-gray-700 text-gray-900 dark:text-white shadow-sm' : 'text-gray-500'
                )}
              >
                <Icon className="w-3.5 h-3.5" />{tab.label}
              </button>
            )
          })}
        </div>
      </div>

      {viewMode === 'flat' && <>
      {/* Status Tabs */}
      <div className="flex gap-1 bg-gray-100 dark:bg-gray-800 rounded-lg p-1 overflow-x-auto">
        {statusTabs.map((tab) => (
          <button
            key={tab.value}
            onClick={() => setStatusFilter(tab.value)}
            className={clsx(
              'flex items-center gap-1.5 px-3 py-1.5 text-sm font-medium rounded-md transition-colors whitespace-nowrap',
              statusFilter === tab.value
                ? 'bg-white dark:bg-gray-700 text-gray-900 dark:text-white shadow-sm'
                : 'text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300'
            )}
          >
            {tab.label}
            {statusCounts[tab.value] !== undefined && (
              <span className={clsx(
                'text-xs px-1.5 py-0.5 rounded-full',
                statusFilter === tab.value
                  ? 'bg-primary-100 text-primary-700 dark:bg-primary-900/30 dark:text-primary-400'
                  : 'bg-gray-200 text-gray-600 dark:bg-gray-700 dark:text-gray-400'
              )}>
                {statusCounts[tab.value] || 0}
              </span>
            )}
          </button>
        ))}
      </div>

      {/* Filters */}
      <div className="flex gap-4 flex-wrap">
        <div className="flex-1 min-w-[200px] relative">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-5 h-5 text-gray-400" />
          <input
            type="text"
            placeholder="Search tasks by message, agent, or ID..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="w-full pl-10 pr-4 py-2 border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900 text-gray-900 dark:text-white focus:ring-2 focus:ring-primary-500 focus:border-transparent"
          />
        </div>
        <div className="relative">
          <Bot className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400" />
          <select
            value={agentFilter}
            onChange={(e) => setAgentFilter(e.target.value)}
            className="pl-9 pr-8 py-2 border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900 text-gray-900 dark:text-white focus:ring-2 focus:ring-primary-500 focus:border-transparent appearance-none text-sm"
          >
            <option value="all">All Agents</option>
            {agentNames.map((name) => (
              <option key={name} value={name}>{name}</option>
            ))}
          </select>
        </div>
      </div>

      {/* Task List */}
      <div className="bg-white dark:bg-gray-900 rounded-xl shadow-sm border border-gray-200 dark:border-gray-800">
        {isLoading ? (
          <div className="p-6 space-y-4">
            {[1, 2, 3, 4, 5].map((i) => (
              <div key={i} className="animate-pulse flex gap-4 items-center">
                <div className="h-4 w-16 bg-gray-200 dark:bg-gray-700 rounded" />
                <div className="h-4 w-20 bg-gray-200 dark:bg-gray-700 rounded" />
                <div className="h-4 flex-1 bg-gray-200 dark:bg-gray-700 rounded" />
                <div className="h-4 w-16 bg-gray-200 dark:bg-gray-700 rounded" />
                <div className="h-4 w-20 bg-gray-200 dark:bg-gray-700 rounded" />
              </div>
            ))}
          </div>
        ) : filteredTasks.length === 0 ? (
          <div className="flex flex-col items-center justify-center py-16 text-gray-500">
            <ListTodo className="w-12 h-12 mb-3 text-gray-300 dark:text-gray-600" />
            <p className="text-gray-500 dark:text-gray-400">
              {tasks.length === 0 ? 'No tasks executed yet' : 'No tasks match your filters'}
            </p>
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full">
              <thead>
                <tr className="text-left text-sm text-gray-500 dark:text-gray-400 border-b border-gray-200 dark:border-gray-700">
                  <th className="px-6 py-3 font-medium">ID</th>
                  <th className="px-6 py-3 font-medium">Agent</th>
                  <th className="px-6 py-3 font-medium">Message</th>
                  <th className="px-6 py-3 font-medium">Status</th>
                  <th className="px-6 py-3 font-medium">Cost</th>
                  <th className="px-6 py-3 font-medium">Time</th>
                  <th className="px-6 py-3 font-medium w-10"></th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-100 dark:divide-gray-800">
                {filteredTasks.map((task) => (
                  <>
                    <tr
                      key={task.id}
                      className="text-sm hover:bg-gray-50 dark:hover:bg-gray-800/50 cursor-pointer"
                      onClick={() => setExpandedTask(expandedTask === task.id ? null : task.id)}
                    >
                      <td className="px-6 py-3 font-mono text-xs text-gray-500">{task.id.slice(0, 8)}</td>
                      <td className="px-6 py-3 text-gray-900 dark:text-white">{task.agentName}</td>
                      <td className="px-6 py-3 max-w-md truncate text-gray-600 dark:text-gray-300">{task.message}</td>
                      <td className="px-6 py-3">
                        <span className={clsx('px-2 py-0.5 rounded-full text-xs font-medium', statusColors[task.status] || statusColors.Pending)}>
                          {getStatusLabel(task.status)}
                        </span>
                      </td>
                      <td className="px-6 py-3 text-gray-600 dark:text-gray-300">${(task.cost || 0).toFixed(4)}</td>
                      <td className="px-6 py-3 text-gray-500 dark:text-gray-400 text-xs">
                        {formatDistanceToNow(new Date(task.createdAt), { addSuffix: true })}
                      </td>
                      <td className="px-6 py-3">
                        {expandedTask === task.id
                          ? <ChevronUp className="w-4 h-4 text-gray-400" />
                          : <ChevronDown className="w-4 h-4 text-gray-400" />}
                      </td>
                    </tr>
                    {expandedTask === task.id && (
                      <tr key={`${task.id}-detail`}>
                        <td colSpan={7} className="px-6 py-4 bg-gray-50 dark:bg-gray-800/30">
                          <div className="space-y-3 max-w-3xl">
                            {task.result && (
                              <div>
                                <p className="text-xs font-medium text-gray-500 dark:text-gray-400 mb-1 uppercase tracking-wide">Result</p>
                                <pre className="text-sm text-gray-700 dark:text-gray-300 whitespace-pre-wrap bg-white dark:bg-gray-900 rounded-lg p-3 border border-gray-200 dark:border-gray-700 max-h-60 overflow-y-auto">
                                  {task.result}
                                </pre>
                              </div>
                            )}
                            {task.error && (
                              <div>
                                <p className="text-xs font-medium text-red-500 dark:text-red-400 mb-1 uppercase tracking-wide">Error</p>
                                <pre className="text-sm text-red-700 dark:text-red-300 whitespace-pre-wrap bg-red-50 dark:bg-red-900/10 rounded-lg p-3 border border-red-200 dark:border-red-800 max-h-60 overflow-y-auto">
                                  {task.error}
                                </pre>
                              </div>
                            )}
                            <div className="flex flex-wrap gap-x-6 gap-y-1 text-xs text-gray-500 dark:text-gray-400">
                              {task.goalId && <span className="text-purple-500">Goal: {task.goalId.slice(0, 8)}</span>}
                              {task.projectId && <span className="text-blue-500">Project: {task.projectId.slice(0, 8)}</span>}
                              {task.issueId && <span className="text-orange-500">Issue: {task.issueId.slice(0, 8)}</span>}
                              {task.tokensIn !== undefined && <span>Tokens In: {task.tokensIn?.toLocaleString()}</span>}
                              {task.tokensOut !== undefined && <span>Tokens Out: {task.tokensOut?.toLocaleString()}</span>}
                              {task.startedAt && <span>Started: {new Date(task.startedAt).toLocaleString()}</span>}
                              {task.endedAt && <span>Ended: {new Date(task.endedAt).toLocaleString()}</span>}
                            </div>
                            {!task.result && !task.error && (
                              <p className="text-sm text-gray-400 dark:text-gray-500 italic">No result or error data available</p>
                            )}
                          </div>
                        </td>
                      </tr>
                    )}
                  </>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      </>}

      {/* Hierarchy View */}
      {viewMode === 'hierarchy' && hierarchy && typeof hierarchy === 'object' && 'goalMap' in hierarchy && (
        <div className="bg-white dark:bg-gray-900 rounded-xl shadow-sm border border-gray-200 dark:border-gray-800">
          <div className="p-4 border-b border-gray-200 dark:border-gray-800">
            <h3 className="text-sm font-semibold text-gray-700 dark:text-gray-300">Goal → Project → Task → Issue</h3>
          </div>
          <div className="divide-y divide-gray-100 dark:divide-gray-800">
            {goals.map((g) => {
              const goalTasks = hierarchy.tasksByGoal?.get(g.id) || []
              const goalProjects = hierarchy.projectsByGoal?.get(g.id) || []
              const totalTasks = goalTasks.length + goalProjects.reduce((sum, p) => sum + (hierarchy.tasksByProject?.get(p.id)?.length || 0), 0)
              const completedTasks = goalTasks.filter(t => t.status === 'Completed').length +
                goalProjects.reduce((sum, p) => sum + (hierarchy.tasksByProject?.get(p.id)?.filter(t => t.status === 'Completed')?.length || 0), 0)

              return (
                <div key={g.id}>
                  {/* Goal row */}
                  <button
                    onClick={() => toggleGoal(g.id)}
                    className="w-full flex items-center gap-3 px-4 py-3 hover:bg-gray-50 dark:hover:bg-gray-800/50 transition-colors"
                  >
                    {expandedGoals.has(g.id) ? <ChevronDown className="w-4 h-4 text-gray-400" /> : <ChevronRight className="w-4 h-4 text-gray-400" />}
                    <Target className="w-4 h-4 text-purple-500" />
                    <span className="font-medium text-gray-900 dark:text-white">{g.name}</span>
                    <span className="text-xs text-gray-400">Goal</span>
                    <span className="ml-auto text-xs text-gray-500">{completedTasks}/{totalTasks} tasks</span>
                    {totalTasks > 0 && (
                      <div className="w-20 h-1.5 bg-gray-200 dark:bg-gray-700 rounded-full overflow-hidden">
                        <div className="h-full bg-green-500 rounded-full" style={{ width: `${totalTasks > 0 ? (completedTasks / totalTasks) * 100 : 0}%` }} />
                      </div>
                    )}
                  </button>

                  {expandedGoals.has(g.id) && (
                    <div className="bg-gray-50/50 dark:bg-gray-800/20">
                      {/* Projects under goal */}
                      {goalProjects.map((p) => {
                        const projTasks = hierarchy.tasksByProject?.get(p.id) || []
                        const projIssues = hierarchy.issuesByProject?.get(p.id) || []
                        return (
                          <div key={p.id}>
                            <button
                              onClick={() => toggleProject(p.id)}
                              className="w-full flex items-center gap-3 px-4 py-2 pl-10 hover:bg-gray-100 dark:hover:bg-gray-800/50"
                            >
                              {expandedProjects.has(p.id) ? <ChevronDown className="w-3.5 h-3.5 text-gray-400" /> : <ChevronRight className="w-3.5 h-3.5 text-gray-400" />}
                              <FolderKanban className="w-4 h-4 text-blue-500" />
                              <span className="text-sm font-medium text-gray-800 dark:text-gray-200">{p.name}</span>
                              <span className="text-xs text-gray-400">Project</span>
                              <span className="ml-auto text-xs text-gray-400">{projTasks.length} tasks, {projIssues.length} issues</span>
                            </button>

                            {expandedProjects.has(p.id) && (
                              <div className="pl-16 space-y-0.5 py-1">
                                {/* Tasks under project */}
                                {projTasks.map((t) => (
                                  <div key={t.id} className="flex items-center gap-2 px-4 py-1.5 text-sm">
                                    <ListTodo className="w-3.5 h-3.5 text-orange-500" />
                                    <span className="text-gray-700 dark:text-gray-300 truncate max-w-md">{t.message.slice(0, 80)}</span>
                                    <span className={clsx('text-xs px-1.5 py-0.5 rounded-full ml-auto', statusColors[t.status] || statusColors.Pending)}>
                                      {t.status}
                                    </span>
                                    <span className="text-xs text-gray-400">{t.agentName}</span>
                                  </div>
                                ))}
                                {/* Issues under project */}
                                {projIssues.map((i) => (
                                  <div key={i.id} className="flex items-center gap-2 px-4 py-1 text-xs text-gray-500 dark:text-gray-400 pl-8">
                                    <span className="w-1.5 h-1.5 bg-gray-400 rounded-full" />
                                    <span className="truncate">{i.name}</span>
                                    <span className={clsx('px-1.5 py-0.5 rounded-full ml-auto',
                                      i.status === 'open' ? 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-400' :
                                      i.status === 'in_progress' ? 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400' :
                                      'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400'
                                    )}>{i.status}</span>
                                    {i.agentRef && <span className="text-gray-400">{i.agentRef}</span>}
                                  </div>
                                ))}
                                {projTasks.length === 0 && projIssues.length === 0 && (
                                  <p className="px-4 py-1 text-xs text-gray-400 italic">No tasks or issues</p>
                                )}
                              </div>
                            )}
                          </div>
                        )
                      })}

                      {/* Direct tasks under goal (no project) */}
                      {goalTasks.map((t) => (
                        <div key={t.id} className="flex items-center gap-2 px-4 py-1.5 pl-10 text-sm">
                          <ListTodo className="w-3.5 h-3.5 text-orange-500" />
                          <span className="text-gray-700 dark:text-gray-300 truncate max-w-md">{t.message.slice(0, 80)}</span>
                          <span className={clsx('text-xs px-1.5 py-0.5 rounded-full ml-auto', statusColors[t.status])}>
                            {t.status}
                          </span>
                        </div>
                      ))}

                      {goalProjects.length === 0 && goalTasks.length === 0 && (
                        <p className="px-4 py-2 pl-10 text-sm text-gray-400 italic">No projects or tasks</p>
                      )}
                    </div>
                  )}
                </div>
              )
            })}

            {/* Orphan tasks (no goal) */}
            {hierarchy.orphanTasks && hierarchy.orphanTasks.length > 0 && (
              <div>
                <div className="px-4 py-3 flex items-center gap-3">
                  <Bot className="w-4 h-4 text-gray-400" />
                  <span className="font-medium text-gray-600 dark:text-gray-400">Standalone Tasks (no Goal)</span>
                  <span className="text-xs text-gray-400 ml-auto">{hierarchy.orphanTasks.length} tasks</span>
                </div>
                <div className="pl-10 space-y-0.5 pb-2">
                  {hierarchy.orphanTasks.slice(0, 20).map((t) => (
                    <div key={t.id} className="flex items-center gap-2 px-4 py-1.5 text-sm">
                      <ListTodo className="w-3.5 h-3.5 text-gray-400" />
                      <span className="text-gray-600 dark:text-gray-400 truncate max-w-md">{t.message.slice(0, 80)}</span>
                      <span className={clsx('text-xs px-1.5 py-0.5 rounded-full ml-auto', statusColors[t.status])}>
                        {t.status}
                      </span>
                      <span className="text-xs text-gray-400">{t.agentName}</span>
                    </div>
                  ))}
                </div>
              </div>
            )}
          </div>
        </div>
      )}

      {/* Goals View */}
      {viewMode === 'goals' && (
        <div className="bg-white dark:bg-gray-900 rounded-xl shadow-sm border border-gray-200 dark:border-gray-800 divide-y divide-gray-100 dark:divide-gray-800">
          {goals.length === 0 ? (
            <p className="p-8 text-center text-gray-400">No goals found</p>
          ) : goals.map((g) => {
            const goalTasks = tasks.filter(t => t.goalId === g.id)
            const completed = goalTasks.filter(t => t.status === 'Completed').length
            const failed = goalTasks.filter(t => t.status === 'Failed').length
            return (
              <div key={g.id} className="p-4 hover:bg-gray-50 dark:hover:bg-gray-800/30">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-3">
                    <Target className="w-5 h-5 text-purple-500" />
                    <div>
                      <h3 className="font-medium text-gray-900 dark:text-white">{g.name}</h3>
                      <p className="text-sm text-gray-500 truncate max-w-lg">{g.description}</p>
                    </div>
                  </div>
                  <div className="flex items-center gap-4 text-sm">
                    <span className="text-gray-500">{goalTasks.length} tasks</span>
                    {completed > 0 && <span className="text-green-600">{completed} success</span>}
                    {failed > 0 && <span className="text-red-600">{failed} fail</span>}
                    <span className={clsx('px-2 py-0.5 rounded-full text-xs', statusColors[g.status] || 'bg-gray-100 text-gray-600')}>{g.status}</span>
                    {g.cost > 0 && <span className="text-gray-400">${g.cost.toFixed(4)}</span>}
                  </div>
                </div>
                {goalTasks.length > 0 && (
                  <div className="mt-2 w-full h-1.5 bg-gray-200 dark:bg-gray-700 rounded-full overflow-hidden">
                    <div className="h-full flex">
                      <div className="bg-green-500" style={{ width: `${goalTasks.length > 0 ? (completed / goalTasks.length) * 100 : 0}%` }} />
                      <div className="bg-red-500" style={{ width: `${goalTasks.length > 0 ? (failed / goalTasks.length) * 100 : 0}%` }} />
                    </div>
                  </div>
                )}
              </div>
            )
          })}
        </div>
      )}

      {/* Projects View */}
      {viewMode === 'projects' && (
        <div className="bg-white dark:bg-gray-900 rounded-xl shadow-sm border border-gray-200 dark:border-gray-800 divide-y divide-gray-100 dark:divide-gray-800">
          {projects.length === 0 ? (
            <p className="p-8 text-center text-gray-400">No projects found</p>
          ) : projects.map((p) => {
            const projTasks = tasks.filter(t => t.projectId === p.id)
            const projIssues = issues.filter(i => i.projectId === p.id)
            const parentGoal = goals.find(g => g.id === p.goalId)
            return (
              <div key={p.id} className="p-4 hover:bg-gray-50 dark:hover:bg-gray-800/30">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-3">
                    <FolderKanban className="w-5 h-5 text-blue-500" />
                    <div>
                      <h3 className="font-medium text-gray-900 dark:text-white">{p.name}</h3>
                      <p className="text-sm text-gray-500">{p.description}</p>
                    </div>
                  </div>
                  <div className="flex items-center gap-4 text-sm">
                    {parentGoal && <span className="text-purple-500 text-xs">← {parentGoal.name}</span>}
                    <span className="text-gray-500">{projTasks.length} tasks</span>
                    <span className="text-gray-400">{projIssues.length} issues</span>
                    <span className={clsx('px-2 py-0.5 rounded-full text-xs', statusColors[p.status] || 'bg-gray-100 text-gray-600')}>{p.status}</span>
                  </div>
                </div>
              </div>
            )
          })}
        </div>
      )}

      {/* Issues View */}
      {viewMode === 'issues' && (
        <div className="bg-white dark:bg-gray-900 rounded-xl shadow-sm border border-gray-200 dark:border-gray-800 divide-y divide-gray-100 dark:divide-gray-800">
          {issues.length === 0 ? (
            <p className="p-8 text-center text-gray-400">No issues found</p>
          ) : issues.map((i) => {
            const parentProject = projects.find(p => p.id === i.projectId)
            const parentGoal = parentProject ? goals.find(g => g.id === parentProject.goalId) : null
            return (
              <div key={i.id} className="p-3 hover:bg-gray-50 dark:hover:bg-gray-800/30">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-3">
                    <span className={clsx('w-2 h-2 rounded-full',
                      i.status === 'open' ? 'bg-yellow-500' :
                      i.status === 'in_progress' ? 'bg-blue-500' :
                      i.status === 'closed' ? 'bg-green-500' : 'bg-gray-400'
                    )} />
                    <div>
                      <span className="text-sm font-medium text-gray-900 dark:text-white">{i.name}</span>
                      {i.description && <p className="text-xs text-gray-500 truncate max-w-lg">{i.description}</p>}
                    </div>
                  </div>
                  <div className="flex items-center gap-3 text-xs">
                    {parentGoal && <span className="text-purple-400">Goal: {parentGoal.name}</span>}
                    {parentProject && <span className="text-blue-400">Proj: {parentProject.name}</span>}
                    {i.agentRef && <span className="text-orange-400 flex items-center gap-1"><Bot className="w-3 h-3" />{i.agentRef}</span>}
                    <span className={clsx('px-1.5 py-0.5 rounded-full',
                      i.status === 'open' ? 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-400' :
                      i.status === 'in_progress' ? 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400' :
                      'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400'
                    )}>{i.status}</span>
                  </div>
                </div>
              </div>
            )
          })}
        </div>
      )}

      {/* Summary */}
      <div className="flex gap-4 text-sm text-gray-500 dark:text-gray-400">
        {viewMode === 'flat' && <span>Showing {filteredTasks.length} of {tasks.length} tasks</span>}
        {viewMode === 'goals' && <span>{goals.length} goals</span>}
        {viewMode === 'projects' && <span>{projects.length} projects</span>}
        {viewMode === 'issues' && <span>{issues.length} issues</span>}
        {viewMode === 'hierarchy' && <span>{goals.length} goals, {projects.length} projects, {tasks.length} tasks, {issues.length} issues</span>}
      </div>
    </div>
  )
}
