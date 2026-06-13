import { useQuery } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import { Section, Spinner } from '@telegram-apps/telegram-ui'
import { listCharacters } from '@/api/characters'
import { CharacterCard } from '@/components/CharacterCard'
import { ErrorState } from '@/components/ErrorState'

export function HomePage() {
  const navigate = useNavigate()
  const { data, error, isLoading, refetch } = useQuery({
    queryKey: ['characters'],
    queryFn: listCharacters,
  })

  if (isLoading) return <Spinner size="l" />
  if (error) return <ErrorState error={error} onRetry={() => refetch()} />

  return (
    <Section header="Characters">
      {data?.map((c) => (
        <CharacterCard
          key={c.id}
          character={c}
          onClick={() => navigate(`/characters/${c.id}`)}
        />
      ))}
    </Section>
  )
}
