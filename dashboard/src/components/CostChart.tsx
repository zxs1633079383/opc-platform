'use client'

import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
} from 'recharts'
import type { CostDataPoint } from '@/types'

interface CostChartProps {
  data: CostDataPoint[]
}

export function CostChart({ data }: CostChartProps) {
  if (data.length === 0) {
    return (
      <div className="flex items-center justify-center h-48 text-gray-500 dark:text-gray-400">
        No cost data available
      </div>
    )
  }

  return (
    <div className="h-48">
      <ResponsiveContainer width="100%" height="100%">
        <LineChart data={data}>
          <CartesianGrid strokeDasharray="3 3" stroke="#374151" opacity={0.2} />
          <XAxis
            dataKey="date"
            tick={{ fontSize: 11 }}
            stroke="#6B7280"
            tickFormatter={(value) => value.slice(5)} // MM-DD
          />
          <YAxis
            tick={{ fontSize: 11 }}
            stroke="#6B7280"
            tickFormatter={(value) => `$${value}`}
          />
          <Tooltip
            contentStyle={{
              backgroundColor: '#1F2937',
              border: 'none',
              borderRadius: '8px',
              color: '#F9FAFB',
            }}
            formatter={(value: number) => [`$${value.toFixed(2)}`, 'Cost']}
          />
          <Line
            type="monotone"
            dataKey="cost"
            stroke="#0EA5E9"
            strokeWidth={2}
            dot={{ fill: '#0EA5E9', r: 4 }}
            activeDot={{ r: 6 }}
          />
        </LineChart>
      </ResponsiveContainer>
    </div>
  )
}
