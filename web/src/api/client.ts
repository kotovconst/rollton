import { getInitDataRaw } from '@/lib/telegram'
import { ApiError } from '@/types/api'
import type { ApiEnvelope } from '@/types/api'

const BASE_URL = import.meta.env.VITE_API_BASE_URL ?? ''

export async function apiFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const headers = new Headers(init?.headers)
  const raw = getInitDataRaw()
  if (raw) headers.set('Authorization', `tma ${raw}`)
  if (init?.body && !headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json')
  }

  let res: Response
  try {
    res = await fetch(`${BASE_URL}${path}`, { ...init, headers })
  } catch (e) {
    throw new ApiError(0, 'NETWORK', e instanceof Error ? e.message : 'network error')
  }

  let envelope: ApiEnvelope<T> | null = null
  try {
    envelope = (await res.json()) as ApiEnvelope<T>
  } catch {
    /* non-JSON body, leave envelope null */
  }

  if (!res.ok) {
    const code = envelope?.error?.code ?? 'UNKNOWN'
    const message = envelope?.error?.message ?? res.statusText
    throw new ApiError(res.status, code, message)
  }

  if (!envelope?.success || envelope.data === undefined) {
    throw new ApiError(res.status, 'PARSE', 'invalid response envelope')
  }

  return envelope.data
}
