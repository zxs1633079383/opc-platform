'use client'

import { useQuery } from '@tanstack/react-query'
import { Plus, RefreshCw } from 'lucide-react'
import { AgentStatusCard } from '@/components/AgentStatusCard'
import { fetchAgents } from '@/lib/api'

export default function AgentsPage() {
  const { data: agents = [], isLoading, refetch } = useQuery({
    queryKey: ['agents'],
    queryFn: fetchAgents,
  })

  return (
    <div className="p-6 space-y-6">
      <div className="flex justify-between items-center">
        <div>
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white">
            Agents
          </h1>
          <p className="text-gray-500 dark:text-gray-400">
            Manage your AI agents
          </p>
        </div>
        <div className="flex gap-2">
          <button
            onClick={() => refetch()}
            className="flex items-center gap-2 px-4 py-2 text-sm text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800 rounded-lg transition-colors"
          >
            <RefreshCw className="w-4 h-4" />
            Refresh
          </button>
          <button className="flex items-center gap-2 px-4 py-2 text-sm text-white bg-primary-600 hover:bg-primary-700 rounded-lg transition-colors">
            <Plus className="w-4 h-4" />
            Add Agent
          </button>
        </div>
      </div>

      {isLoading ? (
        <div className="flex items-center justify-center h-48">
          <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary-500" />
        </div>
      ) : agents.length === 0 ? (
        <div className="bg-white dark:bg-gray-900 rounded-xl shadow-sm border border-gray-200 dark:border-gray-800 p-12 text-center">
          <p className="text-gray-500 dark:text-gray-400 mb-4">
            No agents configured yet
          </p>
          <button className="inline-flex items-center gap-2 px-4 py-2 text-sm text-white bg-primary-600 hover:bg-primary-700 rounded-lg transition-colors">
            <Plus className="w-4 h-4" />
            Create your first agent
          </button>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {agents.map((agent) => (
            <AgentStatusCard key={agent.name} agent={agent} />
          ))}
        </div>
      )}

      {/* Summary Stats */}
      {agents.length > 0 && (
        <div className="bg-white dark:bg-gray-900 rounded-xl shadow-sm border border-gray-200 dark:border-gray-800 p-6">
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">
            Summary
          </h2>
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
            <div>
              <p className="text-sm text-gray-500 dark:text-gray-400">
                Total Agents
              </p>
              <p className="text-2xl font-semibold text-gray-900 dark:text-white">
                {agents.length}
              </p>
            </div>
            <div>
              <p className="text-sm text-gray-500 dark:text-gray-400">
                Running
              </p>
              <p className="text-2xl font-semibold text-green-600">
                {agents.filter((a) => a.phase === 'Running').length}
              </p>
            </div>
            <div>
              <p className="text-sm text-gray-500 dark:text-gray-400">
                Total Cost
              </p>
              <p className="text-2xl font-semibold text-gray-900 dark:text-white">
                ${agents.reduce((sum, a) => sum + (a.metrics?.totalCost || 0), 0).toFixed(2)}
              </p>
            </div>
            <div>
              <p className="text-sm text-gray-500 dark:text-gray-400">
                Tasks Completed
              </p>
              <p className="text-2xl font-semibold text-gray-900 dark:text-white">
                {agents.reduce((sum, a) => sum + (a.metrics?.tasksCompleted || 0), 0)}
              </p>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
