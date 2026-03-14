import { clsx } from 'clsx'
import { ReactNode } from 'react'

interface MetricCardProps {
  title: string
  value: string | number
  total?: number
  subtitle?: string
  icon: ReactNode
  color: 'blue' | 'green' | 'yellow' | 'red' | 'purple'
}

const colorClasses = {
  blue: 'bg-blue-50 text-blue-600 dark:bg-blue-900/20 dark:text-blue-400',
  green: 'bg-green-50 text-green-600 dark:bg-green-900/20 dark:text-green-400',
  yellow: 'bg-yellow-50 text-yellow-600 dark:bg-yellow-900/20 dark:text-yellow-400',
  red: 'bg-red-50 text-red-600 dark:bg-red-900/20 dark:text-red-400',
  purple: 'bg-purple-50 text-purple-600 dark:bg-purple-900/20 dark:text-purple-400',
}

export function MetricCard({ title, value, total, subtitle, icon, color }: MetricCardProps) {
  return (
    <div className="bg-white dark:bg-gray-900 rounded-xl shadow-sm border border-gray-200 dark:border-gray-800 p-5">
      <div className="flex justify-between items-start">
        <div>
          <p className="text-sm text-gray-500 dark:text-gray-400">{title}</p>
          <div className="mt-1 flex items-baseline gap-1">
            <span className="text-2xl font-semibold text-gray-900 dark:text-white">
              {value}
            </span>
            {total !== undefined && (
              <span className="text-sm text-gray-400">/ {total}</span>
            )}
          </div>
          {subtitle && (
            <p className="mt-1 text-xs text-gray-400">{subtitle}</p>
          )}
        </div>
        <div className={clsx('p-2 rounded-lg', colorClasses[color])}>
          {icon}
        </div>
      </div>
    </div>
  )
}
