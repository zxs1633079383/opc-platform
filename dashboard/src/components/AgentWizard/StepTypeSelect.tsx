'use client'

import { Bot, Cpu, Cog, Wrench } from 'lucide-react'

const agentTypes = [
  { value: 'claude-code', label: 'Claude Code', desc: 'Anthropic CLI agent', icon: Bot },
  { value: 'openclaw', label: 'OpenClaw', desc: 'OpenClaw Gateway agent', icon: Cpu },
  { value: 'openai', label: 'OpenAI', desc: 'OpenAI API agent', icon: Cog },
  { value: 'custom', label: 'Custom', desc: 'Custom agent process', icon: Wrench },
]

export function StepTypeSelect({ selected, onSelect }: {
  selected: string
  onSelect: (type: string) => void
}) {
  return (
    <div className="space-y-4">
      <h2 className="text-lg font-semibold text-gray-900 dark:text-white">Select Agent Type</h2>
      <div className="grid grid-cols-2 gap-3">
        {agentTypes.map((at) => {
          const Icon = at.icon
          const isActive = selected === at.value
          return (
            <button
              key={at.value}
              onClick={() => onSelect(at.value)}
              className={`p-4 rounded-xl border text-left transition-all ${
                isActive
                  ? 'border-primary-500 bg-primary-50 dark:bg-primary-900/20 ring-2 ring-primary-500'
                  : 'border-gray-200 dark:border-gray-700 hover:border-gray-300 dark:hover:border-gray-600'
              }`}
            >
              <Icon className={`w-8 h-8 mb-2 ${isActive ? 'text-primary-500' : 'text-gray-400'}`} />
              <div className="text-sm font-medium text-gray-900 dark:text-white">{at.label}</div>
              <div className="text-xs text-gray-500 dark:text-gray-400 mt-0.5">{at.desc}</div>
            </button>
          )
        })}
      </div>
    </div>
  )
}
