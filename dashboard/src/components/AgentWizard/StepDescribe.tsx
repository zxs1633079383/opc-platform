'use client'

import { useQuery } from '@tanstack/react-query'
import { fetchModels } from '@/lib/api'
import type { ModelInfo } from '@/types'

export function StepDescribe({ description, model, fallbackModel, onDescriptionChange, onModelChange, onFallbackChange }: {
  description: string
  model: string
  fallbackModel: string
  onDescriptionChange: (v: string) => void
  onModelChange: (v: string) => void
  onFallbackChange: (v: string) => void
}) {
  const { data: models = [] } = useQuery({
    queryKey: ['models'],
    queryFn: fetchModels,
  })

  const grouped = models.reduce<Record<string, ModelInfo[]>>((acc, m) => {
    const key = m.provider
    if (!acc[key]) acc[key] = []
    acc[key] = [...acc[key], m]
    return acc
  }, {})

  return (
    <div className="space-y-5">
      <h2 className="text-lg font-semibold text-gray-900 dark:text-white">Describe Your Agent</h2>

      <div>
        <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Description</label>
        <textarea
          value={description}
          onChange={(e) => onDescriptionChange(e.target.value)}
          placeholder="What should this agent do?"
          rows={4}
          className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-800 text-gray-900 dark:text-white text-sm focus:ring-2 focus:ring-primary-500 focus:border-transparent"
        />
      </div>

      <div>
        <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Model</label>
        <select
          value={model}
          onChange={(e) => onModelChange(e.target.value)}
          className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-800 text-gray-900 dark:text-white text-sm"
        >
          <option value="">Select a model</option>
          {Object.entries(grouped).map(([provider, providerModels]) => (
            <optgroup key={provider} label={provider}>
              {providerModels.map((m) => (
                <option key={m.id} value={m.id}>
                  {m.displayName} ({m.tier}) - ${m.costPer1k}/1k
                </option>
              ))}
            </optgroup>
          ))}
        </select>
      </div>

      <div>
        <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
          Fallback Model <span className="text-gray-400">(optional)</span>
        </label>
        <select
          value={fallbackModel}
          onChange={(e) => onFallbackChange(e.target.value)}
          className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-800 text-gray-900 dark:text-white text-sm"
        >
          <option value="">None</option>
          {Object.entries(grouped).map(([provider, providerModels]) => (
            <optgroup key={provider} label={provider}>
              {providerModels.map((m) => (
                <option key={m.id} value={m.id}>
                  {m.displayName} ({m.tier})
                </option>
              ))}
            </optgroup>
          ))}
        </select>
      </div>
    </div>
  )
}
