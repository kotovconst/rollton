import { Outlet } from 'react-router-dom'
import { AppRoot } from '@telegram-apps/telegram-ui'
import { useIsTelegramEnv } from '@/lib/telegram'
import { BottomNav } from '@/components/BottomNav'
import { OutsideTelegramNotice } from '@/components/OutsideTelegramNotice'

export function Layout() {
  const inTelegram = useIsTelegramEnv()
  return (
    <AppRoot>
      <div className="flex h-full flex-col">
        <main className="flex-1 overflow-auto">
          {inTelegram ? <Outlet /> : <OutsideTelegramNotice />}
        </main>
        {inTelegram ? <BottomNav /> : null}
      </div>
    </AppRoot>
  )
}
