import { describe, expect, it } from 'vitest'

import { CalcError, isAbortError, isCalcError, uiMessage } from './errors'
import type { CalcErrorCode } from './errors'
import { ERROR_CODES } from './types'

const CLIENT_CODES = ['NETWORK_ERROR', 'MALFORMED_RESPONSE'] as const

describe('uiMessage', () => {
  it.each<CalcErrorCode>([...ERROR_CODES, ...CLIENT_CODES])(
    'has a message a user can read for %s',
    (code) => {
      const message = uiMessage(new CalcError(code, 'internal wording'))

      expect(message).not.toBe('')
      expect(message).not.toContain(code)
    },
  )

  it('describes anything that is not a CalcError as a service fault', () => {
    expect(uiMessage(new TypeError('boom'))).toBe(uiMessage(new CalcError('INTERNAL_ERROR', '')))
  })
})

describe('CalcError', () => {
  it('keeps the server message for logs while the UI reads the code', () => {
    const error = new CalcError('DIVISION_BY_ZERO', 'Cannot divide by zero', {
      status: 422,
      requestId: 'abc',
    })

    expect(isCalcError(error)).toBe(true)
    expect(error.message).toBe('Cannot divide by zero')
    expect(error.status).toBe(422)
    expect(error.requestId).toBe('abc')
  })
})

describe('isAbortError', () => {
  it('recognises the DOMException fetch rejects with', () => {
    const controller = new AbortController()
    controller.abort()

    expect(isAbortError(controller.signal.reason)).toBe(true)
  })

  it('does not mistake a CalcError for cancellation', () => {
    expect(isAbortError(new CalcError('NETWORK_ERROR', 'down'))).toBe(false)
  })
})
