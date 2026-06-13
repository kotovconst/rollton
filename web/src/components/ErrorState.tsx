import { Placeholder, Button } from '@telegram-apps/telegram-ui'
import { ApiError } from '@/types/api'

interface ErrorStateProps {
  error: unknown
  onRetry?: () => void
}

export function ErrorState({ error, onRetry }: ErrorStateProps) {
  const apiErr = error instanceof ApiError ? error : null
  const code = apiErr?.code ?? 'UNKNOWN'

  let header = 'Something went wrong'
  let description = apiErr?.message ?? (error instanceof Error ? error.message : 'Unknown error')
  let showRetry = true

  if (code === 'UNAUTHORIZED') {
    header = 'Open from Telegram'
    description = 'This screen needs Telegram authentication. Open the mini app via the bot.'
    showRetry = false
  } else if (code === 'NETWORK' || code === 'TIMEOUT') {
    header = 'Connection problem'
    description = "Couldn't reach the server. Check your connection and retry."
  }

  return (
    <Placeholder header={header} description={description}>
      {showRetry && onRetry ? <Button onClick={onRetry}>Retry</Button> : null}
    </Placeholder>
  )
}
