// The only place fetch appears. A cancelled call rejects with its AbortError untouched;
// every other failure is a CalcError.

import { CalcError, errorFromResponse, isAbortError, requestIdOf } from './errors'
import type {
  CalculationRequest,
  CalculationResponse,
  Operand,
  Operation,
  OperationId,
} from './types'

// Relative, so the browser only ever calls its own origin (ADR-0012).
const OPERATIONS_PATH = '/api/v1/operations'

interface ResponseTrace {
  status: number
  requestId?: string
}

export async function getOperations(signal?: AbortSignal): Promise<Operation[]> {
  const { payload, trace } = await requestJSON(OPERATIONS_PATH, {
    method: 'GET',
    headers: { Accept: 'application/json' },
    signal,
  })

  return parseCatalog(payload, trace)
}

// The path is built from the contract's template, never from the catalog's `endpoint`, so
// no request address comes out of a response body.
export async function calculate(
  operation: OperationId,
  operands: Operand[],
  signal?: AbortSignal,
): Promise<CalculationResponse> {
  const body: CalculationRequest = { operands }

  const path = `${OPERATIONS_PATH}/${encodeURIComponent(operation)}`
  const { payload, trace } = await requestJSON(path, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
    body: JSON.stringify(body),
    signal,
  })

  return parseCalculation(payload, trace)
}

async function requestJSON(
  path: string,
  init: RequestInit,
): Promise<{ payload: unknown; trace: ResponseTrace }> {
  let response: Response
  try {
    response = await fetch(path, init)
  } catch (cause) {
    if (isAbortError(cause)) throw cause
    throw new CalcError('NETWORK_ERROR', `Request to ${path} could not be sent`, { cause })
  }

  const trace: ResponseTrace = { status: response.status, requestId: requestIdOf(response) }

  if (!response.ok) throw await errorFromResponse(response)

  try {
    return { payload: await response.json(), trace }
  } catch (cause) {
    if (isAbortError(cause)) throw cause
    throw new CalcError('MALFORMED_RESPONSE', `Response from ${path} was not valid JSON`, {
      ...trace,
      cause,
    })
  }
}

function parseCatalog(payload: unknown, trace: ResponseTrace): Operation[] {
  if (!isRecord(payload) || !Array.isArray(payload.operations)) {
    throw malformed('operation catalog', trace)
  }
  if (payload.operations.length === 0) {
    throw malformed('operation catalog, which was empty', trace)
  }

  return payload.operations.map((operation) => parseOperation(operation, trace))
}

function parseOperation(payload: unknown, trace: ResponseTrace): Operation {
  if (
    !isRecord(payload) ||
    !isNonEmptyString(payload.id) ||
    !isNonEmptyString(payload.name) ||
    !isNonEmptyString(payload.symbol) ||
    (payload.arity !== 1 && payload.arity !== 2) ||
    !isNonEmptyString(payload.endpoint) ||
    !Array.isArray(payload.parameters) ||
    payload.parameters.length !== payload.arity ||
    !payload.parameters.every(isParameter)
  ) {
    throw malformed('operation in the catalog', trace)
  }

  return payload as unknown as Operation
}

function isParameter(payload: unknown): boolean {
  return isRecord(payload) && isNonEmptyString(payload.name) && typeof payload.description === 'string'
}

function parseCalculation(payload: unknown, trace: ResponseTrace): CalculationResponse {
  if (
    !isRecord(payload) ||
    !isNonEmptyString(payload.operation) ||
    // A number here would silently undo the rounding policy (ADR-0003).
    typeof payload.result !== 'string' ||
    !Array.isArray(payload.operands) ||
    !payload.operands.every((operand) => typeof operand === 'number')
  ) {
    throw malformed('calculation result', trace)
  }

  return payload as unknown as CalculationResponse
}

function malformed(what: string, trace: ResponseTrace): CalcError {
  return new CalcError('MALFORMED_RESPONSE', `Response was not a valid ${what}`, trace)
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

function isNonEmptyString(value: unknown): value is string {
  return typeof value === 'string' && value.length > 0
}
