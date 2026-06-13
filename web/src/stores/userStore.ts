import { create } from 'zustand'
import type { TelegramUser } from '@/lib/telegram'
import type { UserSettings } from '@/types/api'

interface UserState {
  tgUser: TelegramUser | null
  settings: UserSettings | null
  setTgUser: (u: TelegramUser | null) => void
  setSettings: (s: UserSettings | null) => void
}

export const useUserStore = create<UserState>((set) => ({
  tgUser: null,
  settings: null,
  setTgUser: (tgUser) => set({ tgUser }),
  setSettings: (settings) => set({ settings }),
}))
