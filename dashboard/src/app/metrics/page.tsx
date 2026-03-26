'use client'

import { useQuery } from '@tanstack/react-query'
import { Activity, Clock, DollarSign, RotateCw } from 'lucide-react'
import { LineChart, Line, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts'
import { fetchSystemMetrics } from '@/lib/api'
import { MetricCard } from '@/components/MetricCard'
import { Skeleton } from '@/components/Skeleton'
import { EmptyState } from '@/components/EmptyState'

export default function MetricsPage() {
  const { data: metrics = [], isLoading } = useQuery({
    queryKey: ['system-metrics'],
    queryFn: fetchSystemMetrics,
    refetchInterval: 10000,
  })

  const latest = metrics.length > 0 ? metrics[metrics.length - 1] : null

  if (isLoading) {
    return (
      <div className="p-6 space-y-6">
        <div>
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white">System Metrics</h1>
          <p className="text-gray-500 dark:text-gray-400">Platform health and performance overview</p>
        </div>
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
          {[1, 2, 3, 4].map((i) => <Skeleton key={i} className="h-28 w-full" />)}
        </div>
        <Skeleton className="h-80 w-full" />
      </div>
    )
  }

  if (!latest) {
    return (
      <div className="p-6 space-y-6">
        <div>
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white">System Metrics</h1>
          <p className="text-gray-500 dark:text-gray-400">Platform health and performance overview</p>
        </div>
        <EmptyState message="No metrics data available yet" />
      </div>
    )
  }

  return (
    <div className="p-6 space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-gray-900 dark:text-white">System Metrics</h1>
        <p className="text-gray-500 dark:text-gray-400">Platform health and performance overview</p>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        <MetricCard
          title="Success Rate"
          value={`${(latest.successRate * 100).toFixed(1)}%`}
          icon={<Activity className="w-5 h-5" />}
          color={latest.successRate > 0.9 ? 'green' : latest.successRate > 0.7 ? 'yellow' : 'red'}
        />
        <MetricCard
          title="Avg Latency"
          value={`${latest.avgLatency.toFixed(0)}ms`}
          icon={<Clock className="w-5 h-5" />}
          color={latest.avgLatency < 1000 ? 'blue' : 'yellow'}
        />
        <MetricCard
          title="Cost per Goal"
          value={`$${latest.costPerGoal.toFixed(2)}`}
          icon={<DollarSign className="w-5 h-5" />}
          color="purple"
        />
        <MetricCard
          title="Retry Rate"
          value={`${(latest.retryRate * 100).toFixed(1)}%`}
          icon={<RotateCw className="w-5 h-5" />}
          color={latest.retryRate < 0.1 ? 'green' : 'red'}
        />
      </div>

      {/* Time series chart */}
      {metrics.length > 1 && (
        <div className="bg-white dark:bg-gray-900 rounded-xl shadow-sm border border-gray-200 dark:border-gray-800 p-6">
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">Success Rate Over Time</h2>
          <div className="h-80">
            <ResponsiveContainer width="100%" height="100%">
              <LineChart data={metrics.map((m) => ({
                time: new Date(m.timestamp).toLocaleTimeString(),
                successRate: +(m.successRate * 100).toFixed(1),
                retryRate: +(m.retryRate * 100).toFixed(1),
              }))}>
                <XAxis dataKey="time" tick={{ fontSize: 12 }} />
                <YAxis tick={{ fontSize: 12 }} />
                <Tooltip />
                <Line type="monotone" dataKey="successRate" stroke="#22c55e" strokeWidth={2} name="Success %" dot={false} />
                <Line type="monotone" dataKey="retryRate" stroke="#ef4444" strokeWidth={2} name="Retry %" dot={false} />
              </LineChart>
            </ResponsiveContainer>
          </div>
        </div>
      )}

      {/* Error patterns */}
      {latest.errorPatterns && latest.errorPatterns.length > 0 && (
        <div className="bg-white dark:bg-gray-900 rounded-xl shadow-sm border border-gray-200 dark:border-gray-800 p-6">
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white mb-3">Error Patterns</h2>
          <ul className="space-y-2">
            {latest.errorPatterns.map((pattern, i) => (
              <li key={i} className="text-sm text-red-600 dark:text-red-400 bg-red-50 dark:bg-red-900/10 px-3 py-2 rounded-lg">
                {pattern}
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  )
}
