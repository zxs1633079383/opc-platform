'use client'

import { useQuery } from '@tanstack/react-query'
import { Activity, CheckCircle2, Clock, DollarSign, Zap, Plus, Target, XCircle, Bot, ArrowRight } from 'lucide-react'
import Link from 'next/link'
import { AgentStatusCard } from '@/components/AgentStatusCard'
import { MetricCard } from '@/components/MetricCard'
import { CostChart } from '@/components/CostChart'
import { fetchAgents, fetchMetrics, fetchTasks, fetchCostData } from '@/lib/api'

export default function DashboardPage() {
  const { data: agents = [], isLoading: agentsLoading } = useQuery({
    queryKey: ['agents'],
    queryFn: fetchAgents,
  })

  const { data: metrics } = useQuery({
    queryKey: ['metrics'],
    queryFn: fetchMetrics,
  })

  const { data: tasks = [] } = useQuery({
    queryKey: ['tasks'],
    queryFn: fetchTasks,
  })

  const { data: costData = [] } = useQuery({
    queryKey: ['costData'],
    queryFn: fetchCostData,
  })

  const runningAgents = agents.filter(a => a.phase === 'Running').length
  const stoppedAgents = agents.filter(a => a.phase === 'Stopped').length
  const failedAgents = agents.filter(a => a.phase === 'Failed').length
  const runningTasks = tasks.filter(t => t.status === 'Running').length
  const completedTasks = tasks.filter(t => t.status === 'Completed').length
  const failedTasks = tasks.filter(t => t.status === 'Failed').length


  return (
    <div className="p-6 space-y-6">
      {/* Header */}
      <div className="flex justify-between items-center">
        <div>
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white">
            Dashboard
          </h1>
          <p className="text-gray-500 dark:text-gray-400">
            Agent Orchestration Platform Control Center
          </p>
        </div>
        <div className="flex items-center gap-2 text-sm text-gray-500">
          <span className="inline-block w-2 h-2 bg-green-500 rounded-full animate-pulse" />
          System Online
        </div>
      </div>

      {/* Metric Cards */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        <MetricCard
          title="Active Agents"
          value={runningAgents}
          total={agents.length}
          icon={<Activity className="w-5 h-5" />}
          color="blue"
        />
        <MetricCard
          title="Running Tasks"
          value={runningTasks}
          icon={<Zap className="w-5 h-5" />}
          color="yellow"
        />
        <MetricCard
          title="Completed Today"
          value={completedTasks}
          icon={<CheckCircle2 className="w-5 h-5" />}
          color="green"
        />
        <MetricCard
          title="Today's Cost"
          value={`$${metrics?.todayCost?.toFixed(2) || '0.00'}`}
          subtitle={`Budget: $${metrics?.dailyBudget || '10.00'}`}
          icon={<DollarSign className="w-5 h-5" />}
          color="purple"
        />
      </div>

      {/* Quick Actions */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <Link
          href="/agents"
          className="flex items-center gap-4 p-4 bg-white dark:bg-gray-900 rounded-xl shadow-sm border border-gray-200 dark:border-gray-800 hover:border-primary-300 dark:hover:border-primary-700 transition-colors group"
        >
          <div className="p-3 bg-blue-50 dark:bg-blue-900/20 rounded-lg group-hover:bg-blue-100 dark:group-hover:bg-blue-900/30 transition-colors">
            <Plus className="w-5 h-5 text-blue-600 dark:text-blue-400" />
          </div>
          <div className="flex-1">
            <p className="text-sm font-medium text-gray-900 dark:text-white">Create Agent</p>
            <p className="text-xs text-gray-500 dark:text-gray-400">Add a new AI agent</p>
          </div>
          <ArrowRight className="w-4 h-4 text-gray-400 group-hover:text-primary-500 transition-colors" />
        </Link>
        <Link
          href="/goals"
          className="flex items-center gap-4 p-4 bg-white dark:bg-gray-900 rounded-xl shadow-sm border border-gray-200 dark:border-gray-800 hover:border-primary-300 dark:hover:border-primary-700 transition-colors group"
        >
          <div className="p-3 bg-green-50 dark:bg-green-900/20 rounded-lg group-hover:bg-green-100 dark:group-hover:bg-green-900/30 transition-colors">
            <Target className="w-5 h-5 text-green-600 dark:text-green-400" />
          </div>
          <div className="flex-1">
            <p className="text-sm font-medium text-gray-900 dark:text-white">Execute Goal</p>
            <p className="text-xs text-gray-500 dark:text-gray-400">Create and run a goal</p>
          </div>
          <ArrowRight className="w-4 h-4 text-gray-400 group-hover:text-primary-500 transition-colors" />
        </Link>
        <Link
          href="/tasks?status=Failed"
          className="flex items-center gap-4 p-4 bg-white dark:bg-gray-900 rounded-xl shadow-sm border border-gray-200 dark:border-gray-800 hover:border-primary-300 dark:hover:border-primary-700 transition-colors group"
        >
          <div className="p-3 bg-red-50 dark:bg-red-900/20 rounded-lg group-hover:bg-red-100 dark:group-hover:bg-red-900/30 transition-colors">
            <XCircle className="w-5 h-5 text-red-600 dark:text-red-400" />
          </div>
          <div className="flex-1">
            <p className="text-sm font-medium text-gray-900 dark:text-white">Failed Tasks</p>
            <p className="text-xs text-gray-500 dark:text-gray-400">{failedTasks} tasks need attention</p>
          </div>
          <ArrowRight className="w-4 h-4 text-gray-400 group-hover:text-primary-500 transition-colors" />
        </Link>
      </div>

      {/* Main Content Grid */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Agent Status Overview */}
        <div className="lg:col-span-2 bg-white dark:bg-gray-900 rounded-xl shadow-sm border border-gray-200 dark:border-gray-800 p-6">
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">
            Agent Status
          </h2>
          {agentsLoading ? (
            <div className="space-y-4">
              {[1, 2, 3, 4].map((i) => (
                <div key={i} className="animate-pulse flex gap-4">
                  <div className="h-20 flex-1 bg-gray-200 dark:bg-gray-800 rounded-lg" />
                  <div className="h-20 flex-1 bg-gray-200 dark:bg-gray-800 rounded-lg" />
                </div>
              ))}
            </div>
          ) : agents.length === 0 ? (
            <div className="flex flex-col items-center justify-center h-48 text-gray-500">
              <Bot className="w-12 h-12 mb-3 text-gray-300 dark:text-gray-600" />
              <p className="text-gray-500 dark:text-gray-400 mb-3">No agents configured</p>
              <Link
                href="/agents"
                className="inline-flex items-center gap-2 px-4 py-2 text-sm text-white bg-primary-600 hover:bg-primary-700 rounded-lg transition-colors"
              >
                <Plus className="w-4 h-4" />
                Add your first agent
              </Link>
            </div>
          ) : (
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              {agents.map((agent) => (
                <AgentStatusCard key={agent.name} agent={agent} />
              ))}
            </div>
          )}
        </div>

        {/* Right Column: Agent Distribution + Cost Chart */}
        <div className="space-y-6">
          {/* Agent Status Distribution */}
          {agents.length > 0 && (
            <div className="bg-white dark:bg-gray-900 rounded-xl shadow-sm border border-gray-200 dark:border-gray-800 p-6">
              <h2 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">
                Agent Distribution
              </h2>
              <div className="space-y-3">
                {[
                  { label: 'Running', count: runningAgents, color: 'bg-green-500', textColor: 'text-green-600 dark:text-green-400' },
                  { label: 'Stopped', count: stoppedAgents, color: 'bg-gray-400', textColor: 'text-gray-600 dark:text-gray-400' },
                  { label: 'Failed', count: failedAgents, color: 'bg-red-500', textColor: 'text-red-600 dark:text-red-400' },
                ].map(({ label, count, color, textColor }) => (
                  <div key={label}>
                    <div className="flex justify-between text-sm mb-1">
                      <span className={textColor}>{label}</span>
                      <span className="text-gray-500 dark:text-gray-400">{count} / {agents.length}</span>
                    </div>
                    <div className="w-full h-2 bg-gray-100 dark:bg-gray-800 rounded-full overflow-hidden">
                      <div
                        className={`h-full ${color} rounded-full transition-all duration-500`}
                        style={{ width: agents.length > 0 ? `${(count / agents.length) * 100}%` : '0%' }}
                      />
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Cost Chart */}
          <div className="bg-white dark:bg-gray-900 rounded-xl shadow-sm border border-gray-200 dark:border-gray-800 p-6">
            <h2 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">
              Cost (7 Days)
            </h2>
            <CostChart data={costData} />
          </div>
        </div>
      </div>

      {/* Recent Tasks with Result Summaries */}
      <div className="bg-white dark:bg-gray-900 rounded-xl shadow-sm border border-gray-200 dark:border-gray-800 p-6">
        <div className="flex justify-between items-center mb-4">
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white">
            Recent Tasks
          </h2>
          <Link
            href="/tasks"
            className="text-sm text-primary-600 hover:text-primary-700 dark:text-primary-400"
          >
            View all →
          </Link>
        </div>
        {tasks.length === 0 ? (
          <div className="flex flex-col items-center justify-center py-12 text-gray-500">
            <Clock className="w-12 h-12 mb-3 text-gray-300 dark:text-gray-600" />
            <p className="text-gray-500 dark:text-gray-400">No tasks executed yet</p>
          </div>
        ) : (
          <div className="space-y-3">
            {tasks.slice(0, 8).map((task) => (
              <div
                key={task.id}
                className="flex items-start gap-3 p-3 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-800/50 transition-colors"
              >
                <div className={`mt-1 w-2 h-2 rounded-full flex-shrink-0 ${
                  task.status === 'Running' ? 'bg-blue-500 animate-pulse' :
                  task.status === 'Completed' ? 'bg-green-500' :
                  task.status === 'Failed' ? 'bg-red-500' :
                  'bg-gray-400'
                }`} />
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2">
                    <span className="text-sm font-medium text-gray-900 dark:text-white truncate">
                      {task.message}
                    </span>
                    <span className="text-xs text-gray-400 flex-shrink-0">{task.agentName}</span>
                  </div>
                  {task.result && (
                    <p className="text-xs text-gray-500 dark:text-gray-400 mt-0.5 truncate">
                      {task.result.slice(0, 120)}
                    </p>
                  )}
                  {task.error && (
                    <p className="text-xs text-red-500 dark:text-red-400 mt-0.5 truncate">
                      {task.error.slice(0, 120)}
                    </p>
                  )}
                </div>
                <span className="text-xs text-gray-400 flex-shrink-0">
                  ${(task.cost || 0).toFixed(4)}
                </span>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
