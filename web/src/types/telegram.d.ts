// Minimal ambient typing for window.Telegram.WebApp.
// Tracks only the fields the stub uses.

export {}

interface TelegramUser {
  id: number
  first_name?: string
  last_name?: string
  username?: string
  language_code?: string
}

interface TelegramWebApp {
  initData: string
  initDataUnsafe: { user?: TelegramUser }
  ready(): void
  expand(): void
  openTelegramLink(url: string): void
}

declare global {
  interface Window {
    Telegram?: { WebApp?: TelegramWebApp }
  }
}
