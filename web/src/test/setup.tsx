import '@testing-library/jest-dom/vitest'
import type { ReactElement } from 'react'
import { render, type RenderResult } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter } from 'react-router-dom'
import { AppRoot } from '@telegram-apps/telegram-ui'
import { vi } from 'vitest'

// Stable fake Telegram WebApp so isTelegramEnv() returns true in tests.
vi.stubGlobal('Telegram', {
  WebApp: {
    initData: 'mock-init-data',
    initDataUnsafe: { user: { id: 42, first_name: 'Test', username: 'testuser' } },
    colorScheme: 'light',
    themeParams: {},
    platform: 'unknown',
    version: '7.0',
    ready: () => {},
    expand: () => {},
    openTelegramLink: () => {},
    onEvent: () => {},
    offEvent: () => {},
  },
})

// One fresh QueryClient per render so tests don't share cache.
function freshClient() {
  return new QueryClient({
    defaultOptions: { queries: { retry: false, staleTime: 0 } },
  })
}

export function renderWithProviders(ui: ReactElement): RenderResult {
  return render(
    <QueryClientProvider client={freshClient()}>
      <AppRoot>
        <MemoryRouter>{ui}</MemoryRouter>
      </AppRoot>
    </QueryClientProvider>,
  )
}
