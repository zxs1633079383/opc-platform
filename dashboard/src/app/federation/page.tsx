'use client'

import { useState, useMemo } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  fetchCompanies,
  registerCompany,
  unregisterCompany,
  updateCompanyStatus,
  fetchCompanyAgents,
  fetchCompanyTasks,
  fetchCompanyMetrics,
  pingCompany,
  fetchFederatedAgents,
  fetchFederatedMetrics,
  createFederatedGoal,
} from '@/lib/api'
import type { Company, Agent, Metrics } from '@/types'
import type { FederatedAgent, AggregatedMetrics } from '@/lib/api'

const TYPE_COLORS: Record<string, string> = {
  software: 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400',
  operations: 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400',
  sales: 'bg-purple-100 text-purple-700 dark:bg-purple-900/30 dark:text-purple-400',
  custom: 'bg-gray-100 text-gray-700 dark:bg-gray-800 dark:text-gray-400',
}

const STATUS_BAR_COLORS: Record<string, string> = {
  Online: 'bg-green-500',
  Offline: 'bg-gray-400',
  Busy: 'bg-yellow-500',
}

const TYPE_LABELS: Record<string, string> = {
  software: 'Software',
  operations: 'Operations',
  sales: 'Sales',
  custom: 'Custom',
}

type TabKey = 'companies' | 'allAgents' | 'metrics'

function StatusDot({ status }: { status: Company['status'] }) {
  if (status === 'Online') {
    return (
      <span className="relative flex h-3 w-3">
        <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-green-400 opacity-75" />
        <span className="relative inline-flex rounded-full h-3 w-3 bg-green-500" />
      </span>
    )
  }
  if (status === 'Busy') {
    return (
      <span className="relative flex h-3 w-3">
        <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-yellow-400 opacity-75" />
        <span className="relative inline-flex rounded-full h-3 w-3 bg-yellow-500" />
      </span>
    )
  }
  return <span className="inline-flex rounded-full h-3 w-3 bg-gray-400" />
}

function formatRelativeTime(dateStr: string): string {
  const date = new Date(dateStr)
  const now = new Date()
  const diffMs = now.getTime() - date.getTime()
  const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24))

  if (diffDays === 0) return 'today'
  if (diffDays === 1) return '1d ago'
  if (diffDays < 30) return `${diffDays}d ago`
  const diffMonths = Math.floor(diffDays / 30)
  if (diffMonths < 12) return `${diffMonths}mo ago`
  const diffYears = Math.floor(diffMonths / 12)
  return `${diffYears}y ago`
}

function StatusToggle({
  currentStatus,
  onSetStatus,
}: {
  currentStatus: Company['status']
  onSetStatus: (status: string) => void
}) {
  const statuses: Array<{ value: string; label: string; activeClass: string }> = [
    {
      value: 'Online',
      label: 'Online',
      activeClass: 'bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-400 ring-1 ring-green-300 dark:ring-green-700',
    },
    {
      value: 'Offline',
      label: 'Offline',
      activeClass: 'bg-gray-200 text-gray-700 dark:bg-gray-700 dark:text-gray-300 ring-1 ring-gray-300 dark:ring-gray-600',
    },
    {
      value: 'Busy',
      label: 'Busy',
      activeClass: 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/40 dark:text-yellow-400 ring-1 ring-yellow-300 dark:ring-yellow-700',
    },
  ]

  return (
    <div className="flex rounded-lg bg-gray-100 dark:bg-gray-800 p-0.5">
      {statuses.map((s) => (
        <button
          key={s.value}
          onClick={() => onSetStatus(s.value)}
          className={`px-3 py-1 text-xs font-medium rounded-md transition-all ${
            currentStatus === s.value
              ? s.activeClass
              : 'text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300'
          }`}
        >
          {s.label}
        </button>
      ))}
    </div>
  )
}

