import { act, renderHook, waitFor } from '@testing-library/react'
import { delay, http, HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'

import { errorBody } from '../../test/handlers'
import { operationById } from '../../test/operations'
import { server } from '../../test/server'
import type { CalculatorAction } from './reducer'
import { useCalculator } from './useCalculator'

const ADD = operationById('add')
const ADD_PATH = '/api/v1/operations/add'

const ADDITION: CalculatorAction[] = [
  { type: 'digitPressed', digit: '1' },
  { type: 'operationSelected', operation: ADD },
  { type: 'digitPressed', digit: '2' },
  { type: 'submitted' },
]

function calculator() {
  const { result, unmount } = renderHook(() => useCalculator())

  const press = (actions: CalculatorAction[]) =>
    act(() => {
      for (const action of actions) result.current.dispatch(action)
    })

  return { result, press, unmount }
}

function watchedRequest() {
  const watched = { started: false, aborted: false }

  server.use(
    http.post(ADD_PATH, async ({ request }) => {
      watched.started = true
      request.signal.addEventListener('abort', () => {
        watched.aborted = true
      })
      await delay(200)
      return HttpResponse.json({ operation: 'add', operands: [1, 2], result: '3' })
    }),
  )

  return watched
}

describe('useCalculator', () => {
  it('issues one request for a completed computation and renders what it returns', async () => {
    let requests = 0
    server.use(
      http.post(ADD_PATH, async ({ request }) => {
        requests += 1
        expect(await request.json()).toEqual({ operands: [1, 2] })
        return HttpResponse.json({ operation: 'add', operands: [1, 2], result: '3' })
      }),
    )

    const { result, press } = calculator()
    press(ADDITION)

    expect(result.current.status).toBe('calculating')

    await waitFor(() => expect(result.current.status).toBe('result'))
    expect(result.current.display).toBe('3')
    expect(requests).toBe(1)
  })

  it('surfaces a domain failure as the message for its code', async () => {
    server.use(
      http.post(ADD_PATH, () => errorBody(422, 'DIVISION_BY_ZERO', 'Cannot divide by zero')),
    )

    const { result, press } = calculator()
    press(ADDITION)

    await waitFor(() => expect(result.current.error).toBe('Cannot divide by zero.'))
    expect(result.current.status).toBe('entering')
  })

  it('cancels a request in flight when the calculator is cleared', async () => {
    const inFlight = watchedRequest()

    const { result, press } = calculator()
    press(ADDITION)
    await waitFor(() => expect(inFlight.started).toBe(true))

    press([{ type: 'cleared' }])

    await waitFor(() => expect(inFlight.aborted).toBe(true))
    expect(result.current.status).toBe('entering')
    expect(result.current.display).toBe('0')
    expect(result.current.error).toBeNull()
  })

  it('cancels a request in flight when unmounted', async () => {
    const inFlight = watchedRequest()

    const { press, unmount } = calculator()
    press(ADDITION)
    await waitFor(() => expect(inFlight.started).toBe(true))

    unmount()

    await waitFor(() => expect(inFlight.aborted).toBe(true))
  })
})
