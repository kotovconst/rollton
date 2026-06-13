import { apiFetch } from '@/api/client'
import type { Subscription } from '@/types/api'

export function getSubscription(): Promise<Subscription> {
  return apiFetch<Subscription>('/api/v1/subscription')
}
