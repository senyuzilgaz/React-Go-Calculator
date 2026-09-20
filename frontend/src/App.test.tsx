import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { delay, http, HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'

import App from './App'
import { CATALOG } from './test/catalog'
import { errorBody } from './test/handlers'
import { server } from './test/server'

const catalogLoaded = () => screen.findByRole('button', { name: 'Addition' })

describe('App', () => {
  it('renders a key for every operation the catalog publishes', async () => {
    render(<App />)
    await catalogLoaded()

    const operations = screen.getByRole('region', { name: 'Operations' })

    expect([...operations.querySelectorAll('button')].map((key) => key.textContent)).toEqual(
      CATALOG.operations.map((operation) => operation.symbol),
    )
  })

  it('renders the digits before the catalog answers', async () => {
    server.use(
      http.get('/api/v1/operations', async () => {
        await delay(20)
        return HttpResponse.json(CATALOG)
      }),
    )

    render(<App />)

    expect(screen.getByRole('button', { name: '7' })).toBeEnabled()
    expect(screen.getByText('Loading operations…')).toBeInTheDocument()

    await catalogLoaded()
  })

  it('issues one request for a completed computation and renders what it returns', async () => {
    let requests = 0
    server.use(
      http.post('/api/v1/operations/add', async ({ request }) => {
        requests += 1
        expect(await request.json()).toEqual({ operands: [1, 2] })
        return HttpResponse.json({ operation: 'add', operands: [1, 2], result: '3' })
      }),
    )

    const user = userEvent.setup()
    render(<App />)
    await catalogLoaded()

    await user.click(screen.getByRole('button', { name: '1' }))
    await user.click(screen.getByRole('button', { name: 'Addition' }))
    await user.click(screen.getByRole('button', { name: '2' }))
    await user.click(screen.getByRole('button', { name: 'Equals' }))

    expect(await screen.findByRole('status')).toHaveTextContent('3')
    expect(requests).toBe(1)
  })

  it('renders and calls an operation this frontend has never heard of', async () => {
    const modulo = {
      id: 'modulo',
      name: 'Modulo',
      symbol: 'mod',
      arity: 2,
      parameters: [
        { name: 'dividend', description: 'The value divided.' },
        { name: 'divisor', description: 'The value divided by.' },
      ],
      endpoint: '/api/v1/operations/modulo',
    }
    let path: string | undefined
    let body: unknown
    server.use(
      http.get('/api/v1/operations', () =>
        HttpResponse.json({ operations: [...CATALOG.operations, modulo] }),
      ),
      http.post('/api/v1/operations/modulo', async ({ request }) => {
        path = new URL(request.url).pathname
        body = await request.json()
        return HttpResponse.json({ operation: 'modulo', operands: [7, 3], result: '1' })
      }),
    )

    const user = userEvent.setup()
    render(<App />)
    await catalogLoaded()

    await user.click(screen.getByRole('button', { name: '7' }))
    await user.click(screen.getByRole('button', { name: 'Modulo' }))
    await user.click(screen.getByRole('button', { name: '3' }))
    await user.click(screen.getByRole('button', { name: 'Equals' }))

    expect(await screen.findByRole('status')).toHaveTextContent('1')
    expect(path).toBe('/api/v1/operations/modulo')
    expect(body).toEqual({ operands: [7, 3] })
  })

  it('computes from the physical keyboard', async () => {
    let body: unknown
    server.use(
      http.post('/api/v1/operations/divide', async ({ request }) => {
        body = await request.json()
        return HttpResponse.json({ operation: 'divide', operands: [12, 4], result: '3' })
      }),
    )

    const user = userEvent.setup()
    render(<App />)
    await catalogLoaded()

    await user.keyboard('12/4{Enter}')

    expect(await screen.findByRole('status')).toHaveTextContent('3')
    expect(body).toEqual({ operands: [12, 4] })
  })

  it('accepts typing after a key has been clicked, and submits on =', async () => {
    server.use(
      http.post('/api/v1/operations/add', () =>
        HttpResponse.json({ operation: 'add', operands: [1, 2], result: '3' }),
      ),
    )

    const user = userEvent.setup()
    render(<App />)
    await catalogLoaded()

    await user.click(screen.getByRole('button', { name: '1' }))
    await user.click(screen.getByRole('button', { name: 'Addition' }))
    await user.keyboard('2=')

    expect(await screen.findByRole('status')).toHaveTextContent('3')
  })

  it('clears from the keyboard', async () => {
    const user = userEvent.setup()
    render(<App />)
    await catalogLoaded()

    await user.keyboard('12/4')
    expect(screen.getByRole('status')).toHaveTextContent('4')

    await user.keyboard('{Escape}')

    expect(screen.getByRole('status')).toHaveTextContent('0')
    expect(screen.getByRole('button', { name: 'Equals' })).toBeDisabled()
  })

  it('leaves enter to the key that has focus', async () => {
    const user = userEvent.setup()
    render(<App />)
    await catalogLoaded()

    await user.click(screen.getByRole('button', { name: '7' }))
    await user.keyboard('{Enter}')

    expect(screen.getByRole('status')).toHaveTextContent('77')
  })

  it('cannot submit an incomplete computation', async () => {
    const user = userEvent.setup()
    render(<App />)
    await catalogLoaded()

    expect(screen.getByRole('button', { name: 'Equals' })).toBeDisabled()

    await user.click(screen.getByRole('button', { name: '1' }))
    await user.click(screen.getByRole('button', { name: 'Addition' }))

    expect(screen.getByRole('button', { name: 'Equals' })).toBeDisabled()

    await user.click(screen.getByRole('button', { name: '2' }))

    expect(screen.getByRole('button', { name: 'Equals' })).toBeEnabled()
  })

  it('renders 3 for 12 \u00f7 4', async () => {
    let body: unknown
    server.use(
      http.post('/api/v1/operations/divide', async ({ request }) => {
        body = await request.json()
        return HttpResponse.json({ operation: 'divide', operands: [12, 4], result: '3' })
      }),
    )

    const user = userEvent.setup()
    render(<App />)
    await catalogLoaded()

    await user.click(screen.getByRole('button', { name: '1' }))
    await user.click(screen.getByRole('button', { name: '2' }))
    await user.click(screen.getByRole('button', { name: 'Division' }))

    expect(screen.getByText('12 \u00f7')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: '4' }))
    await user.click(screen.getByRole('button', { name: 'Equals' }))

    expect(await screen.findByRole('status')).toHaveTextContent('3')
    expect(body).toEqual({ operands: [12, 4] })
  })

  it('renders a 422 as the message for its code and never the server prose', async () => {
    const serverProse = 'divide(1, 0): calc: division by zero'
    server.use(
      http.post('/api/v1/operations/divide', () =>
        errorBody(422, 'DIVISION_BY_ZERO', serverProse),
      ),
    )

    const user = userEvent.setup()
    render(<App />)
    await catalogLoaded()

    await user.click(screen.getByRole('button', { name: '1' }))
    await user.click(screen.getByRole('button', { name: 'Division' }))
    await user.click(screen.getByRole('button', { name: '0' }))
    await user.click(screen.getByRole('button', { name: 'Equals' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('Cannot divide by zero.')
    expect(document.body).not.toHaveTextContent(serverProse)
    expect(screen.queryByText(/calc:/)).not.toBeInTheDocument()
  })

  it('keeps the rejected computation ready to retype after a 422', async () => {
    server.use(
      http.post('/api/v1/operations/divide', () =>
        errorBody(422, 'DIVISION_BY_ZERO', 'Cannot divide by zero'),
      ),
    )

    const user = userEvent.setup()
    render(<App />)
    await catalogLoaded()

    await user.click(screen.getByRole('button', { name: '8' }))
    await user.click(screen.getByRole('button', { name: 'Division' }))
    await user.click(screen.getByRole('button', { name: '0' }))
    await user.click(screen.getByRole('button', { name: 'Equals' }))

    await screen.findByRole('alert')

    expect(screen.getByText('8 \u00f7')).toBeInTheDocument()
    expect(screen.getByRole('status')).toHaveTextContent('8')
    expect(screen.getByRole('button', { name: 'Equals' })).toBeDisabled()

    await user.click(screen.getByRole('button', { name: '4' }))

    expect(screen.getByRole('button', { name: 'Equals' })).toBeEnabled()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('offers a retry when the catalog cannot be reached', async () => {
    server.use(
      http.get(
        '/api/v1/operations',
        () => errorBody(500, 'INTERNAL_ERROR', 'An unexpected error occurred'),
        { once: true },
      ),
    )

    const user = userEvent.setup()
    render(<App />)

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'The calculator service had a problem.',
    )

    await user.click(screen.getByRole('button', { name: 'Try again' }))

    await catalogLoaded()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('applies a unary key to the displayed value as soon as it is pressed', async () => {
    let body: unknown
    server.use(
      http.post('/api/v1/operations/sqrt', async ({ request }) => {
        body = await request.json()
        return HttpResponse.json({ operation: 'sqrt', operands: [9], result: '3' })
      }),
    )

    const user = userEvent.setup()
    render(<App />)
    await catalogLoaded()

    await user.click(screen.getByRole('button', { name: '9' }))
    await user.click(screen.getByRole('button', { name: 'Square root' }))

    expect(await screen.findByRole('status')).toHaveTextContent('3')
    expect(body).toEqual({ operands: [9] })
  })

  it('computes √2 + √2 as 2.82842712475, without dropping the addition (ADR-0026)', async () => {
    const sent: unknown[] = []
    server.use(
      http.post('/api/v1/operations/sqrt', async ({ request }) => {
        sent.push(await request.json())
        return HttpResponse.json({ operation: 'sqrt', operands: [2], result: '1.41421356237' })
      }),
      http.post('/api/v1/operations/add', async ({ request }) => {
        sent.push(await request.json())
        return HttpResponse.json({
          operation: 'add',
          operands: [1.41421356237, 1.41421356237],
          result: '2.82842712475',
        })
      }),
    )

    const user = userEvent.setup()
    render(<App />)
    await catalogLoaded()

    await user.click(screen.getByRole('button', { name: '2' }))
    await user.click(screen.getByRole('button', { name: 'Square root' }))
    await screen.findByText('1.41421356237')

    await user.click(screen.getByRole('button', { name: 'Addition' }))
    await user.click(screen.getByRole('button', { name: '2' }))
    await user.click(screen.getByRole('button', { name: 'Square root' }))

    expect(screen.getByText('1.41421356237 +')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Equals' }))

    expect(await screen.findByRole('status')).toHaveTextContent('2.82842712475')
    expect(sent).toEqual([
      { operands: [2] },
      { operands: [2] },
      { operands: [1.41421356237, 1.41421356237] },
    ])
  })
})