function CompanyDetailPanel({ company }: { company: Company }) {
  const { data: agents, isLoading: agentsLoading } = useQuery({
    queryKey: ['company-agents', company.id],
    queryFn: () => fetchCompanyAgents(company.id),
    retry: 1,
  })

  const { data: tasks, isLoading: tasksLoading } = useQuery({
    queryKey: ['company-tasks', company.id],
    queryFn: () => fetchCompanyTasks(company.id),
    retry: 1,
  })

  const { data: metrics, isLoading: metricsLoading } = useQuery({
    queryKey: ['company-metrics', company.id],
    queryFn: () => fetchCompanyMetrics(company.id),
    retry: 1,
  })

  const { data: healthData, isLoading: healthLoading } = useQuery({
    queryKey: ['company-health', company.id],
    queryFn: () => pingCompany(company.id),
    retry: 1,
  })

  const taskSummary = useMemo(() => {
    if (!tasks) return { running: 0, completed: 0, failed: 0 }
    return {
      running: tasks.filter((t) => t.status === 'Running').length,
      completed: tasks.filter((t) => t.status === 'Completed').length,
      failed: tasks.filter((t) => t.status === 'Failed').length,
    }
  }, [tasks])

  // Derive the dashboard URL from the company endpoint.
  const dashboardUrl = company.endpoint.replace(/\/api\/?$/, '')

  return (
    <div className="mt-4 pt-4 border-t border-gray-200 dark:border-gray-700 space-y-4">
      {/* Open Dashboard Button */}
      <a
        href={dashboardUrl}
        target="_blank"
        rel="noopener noreferrer"
        className="w-full flex items-center justify-center gap-2 px-4 py-2 text-sm font-medium text-white bg-primary-600 hover:bg-primary-700 rounded-lg transition-colors"
      >
        Open Company Dashboard
      </a>

      {/* Health Status */}
      <div className="flex items-center gap-2">
        {healthLoading ? (
          <span className="w-4 h-4 animate-spin rounded-full border-2 border-gray-300 border-t-primary-500" />
        ) : healthData?.healthy ? (
          <span className="text-green-500">&#10003;</span>
        ) : (
          <span className="text-red-500">&#10007;</span>
        )}
        <span className="text-sm font-medium text-gray-700 dark:text-gray-300">
          {healthLoading
            ? 'Checking...'
            : healthData?.healthy
              ? 'Reachable'
              : 'Unreachable'}
        </span>
      </div>

      {/* Remote Agents */}
      <div>
        <h4 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
          Remote Agents
        </h4>
        {agentsLoading ? (
          <div className="flex items-center gap-2 text-gray-400 text-sm">
            <span className="w-3 h-3 animate-spin rounded-full border-2 border-gray-300 border-t-primary-500" />
            Fetching remote data...
          </div>
        ) : agents && agents.length > 0 ? (
          <div className="space-y-1.5">
            {agents.map((agent) => (
              <div
                key={agent.name}
                className="flex items-center justify-between px-3 py-1.5 bg-gray-50 dark:bg-gray-800/50 rounded-lg text-sm"
              >
                <span className="font-medium text-gray-900 dark:text-white">{agent.name}</span>
                <span
                  className={`px-2 py-0.5 rounded-full text-xs font-medium ${
                    agent.phase === 'Running'
                      ? 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400'
                      : agent.phase === 'Failed'
                        ? 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400'
                        : 'bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-400'
                  }`}
                >
                  {agent.phase}
                </span>
              </div>
            ))}
          </div>
        ) : (
          <p className="text-xs text-gray-400">No remote agents available</p>
        )}
      </div>

      {/* Task Summary */}
      <div>
        <h4 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
          Task Summary
        </h4>
        {tasksLoading ? (
          <div className="flex items-center gap-2 text-gray-400 text-sm">
            <span className="w-3 h-3 animate-spin rounded-full border-2 border-gray-300 border-t-primary-500" />
            Fetching remote data...
          </div>
        ) : (
          <div className="grid grid-cols-3 gap-2 text-center">
            <div className="px-2 py-1.5 bg-blue-50 dark:bg-blue-900/20 rounded-lg">
              <p className="text-lg font-semibold text-blue-600">{taskSummary.running}</p>
              <p className="text-xs text-gray-500">Running</p>
            </div>
            <div className="px-2 py-1.5 bg-green-50 dark:bg-green-900/20 rounded-lg">
              <p className="text-lg font-semibold text-green-600">{taskSummary.completed}</p>
              <p className="text-xs text-gray-500">Completed</p>
            </div>
            <div className="px-2 py-1.5 bg-red-50 dark:bg-red-900/20 rounded-lg">
              <p className="text-lg font-semibold text-red-600">{taskSummary.failed}</p>
              <p className="text-xs text-gray-500">Failed</p>
            </div>
          </div>
        )}
      </div>

      {/* Remote Metrics */}
      <div>
        <h4 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
          Remote Metrics
        </h4>
        {metricsLoading ? (
          <div className="flex items-center gap-2 text-gray-400 text-sm">
            <span className="w-3 h-3 animate-spin rounded-full border-2 border-gray-300 border-t-primary-500" />
            Fetching remote data...
          </div>
        ) : metrics ? (
          <div className="grid grid-cols-2 gap-2 text-sm">
            <div className="flex justify-between px-3 py-1.5 bg-gray-50 dark:bg-gray-800/50 rounded-lg">
              <span className="text-gray-500">Today Cost</span>
              <span className="font-medium text-gray-900 dark:text-white">
                ${metrics.todayCost?.toFixed(2) ?? '0.00'}
              </span>
            </div>
            <div className="flex justify-between px-3 py-1.5 bg-gray-50 dark:bg-gray-800/50 rounded-lg">
              <span className="text-gray-500">Month Cost</span>
              <span className="font-medium text-gray-900 dark:text-white">
                ${metrics.monthCost?.toFixed(2) ?? '0.00'}
              </span>
            </div>
          </div>
        ) : (
          <p className="text-xs text-gray-400">Failed to fetch remote data</p>
        )}
      </div>
    </div>
  )
}

