// The TypeScript half of api/openapi.yaml, hand-written (ADR-0005, ADR-0014).

// Open union: the catalog is authoritative at runtime, so an id this file does not list is
// data rather than a type error (ADR-0009).
export type KnownOperationId =
  | 'add'
  | 'subtract'
  | 'multiply'
  | 'divide'
  | 'power'
  | 'sqrt'
  | 'percent'

export type OperationId = KnownOperationId | (string & {})

export interface OperationParameter {
  name: string
  description: string
}

export interface Operation {
  id: OperationId
  name: string
  symbol: string
  arity: 1 | 2
  parameters: OperationParameter[]
  endpoint: string
}

export interface OperationCatalog {
  operations: Operation[]
}

export type Operand = number

export interface CalculationRequest {
  operands: Operand[]
}

export interface CalculationResponse {
  operation: OperationId
  operands: Operand[]
  // Rendered verbatim, never re-parsed or re-formatted (ADR-0003).
  result: string
}

// A value, so the runtime check and the type share one list.
export const ERROR_CODES = [
  'INVALID_JSON',
  'INVALID_OPERAND',
  'WRONG_OPERAND_COUNT',
  'UNKNOWN_OPERATION',
  'METHOD_NOT_ALLOWED',
  'DIVISION_BY_ZERO',
  'NEGATIVE_SQRT',
  'RESULT_OVERFLOW',
  'INTERNAL_ERROR',
] as const

export type ErrorCode = (typeof ERROR_CODES)[number]

export interface ApiErrorBody {
  error: {
    code: ErrorCode
    message: string
  }
}
