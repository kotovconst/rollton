import { Outlet } from 'react-router-dom'
import { AppRoot } from '@telegram-apps/telegram-ui'
import { useIsTelegramEnv } from '@/lib/telegram'
import { useMe } from '@/hooks/useMe'
import { AuthBoundary } from '@/components/AuthBoundary'
import { BottomNav } from '@/components/BottomNav'
import { OutsideTelegramNotice } from '@/components/OutsideTelegramNotice'

export function Layout() {
  const inTelegram = useIsTelegramEnv()
  if (!inTelegram) {
    return (
      <AppRoot>
        <OutsideTelegramNotice />
      </AppRoot>
    )
  }
  return (
    <AppRoot>
      <AuthBoundary>
        <div className="flex h-full flex-col">
          <WelcomeHeader />
          <main className="flex-1 overflow-auto">
            <Outlet />
          </main>
          <BottomNav />
        </div>
      </AuthBoundary>
    </AppRoot>
  )
}

function WelcomeHeader() {
  const { data } = useMe()
  if (!data) return null
  return (
    <header className="border-b px-4 py-2 text-sm">
      Hi, {data.user.first_name}
    </header>
  )
}
