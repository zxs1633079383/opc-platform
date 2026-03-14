import { clsx } from 'clsx'
import { formatDistanceToNow } from 'date-fns'
import type { Task } from '@/types'

interface TaskListProps {
  tasks: Task[]
}

const statusColors: Record<string, string> = {
  Running: 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-400',
  Pending: 'bg-gray-100 text-gray-800 dark:bg-gray-800 dark:text-gray-400',
  Completed: 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400',
  Failed: 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-400',
  Cancelled: 'bg-gray-100 text-gray-500 dark:bg-gray-800 dark:text-gray-500',
}

export function TaskList({ tasks }: TaskListProps) {
  if (tasks.length === 0) {
    return (
      <div className="text-center py-8 text-gray-500 dark:text-gray-400">
        No tasks found
      </div>
    )
  }

  return (
    <div className="overflow-x-auto">
      <table className="w-full">
        <thead>
          <tr className="text-left text-sm text-gray-500 dark:text-gray-400 border-b border-gray-200 dark:border-gray-700">
            <th className="pb-3 font-medium">ID</th>
            <th className="pb-3 font-medium">Agent</th>
            <th className="pb-3 font-medium">Message</th>
            <th className="pb-3 font-medium">Status</th>
            <th className="pb-3 font-medium">Cost</th>
            <th className="pb-3 font-medium">Time</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-gray-100 dark:divide-gray-800">
          {tasks.map((task) => (
            <tr
              key={task.id}
              className="text-sm hover:bg-gray-50 dark:hover:bg-gray-800/50"
            >
              <td className="py-3 font-mono text-xs text-gray-500">
                {task.id.slice(0, 8)}
              </td>
              <td className="py-3">
                <span className="text-gray-900 dark:text-white">
                  {task.agentName}
                </span>
              </td>
              <td className="py-3 max-w-md truncate text-gray-600 dark:text-gray-300">
                {task.message}
              </td>
              <td className="py-3">
                <span
                  className={clsx(
                    'px-2 py-0.5 rounded-full text-xs font-medium',
                    statusColors[task.status] || statusColors.Pending
                  )}
                >
                  {task.status}
                </span>
              </td>
              <td className="py-3 text-gray-600 dark:text-gray-300">
                ${(task.cost || 0).toFixed(4)}
              </td>
              <td className="py-3 text-gray-500 dark:text-gray-400">
                {formatDistanceToNow(new Date(task.createdAt), { addSuffix: true })}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