function CompanyCard({
  company,
  onUnregister,
  onSetStatus,
}: {
  company: Company
  onUnregister: (id: string) => void
  onSetStatus: (id: string, status: string) => void
}) {
  const [confirmUnregister, setConfirmUnregister] = useState(false)
  const [expanded, setExpanded] = useState(false)

  return (
    <div className="bg-white dark:bg-gray-900 rounded-xl shadow-sm border border-gray-200 dark:border-gray-800 flex flex-col relative overflow-hidden">
      {/* Health indicator bar */}
      <div className={`h-1 w-full ${STATUS_BAR_COLORS[company.status] || 'bg-gray-400'}`} />

      <div className="p-5 flex flex-col gap-4">
        {/* Header */}
        <div className="flex items-start justify-between">
          <div className="flex items-center gap-3">
            <StatusDot status={company.status} />
            <div>
              <h3 className="text-lg font-semibold text-gray-900 dark:text-white">
                {company.name}
              </h3>
              <div className="flex items-center gap-2 mt-1">
                <span
                  className={`inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium ${TYPE_COLORS[company.type] || TYPE_COLORS.custom}`}
                >
                  {TYPE_LABELS[company.type] || 'Custom'}
                </span>
              </div>
            </div>
          </div>
          <button
            onClick={() => setExpanded(!expanded)}
            className="p-1.5 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-800 transition-colors text-gray-400"
            title="Toggle details"
          >
            {expanded ? '\u25B2' : '\u25BC'}
          </button>
        </div>

        {/* Info section */}
        <div className="space-y-2.5 text-sm">
          <div className="flex items-center gap-2 text-gray-500 dark:text-gray-400">
            <span className="truncate">{company.endpoint}</span>
          </div>
          <div className="flex items-center gap-2 text-gray-500 dark:text-gray-400">
            <span>{company.agents?.length || 0} agents</span>
          </div>
          <div className="flex items-center gap-2 text-gray-500 dark:text-gray-400">
            <span>Joined {formatRelativeTime(company.joinedAt)}</span>
          </div>
        </div>

        {/* Agent tags */}
        {company.agents && company.agents.length > 0 && (
          <div className="flex flex-wrap gap-1">
            {company.agents.map((agent) => (
              <span
                key={agent}
                className="px-2 py-0.5 text-xs rounded-md bg-gray-100 text-gray-600 dark:bg-gray-800 dark:text-gray-400"
              >
                {agent}
              </span>
            ))}
          </div>
        )}

        {/* Expanded detail panel */}
        {expanded && <CompanyDetailPanel company={company} />}

        {/* Actions row */}
        <div className="pt-3 border-t border-gray-100 dark:border-gray-800 space-y-3">
          <StatusToggle
            currentStatus={company.status}
            onSetStatus={(status) => onSetStatus(company.id, status)}
          />

          {confirmUnregister ? (
            <div className="flex items-center gap-2 bg-red-50 dark:bg-red-900/20 rounded-lg p-2">
              <span className="text-xs text-red-600 dark:text-red-400 flex-1">
                Are you sure? This will remove the company.
              </span>
              <button
                onClick={() => onUnregister(company.id)}
                className="px-2 py-1 text-xs font-medium text-white bg-red-600 hover:bg-red-700 rounded transition-colors"
              >
                Confirm
              </button>
              <button
                onClick={() => setConfirmUnregister(false)}
                className="px-2 py-1 text-xs font-medium text-gray-600 dark:text-gray-400 hover:bg-gray-200 dark:hover:bg-gray-700 rounded transition-colors"
              >
                Cancel
              </button>
            </div>
          ) : (
            <button
              onClick={() => setConfirmUnregister(true)}
              className="w-full px-3 py-1.5 text-xs font-medium text-red-600 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-900/20 rounded-lg transition-colors"
            >
              Unregister
            </button>
          )}
        </div>
      </div>
    </div>
  )
}

