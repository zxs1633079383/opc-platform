'use client'

import { Inbox } from 'lucide-react'

export function EmptyState({ message = 'No data yet', action, onAction }: {
  message?: string
  action?: string
  onAction?: () => void
}) {
  return (
    <div className="flex flex-col items-center justify-center py-16 text-gray-400 dark:text-gray-500">
      <Inbox className="w-12 h-12 mb-4" />
      <p className="text-lg">{message}</p>
      {action && onAction && (
        <button onClick={onAction} className="mt-4 px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700">{action}</button>
      )}
    </div>
  )
}
