import type { Operation, OperationId } from '../../api/types'

export type Digit = '0' | '1' | '2' | '3' | '4' | '5' | '6' | '7' | '8' | '9'

export interface PendingCalculation {
  operation: OperationId
  operands: number[]
}

export type CalculatorPhase =
  | { kind: 'entering' }
  | { kind: 'calculating'; request: PendingCalculation }
  | { kind: 'result' }

export interface CalculatorState {
  left: string | null
  right: string | null
  operation: Operation | null
  phase: CalculatorPhase
  error: string | null
}

export type CalculatorAction =
  | { type: 'digitPressed'; digit: Digit }
  | { type: 'decimalPressed' }
  | { type: 'signToggled' }
  | { type: 'cleared' }
  | { type: 'operationSelected'; operation: Operation }
  | { type: 'submitted' }
  | { type: 'calculationSucceeded'; result: string }
  | { type: 'calculationFailed'; message: string }

const ENTERING: CalculatorPhase = { kind: 'entering' }
const RESULT: CalculatorPhase = { kind: 'result' }
const ZERO = '0'

export const initialState: CalculatorState = {
  left: null,
  right: null,
  operation: null,
  phase: ENTERING,
  error: null,
}

export function calculatorReducer(
  state: CalculatorState,
  action: CalculatorAction,
): CalculatorState {
  switch (action.type) {
    case 'digitPressed':
      return editEntry(state, (entry) => appendDigit(entry, action.digit), 'restart')

    case 'decimalPressed':
      return editEntry(state, appendDecimalPoint, 'restart')

    case 'signToggled':
      return editEntry(state, toggleSign, 'continue')

    case 'cleared':
      return initialState

    case 'operationSelected': {
      if (state.phase.kind === 'calculating') return state

      return {
        ...state,
        left: displayValue(state),
        right: null,
        operation: action.operation,
        phase: ENTERING,
        error: null,
      }
    }

    case 'submitted': {
      const request = pendingRequest(state)
      if (request === null) return state

      return { ...state, phase: { kind: 'calculating', request }, error: null }
    }

    case 'calculationSucceeded': {
      if (state.phase.kind !== 'calculating') return state

      return {
        ...state,
        left: action.result,
        right: null,
        operation: null,
        phase: RESULT,
        error: null,
      }
    }

    case 'calculationFailed': {
      if (state.phase.kind !== 'calculating') return state

      return { ...state, right: null, phase: ENTERING, error: action.message }
    }
  }
}

// Typed characters that stand in for a catalog symbol no keyboard has a key for. It maps
// characters to characters: no operation is named here, so an operation is still reachable by
// typing its own symbol and the catalog stays the only list of them (ADR-0009).
const SYMBOL_ALIASES: Record<string, string> = {
  '/': '÷',
  '*': '×',
  x: '×',
  '-': '−',
  r: '√',
}

export function actionForKey(key: string, operations: Operation[]): CalculatorAction | null {
  if (/^[0-9]$/.test(key)) return { type: 'digitPressed', digit: key as Digit }
  if (key === '.' || key === ',') return { type: 'decimalPressed' }
  if (key === 'Enter' || key === '=') return { type: 'submitted' }
  if (key === 'Escape' || key === 'c' || key === 'C') return { type: 'cleared' }

  const symbol = SYMBOL_ALIASES[key] ?? key
  const operation = operations.find((candidate) => candidate.symbol === symbol)

  return operation === undefined ? null : { type: 'operationSelected', operation }
}

export function displayValue(state: CalculatorState): string {
  return state.right ?? state.left ?? ZERO
}

// The operation is shown with its operand in the position the symbol is read in: a unary
// symbol precedes its operand, a binary one follows the operand it was pressed after.
export function pendingExpression(state: CalculatorState): string | null {
  const { operation } = state
  if (operation === null) return null

  const left = state.left ?? ZERO

  return operation.arity === 1 ? `${operation.symbol} ${left}` : `${left} ${operation.symbol}`
}

export function canSubmit(state: CalculatorState): boolean {
  return pendingRequest(state) !== null
}

// Also the submit guard: a computation that cannot be described as a request is not one the
// UI offers. Number() parses the literal that was typed; every arithmetic decision, rounding
// included, stays with the server (ADR-0003, ADR-0009).
function pendingRequest(state: CalculatorState): PendingCalculation | null {
  if (state.phase.kind !== 'entering') return null

  const { operation } = state
  if (operation === null) return null

  const entries = operation.arity === 1 ? [state.left] : [state.left, state.right]
  const operands: number[] = []

  for (const entry of entries) {
    if (entry === null) return null

    const operand = Number(entry)
    if (!Number.isFinite(operand)) return null

    operands.push(operand)
  }

  return { operation: operation.id, operands }
}

function editEntry(
  state: CalculatorState,
  edit: (entry: string | null) => string,
  afterResult: 'restart' | 'continue',
): CalculatorState {
  if (state.phase.kind === 'calculating') return state

  const slot = state.operation?.arity === 2 ? 'right' : 'left'
  const current = slot === 'right' ? state.right : state.left
  const restarting = state.phase.kind === 'result' && afterResult === 'restart'

  const edited = edit(restarting ? null : current)
  if (edited === current && state.phase.kind === 'entering' && state.error === null) return state

  const next: CalculatorState = { ...state, phase: ENTERING, error: null }
  if (slot === 'right') next.right = edited
  else next.left = edited

  return next
}

function appendDigit(entry: string | null, digit: Digit): string {
  if (entry === null || entry === ZERO) return digit
  if (entry === `-${ZERO}`) return `-${digit}`

  return entry + digit
}

function appendDecimalPoint(entry: string | null): string {
  if (entry === null) return `${ZERO}.`

  return entry.includes('.') ? entry : `${entry}.`
}

function toggleSign(entry: string | null): string {
  const value = entry ?? ZERO

  return value.startsWith('-') ? value.slice(1) : `-${value}`
}
