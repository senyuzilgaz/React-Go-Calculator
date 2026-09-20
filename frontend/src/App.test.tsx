import { http } from 'msw'
import { describe, expect, it } from 'vitest'
import { render, screen } from '@testing-library/react'

import App from './App'
import { CalcError, uiMessage } from './api/errors'
import { CATALOG } from './test/catalog'
import { errorBody } from './test/handlers'
import { server } from './test/server'

describe('App', () => {
  it('renders one key per operation the catalog publishes', async () => {
    render(<App />)

    const keys = await screen.findAllByRole('button')

    expect(keys.map((key) => key.textContent)).toEqual(
      CATALOG.operations.map((operation) => operation.symbol),
    )
    expect(screen.getByRole('button', { name: 'Division' })).toBeInTheDocument()
  })

  it('surfaces an unreachable catalog as an error banner', async () => {
    server.use(
      http.get('/api/v1/operations', () =>
        errorBody(500, 'INTERNAL_ERROR', 'An unexpected error occurred'),
      ),
    )

    render(<App />)

    await expect(screen.findByRole('alert')).resolves.toHaveTextContent(
      uiMessage(new CalcError('INTERNAL_ERROR', '')),
    )
    expect(screen.queryAllByRole('button')).toHaveLength(0)
  })
})
