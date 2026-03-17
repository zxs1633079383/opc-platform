'use client'

import { useState, useCallback, useMemo } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { X, Info } from 'lucide-react'
import { applyAgent } from '@/lib/api'
import { useTranslation } from '@/lib/i18n'

interface AddAgentModalProps {
  readonly isOpen: boolean
  readonly onClose: () => void
}

interface AgentFormData {
  readonly name: string
  readonly type: string
  readonly modelName: string
  readonly workdir: string
  readonly maxTokens: number
  readonly costLimitPerTask: string
  readonly costLimitPerDay: string
  readonly recoveryEnabled: boolean
  readonly gatewayUrl: string
  readonly gatewayToken: string
}

// Model catalog: agent type → provider → models
interface ModelInfo {
  readonly name: string
  readonly displayName: string
  readonly inputPer1K: number
  readonly outputPer1K: number
  readonly description: string
  readonly recommended?: boolean
}

interface ProviderInfo {
  readonly provider: string
  readonly displayName: string
  readonly models: readonly ModelInfo[]
}

const MODEL_CATALOG: Record<string, readonly ProviderInfo[]> = {
  'claude-code': [
    {
      provider: 'anthropic',
      displayName: 'Anthropic',
      models: [
        { name: 'claude-sonnet-4', displayName: 'Claude Sonnet 4', inputPer1K: 0.003, outputPer1K: 0.015, description: 'Best coding model, fast', recommended: true },
        { name: 'claude-opus-4', displayName: 'Claude Opus 4', inputPer1K: 0.015, outputPer1K: 0.075, description: 'Deepest reasoning, complex tasks' },
        { name: 'claude-haiku-4-5-20251001', displayName: 'Claude Haiku 4.5', inputPer1K: 0.0008, outputPer1K: 0.004, description: 'Fast and cheap, simple tasks' },
      ],
    },
  ],
  'openclaw': [
    {
      provider: 'anthropic',
      displayName: 'Anthropic',
      models: [
        { name: 'claude-sonnet-4', displayName: 'Claude Sonnet 4', inputPer1K: 0.003, outputPer1K: 0.015, description: 'Best balance of speed & quality', recommended: true },
        { name: 'claude-opus-4', displayName: 'Claude Opus 4', inputPer1K: 0.015, outputPer1K: 0.075, description: 'Maximum reasoning' },
        { name: 'claude-haiku-4-5-20251001', displayName: 'Claude Haiku 4.5', inputPer1K: 0.0008, outputPer1K: 0.004, description: 'Fast worker agent' },
      ],
    },
    {
      provider: 'openai',
      displayName: 'OpenAI',
      models: [
        { name: 'gpt-4o', displayName: 'GPT-4o', inputPer1K: 0.0025, outputPer1K: 0.01, description: 'Multimodal flagship' },
        { name: 'gpt-4o-mini', displayName: 'GPT-4o Mini', inputPer1K: 0.00015, outputPer1K: 0.0006, description: 'Fast and affordable' },
      ],
    },
    {
      provider: 'deepseek',
      displayName: 'DeepSeek',
      models: [
        { name: 'deepseek-v3', displayName: 'DeepSeek V3', inputPer1K: 0.00027, outputPer1K: 0.0011, description: 'Cost-effective coding' },
        { name: 'deepseek-r1', displayName: 'DeepSeek R1', inputPer1K: 0.00055, outputPer1K: 0.0022, description: 'Reasoning model' },
      ],
    },
  ],
  'codex': [
    {
      provider: 'openai',
      displayName: 'OpenAI',
      models: [
        { name: 'o4-mini', displayName: 'O4 Mini', inputPer1K: 0.0011, outputPer1K: 0.0044, description: 'Default Codex model', recommended: true },
        { name: 'o3', displayName: 'O3', inputPer1K: 0.01, outputPer1K: 0.04, description: 'Advanced reasoning' },
        { name: 'o3-mini', displayName: 'O3 Mini', inputPer1K: 0.0011, outputPer1K: 0.0044, description: 'Lightweight reasoning' },
      ],
    },
  ],
  'openai': [
    {
      provider: 'openai',
      displayName: 'OpenAI',
      models: [
        { name: 'gpt-4o', displayName: 'GPT-4o', inputPer1K: 0.0025, outputPer1K: 0.01, description: 'Flagship multimodal', recommended: true },
        { name: 'gpt-4o-mini', displayName: 'GPT-4o Mini', inputPer1K: 0.00015, outputPer1K: 0.0006, description: 'Fast and cheap' },
        { name: 'o4-mini', displayName: 'O4 Mini', inputPer1K: 0.0011, outputPer1K: 0.0044, description: 'Reasoning model' },
        { name: 'o3', displayName: 'O3', inputPer1K: 0.01, outputPer1K: 0.04, description: 'Advanced reasoning' },
        { name: 'o1', displayName: 'O1', inputPer1K: 0.015, outputPer1K: 0.06, description: 'Deep reasoning' },
      ],
    },
  ],
  'custom': [
    {
      provider: 'anthropic',
      displayName: 'Anthropic',
      models: [
        { name: 'claude-sonnet-4', displayName: 'Claude Sonnet 4', inputPer1K: 0.003, outputPer1K: 0.015, description: 'Best coding model' },
        { name: 'claude-opus-4', displayName: 'Claude Opus 4', inputPer1K: 0.015, outputPer1K: 0.075, description: 'Deepest reasoning' },
        { name: 'claude-haiku-4-5-20251001', displayName: 'Claude Haiku 4.5', inputPer1K: 0.0008, outputPer1K: 0.004, description: 'Fast worker' },
      ],
    },
    {
      provider: 'openai',
      displayName: 'OpenAI',
      models: [
        { name: 'gpt-4o', displayName: 'GPT-4o', inputPer1K: 0.0025, outputPer1K: 0.01, description: 'Multimodal flagship' },
        { name: 'gpt-4o-mini', displayName: 'GPT-4o Mini', inputPer1K: 0.00015, outputPer1K: 0.0006, description: 'Fast and affordable' },
      ],
    },
    {
      provider: 'google',
      displayName: 'Google',
      models: [
        { name: 'gemini-2.5-pro', displayName: 'Gemini 2.5 Pro', inputPer1K: 0.00125, outputPer1K: 0.01, description: 'Advanced reasoning' },
        { name: 'gemini-2.5-flash', displayName: 'Gemini 2.5 Flash', inputPer1K: 0.00015, outputPer1K: 0.0006, description: 'Fast and efficient' },
      ],
    },
    {
      provider: 'deepseek',
      displayName: 'DeepSeek',
      models: [
        { name: 'deepseek-v3', displayName: 'DeepSeek V3', inputPer1K: 0.00027, outputPer1K: 0.0011, description: 'Cost-effective coding' },
        { name: 'deepseek-r1', displayName: 'DeepSeek R1', inputPer1K: 0.00055, outputPer1K: 0.0022, description: 'Reasoning model' },
      ],
    },
  ],
}

