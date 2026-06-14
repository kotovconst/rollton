import { describe, it, expect, afterEach } from 'vitest'
import { http, HttpResponse, delay } from 'msw'
import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderWithProviders, server } from '@/test/setup'
import { AuthBoundary } from '@/components/AuthBoundary'

afterEach(() => server.resetHandlers())

describe('AuthBoundary', () => {
  it('renders children on success', async () => {
    renderWithProviders(
      <AuthBoundary>
        <div>Protected content</div>
      </AuthBoundary>,
    )
    expect(await screen.findByText('Protected content')).toBeInTheDocument()
  })

  it('shows "Open from Telegram" on 401', async () => {
    server.use(
      http.get('*/api/v1/me', () =>
        HttpResponse.json(
          { success: false, error: { code: 'UNAUTHORIZED', message: 'invalid init data' } },
          { status: 401 },
        ),
      ),
    )
    renderWithProviders(
      <AuthBoundary>
        <div>Protected content</div>
      </AuthBoundary>,
    )
    expect(await screen.findByText(/Open from Telegram/i)).toBeInTheDocument()
    expect(screen.queryByText('Protected content')).not.toBeInTheDocument()
  })

  it('shows retry on 500 and re-fetches when clicked', async () => {
    let calls = 0
    server.use(
      http.get('*/api/v1/me', () => {
        calls++
        if (calls === 1) {
          return HttpResponse.json(
            { success: false, error: { code: 'INTERNAL_ERROR', message: 'db down' } },
            { status: 500 },
          )
        }
        return HttpResponse.json({
          success: true,
          data: {
            user: { id: 'mock-id', telegram_id: 42, first_name: 'Test' },
            settings: { notifications_enabled: true, preferred_language: 'en' },
          },
        })
      }),
    )

    renderWithProviders(
      <AuthBoundary>
        <div>Protected content</div>
      </AuthBoundary>,
    )

    const retryBtn = await screen.findByRole('button', { name: /retry/i })
    await userEvent.click(retryBtn)
    expect(await screen.findByText('Protected content')).toBeInTheDocument()
  })

  it('shows a loading state while the request is in flight', async () => {
    server.use(
      http.get('*/api/v1/me', async () => {
        await delay(50)
        return HttpResponse.json({
          success: true,
          data: {
            user: { id: 'mock-id', telegram_id: 42, first_name: 'Test' },
            settings: { notifications_enabled: true, preferred_language: 'en' },
          },
        })
      }),
    )

    renderWithProviders(
      <AuthBoundary>
        <div>Protected content</div>
      </AuthBoundary>,
    )
    expect(screen.queryByText('Protected content')).not.toBeInTheDocument()
    expect(await screen.findByText('Protected content')).toBeInTheDocument()
  })
})
