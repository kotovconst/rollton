import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Section, Cell, Switch, Spinner } from '@telegram-apps/telegram-ui'
import { me, updateSettings } from '@/api/user'
import { ErrorState } from '@/components/ErrorState'
import type { UserSettings } from '@/types/api'

export function SettingsPage() {
  const qc = useQueryClient()
  const { data, error, isLoading, refetch } = useQuery({
    queryKey: ['me'],
    queryFn: me,
  })

  const mutation = useMutation({
    mutationFn: (patch: Partial<UserSettings>) => updateSettings(patch),
    onSuccess: (next) => {
      qc.setQueryData(['me'], (prev: { user: unknown; settings: UserSettings } | undefined) =>
        prev ? { ...prev, settings: next } : prev,
      )
    },
  })

  if (isLoading) return <Spinner size="l" />
  if (error || !data) return <ErrorState error={error} onRetry={() => refetch()} />

  return (
    <Section header="Settings">
      <Cell
        after={
          <Switch
            checked={data.settings.notifications_enabled}
            onChange={(e) =>
              mutation.mutate({ notifications_enabled: e.target.checked })
            }
          />
        }
      >
        Notifications
      </Cell>
    </Section>
  )
}
