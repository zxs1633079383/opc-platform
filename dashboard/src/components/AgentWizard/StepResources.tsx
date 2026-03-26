'use client'

const presets = [
  { value: 'light', label: 'Light', desc: '$5/day budget', cost: '$5' },
  { value: 'standard', label: 'Standard', desc: '$20/day budget (recommended)', cost: '$20' },
  { value: 'power', label: 'Power', desc: '$100/day budget', cost: '$100' },
] as const

export function StepResources({ preset, replicas, onExceed, onPresetChange, onReplicasChange, onExceedChange }: {
  preset: string
  replicas: number
  onExceed: string
  onPresetChange: (v: string) => void
  onReplicasChange: (v: number) => void
  onExceedChange: (v: string) => void
}) {
  return (
    <div className="space-y-5">
      <h2 className="text-lg font-semibold text-gray-900 dark:text-white">Resource Allocation</h2>

      <div>
        <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Budget Preset</label>
        <div className="space-y-2">
          {presets.map((p) => (
            <label
              key={p.value}
              className={`flex items-center gap-3 p-3 rounded-lg border cursor-pointer transition-all ${
                preset === p.value
                  ? 'border-primary-500 bg-primary-50 dark:bg-primary-900/20'
                  : 'border-gray-200 dark:border-gray-700 hover:border-gray-300'
              }`}
            >
              <input
                type="radio"
                name="preset"
                value={p.value}
                checked={preset === p.value}
                onChange={() => onPresetChange(p.value)}
                className="text-primary-600"
              />
              <div className="flex-1">
                <span className="text-sm font-medium text-gray-900 dark:text-white">{p.label}</span>
                <span className="text-xs text-gray-500 dark:text-gray-400 ml-2">{p.desc}</span>
              </div>
              <span className="text-sm font-semibold text-gray-700 dark:text-gray-300">{p.cost}/day</span>
            </label>
          ))}
        </div>
      </div>

      <div>
        <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Replicas</label>
        <input
          type="number"
          min={1}
          max={5}
          value={replicas}
          onChange={(e) => onReplicasChange(Math.min(5, Math.max(1, parseInt(e.target.value, 10) || 1)))}
          className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-800 text-gray-900 dark:text-white text-sm"
        />
        <p className="text-xs text-gray-500 mt-1">1-5 instances</p>
      </div>

      <div>
        <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">On Budget Exceed</label>
        <select
          value={onExceed}
          onChange={(e) => onExceedChange(e.target.value)}
          className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-800 text-gray-900 dark:text-white text-sm"
        >
          <option value="pause">Pause agent</option>
          <option value="alert">Alert only</option>
          <option value="reject">Reject new tasks</option>
        </select>
      </div>
    </div>
  )
}
