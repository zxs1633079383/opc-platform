'use client'

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Plus, RefreshCw, Play, Square, RotateCw, Bot, X, DollarSign, Cpu, AlertTriangle } from 'lucide-react'
import { clsx } from 'clsx'
import { useState } from 'react'
import { fetchAgents, startAgent, stopAgent, restartAgent, createAgent } from '@/lib/api'

const phaseColors: Record<string, string> = {
  Running: 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400',
  Starting: 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-400',
  Stopped: 'bg-gray-100 text-gray-800 dark:bg-gray-800 dark:text-gray-400',
  Failed: 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-400',
  Retrying: 'bg-orange-100 text-orange-800 dark:bg-orange-900/30 dark:text-orange-400',
}

const phaseIndicators: Record<string, string> = {
  Running: 'bg-green-500',
  Starting: 'bg-yellow-500 animate-pulse',
  Stopped: 'bg-gray-400',
  Failed: 'bg-red-500',
  Retrying: 'bg-orange-500 animate-pulse',
}

function formatTokens(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`
  return String(n)
}

export default function AgentsPage() {
  const queryClient = useQueryClient()
  const [showModal, setShowModal] = useState(false)
  const [newAgent, setNewAgent] = useState({ name: '', type: 'claude-code', model: '' })
  const [actionError, setActionError] = useState<string | null>(null)

  const { data: agents = [], isLoading, refetch } = useQuery({
    queryKey: ['agents'],
    queryFn: fetchAgents,
  })

  const startMutation = useMutation({
    mutationFn: startAgent,
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['agents'] }); setActionError(null) },
    onError: (err: Error) => setActionError(err.message),
  })

  const stopMutation = useMutation({
    mutationFn: stopAgent,
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['agents'] }); setActionError(null) },
    onError: (err: Error) => setActionError(err.message),
  })

  const restartMutation = useMutation({
    mutationFn: restartAgent,
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['agents'] }); setActionError(null) },
    onError: (err: Error) => setActionError(err.message),
  })

  const createMutation = useMutation({
    mutationFn: () => createAgent(newAgent),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['agents'] })
      setShowModal(false)
      setNewAgent({ name: '', type: 'claude-code', model: '' })
    },
    onError: (err: Error) => setActionError(err.message),
  })

  const isActing = startMutation.isPending || stopMutation.isPending || restartMutation.isPending

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
          <button
            onClick={() => setShowModal(true)}
            className="flex items-center gap-2 px-4 py-2 text-sm text-white bg-primary-600 hover:bg-primary-700 rounded-lg transition-colors"
          >
            <Plus className="w-4 h-4" />
            Add Agent
          </button>
        </div>
      </div>

      {/* Action Error Banner */}
      {actionError && (
        <div className="flex items-center gap-3 p-3 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg">
          <AlertTriangle className="w-5 h-5 text-red-600 dark:text-red-400 flex-shrink-0" />
          <p className="text-sm text-red-700 dark:text-red-300 flex-1">{actionError}</p>
          <button onClick={() => setActionError(null)} className="text-red-500 hover:text-red-700">
            <X className="w-4 h-4" />
          </button>
        </div>
      )}

      {isLoading ? (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {[1, 2, 3].map((i) => (
            <div key={i} className="animate-pulse bg-white dark:bg-gray-900 rounded-xl border border-gray-200 dark:border-gray-800 p-6">
              <div className="h-4 bg-gray-200 dark:bg-gray-700 rounded w-1/3 mb-3" />
              <div className="h-3 bg-gray-200 dark:bg-gray-700 rounded w-1/4 mb-4" />
              <div className="grid grid-cols-2 gap-3">
                <div className="h-3 bg-gray-200 dark:bg-gray-700 rounded" />
                <div className="h-3 bg-gray-200 dark:bg-gray-700 rounded" />
                <div className="h-3 bg-gray-200 dark:bg-gray-700 rounded" />
                <div className="h-3 bg-gray-200 dark:bg-gray-700 rounded" />
              </div>
            </div>
          ))}
        </div>
      ) : agents.length === 0 ? (
        <div className="bg-white dark:bg-gray-900 rounded-xl shadow-sm border border-gray-200 dark:border-gray-800 p-12 text-center">
          <Bot className="w-16 h-16 text-gray-300 dark:text-gray-600 mx-auto mb-4" />
          <h3 className="text-lg font-medium text-gray-900 dark:text-white mb-2">No agents configured yet</h3>
          <p className="text-gray-500 dark:text-gray-400 mb-6 max-w-sm mx-auto">
            Create your first agent to start orchestrating AI tasks
          </p>
          <button
            onClick={() => setShowModal(true)}
            className="inline-flex items-center gap-2 px-4 py-2 text-sm text-white bg-primary-600 hover:bg-primary-700 rounded-lg transition-colors"
          >
            <Plus className="w-4 h-4" />
            Create your first agent
          </button>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {agents.map((agent) => {
            const metrics = agent.metrics || {
              tasksCompleted: 0, tasksFailed: 0, tasksRunning: 0,
              totalTokensIn: 0, totalTokensOut: 0, totalCost: 0, uptimeSeconds: 0,
            }
            return (
              <div key={agent.name} className="bg-white dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-xl p-5 hover:shadow-md transition-shadow">
                <div className="flex items-start justify-between mb-3">
                  <div className="flex items-center gap-2">
                    <span className={clsx('w-2 h-2 rounded-full', phaseIndicators[agent.phase] || 'bg-gray-400')} />
                    <h3 className="font-medium text-gray-900 dark:text-white">{agent.name}</h3>
                  </div>
                  <span className={clsx('px-2 py-0.5 rounded-full text-xs font-medium', phaseColors[agent.phase] || phaseColors.Stopped)}>
                    {agent.phase}
                  </span>
                </div>

                <p className="text-sm text-gray-500 dark:text-gray-400 mb-3 capitalize">{agent.type}</p>

                {/* Token & Cost Stats */}
                <div className="grid grid-cols-2 gap-2 text-xs mb-4">
                  <div className="flex items-center gap-1.5 text-gray-600 dark:text-gray-300">
                    <Cpu className="w-3.5 h-3.5 text-gray-400" />
                    <span>{formatTokens(metrics.totalTokensIn + metrics.totalTokensOut)} tokens</span>
                  </div>
                  <div className="flex items-center gap-1.5 text-gray-600 dark:text-gray-300">
                    <DollarSign className="w-3.5 h-3.5 text-gray-400" />
                    <span>${(metrics.totalCost || 0).toFixed(2)}</span>
                  </div>
                  <div className="text-gray-500 dark:text-gray-400">
                    {metrics.tasksCompleted} completed
                  </div>
                  <div className="text-gray-500 dark:text-gray-400">
                    {metrics.tasksRunning} running
                  </div>
                </div>

                {/* Quick Actions */}
                <div className="flex gap-2 pt-3 border-t border-gray-100 dark:border-gray-800">
                  {agent.phase !== 'Running' ? (
                    <button
                      onClick={() => startMutation.mutate(agent.name)}
                      disabled={isActing}
                      className="flex-1 flex items-center justify-center gap-1.5 px-3 py-1.5 text-xs font-medium text-green-700 dark:text-green-400 bg-green-50 dark:bg-green-900/20 hover:bg-green-100 dark:hover:bg-green-900/30 rounded-lg transition-colors disabled:opacity-50"
                    >
                      <Play className="w-3.5 h-3.5" />
                      Start
                    </button>
                  ) : (
                    <button
                      onClick={() => stopMutation.mutate(agent.name)}
                      disabled={isActing}
                      className="flex-1 flex items-center justify-center gap-1.5 px-3 py-1.5 text-xs font-medium text-red-700 dark:text-red-400 bg-red-50 dark:bg-red-900/20 hover:bg-red-100 dark:hover:bg-red-900/30 rounded-lg transition-colors disabled:opacity-50"
                    >
                      <Square className="w-3.5 h-3.5" />
                      Stop
                    </button>
                  )}
                  <button
                    onClick={() => restartMutation.mutate(agent.name)}
                    disabled={isActing}
                    className="flex-1 flex items-center justify-center gap-1.5 px-3 py-1.5 text-xs font-medium text-blue-700 dark:text-blue-400 bg-blue-50 dark:bg-blue-900/20 hover:bg-blue-100 dark:hover:bg-blue-900/30 rounded-lg transition-colors disabled:opacity-50"
                  >
                    <RotateCw className="w-3.5 h-3.5" />
                    Restart
                  </button>
                </div>
              </div>
            )
          })}
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
              <p className="text-sm text-gray-500 dark:text-gray-400">Total Agents</p>
              <p className="text-2xl font-semibold text-gray-900 dark:text-white">{agents.length}</p>
            </div>
            <div>
              <p className="text-sm text-gray-500 dark:text-gray-400">Running</p>
              <p className="text-2xl font-semibold text-green-600">
                {agents.filter((a) => a.phase === 'Running').length}
              </p>
            </div>
            <div>
              <p className="text-sm text-gray-500 dark:text-gray-400">Total Cost</p>
              <p className="text-2xl font-semibold text-gray-900 dark:text-white">
                ${agents.reduce((sum, a) => sum + (a.metrics?.totalCost || 0), 0).toFixed(2)}
              </p>
            </div>
            <div>
              <p className="text-sm text-gray-500 dark:text-gray-400">Total Tokens</p>
              <p className="text-2xl font-semibold text-gray-900 dark:text-white">
                {formatTokens(agents.reduce((sum, a) => sum + (a.metrics?.totalTokensIn || 0) + (a.metrics?.totalTokensOut || 0), 0))}
              </p>
            </div>
          </div>
        </div>
      )}

      {/* Add Agent Modal */}
      {showModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center">
          <div className="fixed inset-0 bg-black/50" onClick={() => setShowModal(false)} />
          <div className="relative bg-white dark:bg-gray-900 rounded-xl shadow-xl border border-gray-200 dark:border-gray-800 p-6 w-full max-w-md mx-4">
            <div className="flex items-center justify-between mb-6">
              <h2 className="text-lg font-semibold text-gray-900 dark:text-white">Add Agent</h2>
              <button onClick={() => setShowModal(false)} className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300">
                <X className="w-5 h-5" />
              </button>
            </div>
            <div className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Agent Name</label>
                <input
                  type="text"
                  value={newAgent.name}
                  onChange={(e) => setNewAgent({ ...newAgent, name: e.target.value })}
                  placeholder="e.g., my-agent"
                  className="w-full px-3 py-2 border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-800 text-gray-900 dark:text-white focus:ring-2 focus:ring-primary-500 focus:border-transparent"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Agent Type</label>
                <select
                  value={newAgent.type}
                  onChange={(e) => setNewAgent({ ...newAgent, type: e.target.value })}
                  className="w-full px-3 py-2 border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-800 text-gray-900 dark:text-white focus:ring-2 focus:ring-primary-500 focus:border-transparent"
                >
                  <option value="claude-code">Claude Code</option>
                  <option value="openclaw">OpenClaw</option>
                  <option value="codex">Codex</option>
                </select>
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Model (optional)</label>
                <input
                  type="text"
                  value={newAgent.model}
                  onChange={(e) => setNewAgent({ ...newAgent, model: e.target.value })}
                  placeholder="e.g., claude-sonnet-4-20250514"
                  className="w-full px-3 py-2 border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-800 text-gray-900 dark:text-white focus:ring-2 focus:ring-primary-500 focus:border-transparent"
                />
              </div>
            </div>
            <div className="flex justify-end gap-3 mt-6">
              <button
                onClick={() => setShowModal(false)}
                className="px-4 py-2 text-sm text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800 rounded-lg transition-colors"
              >
                Cancel
              </button>
              <button
                onClick={() => createMutation.mutate()}
                disabled={!newAgent.name || createMutation.isPending}
                className="px-4 py-2 text-sm text-white bg-primary-600 hover:bg-primary-700 rounded-lg transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
              >
                {createMutation.isPending ? 'Creating...' : 'Create Agent'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