const COMPANY_TYPES = ['software', 'operations', 'sales', 'custom'] as const

function RegisterModal({
  isOpen,
  onClose,
  onSubmit,
}: {
  isOpen: boolean
  onClose: () => void
  onSubmit: (data: { name: string; endpoint: string; type: string; agents?: string[] }) => void
}) {
  const [name, setName] = useState('')
  const [endpoint, setEndpoint] = useState('')
  const [type, setType] = useState('software')
  const [agentsStr, setAgentsStr] = useState('')

  if (!isOpen) return null

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    const agents = agentsStr
      .split(',')
      .map((s) => s.trim())
      .filter(Boolean)
    onSubmit({ name, endpoint, type, agents: agents.length > 0 ? agents : undefined })
    setName('')
    setEndpoint('')
    setType('software')
    setAgentsStr('')
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
      <div className="bg-white dark:bg-gray-900 rounded-xl shadow-xl border border-gray-200 dark:border-gray-800 w-full max-w-md mx-4">
        <div className="flex items-center justify-between p-6 border-b border-gray-200 dark:border-gray-800">
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white">
            Register Company
          </h2>
          <button
            onClick={onClose}
            className="p-1 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-800 transition-colors text-gray-400"
          >
            X
          </button>
        </div>
        <form onSubmit={handleSubmit} className="p-6 space-y-4">
          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
              Company Name
            </label>
            <input
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              required
              placeholder="e.g. Frontend Team"
              className="w-full px-3 py-2 rounded-lg border border-gray-300 dark:border-gray-700 bg-white dark:bg-gray-800 text-gray-900 dark:text-white focus:ring-2 focus:ring-primary-500 focus:border-transparent text-sm"
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
              Endpoint URL
            </label>
            <input
              type="url"
              value={endpoint}
              onChange={(e) => setEndpoint(e.target.value)}
              required
              placeholder="http://192.168.1.100:9527"
              className="w-full px-3 py-2 rounded-lg border border-gray-300 dark:border-gray-700 bg-white dark:bg-gray-800 text-gray-900 dark:text-white focus:ring-2 focus:ring-primary-500 focus:border-transparent text-sm"
            />
            <p className="mt-1 text-xs text-gray-400">
              The URL of the remote OPC Platform instance
            </p>
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1.5">
              Type
            </label>
            <div className="grid grid-cols-2 gap-2">
              {COMPANY_TYPES.map((t) => (
                <button
                  key={t}
                  type="button"
                  onClick={() => setType(t)}
                  className={`flex items-center gap-2 px-3 py-2.5 rounded-lg border-2 transition-all text-left ${
                    type === t
                      ? 'border-primary-500 bg-primary-50 dark:bg-primary-900/20'
                      : 'border-gray-200 dark:border-gray-700 hover:border-gray-300 dark:hover:border-gray-600'
                  }`}
                >
                  <span
                    className={`text-sm font-medium ${
                      type === t
                        ? 'text-primary-700 dark:text-primary-300'
                        : 'text-gray-700 dark:text-gray-300'
                    }`}
                  >
                    {TYPE_LABELS[t]}
                  </span>
                </button>
              ))}
            </div>
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
              Agents (comma-separated)
            </label>
            <input
              type="text"
              value={agentsStr}
              onChange={(e) => setAgentsStr(e.target.value)}
              placeholder="e.g. coder, reviewer, tester"
              className="w-full px-3 py-2 rounded-lg border border-gray-300 dark:border-gray-700 bg-white dark:bg-gray-800 text-gray-900 dark:text-white focus:ring-2 focus:ring-primary-500 focus:border-transparent text-sm"
            />
          </div>
          <div className="flex justify-end gap-3 pt-2">
            <button
              type="button"
              onClick={onClose}
              className="px-4 py-2 text-sm text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800 rounded-lg transition-colors"
            >
              Cancel
            </button>
            <button
              type="submit"
              className="px-4 py-2 text-sm text-white bg-primary-600 hover:bg-primary-700 rounded-lg transition-colors"
            >
              Register
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}

