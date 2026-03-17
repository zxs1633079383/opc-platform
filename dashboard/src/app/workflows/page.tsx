'use client'

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Plus, Play, Pause, Clock, Workflow, X, Trash2, AlertTriangle } from 'lucide-react'
import { useState } from 'react'
import { fetchWorkflows, runWorkflow, deleteWorkflow } from '@/lib/api'

export default function WorkflowsPage() {
  const queryClient = useQueryClient()
  const [showCreateModal, setShowCreateModal] = useState(false)
  const [actionError, setActionError] = useState<string | null>(null)

  const { data: workflows = [], isLoading } = useQuery({
    queryKey: ['workflows'],
    queryFn: fetchWorkflows,
  })

  const runMutation = useMutation({
    mutationFn: runWorkflow,
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['workflows'] }); setActionError(null) },
    onError: (err: Error) => setActionError(err.message),
  })

  const deleteMutation = useMutation({
    mutationFn: deleteWorkflow,
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['workflows'] }); setActionError(null) },
    onError: (err: Error) => setActionError(err.message),
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
        <button
          onClick={() => setShowCreateModal(true)}
          className="flex items-center gap-2 px-4 py-2 text-sm text-white bg-primary-600 hover:bg-primary-700 rounded-lg transition-colors"
        >
          <Plus className="w-4 h-4" />
          Create Workflow
        </button>
      </div>

      {/* Error Banner */}
      {actionError && (
        <div className="flex items-center gap-3 p-3 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg">
          <AlertTriangle className="w-5 h-5 text-red-600 dark:text-red-400 flex-shrink-0" />
          <p className="text-sm text-red-700 dark:text-red-300 flex-1">{actionError}</p>
          <button onClick={() => setActionError(null)} className="text-red-500 hover:text-red-700">
            <X className="w-4 h-4" />
          </button>
        </div>
      )}

      {isLoading ? (
        <div className="grid gap-4">
          {[1, 2].map((i) => (
            <div key={i} className="animate-pulse bg-white dark:bg-gray-900 rounded-xl border border-gray-200 dark:border-gray-800 p-6">
              <div className="h-5 bg-gray-200 dark:bg-gray-700 rounded w-1/4 mb-3" />
              <div className="h-3 bg-gray-200 dark:bg-gray-700 rounded w-1/6 mb-4" />
              <div className="flex gap-3">
                <div className="h-7 bg-gray-200 dark:bg-gray-700 rounded w-20" />
                <div className="h-7 bg-gray-200 dark:bg-gray-700 rounded w-20" />
                <div className="h-7 bg-gray-200 dark:bg-gray-700 rounded w-20" />
              </div>
            </div>
          ))}
        </div>
      ) : workflows.length === 0 ? (
        <div className="bg-white dark:bg-gray-900 rounded-xl shadow-sm border border-gray-200 dark:border-gray-800 p-12">
          <div className="max-w-md mx-auto text-center">
            <Workflow className="w-16 h-16 text-gray-300 dark:text-gray-600 mx-auto mb-4" />
            <h3 className="text-lg font-medium text-gray-900 dark:text-white mb-2">
              No workflows yet
            </h3>
            <p className="text-gray-500 dark:text-gray-400 mb-6">
              Create your first workflow to automate multi-step agent tasks
            </p>
            <button
              onClick={() => setShowCreateModal(true)}
              className="inline-flex items-center gap-2 px-4 py-2 text-sm text-white bg-primary-600 hover:bg-primary-700 rounded-lg transition-colors"
            >
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
                  <h3 className="text-lg font-medium text-gray-900 dark:text-white">
                    {workflow.name}
                  </h3>
                  {workflow.schedule && (
                    <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">
                      <Clock className="w-4 h-4 inline mr-1" />
                      {workflow.schedule}
                    </p>
                  )}
                </div>
                <div className="flex gap-2">
                  <button
                    onClick={() => runMutation.mutate(workflow.name)}
                    disabled={runMutation.isPending}
                    className="p-2 text-green-600 hover:bg-green-50 dark:hover:bg-green-900/20 rounded-lg transition-colors disabled:opacity-50"
                    title="Run workflow"
                  >
                    <Play className="w-5 h-5" />
                  </button>
                  <button
                    onClick={() => {
                      if (confirm(`Delete workflow "${workflow.name}"?`)) {
                        deleteMutation.mutate(workflow.name)
                      }
                    }}
                    disabled={deleteMutation.isPending}
                    className="p-2 text-red-600 hover:bg-red-50 dark:hover:bg-red-900/20 rounded-lg transition-colors disabled:opacity-50"
                    title="Delete workflow"
                  >
                    <Trash2 className="w-5 h-5" />
                  </button>
                </div>
              </div>
              <div className="mt-4 flex flex-wrap gap-2">
                {(workflow.steps || []).map((step, i) => (
                  <div
                    key={step.name}
                    className="flex items-center gap-2"
                  >
                    {i > 0 && <span className="text-gray-300 dark:text-gray-600">→</span>}
                    <span className="px-2 py-1 bg-gray-100 dark:bg-gray-800 rounded text-sm text-gray-700 dark:text-gray-300">
                      {step.name}
                    </span>
                  </div>
                ))}
                {(!workflow.steps || workflow.steps.length === 0) && (
                  <span className="text-sm text-gray-400 dark:text-gray-500 italic">No steps defined</span>
                )}
              </div>
            </div>
          ))}
        </div>
      )}

      {/* Create Workflow Modal */}
      {showCreateModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center">
          <div className="fixed inset-0 bg-black/50" onClick={() => setShowCreateModal(false)} />
          <div className="relative bg-white dark:bg-gray-900 rounded-xl shadow-xl border border-gray-200 dark:border-gray-800 p-6 w-full max-w-lg mx-4">
            <div className="flex items-center justify-between mb-6">
              <h2 className="text-lg font-semibold text-gray-900 dark:text-white">Create Workflow</h2>
              <button onClick={() => setShowCreateModal(false)} className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300">
                <X className="w-5 h-5" />
              </button>
            </div>
            <div className="space-y-4">
              <p className="text-sm text-gray-500 dark:text-gray-400">
                Workflows are defined via YAML configuration. Create a workflow YAML file and apply it using the CLI:
              </p>
              <pre className="bg-gray-50 dark:bg-gray-800 rounded-lg p-4 text-sm text-gray-700 dark:text-gray-300 overflow-x-auto">
{`# workflow.yaml
apiVersion: opc/v1
kind: Workflow
metadata:
  name: my-workflow
spec:
  schedule: "0 9 * * *"
  steps:
    - name: step-1
      agent: my-agent
      message: "Do something"
    - name: step-2
      agent: my-agent
      message: "Do another thing"
      dependsOn:
        - step-1`}
              </pre>
              <pre className="bg-gray-50 dark:bg-gray-800 rounded-lg p-3 text-sm font-mono text-gray-600 dark:text-gray-400">
                opctl apply -f workflow.yaml
              </pre>
            </div>
            <div className="flex justify-end mt-6">
              <button
                onClick={() => setShowCreateModal(false)}
                className="px-4 py-2 text-sm text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800 rounded-lg transition-colors"
              >
                Close
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
