'use client'

import { useQuery } from '@tanstack/react-query'
import { Plus, Play, Pause, Clock } from 'lucide-react'
import { fetchWorkflows } from '@/lib/api'

export default function WorkflowsPage() {
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
              <div className="mt-4 flex gap-2">
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
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