const AGENT_TYPES = [
  { value: 'claude-code', label: 'Claude Code', description: 'Anthropic CLI agent' },
  { value: 'openclaw', label: 'OpenClaw', description: 'Connect to OpenClaw Gateway' },
  { value: 'codex', label: 'Codex', description: 'OpenAI Codex CLI' },
  { value: 'openai', label: 'OpenAI API', description: 'Direct OpenAI API' },
  { value: 'custom', label: 'Custom', description: 'Custom agent process' },
] as const

const INITIAL_FORM: AgentFormData = {
  name: '',
  type: 'claude-code',
  modelName: 'claude-sonnet-4',
  workdir: '/workspace/my-project',
  maxTokens: 16384,
  costLimitPerTask: '$2',
  costLimitPerDay: '$50',
  recoveryEnabled: true,
  gatewayUrl: 'ws://localhost:18789',
  gatewayToken: '',
}

function getProviderForModel(agentType: string, modelName: string): string {
  const providers = MODEL_CATALOG[agentType] || []
  for (const p of providers) {
    if (p.models.some((m) => m.name === modelName)) return p.provider
  }
  return providers[0]?.provider || 'custom'
}

function formatPrice(price: number): string {
  if (price < 0.001) return `$${(price * 1000).toFixed(2)}/M`
  return `$${price.toFixed(4)}/1K`
}

