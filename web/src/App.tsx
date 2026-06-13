import { createBrowserRouter, RouterProvider } from 'react-router-dom'
import { QueryClientProvider } from '@tanstack/react-query'
import { queryClient } from '@/lib/queryClient'
import { Layout } from '@/components/Layout'
import { ErrorBoundary } from '@/components/ErrorBoundary'
import { HomePage } from '@/pages/HomePage'
import { CharacterPage } from '@/pages/CharacterPage'
import { SettingsPage } from '@/pages/SettingsPage'
import { SubscriptionPage } from '@/pages/SubscriptionPage'

const router = createBrowserRouter([
  {
    path: '/',
    element: <Layout />,
    children: [
      { index: true, element: <HomePage /> },
      { path: 'characters/:id', element: <CharacterPage /> },
      { path: 'settings', element: <SettingsPage /> },
      { path: 'subscription', element: <SubscriptionPage /> },
    ],
  },
])

export function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <ErrorBoundary>
        <RouterProvider router={router} />
      </ErrorBoundary>
    </QueryClientProvider>
  )
}
