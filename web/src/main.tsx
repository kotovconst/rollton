import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './styles/index.css'
import { initSDK } from '@/lib/telegram'
import { App } from '@/App'

initSDK()

const rootEl = document.getElementById('root')
if (!rootEl) throw new Error('root element not found')

createRoot(rootEl).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