function buildYaml(form: AgentFormData): string {
  if (form.type === 'openclaw') {
    // OpenClaw: connect to existing gateway, no model selection needed.
    const tokenLine = form.gatewayToken
      ? `\n  env:\n    OPENCLAW_GATEWAY_TOKEN: "${form.gatewayToken}"`
      : ''
    return `apiVersion: opc/v1
kind: AgentSpec
metadata:
  name: ${form.name}
  labels:
    role: agent
spec:
  type: openclaw
  runtime:
    timeout:
      task: "600s"
  resources:
    costLimit:
      perTask: "${form.costLimitPerTask}"
      perDay: "${form.costLimitPerDay}"
  context:
    workdir: ${form.workdir}
  protocol:
    type: websocket
    format: "${form.gatewayUrl}"${tokenLine}
  recovery:
    enabled: ${form.recoveryEnabled}
    maxRestarts: 3`
  }

  const provider = getProviderForModel(form.type, form.modelName)
  return `apiVersion: opc/v1
kind: AgentSpec
metadata:
  name: ${form.name}
  labels:
    role: agent
spec:
  type: ${form.type}
  runtime:
    model:
      provider: ${provider}
      name: ${form.modelName}
    inference:
      maxTokens: ${form.maxTokens}
    timeout:
      task: "600s"
  resources:
    tokenBudget:
      perTask: 200000
      perDay: 5000000
    costLimit:
      perTask: "${form.costLimitPerTask}"
      perDay: "${form.costLimitPerDay}"
  context:
    workdir: ${form.workdir}
  recovery:
    enabled: ${form.recoveryEnabled}
    maxRestarts: 3`
}

