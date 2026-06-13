import { useQuery } from '@tanstack/react-query'
import { Section, Cell, Spinner } from '@telegram-apps/telegram-ui'
import { getSubscription } from '@/api/subscription'
import { ErrorState } from '@/components/ErrorState'

export function SubscriptionPage() {
  const { data, error, isLoading, refetch } = useQuery({
    queryKey: ['subscription'],
    queryFn: getSubscription,
  })

  if (isLoading) return <Spinner size="l" />
  if (error || !data) return <ErrorState error={error} onRetry={() => refetch()} />

  return (
    <Section header="Subscription">
      <Cell description={data.renews_at ? `Renews at ${data.renews_at}` : 'No active renewal'}>
        Plan: {data.tier}
      </Cell>
    </Section>
  )
}
