import { apiFetch } from '@/api/client'
import type { Character } from '@/types/api'

export function listCharacters(): Promise<Character[]> {
  return apiFetch<Character[]>('/api/v1/characters')
}

export function getCharacter(id: string): Promise<Character> {
  return apiFetch<Character>(`/api/v1/characters/${encodeURIComponent(id)}`)
}
