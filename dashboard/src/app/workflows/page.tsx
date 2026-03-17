'use client'

import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Plus, Play, Pause, Clock, ChevronDown, ChevronRight, CheckCircle, XCircle, Loader2 } from 'lucide-react'
import { fetchWorkflows, fetchWorkflowRuns } from '@/lib/api'
import type { WorkflowRun, WorkflowStepResult } from '@/types'

function RunSteps({ stepsJson }: { stepsJson: string }) {
  let steps: WorkflowStepResult[] = []
  try {
    steps = JSON.parse(stepsJson || '[]')
  } catch {
    return <span className="text-xs text-gray-400">Invalid steps data</span>
  }

  if (steps.length === 0) {
    return <span className="text-xs text-gray-400">No steps</span>
  }

  return (
    <div className="mt-2 space-y-1">
      {steps.map((step) => (
        <div key={step.name} className="flex items-center gap-2 text-sm">
          {step.status === 'Completed' ? (
            <CheckCircle className="w-4 h-4 text-green-500" />
          ) : step.status === 'Failed' ? (
            <XCircle className="w-4 h-4 text-red-500" />
          ) : step.status === 'Running' ? (
            <Loader2 className="w-4 h-4 text-blue-500 animate-spin" />
          ) : (
            <div className="w-4 h-4 rounded-full border border-gray-300" />
          )}
          <span className="text-gray-700 dark:text-gray-300">{step.name}</span>
          <span className="text-xs text-gray-400">{step.status}</span>
          {step.error && (
            <span className="text-xs text-red-400 ml-2">{step.error}</span>
          )}
        </div>
      ))}
    </div>
  )
}

function WorkflowRunHistory({ workflowName }: { workflowName: string }) {
  const [expandedRun, setExpandedRun] = useState<string | null>(null)

  const { data: runs = [], isLoading } = useQuery({
    queryKey: ['workflowRuns', workflowName],
    queryFn: () => fetchWorkflowRuns(workflowName),
  })

  if (isLoading) {
    return (
      <div className="flex items-center gap-2 text-sm text-gray-400 mt-3">
        <Loader2 className="w-4 h-4 animate-spin" />
        Loading runs...
      </div>
    )
  }

  if (runs.length === 0) {
    return (
      <p className="text-sm text-gray-400 mt-3">No run history yet</p>
    )
  }

  return (
    <div className="mt-3 space-y-2">
      <h4 className="text-sm font-medium text-gray-600 dark:text-gray-400">Run History</h4>
      {runs.map((run: WorkflowRun) => (
        <div
          key={run.id}
          className="border border-gray-100 dark:border-gray-700 rounded-lg p-3"
        >
          <button
            onClick={() => setExpandedRun(expandedRun === run.id ? null : run.id)}
            className="flex items-center gap-2 w-full text-left"
          >
            {expandedRun === run.id ? (
              <ChevronDown className="w-4 h-4 text-gray-400" />
            ) : (
              <ChevronRight className="w-4 h-4 text-gray-400" />
            )}
            <span className="text-sm font-mono text-gray-600 dark:text-gray-300">
              {run.id.slice(0, 12)}
            </span>
            <span className={`text-xs px-2 py-0.5 rounded-full ${
              run.status === 'Completed' ? 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400' :
              run.status === 'Failed' ? 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400' :
              run.status === 'Running' ? 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400' :
              'bg-gray-100 text-gray-600 dark:bg-gray-800 dark:text-gray-400'
            }`}>
              {run.status}
            </span>
            <span className="text-xs text-gray-400 ml-auto">
              {new Date(run.startedAt).toLocaleString()}
            </span>
          </button>
          {expandedRun === run.id && (
            <RunSteps stepsJson={run.stepsJson ?? (typeof run.steps === 'string' ? run.steps : JSON.stringify(run.steps ?? []))} />
          )}
        </div>
      ))}
    </div>
  )
}

