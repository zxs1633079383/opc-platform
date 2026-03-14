'use client'

import { useQuery } from '@tanstack/react-query'
import { Download, Calendar } from 'lucide-react'
import { CostChart } from '@/components/CostChart'
import { MetricCard } from '@/components/MetricCard'
import { fetchCostData, fetchMetrics } from '@/lib/api'

export default function CostsPage() {
  const { data: costData = [] } = useQuery({
    queryKey: ['costData'],
    queryFn: fetchCostData,
  })

  const { data: metrics } = useQuery({
    queryKey: ['metrics'],
    queryFn: fetchMetrics,
  })

  const todayCost = metrics?.todayCost || 0
  const monthCost = metrics?.monthCost || 0
  const dailyBudget = metrics?.dailyBudget || 10
  const monthlyBudget = metrics?.monthlyBudget || 200

  return (
    <div className="p-6 space-y-6">
      <div className="flex justify-between items-center">
        <div>
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white">
            Cost Management
          </h1>
          <p className="text-gray-500 dark:text-gray-400">
            Monitor and control your AI spending
          </p>
        </div>
        <button className="flex items-center gap-2 px-4 py-2 text-sm text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 transition-colors">
          <Download className="w-4 h-4" />
          Export CSV
        </button>
      </div>

      {/* Cost Overview Cards */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        <MetricCard
          title="Today's Cost"
          value={`$${todayCost.toFixed(2)}`}
          subtitle={`Budget: $${dailyBudget}`}
          icon={<Calendar className="w-5 h-5" />}
          color={todayCost > dailyBudget * 0.8 ? 'red' : 'blue'}
        />
        <MetricCard
          title="This Month"
          value={`$${monthCost.toFixed(2)}`}
          subtitle={`Budget: $${monthlyBudget}`}
          icon={<Calendar className="w-5 h-5" />}
          color={monthCost > monthlyBudget * 0.8 ? 'red' : 'green'}
        />
        <MetricCard
          title="Daily Average"
          value={`$${(monthCost / new Date().getDate()).toFixed(2)}`}
          icon={<Calendar className="w-5 h-5" />}
          color="purple"
        />
        <MetricCard
          title="Projected Monthly"
          value={`$${((monthCost / new Date().getDate()) * 30).toFixed(2)}`}
          icon={<Calendar className="w-5 h-5" />}
          color="yellow"
        />
      </div>

      {/* Cost Chart */}
      <div className="bg-white dark:bg-gray-900 rounded-xl shadow-sm border border-gray-200 dark:border-gray-800 p-6">
        <h2 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">
          Daily Cost Trend (Last 7 Days)
        </h2>
        <div className="h-80">
          <CostChart data={costData} />
        </div>
      </div>

      {/* Budget Alerts */}
      <div className="bg-white dark:bg-gray-900 rounded-xl shadow-sm border border-gray-200 dark:border-gray-800 p-6">
        <h2 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">
          Budget Settings
        </h2>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
              Daily Budget
            </label>
            <div className="flex items-center gap-2">
              <span className="text-gray-500">$</span>
              <input
                type="number"
                defaultValue={dailyBudget}
                className="flex-1 px-3 py-2 border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-800 text-gray-900 dark:text-white"
              />
            </div>
            <p className="mt-1 text-sm text-gray-500">
              Alert when 80% reached, pause at 100%
            </p>
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
              Monthly Budget
            </label>
            <div className="flex items-center gap-2">
              <span className="text-gray-500">$</span>
              <input
                type="number"
                defaultValue={monthlyBudget}
                className="flex-1 px-3 py-2 border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-800 text-gray-900 dark:text-white"
              />
            </div>
            <p className="mt-1 text-sm text-gray-500">
              Alert when 80% reached, pause at 100%
            </p>
          </div>
        </div>
        <div className="mt-4">
          <button className="px-4 py-2 text-sm text-white bg-primary-600 hover:bg-primary-700 rounded-lg transition-colors">
            Save Settings
          </button>
        </div>
      </div>
    </div>
  )
}
