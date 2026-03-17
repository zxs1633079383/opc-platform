'use client'

import { useQuery } from '@tanstack/react-query'
import { RefreshCw, Filter, Download, ChevronDown, ChevronRight, Wrench, Zap } from 'lucide-react'
import { useState, useEffect, useRef, useMemo } from 'react'
import { clsx } from 'clsx'
import { fetchLogs } from '@/lib/api'
import type { LogEntry } from '@/types'
import { useTranslation } from '@/lib/i18n'

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

const levelRowBg: Record<string, string> = {
  debug: '',
  info: '',
  warn: 'bg-yellow-900/10',
  error: 'bg-red-900/10',
}

function ToolSkillCard({ log }: { log: LogEntry }) {
  const [expanded, setExpanded] = useState(false)
  const toolName = log.fields?.toolName as string | undefined
  const skillName = log.fields?.skillName as string | undefined
  const name = toolName || skillName
  if (!name) return null

  const status = (log.fields?.status as string) || 'completed'
  const input = log.fields?.input as string | Record<string, unknown> | undefined
  const output = log.fields?.output as string | Record<string, unknown> | undefined

  return (
    <div className="mt-1 ml-8 border border-gray-700 rounded-lg overflow-hidden">
      <button
        type="button"
        onClick={() => setExpanded(!expanded)}
        className="w-full flex items-center gap-2 px-3 py-2 text-sm bg-gray-800/50 hover:bg-gray-800 transition-colors"
      >
        {expanded ? (
          <ChevronDown className="w-3.5 h-3.5 text-gray-400" />
        ) : (
          <ChevronRight className="w-3.5 h-3.5 text-gray-400" />
        )}
        {toolName ? (
          <Wrench className="w-3.5 h-3.5 text-orange-400" />
        ) : (
          <Zap className="w-3.5 h-3.5 text-purple-400" />
        )}
        <span className="text-gray-200 font-medium">{name}</span>
        <span
          className={clsx(
            'ml-auto px-1.5 py-0.5 rounded text-xs',
            status === 'completed' || status === 'success'
              ? 'bg-green-900/40 text-green-400'
              : status === 'failed' || status === 'error'
                ? 'bg-red-900/40 text-red-400'
                : 'bg-gray-700 text-gray-400'
          )}
        >
          {status}
        </span>
      </button>
      {expanded && (input || output) && (
        <div className="px-3 py-2 space-y-2 text-xs font-mono">
          {input && (
            <div>
              <span className="text-gray-500">Input:</span>
              <pre className="text-gray-300 whitespace-pre-wrap break-all mt-0.5">
                {typeof input === 'string' ? input : JSON.stringify(input, null, 2)}
              </pre>
            </div>
          )}
          {output && (
            <div>
              <span className="text-gray-500">Output:</span>
              <pre className="text-gray-300 whitespace-pre-wrap break-all mt-0.5">
                {typeof output === 'string' ? output : JSON.stringify(output, null, 2)}
              </pre>
            </div>
          )}
        </div>
      )}
    </div>
  )
}

function TokenInfo({ log }: { log: LogEntry }) {
  const tokensIn = log.fields?.tokensIn as number | undefined
  const tokensOut = log.fields?.tokensOut as number | undefined
  const cost = log.fields?.cost as number | undefined
  const duration = log.fields?.duration as string | undefined

  if (!tokensIn && !tokensOut && !cost && !duration) return null

  return (
    <span className="ml-2 text-xs text-gray-500 inline-flex items-center gap-2">
      {tokensIn != null && <span>in:{tokensIn}</span>}
      {tokensOut != null && <span>out:{tokensOut}</span>}
      {cost != null && <span>${cost.toFixed(4)}</span>}
      {duration && <span>{duration}</span>}
    </span>
  )
}

