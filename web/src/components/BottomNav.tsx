import { NavLink, useLocation } from 'react-router-dom'
import { Tabbar } from '@telegram-apps/telegram-ui'

const tabs = [
  { id: 'home', label: 'Home', path: '/' },
  { id: 'settings', label: 'Settings', path: '/settings' },
  { id: 'subscription', label: 'Plan', path: '/subscription' },
] as const

export function BottomNav() {
  const { pathname } = useLocation()
  return (
    <Tabbar>
      {tabs.map((tab) => {
        const selected = tab.path === '/' ? pathname === '/' : pathname.startsWith(tab.path)
        return (
          <Tabbar.Item key={tab.id} selected={selected} text={tab.label}>
            <NavLink to={tab.path} style={{ position: 'absolute', inset: 0 }} aria-label={tab.label} />
          </Tabbar.Item>
        )
      })}
    </Tabbar>
  )
}
