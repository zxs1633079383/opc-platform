'use client'

import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { X, ArrowLeft, ArrowRight } from 'lucide-react'
import { createAgentWizard } from '@/lib/api'
import type { WizardRequest } from '@/types'
import { StepTypeSelect } from './StepTypeSelect'
import { StepDescribe } from './StepDescribe'
import { StepResources } from './StepResources'
import { StepConfirm } from './StepConfirm'

type Step = 'typeSelect' | 'describe' | 'resources' | 'confirm'
const STEPS: Step[] = ['typeSelect', 'describe', 'resources', 'confirm']
const STEP_LABELS = ['Type', 'Describe', 'Resources', 'Confirm']

interface AgentWizardProps {
  readonly isOpen: boolean
  readonly onClose: () => void
}

export function AgentWizard({ isOpen, onClose }: AgentWizardProps) {
  const queryClient = useQueryClient()
  const [step, setStep] = useState<Step>('typeSelect')
  const [type, setType] = useState('claude-code')
  const [description, setDescription] = useState('')
  const [model, setModel] = useState('')
  const [fallbackModel, setFallbackModel] = useState('')
  const [preset, setPreset] = useState('standard')
  const [replicas, setReplicas] = useState(1)
  const [onExceed, setOnExceed] = useState('pause')
  const [error, setError] = useState<string | null>(null)

  const mutation = useMutation({
    mutationFn: (data: WizardRequest) => createAgentWizard(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['agents'] })
      resetAndClose()
    },
    onError: (err: Error) => setError(err.message),
  })

  const resetAndClose = () => {
    setStep('typeSelect')
    setType('claude-code')
    setDescription('')
    setModel('')
    setFallbackModel('')
    setPreset('standard')
    setReplicas(1)
    setOnExceed('pause')
    setError(null)
    onClose()
  }

  const currentIndex = STEPS.indexOf(step)
  const canGoBack = currentIndex > 0
  const canGoForward = currentIndex < STEPS.length - 1

  const goBack = () => {
    if (canGoBack) setStep(STEPS[currentIndex - 1])
  }

  const goForward = () => {
    if (canGoForward) setStep(STEPS[currentIndex + 1])
  }

  const handleSubmit = () => {
    setError(null)
    mutation.mutate({
      type,
      description,
      model,
      fallbackModel: fallbackModel || undefined,
      preset: preset as WizardRequest['preset'],
      replicas,
      onExceed,
    })
  }

  if (!isOpen) return null

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50" onClick={resetAndClose}>
      <div className="bg-white dark:bg-gray-900 rounded-xl shadow-xl w-full max-w-lg mx-4 max-h-[90vh] overflow-y-auto" onClick={(e) => e.stopPropagation()}>
        {/* Header */}
        <div className="flex items-center justify-between p-5 border-b border-gray-200 dark:border-gray-800">
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white">Agent Wizard</h2>
          <button onClick={resetAndClose} className="text-gray-400 hover:text-gray-600">
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Step indicators */}
        <div className="flex items-center gap-1 px-5 pt-4">
          {STEP_LABELS.map((label, i) => (
            <div key={label} className="flex items-center gap-1 flex-1">
              <div className={`flex items-center justify-center w-6 h-6 rounded-full text-xs font-medium ${
                i <= currentIndex
                  ? 'bg-primary-500 text-white'
                  : 'bg-gray-200 dark:bg-gray-700 text-gray-500'
              }`}>
                {i + 1}
              </div>
              <span className={`text-xs hidden sm:inline ${i <= currentIndex ? 'text-primary-600 dark:text-primary-400' : 'text-gray-400'}`}>
                {label}
              </span>
              {i < STEP_LABELS.length - 1 && (
                <div className={`flex-1 h-0.5 mx-1 ${i < currentIndex ? 'bg-primary-500' : 'bg-gray-200 dark:bg-gray-700'}`} />
              )}
            </div>
          ))}
        </div>

        {/* Content */}
        <div className="p-5">
          {error && (
            <div className="mb-4 p-3 text-sm text-red-700 bg-red-50 dark:bg-red-900/20 dark:text-red-400 rounded-lg border border-red-200 dark:border-red-800">
              {error}
            </div>
          )}

          {step === 'typeSelect' && (
            <StepTypeSelect selected={type} onSelect={(t) => { setType(t); goForward() }} />
          )}
          {step === 'describe' && (
            <StepDescribe
              description={description}
              model={model}
              fallbackModel={fallbackModel}
              onDescriptionChange={setDescription}
              onModelChange={setModel}
              onFallbackChange={setFallbackModel}
            />
          )}
          {step === 'resources' && (
            <StepResources
              preset={preset}
              replicas={replicas}
              onExceed={onExceed}
              onPresetChange={setPreset}
              onReplicasChange={setReplicas}
              onExceedChange={setOnExceed}
            />
          )}
          {step === 'confirm' && (
            <StepConfirm
              data={{ type, description, model, fallbackModel, preset, replicas, onExceed }}
              isPending={mutation.isPending}
              onSubmit={handleSubmit}
            />
          )}
        </div>

        {/* Footer navigation (except on confirm step which has its own button) */}
        {step !== 'confirm' && (
          <div className="flex justify-between px-5 pb-5">
            <button
              onClick={goBack}
              disabled={!canGoBack}
              className="flex items-center gap-1 px-3 py-2 text-sm text-gray-600 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-800 rounded-lg disabled:opacity-30"
            >
              <ArrowLeft className="w-4 h-4" />
              Back
            </button>
            {step !== 'typeSelect' && (
              <button
                onClick={goForward}
                disabled={!canGoForward}
                className="flex items-center gap-1 px-3 py-2 text-sm text-white bg-primary-600 hover:bg-primary-700 rounded-lg disabled:opacity-50"
              >
                Next
                <ArrowRight className="w-4 h-4" />
              </button>
            )}
          </div>
        )}
      </div>
    </div>
  )
}
