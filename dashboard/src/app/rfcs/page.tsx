'use client'

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { FileText } from 'lucide-react'
import { fetchRFCs, approveRFC, rejectRFC } from '@/lib/api'
import { RFCCard } from '@/components/RFCCard'
import { EmptyState } from '@/components/EmptyState'
import { Skeleton } from '@/components/Skeleton'

export default function RFCsPage() {
  const queryClient = useQueryClient()

  const { data: rfcs = [], isLoading } = useQuery({
    queryKey: ['rfcs'],
    queryFn: fetchRFCs,
  })

  const approveMutation = useMutation({
    mutationFn: approveRFC,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['rfcs'] }),
  })

  const rejectMutation = useMutation({
    mutationFn: rejectRFC,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['rfcs'] }),
  })

  return (
    <div className="p-6 space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-gray-900 dark:text-white">RFCs</h1>
        <p className="text-gray-500 dark:text-gray-400">
          Review and approve system change proposals
        </p>
      </div>

      {isLoading ? (
        <div className="space-y-4">
          {[1, 2, 3].map((i) => (
            <Skeleton key={i} className="h-48 w-full" />
          ))}
        </div>
      ) : rfcs.length === 0 ? (
        <EmptyState message="No RFCs submitted yet" />
      ) : (
        <div className="space-y-4">
          {rfcs.map((rfc) => (
            <RFCCard
              key={rfc.id}
              rfc={rfc}
              onApprove={() => approveMutation.mutate(rfc.id)}
              onReject={() => rejectMutation.mutate(rfc.id)}
            />
          ))}
        </div>
      )}
    </div>
  )
}
