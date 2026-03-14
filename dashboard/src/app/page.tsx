'use client'

import { useQuery } from '@tanstack/react-query'
import { Activity, AlertCircle, CheckCircle2, Clock, DollarSign, Zap } from 'lucide-react'
import { AgentStatusCard } from '@/components/AgentStatusCard'
import { MetricCard } from '@/components/MetricCard'
import { CostChart } from '@/components/CostChart'
import { TaskList } from '@/components/TaskList'
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

      {/* Main Content Grid */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Agent Status Overview */}
        <div className="lg:col-span-2 bg-white dark:bg-gray-900 rounded-xl shadow-sm border border-gray-200 dark:border-gray-800 p-6">
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">
            Agent Status
          </h2>
          {agentsLoading ? (
            <div className="flex items-center justify-center h-48">
              <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary-500" />
            </div>
          ) : agents.length === 0 ? (
            <div className="flex flex-col items-center justify-center h-48 text-gray-500">
              <AlertCircle className="w-8 h-8 mb-2" />
              <p>No agents configured</p>
            </div>
          ) : (
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              {agents.map((agent) => (
                <AgentStatusCard key={agent.name} agent={agent} />
              ))}
            </div>
          )}
        </div>

        {/* Cost Chart */}
        <div className="bg-white dark:bg-gray-900 rounded-xl shadow-sm border border-gray-200 dark:border-gray-800 p-6">
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">
            Cost (7 Days)
          </h2>
          <CostChart data={costData} />
        </div>
      </div>

      {/* Recent Tasks */}
      <div className="bg-white dark:bg-gray-900 rounded-xl shadow-sm border border-gray-200 dark:border-gray-800 p-6">
        <div className="flex justify-between items-center mb-4">
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white">
            Recent Tasks
          </h2>
          <a
            href="/tasks"
            className="text-sm text-primary-600 hover:text-primary-700 dark:text-primary-400"
          >
            View all →
          </a>
        </div>
        <TaskList tasks={tasks.slice(0, 10)} />
      </div>
    </div>
  )
}
