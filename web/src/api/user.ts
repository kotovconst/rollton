import { apiFetch } from '@/api/client'
import type { User, UserSettings } from '@/types/api'

export function me(): Promise<{ user: User; settings: UserSettings }> {
  return apiFetch<{ user: User; settings: UserSettings }>('/api/v1/me')
}

export function updateSettings(input: Partial<UserSettings>): Promise<UserSettings> {
  return apiFetch<UserSettings>('/api/v1/settings', {
    method: 'PATCH',
    body: JSON.stringify(input),
  })
}
