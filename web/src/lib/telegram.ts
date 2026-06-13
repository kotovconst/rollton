// Thin facade over window.Telegram.WebApp so the rest of the app
// never touches the global directly. Telegram's `telegram-web-app.js`
// loads synchronously in index.html, so by the time React mounts the
// global is populated. Hooks just return live values — no state needed.

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
  return getTelegramUser()
}

export function useInitDataRaw(): string {
  return getInitDataRaw()
}

export function useIsTelegramEnv(): boolean {
  return isTelegramEnv()
}
