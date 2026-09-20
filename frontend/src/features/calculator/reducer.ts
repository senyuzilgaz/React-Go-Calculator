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

export const initialState: CalculatorState = {
  left: null,
  right: null,
  operation: null,
  phase: { kind: 'entering' },
  error: null,
}

export function calculatorReducer(
  _state: CalculatorState,
  _action: CalculatorAction,
): CalculatorState {
  throw new Error('calculatorReducer is not implemented')
}

export function displayValue(_state: CalculatorState): string {
  throw new Error('displayValue is not implemented')
}

export function canSubmit(_state: CalculatorState): boolean {
  throw new Error('canSubmit is not implemented')
}
