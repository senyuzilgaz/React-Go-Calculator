import { http, HttpResponse } from 'msw'

import type { ErrorCode } from '../api/types'
import { CATALOG } from './catalog'

export const REQUEST_ID = '01J9ZC8N2K4QWERTY0123456789'

const traceHeaders = { 'X-Request-Id': REQUEST_ID }

// Only the catalog is served by default; a test that calculates declares its own response.
export const handlers = [
  http.get('/api/v1/operations', () => HttpResponse.json(CATALOG, { headers: traceHeaders })),
]

export function errorBody(status: number, code: ErrorCode, message: string) {
  return HttpResponse.json({ error: { code, message } }, { status, headers: traceHeaders })
}
