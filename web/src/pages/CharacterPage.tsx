import { useQuery } from '@tanstack/react-query'
import { useParams } from 'react-router-dom'
import { Section, Button, Spinner, Avatar } from '@telegram-apps/telegram-ui'
import { getCharacter } from '@/api/characters'
import { ErrorState } from '@/components/ErrorState'
import { openCharacterBot, useTelegramUser } from '@/lib/telegram'

export function CharacterPage() {
  const { id = '' } = useParams<{ id: string }>()
  const tgUser = useTelegramUser()
  const { data, error, isLoading, refetch } = useQuery({
    queryKey: ['character', id],
    queryFn: () => getCharacter(id),
    enabled: !!id,
  })

  if (isLoading) return <Spinner size="l" />
  if (error || !data) return <ErrorState error={error} onRetry={() => refetch()} />

  return (
    <Section header={data.name}>
      <div className="flex flex-col items-center gap-3 p-4">
        <Avatar size={96} src={data.avatar_url} />
        <p className="text-center">{data.blurb}</p>
        <Button
          size="l"
          onClick={() =>
            openCharacterBot(data.bot_username, `ref_${tgUser?.id ?? 'anon'}`)
          }
        >
          Open chat
        </Button>
      </div>
    </Section>
  )
}