export default function LogsPage() {
  const { t } = useTranslation()
  const [levelFilter, setLevelFilter] = useState<string>('all')
  const [agentFilter, setAgentFilter] = useState<string>('all')
  const [taskFilter, setTaskFilter] = useState<string>('')
  const [autoScroll, setAutoScroll] = useState(true)
  const logContainerRef = useRef<HTMLDivElement>(null)

  const { data: logs = [], refetch } = useQuery({
    queryKey: ['logs'],
    queryFn: () => fetchLogs(100),
    refetchInterval: 5000,
  })

  const agentNames = useMemo(() => {
    const names = new Set<string>()
    for (const log of logs) {
      if (log.agent) names.add(log.agent)
    }
    return Array.from(names).sort()
  }, [logs])

  const taskIds = useMemo(() => {
    const ids = new Set<string>()
    for (const log of logs) {
      if (log.taskId) ids.add(log.taskId)
    }
    return Array.from(ids).sort()
  }, [logs])

  const filteredLogs = useMemo(
    () =>
      logs.filter((log) => {
        if (levelFilter !== 'all' && log.level !== levelFilter) return false
        if (agentFilter !== 'all' && log.agent !== agentFilter) return false
        if (taskFilter && log.taskId !== taskFilter) return false
        return true
      }),
    [logs, levelFilter, agentFilter, taskFilter]
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
            {t('logs.title')}
          </h1>
          <p className="text-gray-500 dark:text-gray-400">
            {t('logs.subtitle')}
          </p>
        </div>
        <div className="flex gap-2">
          <button
            onClick={() => refetch()}
            className="flex items-center gap-2 px-4 py-2 text-sm text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800 rounded-lg transition-colors"
          >
            <RefreshCw className="w-4 h-4" />
            {t('logs.refresh')}
          </button>
          <button className="flex items-center gap-2 px-4 py-2 text-sm text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 transition-colors">
            <Download className="w-4 h-4" />
            {t('logs.export')}
          </button>
        </div>
      </div>

      {/* Filters */}
      <div className="flex items-center gap-4 flex-wrap">
        <div className="flex items-center gap-2">
          <Filter className="w-4 h-4 text-gray-400" />
          <select
            value={levelFilter}
            onChange={(e) => setLevelFilter(e.target.value)}
            className="px-3 py-1.5 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900 text-gray-900 dark:text-white"
          >
            <option value="all">{t('logs.allLevels')}</option>
            <option value="debug">Debug</option>
            <option value="info">Info</option>
            <option value="warn">Warning</option>
            <option value="error">Error</option>
          </select>
        </div>

        <select
          value={agentFilter}
          onChange={(e) => setAgentFilter(e.target.value)}
          className="px-3 py-1.5 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900 text-gray-900 dark:text-white"
        >
          <option value="all">{t('logs.allAgents')}</option>
          {agentNames.map((name) => (
            <option key={name} value={name}>
              {name}
            </option>
          ))}
        </select>

        <select
          value={taskFilter}
          onChange={(e) => setTaskFilter(e.target.value)}
          className="px-3 py-1.5 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900 text-gray-900 dark:text-white"
        >
          <option value="">{t('logs.allTasks')}</option>
          {taskIds.map((id) => (
            <option key={id} value={id}>
              {id}
            </option>
          ))}
        </select>

        <label className="flex items-center gap-2 text-sm text-gray-600 dark:text-gray-400">
          <input
            type="checkbox"
            checked={autoScroll}
            onChange={(e) => setAutoScroll(e.target.checked)}
            className="rounded border-gray-300 text-primary-600 focus:ring-primary-500"
          />
          {t('logs.autoScroll')}
        </label>
        <span className="text-sm text-gray-500 dark:text-gray-400">
          {filteredLogs.length} {t('logs.entries')}
        </span>
      </div>

      {/* Log Output */}
      <div
        ref={logContainerRef}
        className="flex-1 bg-gray-900 rounded-xl p-4 font-mono text-sm overflow-auto"
      >
        {filteredLogs.length === 0 ? (
          <div className="text-gray-500 text-center py-8">
            {t('logs.noLogs')}
          </div>
        ) : (
          <div className="space-y-1">
            {filteredLogs.map((log, index) => (
              <div key={index}>
                <div
                  className={clsx(
                    'flex items-start gap-3 py-1 px-2 rounded',
                    levelRowBg[log.level],
                    'hover:bg-gray-800/50'
                  )}
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
                  <TokenInfo log={log} />
                </div>
                {log.fields && (String(log.fields.toolName || '') || String(log.fields.skillName || '')) ? (
                  <ToolSkillCard log={log} />
                ) : null}
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
