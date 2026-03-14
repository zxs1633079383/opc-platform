import { clsx } from 'clsx'
import { Activity, Clock, Cpu, DollarSign } from 'lucide-react'
import type { Agent } from '@/types'

interface AgentStatusCardProps {
  agent: Agent
}

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

export function AgentStatusCard({ agent }: AgentStatusCardProps) {
  const metrics: Agent['metrics'] = agent.metrics || {
    tasksCompleted: 0,
    tasksFailed: 0,
    tasksRunning: 0,
    totalTokensIn: 0,
    totalTokensOut: 0,
    totalCost: 0,
    uptimeSeconds: 0,
  }

  return (
    <div className="border border-gray-200 dark:border-gray-700 rounded-lg p-4 hover:shadow-md transition-shadow">
      <div className="flex items-start justify-between mb-3">
        <div className="flex items-center gap-2">
          <span
            className={clsx(
              'w-2 h-2 rounded-full',
              phaseIndicators[agent.phase] || 'bg-gray-400'
            )}
          />
          <h3 className="font-medium text-gray-900 dark:text-white">
            {agent.name}
          </h3>
        </div>
        <span
          className={clsx(
            'px-2 py-0.5 rounded-full text-xs font-medium',
            phaseColors[agent.phase] || phaseColors.Stopped
          )}
        >
          {agent.phase}
        </span>
      </div>

      <p className="text-sm text-gray-500 dark:text-gray-400 mb-3 capitalize">
        {agent.type}
      </p>

      <div className="grid grid-cols-2 gap-3 text-sm">
        <div className="flex items-center gap-1.5 text-gray-600 dark:text-gray-300">
          <Activity className="w-4 h-4 text-gray-400" />
          <span>{metrics.tasksRunning || 0} running</span>
        </div>
        <div className="flex items-center gap-1.5 text-gray-600 dark:text-gray-300">
          <Cpu className="w-4 h-4 text-gray-400" />
          <span>{metrics.tasksCompleted || 0} done</span>
        </div>
        <div className="flex items-center gap-1.5 text-gray-600 dark:text-gray-300">
          <DollarSign className="w-4 h-4 text-gray-400" />
          <span>${(metrics.totalCost || 0).toFixed(2)}</span>
        </div>
        <div className="flex items-center gap-1.5 text-gray-600 dark:text-gray-300">
          <Clock className="w-4 h-4 text-gray-400" />
          <span>{formatUptime(metrics.uptimeSeconds || 0)}</span>
        </div>
      </div>
    </div>
  )
}

function formatUptime(seconds: number): string {
  if (seconds < 60) return `${seconds}s`
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m`
  if (seconds < 86400) return `${Math.floor(seconds / 3600)}h`
  return `${Math.floor(seconds / 86400)}d`
}
