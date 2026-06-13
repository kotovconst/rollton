// Thin facade over window.Telegram.WebApp so the rest of the app
// never touches the global directly. Hook variants subscribe via
// React state on mount; the WebApp object itself is mutable but
// initData / user don't change after init, so a single read is enough.

import { useEffect, useState } from 'react'

export interface TelegramUser {
  id: number
  first_name?: string
  last_name?: string
  username?: string
  language_code?: string
}

export function initSDK(): void {
  const wa = window.Telegram?.WebApp
  if (!wa) return
  wa.ready()
  wa.expand()
}

export function getInitDataRaw(): string {
  return window.Telegram?.WebApp?.initData ?? ''
}

export function getTelegramUser(): TelegramUser | null {
  return window.Telegram?.WebApp?.initDataUnsafe?.user ?? null
}

export function isTelegramEnv(): boolean {
  return typeof window !== 'undefined' && !!window.Telegram?.WebApp?.initData
}

export function openCharacterBot(username: string, payload: string): void {
  const url = `https://t.me/${username}?start=${encodeURIComponent(payload)}`
  const wa = window.Telegram?.WebApp
  if (wa?.openTelegramLink) {
    wa.openTelegramLink(url)
  } else if (typeof window !== 'undefined') {
    window.open(url, '_blank')
  }
}

export function useTelegramUser(): TelegramUser | null {
  const [user, setUser] = useState<TelegramUser | null>(getTelegramUser())
  useEffect(() => setUser(getTelegramUser()), [])
  return user
}

export function useInitDataRaw(): string {
  const [raw, setRaw] = useState<string>(getInitDataRaw())
  useEffect(() => setRaw(getInitDataRaw()), [])
  return raw
}

export function useIsTelegramEnv(): boolean {
  const [v, setV] = useState<boolean>(isTelegramEnv())
  useEffect(() => setV(isTelegramEnv()), [])
  return v
}