// ---- Create Federated Goal Modal ----

function CreateGoalModal({
  isOpen,
  onClose,
  companies,
}: {
  isOpen: boolean
  onClose: () => void
  companies: Company[]
}) {
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [selectedCompanies, setSelectedCompanies] = useState<string[]>([])
  const [result, setResult] = useState<{ goalId: string; dispatched: Array<{ companyId: string; companyName: string; status: string; error?: string }> } | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  if (!isOpen) return null

  const toggleCompany = (id: string) => {
    setSelectedCompanies((prev) =>
      prev.includes(id) ? prev.filter((c) => c !== id) : [...prev, id]
    )
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)
    setResult(null)
    setSubmitting(true)

    try {
      const res = await createFederatedGoal({
        name,
        description,
        companies: selectedCompanies,
      })
      setResult(res)
    } catch (err: any) {
      setError(err.message || 'Failed to create federated goal')
    } finally {
      setSubmitting(false)
    }
  }

  const handleClose = () => {
    setName('')
    setDescription('')
    setSelectedCompanies([])
    setResult(null)
    setError(null)
    onClose()
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
      <div className="bg-white dark:bg-gray-900 rounded-xl shadow-xl border border-gray-200 dark:border-gray-800 w-full max-w-lg mx-4 max-h-[90vh] overflow-y-auto">
        <div className="flex items-center justify-between p-6 border-b border-gray-200 dark:border-gray-800">
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white">
            Create Federated Goal
          </h2>
          <button
            onClick={handleClose}
            className="p-1 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-800 transition-colors text-gray-400"
          >
            X
          </button>
        </div>

        {result ? (
          <div className="p-6 space-y-4">
            <div className="bg-green-50 dark:bg-green-900/20 rounded-lg p-4">
              <h3 className="text-sm font-semibold text-green-700 dark:text-green-400 mb-2">
                Goal Created: {result.goalId}
              </h3>
              <div className="space-y-2">
                {result.dispatched.map((d, idx) => (
                  <div key={idx} className="flex items-center justify-between text-sm">
                    <span className="text-gray-700 dark:text-gray-300">
                      {d.companyName || d.companyId}
                    </span>
                    <span
                      className={`px-2 py-0.5 rounded-full text-xs font-medium ${
                        d.status === 'dispatched'
                          ? 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400'
                          : d.status === 'skipped'
                            ? 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-400'
                            : 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400'
                      }`}
                    >
                      {d.status}
                    </span>
                  </div>
                ))}
              </div>
            </div>
            <div className="flex justify-end">
              <button
                onClick={handleClose}
                className="px-4 py-2 text-sm text-white bg-primary-600 hover:bg-primary-700 rounded-lg transition-colors"
              >
                Done
              </button>
            </div>
          </div>
        ) : (
          <form onSubmit={handleSubmit} className="p-6 space-y-4">
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                Goal Name
              </label>
              <input
                type="text"
                value={name}
                onChange={(e) => setName(e.target.value)}
                required
                placeholder="e.g. Build new landing page"
                className="w-full px-3 py-2 rounded-lg border border-gray-300 dark:border-gray-700 bg-white dark:bg-gray-800 text-gray-900 dark:text-white focus:ring-2 focus:ring-primary-500 focus:border-transparent text-sm"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                Description
              </label>
              <textarea
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                rows={3}
                placeholder="Describe the goal in detail..."
                className="w-full px-3 py-2 rounded-lg border border-gray-300 dark:border-gray-700 bg-white dark:bg-gray-800 text-gray-900 dark:text-white focus:ring-2 focus:ring-primary-500 focus:border-transparent text-sm"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                Target Companies (select one or more)
              </label>
              {companies.length === 0 ? (
                <p className="text-xs text-gray-400">No companies registered. Register a company first.</p>
              ) : (
                <div className="space-y-2">
                  {companies.map((c) => (
                    <label
                      key={c.id}
                      className={`flex items-center gap-3 px-3 py-2.5 rounded-lg border-2 cursor-pointer transition-all ${
                        selectedCompanies.includes(c.id)
                          ? 'border-primary-500 bg-primary-50 dark:bg-primary-900/20'
                          : 'border-gray-200 dark:border-gray-700 hover:border-gray-300'
                      }`}
                    >
                      <input
                        type="checkbox"
                        checked={selectedCompanies.includes(c.id)}
                        onChange={() => toggleCompany(c.id)}
                        className="rounded border-gray-300 text-primary-600 focus:ring-primary-500"
                      />
                      <div className="flex-1">
                        <span className="text-sm font-medium text-gray-900 dark:text-white">
                          {c.name}
                        </span>
                        <span className="ml-2 text-xs text-gray-400">{c.status}</span>
                      </div>
                      <span
                        className={`inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium ${TYPE_COLORS[c.type] || TYPE_COLORS.custom}`}
                      >
                        {TYPE_LABELS[c.type] || 'Custom'}
                      </span>
                    </label>
                  ))}
                </div>
              )}
            </div>
            {error && (
              <div className="bg-red-50 dark:bg-red-900/20 rounded-lg p-3">
                <p className="text-sm text-red-600 dark:text-red-400">{error}</p>
              </div>
            )}
            <div className="flex justify-end gap-3 pt-2">
              <button
                type="button"
                onClick={handleClose}
                className="px-4 py-2 text-sm text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800 rounded-lg transition-colors"
              >
                Cancel
              </button>
              <button
                type="submit"
                disabled={submitting || selectedCompanies.length === 0 || !name}
                className="px-4 py-2 text-sm text-white bg-primary-600 hover:bg-primary-700 disabled:opacity-50 disabled:cursor-not-allowed rounded-lg transition-colors"
              >
                {submitting ? 'Sending...' : 'Create Goal'}
              </button>
            </div>
          </form>
        )}
      </div>
    </div>
  )
}