export function AddAgentModal({ isOpen, onClose }: AddAgentModalProps) {
  const { t } = useTranslation()
  const [form, setForm] = useState<AgentFormData>(INITIAL_FORM)
  const [error, setError] = useState<string | null>(null)
  const queryClient = useQueryClient()

  const mutation = useMutation({
    mutationFn: (yaml: string) => applyAgent(yaml),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['agents'] })
      setForm(INITIAL_FORM)
      setError(null)
      onClose()
    },
    onError: (err: Error) => {
      setError(err.message)
    },
  })

  const providers = useMemo(() => MODEL_CATALOG[form.type] || [], [form.type])

  const selectedModel = useMemo(() => {
    for (const p of providers) {
      const m = p.models.find((model) => model.name === form.modelName)
      if (m) return m
    }
    return null
  }, [providers, form.modelName])

  const updateField = useCallback(
    <K extends keyof AgentFormData>(key: K, value: AgentFormData[K]) => {
      setForm((prev) => ({ ...prev, [key]: value }))
    },
    []
  )

  const handleTypeChange = useCallback((newType: string) => {
    const typeProviders = MODEL_CATALOG[newType] || []
    let defaultModel = ''
    for (const p of typeProviders) {
      const rec = p.models.find((m) => m.recommended)
      if (rec) { defaultModel = rec.name; break }
    }
    if (!defaultModel && typeProviders[0]?.models[0]) {
      defaultModel = typeProviders[0].models[0].name
    }
    setForm((prev) => ({ ...prev, type: newType, modelName: defaultModel }))
  }, [])

  const handleSubmit = useCallback(
    (e: React.FormEvent) => {
      e.preventDefault()
      setError(null)

      if (!form.name.trim()) {
        setError(t('addAgent.nameRequired'))
        return
      }

      if (!/^[a-z0-9][a-z0-9-]*[a-z0-9]$/.test(form.name) && form.name.length > 1) {
        setError(t('addAgent.nameInvalid'))
        return
      }

      const yaml = buildYaml(form)
      mutation.mutate(yaml)
    },
    [form, mutation, t]
  )

  const handleBackdropClick = useCallback(
    (e: React.MouseEvent<HTMLDivElement>) => {
      if (e.target === e.currentTarget) onClose()
    },
    [onClose]
  )

  if (!isOpen) return null

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50"
      onClick={handleBackdropClick}
    >
      <div className="bg-white dark:bg-gray-900 rounded-xl shadow-xl border border-gray-200 dark:border-gray-800 w-full max-w-lg mx-4 max-h-[90vh] overflow-y-auto">
        <div className="flex items-center justify-between p-6 border-b border-gray-200 dark:border-gray-800">
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white">
            {t('addAgent.title')}
          </h2>
          <button
            onClick={onClose}
            className="p-1 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 rounded-lg transition-colors"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        <form onSubmit={handleSubmit} className="p-6 space-y-4">
          {error && (
            <div className="p-3 text-sm text-red-700 bg-red-50 dark:bg-red-900/20 dark:text-red-400 rounded-lg border border-red-200 dark:border-red-800">
              {error}
            </div>
          )}

          {/* Agent Name */}
          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
              {t('addAgent.name')} <span className="text-red-500">*</span>
            </label>
            <input
              type="text"
              value={form.name}
              onChange={(e) => updateField('name', e.target.value)}
              placeholder="my-agent"
              className="w-full px-3 py-2 border border-gray-300 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-800 text-gray-900 dark:text-white placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent text-sm"
              required
            />
          </div>

          {/* Agent Type - card selector */}
          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
              {t('addAgent.type')} <span className="text-red-500">*</span>
            </label>
            <div className="grid grid-cols-2 gap-2">
              {AGENT_TYPES.map((at) => (
                <button
                  key={at.value}
                  type="button"
                  onClick={() => handleTypeChange(at.value)}
                  className={`p-3 rounded-lg border text-left transition-all ${
                    form.type === at.value
                      ? 'border-primary-500 bg-primary-50 dark:bg-primary-900/20 ring-1 ring-primary-500'
                      : 'border-gray-200 dark:border-gray-700 hover:border-gray-300 dark:hover:border-gray-600'
                  }`}
                >
                  <div className="text-sm font-medium text-gray-900 dark:text-white">{at.label}</div>
                  <div className="text-xs text-gray-500 dark:text-gray-400">{at.description}</div>
                </button>
              ))}
            </div>
          </div>

          {/* OpenClaw: Gateway config instead of model selector */}
          {form.type === 'openclaw' ? (
            <div className="space-y-3">
              <div className="flex items-start gap-2 p-3 bg-orange-50 dark:bg-orange-900/10 rounded-lg border border-orange-200 dark:border-orange-800">
                <Info className="w-4 h-4 text-orange-500 shrink-0 mt-0.5" />
                <div className="text-xs text-orange-700 dark:text-orange-300">
                  {t('addAgent.openclawInfo')}
                </div>
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                  {t('addAgent.gatewayUrl')} <span className="text-red-500">*</span>
                </label>
                <input
                  type="text"
                  value={form.gatewayUrl}
                  onChange={(e) => updateField('gatewayUrl', e.target.value)}
                  placeholder="ws://localhost:18789"
                  className="w-full px-3 py-2 border border-gray-300 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-800 text-gray-900 dark:text-white placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent text-sm font-mono"
                />
                <p className="mt-1 text-xs text-gray-500 dark:text-gray-400">
                  {t('addAgent.gatewayUrlHint')}
                </p>
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                  {t('addAgent.gatewayToken')}
                </label>
                <input
                  type="password"
                  value={form.gatewayToken}
                  onChange={(e) => updateField('gatewayToken', e.target.value)}
                  placeholder={t('addAgent.gatewayTokenPlaceholder')}
                  className="w-full px-3 py-2 border border-gray-300 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-800 text-gray-900 dark:text-white placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent text-sm"
                />
                <p className="mt-1 text-xs text-gray-500 dark:text-gray-400">
                  {t('addAgent.gatewayTokenHint')}
                </p>
              </div>
            </div>
          ) : (
            <>
              {/* Model Selector - grouped by provider */}
              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                  {t('addAgent.model')}
                </label>
                <div className="space-y-3">
                  {providers.map((providerGroup) => (
                    <div key={providerGroup.provider}>
                      <div className="text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider mb-1.5">
                        {providerGroup.displayName}
                      </div>
                      <div className="space-y-1">
                        {providerGroup.models.map((model) => (
                          <button
                            key={model.name}
                            type="button"
                            onClick={() => updateField('modelName', model.name)}
                            className={`w-full p-2.5 rounded-lg border text-left transition-all flex items-center justify-between ${
                              form.modelName === model.name
                                ? 'border-primary-500 bg-primary-50 dark:bg-primary-900/20 ring-1 ring-primary-500'
                                : 'border-gray-200 dark:border-gray-700 hover:border-gray-300 dark:hover:border-gray-600'
                            }`}
                          >
                            <div className="flex-1 min-w-0">
                              <div className="flex items-center gap-2">
                                <span className="text-sm font-medium text-gray-900 dark:text-white">
                                  {model.displayName}
                                </span>
                                {model.recommended && (
                                  <span className="px-1.5 py-0.5 text-[10px] font-medium bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400 rounded">
                                    {t('addAgent.recommended')}
                                  </span>
                                )}
                              </div>
                              <div className="text-xs text-gray-500 dark:text-gray-400">{model.description}</div>
                            </div>
                            <div className="text-right shrink-0 ml-3">
                              <div className="text-xs text-gray-500 dark:text-gray-400">
                                ↑ {formatPrice(model.inputPer1K)}
                              </div>
                              <div className="text-xs text-gray-500 dark:text-gray-400">
                                ↓ {formatPrice(model.outputPer1K)}
                              </div>
                            </div>
                          </button>
                        ))}
                      </div>
                    </div>
                  ))}
                </div>
              </div>

              {/* Selected model info */}
              {selectedModel && (
                <div className="flex items-start gap-2 p-3 bg-blue-50 dark:bg-blue-900/10 rounded-lg border border-blue-200 dark:border-blue-800">
                  <Info className="w-4 h-4 text-blue-500 shrink-0 mt-0.5" />
                  <div className="text-xs text-blue-700 dark:text-blue-300">
                    <span className="font-medium">{selectedModel.displayName}</span>: Input {formatPrice(selectedModel.inputPer1K)} · Output {formatPrice(selectedModel.outputPer1K)}
                    {' '}· Est. ~${((selectedModel.inputPer1K * 10 + selectedModel.outputPer1K * 2)).toFixed(3)}/task (10K in + 2K out)
                  </div>
                </div>
              )}
            </>
          )}

          {/* Working Directory */}
          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
              {t('addAgent.workdir')}
            </label>
            <input
              type="text"
              value={form.workdir}
              onChange={(e) => updateField('workdir', e.target.value)}
              placeholder="/workspace/my-project"
              className="w-full px-3 py-2 border border-gray-300 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-800 text-gray-900 dark:text-white placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent text-sm"
            />
          </div>

          {/* Limits */}
          <div className={`grid ${form.type === 'openclaw' ? 'grid-cols-2' : 'grid-cols-3'} gap-3`}>
            {form.type !== 'openclaw' && (
              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                  {t('addAgent.maxTokens')}
                </label>
                <input
                  type="number"
                  value={form.maxTokens}
                  onChange={(e) => updateField('maxTokens', parseInt(e.target.value, 10) || 0)}
                  className="w-full px-3 py-2 border border-gray-300 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-800 text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent text-sm"
                />
              </div>
            )}
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                {t('addAgent.costPerTask')}
              </label>
              <input
                type="text"
                value={form.costLimitPerTask}
                onChange={(e) => updateField('costLimitPerTask', e.target.value)}
                placeholder="$2"
                className="w-full px-3 py-2 border border-gray-300 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-800 text-gray-900 dark:text-white placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent text-sm"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                {t('addAgent.costPerDay')}
              </label>
              <input
                type="text"
                value={form.costLimitPerDay}
                onChange={(e) => updateField('costLimitPerDay', e.target.value)}
                placeholder="$50"
                className="w-full px-3 py-2 border border-gray-300 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-800 text-gray-900 dark:text-white placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent text-sm"
              />
            </div>
          </div>

          {/* Recovery */}
          <div className="flex items-center gap-2">
            <input
              type="checkbox"
              id="recoveryEnabled"
              checked={form.recoveryEnabled}
              onChange={(e) => updateField('recoveryEnabled', e.target.checked)}
              className="w-4 h-4 text-primary-600 border-gray-300 rounded focus:ring-primary-500"
            />
            <label
              htmlFor="recoveryEnabled"
              className="text-sm font-medium text-gray-700 dark:text-gray-300"
            >
              {t('addAgent.recovery')}
            </label>
          </div>

          {/* Actions */}
          <div className="flex justify-end gap-3 pt-4 border-t border-gray-200 dark:border-gray-800">
            <button
              type="button"
              onClick={onClose}
              className="px-4 py-2 text-sm text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800 rounded-lg transition-colors"
            >
              {t('addAgent.cancel')}
            </button>
            <button
              type="submit"
              disabled={mutation.isPending}
              className="px-4 py-2 text-sm text-white bg-primary-600 hover:bg-primary-700 disabled:opacity-50 disabled:cursor-not-allowed rounded-lg transition-colors"
            >
              {mutation.isPending ? t('addAgent.creating') : t('addAgent.create')}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
