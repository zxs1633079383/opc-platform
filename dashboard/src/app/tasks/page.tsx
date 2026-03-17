'use client'

import { useQuery } from '@tanstack/react-query'
import { Search, ChevronDown, ChevronUp, ListTodo, Bot } from 'lucide-react'
import { useState, useMemo } from 'react'
import { clsx } from 'clsx'
import { formatDistanceToNow } from 'date-fns'
import { fetchTasks } from '@/lib/api'
import { useSearchParams } from 'next/navigation'

const statusTabs = [
  { value: 'all', label: 'All' },
  { value: 'Running', label: 'Running' },
  { value: 'Completed', label: 'Completed' },
  { value: 'Failed', label: 'Failed' },
  { value: 'Pending', label: 'Pending' },
  { value: 'Cancelled', label: 'Cancelled' },
]

const statusColors: Record<string, string> = {
  Running: 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-400',
  Pending: 'bg-gray-100 text-gray-800 dark:bg-gray-800 dark:text-gray-400',
  Completed: 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400',
  Failed: 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-400',
  Cancelled: 'bg-gray-100 text-gray-500 dark:bg-gray-800 dark:text-gray-500',
}

export default function TasksPage() {
  const searchParams = useSearchParams()
  const initialStatus = searchParams.get('status') || 'all'

  const [search, setSearch] = useState('')
  const [statusFilter, setStatusFilter] = useState<string>(initialStatus)
  const [agentFilter, setAgentFilter] = useState<string>('all')
  const [expandedTask, setExpandedTask] = useState<string | null>(null)

  const { data: tasks = [], isLoading } = useQuery({
    queryKey: ['tasks'],
    queryFn: fetchTasks,
  })

  const agentNames = useMemo(() => {
    const names = new Set(tasks.map(t => t.agentName))
    return Array.from(names).sort()
  }, [tasks])

  const statusCounts = useMemo(() => {
    const counts: Record<string, number> = { all: tasks.length }
    for (const task of tasks) {
      counts[task.status] = (counts[task.status] || 0) + 1
    }
    return counts
  }, [tasks])

  const filteredTasks = tasks.filter((task) => {
    const matchesSearch =
      search === '' ||
      task.message.toLowerCase().includes(search.toLowerCase()) ||
      task.agentName.toLowerCase().includes(search.toLowerCase()) ||
      task.id.toLowerCase().includes(search.toLowerCase())

    const matchesStatus =
      statusFilter === 'all' || task.status === statusFilter

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
      </div>

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
                          {task.status}
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
                            <div className="flex gap-6 text-xs text-gray-500 dark:text-gray-400">
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

      {/* Summary */}
      <div className="flex gap-4 text-sm text-gray-500 dark:text-gray-400">
        <span>Showing {filteredTasks.length} of {tasks.length} tasks</span>
      </div>
    </div>
  )
}