function StatCard({
  label,
  value,
  colorClass,
}: {
  label: string
  value: number | string
  colorClass: string
}) {
  return (
    <div className="bg-white dark:bg-gray-900 rounded-xl shadow-sm border border-gray-200 dark:border-gray-800 p-4">
      <p className="text-sm text-gray-500 dark:text-gray-400">{label}</p>
      <p className={`text-2xl font-semibold ${colorClass}`}>{value}</p>
    </div>
  )
}

// ---- All Agents Tab ----

function AllAgentsTab() {
  const { data: agents = [], isLoading } = useQuery({
    queryKey: ['federated-agents'],
    queryFn: fetchFederatedAgents,
    refetchInterval: 15000,
  })

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-48">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary-500" />
      </div>
    )
  }

  if (agents.length === 0) {
    return (
      <div className="bg-white dark:bg-gray-900 rounded-xl shadow-sm border border-gray-200 dark:border-gray-800 p-12 text-center">
        <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-2">
          No remote agents available
        </h3>
        <p className="text-gray-500 dark:text-gray-400 text-sm">
          Aggregated view of all agents across the federation
        </p>
      </div>
    )
  }

  return (
    <div className="space-y-4">
      <p className="text-sm text-gray-500 dark:text-gray-400">
        Aggregated view of all agents across the federation
      </p>
      <div className="bg-white dark:bg-gray-900 rounded-xl shadow-sm border border-gray-200 dark:border-gray-800 overflow-hidden">
        <table className="min-w-full divide-y divide-gray-200 dark:divide-gray-800">
          <thead className="bg-gray-50 dark:bg-gray-800/50">
            <tr>
              <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Agent</th>
              <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Company</th>
              <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Type</th>
              <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Status</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-200 dark:divide-gray-800">
            {agents.map((agent, idx) => (
              <tr key={`${agent.companyId}-${agent.name}-${idx}`} className="hover:bg-gray-50 dark:hover:bg-gray-800/50">
                <td className="px-4 py-3 text-sm font-medium text-gray-900 dark:text-white">{agent.name}</td>
                <td className="px-4 py-3 text-sm text-gray-500 dark:text-gray-400">
                  <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs bg-blue-50 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400">
                    {agent.company}
                  </span>
                </td>
                <td className="px-4 py-3 text-sm text-gray-500 dark:text-gray-400">{agent.type}</td>
                <td className="px-4 py-3 text-sm">
                  <span
                    className={`px-2 py-0.5 rounded-full text-xs font-medium ${
                      agent.phase === 'Running'
                        ? 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400'
                        : agent.phase === 'Failed'
                          ? 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400'
                          : 'bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-400'
                    }`}
                  >
                    {agent.phase}
                  </span>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}

// ---- Metrics Tab ----

function MetricsTab() {
  const { data: metrics, isLoading } = useQuery({
    queryKey: ['federated-metrics'],
    queryFn: fetchFederatedMetrics,
    refetchInterval: 15000,
  })

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-48">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary-500" />
      </div>
    )
  }

  if (!metrics) return null

  return (
    <div className="space-y-4">
      <p className="text-sm text-gray-500 dark:text-gray-400">
        Combined metrics across all federated companies
      </p>
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        <StatCard label="Companies" value={metrics.companyCount ?? 0} colorClass="text-gray-900 dark:text-white" />
        <StatCard label="Online" value={metrics.onlineCount ?? 0} colorClass="text-green-600" />
        <StatCard label="Active Agents" value={metrics.runningAgents ?? 0} colorClass="text-blue-600" />
        <StatCard label="Running Tasks" value={metrics.runningTasks ?? 0} colorClass="text-purple-600" />
      </div>
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        <StatCard label="Total Agents" value={metrics.totalAgents ?? 0} colorClass="text-gray-900 dark:text-white" />
        <StatCard label="Completed" value={metrics.completedTasks ?? 0} colorClass="text-green-600" />
        <StatCard label="Failed" value={metrics.failedTasks ?? 0} colorClass="text-red-500" />
        <StatCard label="Today Cost" value={`$${(metrics.todayCost ?? 0).toFixed(2)}`} colorClass="text-orange-600" />
      </div>
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        <div className="bg-white dark:bg-gray-900 rounded-xl shadow-sm border border-gray-200 dark:border-gray-800 p-6">
          <h3 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-4">Month Cost</h3>
          <p className="text-3xl font-bold text-gray-900 dark:text-white">
            ${(metrics.monthCost ?? 0).toFixed(2)}
          </p>
        </div>
        <div className="bg-white dark:bg-gray-900 rounded-xl shadow-sm border border-gray-200 dark:border-gray-800 p-6">
          <h3 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-4">Total Tasks</h3>
          <p className="text-3xl font-bold text-gray-900 dark:text-white">
            {metrics.totalTasks ?? 0}
          </p>
        </div>
      </div>
    </div>
  )
}

// ---- Main Page ----

export default function FederationPage() {
  const [isRegisterModalOpen, setIsRegisterModalOpen] = useState(false)
  const [isGoalModalOpen, setIsGoalModalOpen] = useState(false)
  const [activeTab, setActiveTab] = useState<TabKey>('companies')
  const queryClient = useQueryClient()

  const { data: companies = [], isLoading, refetch } = useQuery({
    queryKey: ['companies'],
    queryFn: fetchCompanies,
  })

  const [registerError, setRegisterError] = useState<string | null>(null)
  const registerMutation = useMutation({
    mutationFn: registerCompany,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['companies'] })
      setIsRegisterModalOpen(false)
      setRegisterError(null)
    },
    onError: (err: Error) => {
      setRegisterError(err.message)
    },
  })

  const unregisterMutation = useMutation({
    mutationFn: unregisterCompany,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['companies'] })
    },
  })

  const statusMutation = useMutation({
    mutationFn: ({ id, status }: { id: string; status: string }) =>
      updateCompanyStatus(id, status),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['companies'] })
    },
  })

  const stats = useMemo(() => ({
    total: companies.length,
    online: companies.filter((c) => c.status === 'Online').length,
    offline: companies.filter((c) => c.status === 'Offline').length,
    busy: companies.filter((c) => c.status === 'Busy').length,
  }), [companies])

  const tabs: Array<{ key: TabKey; label: string }> = [
    { key: 'companies', label: 'Companies' },
    { key: 'allAgents', label: 'All Agents' },
    { key: 'metrics', label: 'Metrics' },
  ]

  return (
    <div className="p-6 space-y-6">
      {/* Header */}
      <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4">
        <div>
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white">
            Federation
          </h1>
          <p className="text-gray-500 dark:text-gray-400 mt-1">
            Monitor and manage agent clusters across multiple companies
          </p>
        </div>
        <div className="flex gap-2">
          <button
            onClick={() => setIsGoalModalOpen(true)}
            className="flex items-center gap-2 px-4 py-2 text-sm text-white bg-green-600 hover:bg-green-700 rounded-lg transition-colors"
          >
            Create Goal
          </button>
          <button
            onClick={() => refetch()}
            className="flex items-center gap-2 px-4 py-2 text-sm text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800 rounded-lg transition-colors"
          >
            Refresh
          </button>
          <button
            onClick={() => setIsRegisterModalOpen(true)}
            className="flex items-center gap-2 px-4 py-2 text-sm text-white bg-primary-600 hover:bg-primary-700 rounded-lg transition-colors"
          >
            Register Company
          </button>
        </div>
      </div>

      {/* Info box */}
      <div className="bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-xl p-4">
        <p className="text-sm text-blue-700 dark:text-blue-300">
          Each company is a remote OPC Platform instance. Register their endpoint URL to monitor and manage their agents remotely. Use &quot;Create Goal&quot; to send tasks to multiple companies simultaneously.
        </p>
      </div>

      {/* Tabs */}
      <div className="border-b border-gray-200 dark:border-gray-800">
        <nav className="flex space-x-6">
          {tabs.map((tab) => {
            const isActive = activeTab === tab.key
            return (
              <button
                key={tab.key}
                onClick={() => setActiveTab(tab.key)}
                className={`flex items-center gap-2 py-3 px-1 border-b-2 text-sm font-medium transition-colors ${
                  isActive
                    ? 'border-primary-500 text-primary-600 dark:text-primary-400'
                    : 'border-transparent text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300 hover:border-gray-300'
                }`}
              >
                {tab.label}
              </button>
            )
          })}
        </nav>
      </div>

      {/* Tab Content */}
      {activeTab === 'companies' && (
        <>
          {/* Stats */}
          {companies.length > 0 && (
            <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
              <StatCard label="Total" value={stats.total} colorClass="text-gray-900 dark:text-white" />
              <StatCard label="Online" value={stats.online} colorClass="text-green-600" />
              <StatCard label="Offline" value={stats.offline} colorClass="text-red-500" />
              <StatCard label="Busy" value={stats.busy} colorClass="text-yellow-600" />
            </div>
          )}

          {/* Content */}
          {isLoading ? (
            <div className="flex items-center justify-center h-48">
              <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary-500" />
            </div>
          ) : companies.length === 0 ? (
            <div className="bg-white dark:bg-gray-900 rounded-xl shadow-sm border border-gray-200 dark:border-gray-800 p-16 text-center">
              <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-2">
                No companies registered yet
              </h3>
              <p className="text-gray-500 dark:text-gray-400 mb-6 max-w-sm mx-auto">
                Register your first company to start monitoring and managing agent clusters across your organization.
              </p>
              <button
                onClick={() => setIsRegisterModalOpen(true)}
                className="inline-flex items-center gap-2 px-4 py-2 text-sm text-white bg-primary-600 hover:bg-primary-700 rounded-lg transition-colors"
              >
                Register your first company
              </button>
            </div>
          ) : (
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
              {companies.map((company) => (
                <CompanyCard
                  key={company.id}
                  company={company}
                  onUnregister={(id) => unregisterMutation.mutate(id)}
                  onSetStatus={(id, status) => statusMutation.mutate({ id, status })}
                />
              ))}
            </div>
          )}
        </>
      )}

      {activeTab === 'allAgents' && <AllAgentsTab />}

      {activeTab === 'metrics' && <MetricsTab />}

      {/* Modals */}
      <RegisterModal
        isOpen={isRegisterModalOpen}
        onClose={() => setIsRegisterModalOpen(false)}
        onSubmit={(data) => registerMutation.mutate(data)}
      />

      <CreateGoalModal
        isOpen={isGoalModalOpen}
        onClose={() => setIsGoalModalOpen(false)}
        companies={companies}
      />
    </div>
  )
}
