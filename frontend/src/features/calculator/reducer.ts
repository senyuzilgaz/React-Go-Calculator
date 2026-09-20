import type { Operation, OperationId } from '../../api/types'

export type Digit = '0' | '1' | '2' | '3' | '4' | '5' | '6' | '7' | '8' | '9'

export interface PendingCalculation {
  operation: OperationId
  operands: number[]
}

// What the returned string settles: an `entry` replaces the operand being edited and leaves the
// operation waiting for it standing, a `computation` completes the operation itself (ADR-0026).
export type Settlement = 'entry' | 'computation'

export type CalculatorPhase =
  | { kind: 'entering' }
  | { kind: 'calculating'; request: PendingCalculation; settles: Settlement }
  | { kind: 'result' }

export interface CalculatorState {
  left: string | null
  right: string | null
  operation: Operation | null
  phase: CalculatorPhase
  error: string | null
  // The displayed entry was placed there by the calculator, not typed, so a digit starts a
  // new one instead of extending it. A unary operation edits the slot its operand already
  // occupies, which is why selecting one sets this as a result does.
  carriedEntry: boolean
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
  carriedEntry: false,
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

      // A unary operation applies to the displayed value the moment it is pressed, so it never
      // occupies the operation slot. Deferring it there until `=` is what made `√2 + √2` evict
      // the pending `+` and compute `sqrt(2)` (ADR-0026).
      if (action.operation.arity === 1) {
        const operand = operandOf(displayValue(state))
        if (operand === null) return state

        return {
          ...state,
          phase: {
            kind: 'calculating',
            request: { operation: action.operation.id, operands: [operand] },
            settles: 'entry',
          },
          error: null,
        }
      }

      return {
        ...state,
        left: displayValue(state),
        right: null,
        operation: action.operation,
        phase: ENTERING,
        error: null,
        carriedEntry: true,
      }
    }

    case 'submitted': {
      const request = pendingRequest(state)
      if (request === null) return state

      return {
        ...state,
        phase: { kind: 'calculating', request, settles: 'computation' },
        error: null,
      }
    }

    case 'calculationSucceeded': {
      if (state.phase.kind !== 'calculating') return state

      if (state.phase.settles === 'entry') return writeEntry(state, action.result)

      return {
        ...state,
        left: action.result,
        right: null,
        operation: null,
        phase: RESULT,
        error: null,
        carriedEntry: true,
      }
    }

    case 'calculationFailed': {
      if (state.phase.kind !== 'calculating') return state

      // A rejected entry leaves the operand that was typed alone: it is still a valid operand
      // for the operation still waiting for it, which rejected nothing.
      if (state.phase.settles === 'entry') {
        return { ...state, phase: ENTERING, error: action.message }
      }

      return { ...state, right: null, phase: ENTERING, error: action.message }
    }
  }
}

// Stand-ins for catalog symbols a keyboard may have no key for. `^` is a dead key on Turkish
// and German layouts, so it needs a letter too. Character to character only: no operation is
// named here, so the catalog stays the only list of them (ADR-0009).
const SYMBOL_ALIASES: Record<string, string> = {
  '/': '÷',
  '*': '×',
  x: '×',
  '-': '−',
  r: '√',
  p: '^',
}

export function actionForKey(key: string, operations: Operation[]): CalculatorAction | null {
  if (/^[0-9]$/.test(key)) return { type: 'digitPressed', digit: key as Digit }
  if (key === '.' || key === ',') return { type: 'decimalPressed' }
  if (key === 'Enter' || key === '=') return { type: 'submitted' }
  if (key === 'Escape' || key === 'c' || key === 'C') return { type: 'cleared' }

  const symbol = SYMBOL_ALIASES[key.toLowerCase()] ?? key
  const operation = operations.find((candidate) => candidate.symbol === symbol)

  return operation === undefined ? null : { type: 'operationSelected', operation }
}

export function displayValue(state: CalculatorState): string {
  return state.right ?? state.left ?? ZERO
}

// The symbol follows the operand it was pressed after, which is how a binary operation is read.
// Only a binary one is ever pending: a unary operation is applied and gone (ADR-0026).
export function pendingExpression(state: CalculatorState): string | null {
  const { operation } = state
  if (operation === null) return null

  return `${state.left ?? ZERO} ${operation.symbol}`
}

export function canSubmit(state: CalculatorState): boolean {
  return pendingRequest(state) !== null
}

// Also the submit guard: a computation that cannot be described as a request is not one the UI
// offers. Only a binary operation reaches here, so `=` always needs both operands, and a
// failure that cleared `right` cannot be resent unchanged (ADR-0026 supersedes ADR-0025).
function pendingRequest(state: CalculatorState): PendingCalculation | null {
  if (state.phase.kind !== 'entering') return null

  const { operation } = state
  if (operation === null) return null

  const left = operandOf(state.left)
  const right = operandOf(state.right)
  if (left === null || right === null) return null

  return { operation: operation.id, operands: [left, right] }
}

// Number() parses the literal that was typed; every arithmetic decision, rounding included,
// stays with the server (ADR-0003, ADR-0009).
function operandOf(entry: string | null): number | null {
  if (entry === null) return null

  const operand = Number(entry)

  return Number.isFinite(operand) ? operand : null
}

function editEntry(
  state: CalculatorState,
  edit: (entry: string | null) => string,
  onCarried: 'restart' | 'continue',
): CalculatorState {
  if (state.phase.kind === 'calculating') return state

  const slot = activeSlot(state)
  const current = slot === 'right' ? state.right : state.left
  const restarting = state.carriedEntry && onCarried === 'restart'

  const edited = edit(restarting ? null : current)
  if (edited === current && !state.carriedEntry && state.error === null) return state

  const next: CalculatorState = { ...state, phase: ENTERING, error: null, carriedEntry: false }
  if (slot === 'right') next.right = edited
  else next.left = edited

  return next
}

// The slot a keystroke edits: a binary operation is waiting for its second operand, anything
// else is still building the first.
function activeSlot(state: CalculatorState): 'left' | 'right' {
  return state.operation?.arity === 2 ? 'right' : 'left'
}

// A returned entry was placed by the calculator, so the next digit restarts it (ADR-0024).
function writeEntry(state: CalculatorState, entry: string): CalculatorState {
  const next: CalculatorState = { ...state, phase: ENTERING, error: null, carriedEntry: true }
  if (activeSlot(state) === 'right') next.right = entry
  else next.left = entry

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
