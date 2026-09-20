import { ERROR_CODES, type ErrorCode } from './types'

// Failures the server never classified. They are not contract codes, so they stay out of
// types.ts. A code outside the contract's closed enum is MALFORMED_RESPONSE too (ADR-0019).
export type ClientErrorCode = 'NETWORK_ERROR' | 'MALFORMED_RESPONSE'

export type CalcErrorCode = ErrorCode | ClientErrorCode

interface CalcErrorOptions {
  status?: number
  requestId?: string
  cause?: unknown
}

export class CalcError extends Error {
  readonly code: CalcErrorCode
  readonly status?: number
  readonly requestId?: string

  constructor(code: CalcErrorCode, message: string, options: CalcErrorOptions = {}) {
    super(message, { cause: options.cause })
    this.name = 'CalcError'
    this.code = code
    this.status = options.status
    this.requestId = options.requestId
  }
}

export function isCalcError(value: unknown): value is CalcError {
  return value instanceof CalcError
}

// fetch rejects with a DOMException, so the name is the only reliable discriminator.
export function isAbortError(value: unknown): boolean {
  return value instanceof Error && value.name === 'AbortError'
}

const REQUEST_ID_HEADER = 'X-Request-Id'

export function requestIdOf(response: Response): string | undefined {
  return response.headers.get(REQUEST_ID_HEADER) ?? undefined
}

export async function errorFromResponse(response: Response): Promise<CalcError> {
  const requestId = requestIdOf(response)

  let body: unknown
  try {
    body = await response.json()
  } catch (cause) {
    if (isAbortError(cause)) throw cause
    return new CalcError(
      'MALFORMED_RESPONSE',
      `HTTP ${response.status} carried no readable error envelope`,
      { status: response.status, requestId, cause },
    )
  }

  const detail = errorDetailOf(body)
  if (!detail) {
    return new CalcError(
      'MALFORMED_RESPONSE',
      `HTTP ${response.status} did not carry a documented error envelope`,
      { status: response.status, requestId },
    )
  }

  return new CalcError(detail.code, detail.message, { status: response.status, requestId })
}

function errorDetailOf(body: unknown): { code: ErrorCode; message: string } | null {
  if (typeof body !== 'object' || body === null || !('error' in body)) return null

  const detail = (body as { error: unknown }).error
  if (typeof detail !== 'object' || detail === null) return null

  const { code, message } = detail as { code?: unknown; message?: unknown }
  if (!isErrorCode(code) || typeof message !== 'string') return null

  return { code, message }
}

function isErrorCode(value: unknown): value is ErrorCode {
  return typeof value === 'string' && (ERROR_CODES as readonly string[]).includes(value)
}

// Total record: a code added to the contract fails to compile until it has a message.
const MESSAGES: Record<CalcErrorCode, string> = {
  DIVISION_BY_ZERO: 'Cannot divide by zero.',
  NEGATIVE_SQRT: 'Cannot take the square root of a negative number.',
  RESULT_OVERFLOW: 'That result is too large to display.',
  INVALID_OPERAND: 'That number is not valid.',
  WRONG_OPERAND_COUNT: 'That operation needs a different number of values.',
  UNKNOWN_OPERATION: 'That operation is not available.',
  INVALID_JSON: 'The calculator could not send that request.',
  METHOD_NOT_ALLOWED: 'The calculator could not send that request.',
  INTERNAL_ERROR: 'The calculator service had a problem. Try again.',
  NETWORK_ERROR: 'Cannot reach the calculator service.',
  MALFORMED_RESPONSE: 'The calculator service sent an unexpected response.',
}

export function uiMessage(error: unknown): string {
  return isCalcError(error) ? MESSAGES[error.code] : MESSAGES.INTERNAL_ERROR
}
