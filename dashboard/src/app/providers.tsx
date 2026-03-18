'use client'

import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { useState } from 'react'
import { I18nProvider } from '@/lib/i18n'
import { useSSE } from '@/lib/useSSE'

function SSEConnector() {
  useSSE()
  return null
}

export function Providers({ children }: { children: React.ReactNode }) {
  const [queryClient] = useState(
    () =>
      new QueryClient({
        defaultOptions: {
          queries: {
            staleTime: 1000 * 5, // 5 seconds (SSE handles refresh)
          },
        },
      })
  )

  return (
    <QueryClientProvider client={queryClient}>
      <I18nProvider>
        <SSEConnector />
        {children}
      </I18nProvider>
    </QueryClientProvider>
  )
}
