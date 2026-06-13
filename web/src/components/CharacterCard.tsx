import { Cell, Avatar } from '@telegram-apps/telegram-ui'
import type { Character } from '@/types/api'

interface Props {
  character: Character
  onClick?: () => void
}

export function CharacterCard({ character, onClick }: Props) {
  return (
    <Cell
      before={<Avatar size={48} src={character.avatar_url} />}
      description={character.blurb}
      onClick={onClick}
    >
      {character.name}
    </Cell>
  )
}
