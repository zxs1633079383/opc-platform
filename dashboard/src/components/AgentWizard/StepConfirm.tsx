'use client'

import { Loader2 } from 'lucide-react'
import { YAMLPreview } from './YAMLPreview'

function generateYAML(data: {
  type: string; description: string; model: string;
  fallbackModel: string; preset: string; replicas: number; onExceed: string;
}): string {
  const budgetMap: Record<string, string> = { light: '$5', standard: '$20', power: '$100' }
  const budget = budgetMap[data.preset] || '$20'
  const fallbackLine = data.fallbackModel ? `\n      fallback: ${data.fallbackModel}` : ''
  return `apiVersion: opc/v1
kind: AgentSpec
metadata:
  name: wizard-agent
spec:
  type: ${data.type}
  description: "${data.description}"
  runtime:
    model:
      name: ${data.model}${fallbackLine}
  resources:
    costLimit:
      perDay: "${budget}"
    replicas: ${data.replicas}
    onExceed: ${data.onExceed}
  recovery:
    enabled: true
    maxRestarts: 3`
}

export function StepConfirm({ data, isPending, onSubmit }: {
  data: {
    type: string; description: string; model: string;
    fallbackModel: string; preset: string; replicas: number; onExceed: string;
  }
  isPending: boolean
  onSubmit: () => void
}) {
  const yaml = generateYAML(data)

  return (
    <div className="space-y-5">
      <h2 className="text-lg font-semibold text-gray-900 dark:text-white">Review & Create</h2>

      <div className="bg-gray-50 dark:bg-gray-800 rounded-lg p-4 space-y-2 text-sm">
        <div className="flex justify-between">
          <span className="text-gray-500">Type</span>
          <span className="font-medium text-gray-900 dark:text-white">{data.type}</span>
        </div>
        <div className="flex justify-between">
          <span className="text-gray-500">Model</span>
          <span className="font-medium text-gray-900 dark:text-white">{data.model || 'Not selected'}</span>
        </div>
        {data.fallbackModel && (
          <div className="flex justify-between">
            <span className="text-gray-500">Fallback</span>
            <span className="font-medium text-gray-900 dark:text-white">{data.fallbackModel}</span>
          </div>
        )}
        <div className="flex justify-between">
          <span className="text-gray-500">Preset</span>
          <span className="font-medium text-gray-900 dark:text-white capitalize">{data.preset}</span>
        </div>
        <div className="flex justify-between">
          <span className="text-gray-500">Replicas</span>
          <span className="font-medium text-gray-900 dark:text-white">{data.replicas}</span>
        </div>
        <div className="flex justify-between">
          <span className="text-gray-500">On Exceed</span>
          <span className="font-medium text-gray-900 dark:text-white capitalize">{data.onExceed}</span>
        </div>
      </div>

      {data.description && (
        <div>
          <span className="text-sm text-gray-500">Description</span>
          <p className="text-sm text-gray-900 dark:text-white mt-1">{data.description}</p>
        </div>
      )}

      <YAMLPreview yaml={yaml} />

      <button
        onClick={onSubmit}
        disabled={isPending || !data.model}
        className="w-full flex items-center justify-center gap-2 px-4 py-3 text-sm font-medium text-white bg-primary-600 hover:bg-primary-700 rounded-lg disabled:opacity-50 transition-colors"
      >
        {isPending && <Loader2 className="w-4 h-4 animate-spin" />}
        {isPending ? 'Creating...' : 'Create Agent'}
      </button>
    </div>
  )
}
