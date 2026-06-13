import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { http, HttpResponse } from 'msw'
import { setupServer } from 'msw/node'
import { screen } from '@testing-library/react'
import { renderWithProviders } from '@/test/setup'
import { HomePage } from '@/pages/HomePage'

const server = setupServer(
  http.get('*/api/v1/characters', () =>
    HttpResponse.json({
      success: true,
      data: [
        { id: 'sherlock', name: 'Sherlock', blurb: 'Detective', bot_username: 'sherlock_bot' },
      ],
    }),
  ),
)

describe('HomePage', () => {
  beforeEach(() => server.listen())
  afterEach(() => server.resetHandlers())

  it('renders a character after loading', async () => {
    renderWithProviders(<HomePage />)
    expect(await screen.findByText('Sherlock')).toBeInTheDocument()
  })
})
