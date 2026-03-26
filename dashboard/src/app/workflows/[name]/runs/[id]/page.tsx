'use client'

import { useQuery } from '@tanstack/react-query'
import { useParams } from 'next/navigation'
import Link from 'next/link'
import { ArrowLeft, CheckCircle, XCircle, Clock, Loader2 } from 'lucide-react'
import { fetchWorkflowRunSteps } from '@/lib/api'
import { Skeleton } from '@/components/Skeleton'
import { EmptyState } from '@/components/EmptyState'

const statusConfig: Record<string, { icon: typeof CheckCircle; color: string }> = {
  completed: { icon: CheckCircle, color: 'text-green-500' },
  failed: { icon: XCircle, color: 'text-red-500' },
  running: { icon: Loader2, color: 'text-blue-500' },
  pending: { icon: Clock, color: 'text-gray-400' },
}

export default function WorkflowRunDetailPage() {
  const params = useParams()
  const name = params.name as string
  const id = params.id as string

  const { data: steps = [], isLoading } = useQuery({
    queryKey: ['workflow-run-steps', name, id],
    queryFn: () => fetchWorkflowRunSteps(name, id),
  })

  return (
    <div className="p-6 space-y-6">
      <div className="flex items-center gap-3">
        <Link href="/workflows" className="p-1 hover:bg-gray-100 dark:hover:bg-gray-800 rounded">
          <ArrowLeft className="w-5 h-5 text-gray-400" />
        </Link>
        <div>
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white">Run: {id}</h1>
          <p className="text-gray-500 dark:text-gray-400">Workflow: {name}</p>
        </div>
      </div>

      {isLoading ? (
        <div className="space-y-4">
          {[1, 2, 3].map((i) => <Skeleton key={i} className="h-24 w-full" />)}
        </div>
      ) : steps.length === 0 ? (
        <EmptyState message="No steps found for this run" />
      ) : (
        <div className="relative">
          {/* Vertical timeline line */}
          <div className="absolute left-6 top-0 bottom-0 w-0.5 bg-gray-200 dark:bg-gray-700" />

          <div className="space-y-4">
            {steps.map((step, i) => {
              const statusKey = (step.status || 'pending').toLowerCase()
              const config = statusConfig[statusKey] || statusConfig.pending
              const Icon = config.icon

              return (
                <div key={`${step.name}-${i}`} className="relative pl-14">
                  {/* Timeline dot */}
                  <div className="absolute left-4 top-4 w-4 h-4 bg-white dark:bg-gray-950 rounded-full border-2 border-gray-300 dark:border-gray-600 flex items-center justify-center">
                    <div className={`w-2 h-2 rounded-full ${statusKey === 'completed' ? 'bg-green-500' : statusKey === 'failed' ? 'bg-red-500' : statusKey === 'running' ? 'bg-blue-500 animate-pulse' : 'bg-gray-400'}`} />
                  </div>

                  <div className="bg-white dark:bg-gray-900 rounded-xl border border-gray-200 dark:border-gray-800 p-5">
                    <div className="flex items-center justify-between mb-2">
                      <div className="flex items-center gap-2">
                        <Icon className={`w-5 h-5 ${config.color} ${statusKey === 'running' ? 'animate-spin' : ''}`} />
                        <h3 className="font-medium text-gray-900 dark:text-white">{step.name}</h3>
                      </div>
                      <span className={`text-xs font-medium capitalize ${config.color}`}>{step.status}</span>
                    </div>

                    {step.result && (
                      <div className="mt-2 text-sm text-gray-600 dark:text-gray-400 bg-gray-50 dark:bg-gray-800 rounded-lg p-3 max-h-32 overflow-auto">
                        <pre className="whitespace-pre-wrap">{step.result}</pre>
                      </div>
                    )}

                    {step.error && (
                      <div className="mt-2 text-sm text-red-600 dark:text-red-400 bg-red-50 dark:bg-red-900/10 rounded-lg p-3">
                        {step.error}
                      </div>
                    )}
                  </div>
                </div>
              )
            })}
          </div>
        </div>
      )}
    </div>
  )
}
