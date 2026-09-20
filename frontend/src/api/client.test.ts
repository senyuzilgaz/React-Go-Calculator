import { delay, http, HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'

import { CATALOG } from '../test/catalog'
import { errorBody, REQUEST_ID } from '../test/handlers'
import { server } from '../test/server'
import { calculate, getOperations } from './client'
import { CalcError, isAbortError, isCalcError } from './errors'

const ADD_PATH = '/api/v1/operations/add'

async function rejection(pending: Promise<unknown>): Promise<unknown> {
  return pending.then(
    () => expect.fail('expected the call to reject'),
    (error: unknown) => error,
  )
}

async function calcError(pending: Promise<unknown>): Promise<CalcError> {
  const error = await rejection(pending)
  if (!isCalcError(error)) expect.fail(`expected a CalcError, got ${String(error)}`)
  return error
}

describe('getOperations', () => {
  it('returns the catalog as published', async () => {
    await expect(getOperations()).resolves.toEqual(CATALOG.operations)
  })

  it('issues one GET to the documented path', async () => {
    const requests: string[] = []
    server.use(
      http.get('/api/v1/operations', ({ request }) => {
        requests.push(`${request.method} ${new URL(request.url).pathname}`)
        return HttpResponse.json(CATALOG)
      }),
    )

    await getOperations()

    expect(requests).toEqual(['GET /api/v1/operations'])
  })

  it('reports a server fault by its code, status, and request id', async () => {
    server.use(
      http.get('/api/v1/operations', () =>
        errorBody(500, 'INTERNAL_ERROR', 'An unexpected error occurred'),
      ),
    )

    const error = await calcError(getOperations())

    expect(error.code).toBe('INTERNAL_ERROR')
    expect(error.status).toBe(500)
    expect(error.requestId).toBe(REQUEST_ID)
  })

  it('rejects a catalog entry that does not match the contract', async () => {
    const [add, ...rest] = CATALOG.operations
    server.use(
      http.get('/api/v1/operations', () =>
        HttpResponse.json({ operations: [{ ...add, arity: 3 }, ...rest] }),
      ),
    )

    expect((await calcError(getOperations())).code).toBe('MALFORMED_RESPONSE')
  })

  it('rejects a body that is not JSON at all', async () => {
    server.use(http.get('/api/v1/operations', () => HttpResponse.text('<html>gateway</html>')))

    expect((await calcError(getOperations())).code).toBe('MALFORMED_RESPONSE')
  })

  it('reports an unreachable service as a network failure', async () => {
    server.use(http.get('/api/v1/operations', () => HttpResponse.error()))

    const error = await calcError(getOperations())

    expect(error.code).toBe('NETWORK_ERROR')
    expect(error.status).toBeUndefined()
  })
})

describe('calculate', () => {
  it('sends the operands as a JSON array to the operation resource', async () => {
    let received: unknown
    server.use(
      http.post(ADD_PATH, async ({ request }) => {
        received = await request.json()
        return HttpResponse.json({ operation: 'add', operands: [0.1, 0.2], result: '0.3' })
      }),
    )

    await calculate('add', [0.1, 0.2])

    expect(received).toEqual({ operands: [0.1, 0.2] })
  })

  it('renders the rounded result string exactly as received', async () => {
    server.use(
      http.post('/api/v1/operations/divide', () =>
        HttpResponse.json({ operation: 'divide', operands: [1, 3], result: '0.333333333333' }),
      ),
    )

    const response = await calculate('divide', [1, 3])

    expect(response.result).toBe('0.333333333333')
  })

  it('sends a single operand for a unary operation', async () => {
    let received: unknown
    server.use(
      http.post('/api/v1/operations/sqrt', async ({ request }) => {
        received = await request.json()
        return HttpResponse.json({ operation: 'sqrt', operands: [9], result: '3' })
      }),
    )

    await calculate('sqrt', [9])

    expect(received).toEqual({ operands: [9] })
  })

  it('reports an undefined computation by its domain code', async () => {
    server.use(
      http.post('/api/v1/operations/divide', () =>
        errorBody(422, 'DIVISION_BY_ZERO', 'Cannot divide by zero'),
      ),
    )

    const error = await calcError(calculate('divide', [1, 0]))

    expect(error.code).toBe('DIVISION_BY_ZERO')
    expect(error.status).toBe(422)
    expect(error.message).toBe('Cannot divide by zero')
  })

  it('reports an operation the server does not have', async () => {
    server.use(
      http.post('/api/v1/operations/modulo', () =>
        errorBody(404, 'UNKNOWN_OPERATION', "Unknown operation 'modulo'"),
      ),
    )

    expect((await calcError(calculate('modulo', [7, 2]))).code).toBe('UNKNOWN_OPERATION')
  })

  it('escapes the operation identifier in the path', async () => {
    let path: string | undefined
    server.use(
      http.post('/api/v1/operations/:op', ({ request }) => {
        path = new URL(request.url).pathname
        return errorBody(404, 'UNKNOWN_OPERATION', 'Unknown operation')
      }),
    )

    await rejection(calculate('../healthz', [1]))

    expect(path).toBe('/api/v1/operations/..%2Fhealthz')
  })

  it('refuses a result sent as a JSON number', async () => {
    server.use(
      http.post(ADD_PATH, () =>
        HttpResponse.json({ operation: 'add', operands: [0.1, 0.2], result: 0.30000000000000004 }),
      ),
    )

    expect((await calcError(calculate('add', [0.1, 0.2]))).code).toBe('MALFORMED_RESPONSE')
  })

  it('refuses an error code outside the contract', async () => {
    server.use(
      http.post(ADD_PATH, () =>
        HttpResponse.json({ error: { code: 'TEAPOT', message: 'brewing' } }, { status: 422 }),
      ),
    )

    const error = await calcError(calculate('add', [1, 2]))

    expect(error.code).toBe('MALFORMED_RESPONSE')
    expect(error.status).toBe(422)
  })

  it('re-throws cancellation untouched, as the caller asked for it', async () => {
    server.use(
      http.post(ADD_PATH, async () => {
        await delay(100)
        return HttpResponse.json({ operation: 'add', operands: [1, 2], result: '3' })
      }),
    )

    const controller = new AbortController()
    const pending = calculate('add', [1, 2], controller.signal)
    controller.abort()

    const error = await rejection(pending)

    expect(isAbortError(error)).toBe(true)
    expect(isCalcError(error)).toBe(false)
  })
})
