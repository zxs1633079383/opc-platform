'use client'

import { useEffect, useRef, useCallback } from 'react'
import { useQueryClient } from '@tanstack/react-query'

const API_BASE = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:9527/api'

/**
 * SSE hook: connects to /api/events and auto-invalidates react-query caches
 * when data changes are detected.
 */
export function useSSE() {
  const queryClient = useQueryClient()
  const lastSnapshot = useRef<string>('')

  const invalidateAll = useCallback(() => {
    queryClient.invalidateQueries({ queryKey: ['tasks'] })
    queryClient.invalidateQueries({ queryKey: ['goals'] })
    queryClient.invalidateQueries({ queryKey: ['agents'] })
    queryClient.invalidateQueries({ queryKey: ['projects'] })
    queryClient.invalidateQueries({ queryKey: ['issues'] })
    queryClient.invalidateQueries({ queryKey: ['companies'] })
  }, [queryClient])

  useEffect(() => {
    let es: EventSource | null = null
    let retryTimeout: ReturnType<typeof setTimeout>

    function connect() {
      es = new EventSource(`${API_BASE}/events`)

      es.onmessage = (event) => {
        // Only invalidate if data actually changed
        if (event.data !== lastSnapshot.current) {
          lastSnapshot.current = event.data
          invalidateAll()
        }
      }

      es.onerror = () => {
        es?.close()
        // Reconnect after 5s
        retryTimeout = setTimeout(connect, 5000)
      }
    }

    connect()

    return () => {
      es?.close()
      clearTimeout(retryTimeout)
    }
  }, [invalidateAll])
}