export default function WorkflowsPage() {
  const [expandedWorkflow, setExpandedWorkflow] = useState<string | null>(null)

  const { data: workflows = [], isLoading } = useQuery({
    queryKey: ['workflows'],
    queryFn: fetchWorkflows,
  })

  return (
    <div className="p-6 space-y-6">
      <div className="flex justify-between items-center">
        <div>
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white">
            Workflows
          </h1>
          <p className="text-gray-500 dark:text-gray-400">
            Manage automated workflows and schedules
          </p>
        </div>
        <button className="flex items-center gap-2 px-4 py-2 text-sm text-white bg-primary-600 hover:bg-primary-700 rounded-lg transition-colors">
          <Plus className="w-4 h-4" />
          Create Workflow
        </button>
      </div>

      {isLoading ? (
        <div className="flex items-center justify-center h-48">
          <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary-500" />
        </div>
      ) : workflows.length === 0 ? (
        <div className="bg-white dark:bg-gray-900 rounded-xl shadow-sm border border-gray-200 dark:border-gray-800 p-12">
          <div className="max-w-md mx-auto text-center">
            <div className="w-16 h-16 bg-gray-100 dark:bg-gray-800 rounded-full flex items-center justify-center mx-auto mb-4">
              <Clock className="w-8 h-8 text-gray-400" />
            </div>
            <h3 className="text-lg font-medium text-gray-900 dark:text-white mb-2">
              No workflows yet
            </h3>
            <p className="text-gray-500 dark:text-gray-400 mb-6">
              Create your first workflow to automate multi-step agent tasks
            </p>
            <button className="inline-flex items-center gap-2 px-4 py-2 text-sm text-white bg-primary-600 hover:bg-primary-700 rounded-lg transition-colors">
              <Plus className="w-4 h-4" />
              Create Workflow
            </button>
          </div>
        </div>
      ) : (
        <div className="grid gap-4">
          {workflows.map((workflow) => (
            <div
              key={workflow.name}
              className="bg-white dark:bg-gray-900 rounded-xl shadow-sm border border-gray-200 dark:border-gray-800 p-6"
            >
              <div className="flex items-start justify-between">
                <div>
                  <button
                    onClick={() => setExpandedWorkflow(expandedWorkflow === workflow.name ? null : workflow.name)}
                    className="flex items-center gap-2"
                  >
                    {expandedWorkflow === workflow.name ? (
                      <ChevronDown className="w-5 h-5 text-gray-400" />
                    ) : (
                      <ChevronRight className="w-5 h-5 text-gray-400" />
                    )}
                    <h3 className="text-lg font-medium text-gray-900 dark:text-white">
                      {workflow.name}
                    </h3>
                  </button>
                  {workflow.schedule && (
                    <p className="text-sm text-gray-500 dark:text-gray-400 mt-1 ml-7">
                      <Clock className="w-4 h-4 inline mr-1" />
                      {workflow.schedule}
                    </p>
                  )}
                </div>
                <div className="flex gap-2">
                  {workflow.enabled ? (
                    <button className="p-2 text-yellow-600 hover:bg-yellow-50 dark:hover:bg-yellow-900/20 rounded-lg">
                      <Pause className="w-5 h-5" />
                    </button>
                  ) : (
                    <button className="p-2 text-green-600 hover:bg-green-50 dark:hover:bg-green-900/20 rounded-lg">
                      <Play className="w-5 h-5" />
                    </button>
                  )}
                </div>
              </div>
              <div className="mt-4 flex gap-2 ml-7">
                {workflow.steps.map((step, i) => (
                  <div
                    key={step.name}
                    className="flex items-center gap-2"
                  >
                    {i > 0 && <span className="text-gray-300">→</span>}
                    <span className="px-2 py-1 bg-gray-100 dark:bg-gray-800 rounded text-sm">
                      {step.name}
                    </span>
                  </div>
                ))}
              </div>
              {expandedWorkflow === workflow.name && (
                <div className="ml-7">
                  <WorkflowRunHistory workflowName={workflow.name} />
                </div>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
