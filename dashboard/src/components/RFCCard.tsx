'use client'

import { CheckCircle, XCircle } from 'lucide-react'
import type { RFC } from '@/types'

const statusBadge: Record<RFC['status'], string> = {
  pending: 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-400',
  approved: 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400',
  rejected: 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400',
}

export function RFCCard({ rfc, onApprove, onReject }: {
  rfc: RFC
  onApprove: () => void
  onReject: () => void
}) {
  const isPending = rfc.status === 'pending'

  return (
    <div className="bg-white dark:bg-gray-900 rounded-xl shadow-sm border border-gray-200 dark:border-gray-800 p-5">
      <div className="flex items-start justify-between mb-3">
        <h3 className="text-lg font-semibold text-gray-900 dark:text-white">{rfc.title}</h3>
        <span className={`px-2.5 py-0.5 rounded-full text-xs font-medium capitalize ${statusBadge[rfc.status]}`}>
          {rfc.status}
        </span>
      </div>

      <div className="space-y-3 text-sm">
        <div>
          <span className="font-medium text-gray-700 dark:text-gray-300">Problem: </span>
          <span className="text-gray-600 dark:text-gray-400">{rfc.problem}</span>
        </div>
        <div>
          <span className="font-medium text-gray-700 dark:text-gray-300">Solution: </span>
          <span className="text-gray-600 dark:text-gray-400">{rfc.solution}</span>
        </div>
        <div>
          <span className="font-medium text-gray-700 dark:text-gray-300">Expected Benefit: </span>
          <span className="text-gray-600 dark:text-gray-400">{rfc.expectedBenefit}</span>
        </div>
        <div>
          <span className="font-medium text-gray-700 dark:text-gray-300">Risk: </span>
          <span className="text-gray-600 dark:text-gray-400">{rfc.risk}</span>
        </div>
      </div>

      {isPending && (
        <div className="flex gap-2 mt-4 pt-4 border-t border-gray-200 dark:border-gray-800">
          <button
            onClick={onApprove}
            className="flex items-center gap-1.5 px-3 py-1.5 text-sm font-medium text-green-700 dark:text-green-400 bg-green-50 dark:bg-green-900/20 hover:bg-green-100 dark:hover:bg-green-900/30 rounded-lg"
          >
            <CheckCircle className="w-4 h-4" />
            Approve
          </button>
          <button
            onClick={onReject}
            className="flex items-center gap-1.5 px-3 py-1.5 text-sm font-medium text-red-700 dark:text-red-400 bg-red-50 dark:bg-red-900/20 hover:bg-red-100 dark:hover:bg-red-900/30 rounded-lg"
          >
            <XCircle className="w-4 h-4" />
            Reject
          </button>
        </div>
      )}

      <p className="text-xs text-gray-400 mt-3">
        Created: {new Date(rfc.createdAt).toLocaleDateString()}
      </p>
    </div>
  )
}
