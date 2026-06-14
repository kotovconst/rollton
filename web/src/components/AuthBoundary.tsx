import type { ReactNode } from 'react'
import { Spinner } from '@telegram-apps/telegram-ui'
import { useMe } from '@/hooks/useMe'
import { ErrorState } from '@/components/ErrorState'

export function AuthBoundary({ children }: { children: ReactNode }) {
  const { data, error, isLoading, refetch } = useMe()
  if (isLoading) return <Spinner size="l" />
  if (error || !data) return <ErrorState error={error} onRetry={() => refetch()} />
  return <>{children}</>
}
