// Wire-format types matching the rolltonchatbot Go backend's response envelope.

export interface ApiEnvelope<T> {
  success: boolean
  data?: T
  error?: { code: string; message: string }
}

export interface User {
  id: string
  telegram_id: number
  username?: string
  first_name?: string
  last_name?: string
}

export interface UserSettings {
  notifications_enabled: boolean
  preferred_language: string
}

export interface Character {
  id: string
  name: string
  blurb: string
  avatar_url?: string
  bot_username: string
}

export interface Subscription {
  tier: 'free' | 'pro' | 'unlimited'
  renews_at?: string
}

export class ApiError extends Error {
  status: number
  code: string

  constructor(status: number, code: string, message: string) {
    super(message)
    this.status = status
    this.code = code
    this.name = 'ApiError'
  }
}
