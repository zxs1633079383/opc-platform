'use client'

import { useQuery } from '@tanstack/react-query'
import { RefreshCw, Filter, Download } from 'lucide-react'
import { useState, useEffect, useRef } from 'react'
import { clsx } from 'clsx'
import { fetchLogs } from '@/lib/api'
import type { LogEntry } from '@/types'

// Mock log data for development
const mockLogs: LogEntry[] = [
  {
    timestamp: new Date().toISOString(),
    level: 'info',
    message: 'Agent coder-main started successfully',
    agent: 'coder-main',
  },
  {
    timestamp: new Date(Date.now() - 1000 * 5).toISOString(),
    level: 'info',
    message: 'Task task-001 dispatched to agent coder-main',
    taskId: 'task-001',
    agent: 'coder-main',
  },
  {
    timestamp: new Date(Date.now() - 1000 * 10).toISOString(),
    level: 'debug',
    message: 'Health check passed',
    agent: 'code-reviewer',
  },
  {
    timestamp: new Date(Date.now() - 1000 * 30).toISOString(),
    level: 'warn',
    message: 'Token budget 80% reached for agent coder-main',
    agent: 'coder-main',
  },
  {
    timestamp: new Date(Date.now() - 1000 * 60).toISOString(),
    level: 'error',
    message: 'Task task-004 failed: timeout after 5 minutes',
    taskId: 'task-004',
    agent: 'task-runner',
  },
  {
    timestamp: new Date(Date.now() - 1000 * 120).toISOString(),
    level: 'info',
    message: 'Workflow daily-report completed successfully',
  },
]

const levelColors: Record<string, string> = {
  debug: 'text-gray-400',
  info: 'text-blue-500',
  warn: 'text-yellow-500',
  error: 'text-red-500',
}

const levelBadgeColors: Record<string, string> = {
  debug: 'bg-gray-100 text-gray-600 dark:bg-gray-800 dark:text-gray-400',
  info: 'bg-blue-100 text-blue-600 dark:bg-blue-900/30 dark:text-blue-400',
  warn: 'bg-yellow-100 text-yellow-600 dark:bg-yellow-900/30 dark:text-yellow-400',
  error: 'bg-red-100 text-red-600 dark:bg-red-900/30 dark:text-red-400',
}

export default function LogsPage() {
  const [levelFilter, setLevelFilter] = useState<string>('all')
  const [autoScroll, setAutoScroll] = useState(true)
  const logContainerRef = useRef<HTMLDivElement>(null)

  const { data: logs = mockLogs, refetch } = useQuery({
    queryKey: ['logs'],
    queryFn: () => fetchLogs(100),
    refetchInterval: 5000, // Refresh every 5 seconds
  })

  const filteredLogs = logs.filter(
    (log) => levelFilter === 'all' || log.level === levelFilter
  )

  useEffect(() => {
    if (autoScroll && logContainerRef.current) {
      logContainerRef.current.scrollTop = 0
    }
  }, [logs, autoScroll])

  return (
    <div className="p-6 space-y-6 h-full flex flex-col">
      <div className="flex justify-between items-center">
        <div>
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white">
            Logs
          </h1>
          <p className="text-gray-500 dark:text-gray-400">
            Real-time system logs
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
          <button className="flex items-center gap-2 px-4 py-2 text-sm text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 transition-colors">
            <Download className="w-4 h-4" />
            Export
          </button>
        </div>
      </div>

      {/* Filters */}
      <div className="flex items-center gap-4">
        <div className="flex items-center gap-2">
          <Filter className="w-4 h-4 text-gray-400" />
          <select
            value={levelFilter}
            onChange={(e) => setLevelFilter(e.target.value)}
            className="px-3 py-1.5 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900 text-gray-900 dark:text-white"
          >
            <option value="all">All Levels</option>
            <option value="debug">Debug</option>
            <option value="info">Info</option>
            <option value="warn">Warning</option>
            <option value="error">Error</option>
          </select>
        </div>
        <label className="flex items-center gap-2 text-sm text-gray-600 dark:text-gray-400">
          <input
            type="checkbox"
            checked={autoScroll}
            onChange={(e) => setAutoScroll(e.target.checked)}
            className="rounded border-gray-300 text-primary-600 focus:ring-primary-500"
          />
          Auto-scroll
        </label>
        <span className="text-sm text-gray-500 dark:text-gray-400">
          {filteredLogs.length} entries
        </span>
      </div>

      {/* Log Output */}
      <div
        ref={logContainerRef}
        className="flex-1 bg-gray-900 rounded-xl p-4 font-mono text-sm overflow-auto"
      >
        {filteredLogs.length === 0 ? (
          <div className="text-gray-500 text-center py-8">
            No logs to display
          </div>
        ) : (
          <div className="space-y-1">
            {filteredLogs.map((log, index) => (
              <div
                key={index}
                className="flex items-start gap-3 py-1 hover:bg-gray-800/50 px-2 rounded"
              >
                <span className="text-gray-500 shrink-0">
                  {new Date(log.timestamp).toLocaleTimeString()}
                </span>
                <span
                  className={clsx(
                    'px-1.5 py-0.5 rounded text-xs font-medium uppercase shrink-0',
                    levelBadgeColors[log.level]
                  )}
                >
                  {log.level}
                </span>
                {log.agent && (
                  <span className="text-purple-400 shrink-0">[{log.agent}]</span>
                )}
                {log.taskId && (
                  <span className="text-cyan-400 shrink-0">[{log.taskId}]</span>
                )}
                <span className={clsx('text-gray-300', levelColors[log.level])}>
                  {log.message}
                </span>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
