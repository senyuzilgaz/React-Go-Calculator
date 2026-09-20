import { describe, expect, it } from 'vitest'

import type { Operation } from '../../api/types'
import { operationById } from '../../test/operations'
import {
  calculatorReducer,
  canSubmit,
  displayValue,
  initialState,
  type CalculatorAction,
  type CalculatorState,
  type Digit,
} from './reducer'

const ADD = operationById('add')
const DIVIDE = operationById('divide')
const SQRT = operationById('sqrt')

const select = (operation: Operation): CalculatorAction => ({ type: 'operationSelected', operation })
const submit: CalculatorAction = { type: 'submitted' }
const sign: CalculatorAction = { type: 'signToggled' }
const clear: CalculatorAction = { type: 'cleared' }
const succeeded = (result: string): CalculatorAction => ({ type: 'calculationSucceeded', result })
const failed = (message: string): CalculatorAction => ({ type: 'calculationFailed', message })

function keys(entry: string): CalculatorAction[] {
  return [...entry].map((character) =>
    character === '.'
      ? { type: 'decimalPressed' }
      : { type: 'digitPressed', digit: character as Digit },
  )
}

function run(actions: CalculatorAction[], from: CalculatorState = initialState): CalculatorState {
  return actions.reduce(calculatorReducer, from)
}

describe('entering an operand', () => {
  it('starts at zero with nothing to submit', () => {
    expect(displayValue(initialState)).toBe('0')
    expect(canSubmit(initialState)).toBe(false)
  })

  it('accumulates digits', () => {
    expect(displayValue(run(keys('12')))).toBe('12')
  })

  it('replaces a leading zero', () => {
    expect(displayValue(run(keys('05')))).toBe('5')
  })

  it('does not accumulate zeros', () => {
    expect(displayValue(run(keys('000')))).toBe('0')
  })

  it('starts a decimal entry from zero', () => {
    expect(displayValue(run(keys('.5')))).toBe('0.5')
  })

  it('accepts one decimal point only', () => {
    expect(displayValue(run(keys('1.2.3')))).toBe('1.23')
  })

  it('leaves the state untouched when the decimal point is redundant', () => {
    const state = run(keys('1.'))

    expect(calculatorReducer(state, { type: 'decimalPressed' })).toBe(state)
  })

  it('toggles the sign of the entry', () => {
    expect(displayValue(run([...keys('12'), sign]))).toBe('-12')
    expect(displayValue(run([...keys('12'), sign, sign]))).toBe('12')
  })

  it('toggles the sign of an entry not yet started', () => {
    expect(displayValue(run([sign]))).toBe('-0')
  })

  it('replaces the zero of a negative entry', () => {
    expect(displayValue(run([sign, ...keys('5')]))).toBe('-5')
  })

  it('returns to the initial state when cleared', () => {
    expect(run([...keys('12'), clear])).toEqual(initialState)
  })
})

describe('choosing an operation', () => {
  it('commits the displayed value as the left operand', () => {
    const state = run([...keys('12'), select(DIVIDE)])

    expect(state.left).toBe('12')
    expect(state.operation).toBe(DIVIDE)
    expect(state.right).toBeNull()
    expect(displayValue(state)).toBe('12')
  })

  it('directs the following digits into the second operand', () => {
    const state = run([...keys('12'), select(DIVIDE), ...keys('4')])

    expect(state.left).toBe('12')
    expect(state.right).toBe('4')
    expect(displayValue(state)).toBe('4')
  })

  it('takes the displayed value as the new left operand when another operation follows', () => {
    const state = run([...keys('12'), select(DIVIDE), ...keys('4'), select(ADD)])

    expect(state.left).toBe('4')
    expect(state.right).toBeNull()
    expect(state.operation).toBe(ADD)
  })

  it('defaults the left operand to zero when nothing was entered', () => {
    expect(run([select(ADD)]).left).toBe('0')
  })

  it('is not submittable until the second operand exists', () => {
    expect(canSubmit(run([...keys('12'), select(DIVIDE)]))).toBe(false)
    expect(canSubmit(run([...keys('12'), select(DIVIDE), ...keys('4')]))).toBe(true)
  })

  it('is submittable at once for a unary operation', () => {
    expect(canSubmit(run([...keys('9'), select(SQRT)]))).toBe(true)
  })

  it('is not submittable without an operation', () => {
    expect(canSubmit(run(keys('12')))).toBe(false)
  })
})

describe('submitting', () => {
  const inFlight = () => run([...keys('10'), select(DIVIDE), ...keys('4'), submit])

  it('requests one calculation, with the operands in the order the catalog names them', () => {
    expect(inFlight().phase).toEqual({
      kind: 'calculating',
      request: { operation: 'divide', operands: [10, 4] },
    })
  })

  it('requests a single operand for a unary operation', () => {
    expect(run([...keys('9'), select(SQRT), submit]).phase).toEqual({
      kind: 'calculating',
      request: { operation: 'sqrt', operands: [9] },
    })
  })

  it('ignores a second = while the first is in flight', () => {
    const state = inFlight()

    expect(calculatorReducer(state, submit)).toBe(state)
  })

  it('ignores every key while a calculation is in flight', () => {
    const state = inFlight()

    for (const action of [...keys('7.'), sign, select(ADD)]) {
      expect(calculatorReducer(state, action)).toBe(state)
    }
  })

  it('ignores = while the computation is incomplete', () => {
    const state = run([...keys('12'), select(DIVIDE)])

    expect(calculatorReducer(state, submit)).toBe(state)
  })

  it('abandons a calculation in flight when cleared', () => {
    expect(calculatorReducer(inFlight(), clear)).toEqual(initialState)
  })
})

describe('a returned result', () => {
  const calculated = (result: string) =>
    run([...keys('0.1'), select(ADD), ...keys('0.2'), submit, succeeded(result)])

  it('is displayed verbatim', () => {
    expect(displayValue(calculated('0.3'))).toBe('0.3')
  })

  it('is not reformatted out of scientific notation', () => {
    expect(displayValue(calculated('1.21932631113e+17'))).toBe('1.21932631113e+17')
  })

  it('completes the computation', () => {
    const state = calculated('0.3')

    expect(state.phase).toEqual({ kind: 'result' })
    expect(state.operation).toBeNull()
    expect(state.right).toBeNull()
    expect(canSubmit(state)).toBe(false)
  })

  it('becomes the left operand of the operation pressed next', () => {
    const state = calculatorReducer(calculated('0.3'), select(ADD))

    expect(state.left).toBe('0.3')
    expect(state.operation).toBe(ADD)
    expect(state.right).toBeNull()
  })

  it('is discarded when a digit starts a new entry', () => {
    const state = calculatorReducer(calculated('0.3'), { type: 'digitPressed', digit: '7' })

    expect(displayValue(state)).toBe('7')
    expect(state.phase).toEqual({ kind: 'entering' })
  })

  it('is discarded when a decimal point starts a new entry', () => {
    expect(displayValue(calculatorReducer(calculated('0.3'), { type: 'decimalPressed' }))).toBe('0.')
  })

  it('is negated as the next operand rather than recomputed', () => {
    const state = calculatorReducer(calculated('0.3'), sign)

    expect(displayValue(state)).toBe('-0.3')
    expect(state.phase).toEqual({ kind: 'entering' })
  })

  it('is ignored when it arrives after a clear', () => {
    const state = run([...keys('1'), select(ADD), ...keys('2'), submit, clear])

    expect(calculatorReducer(state, succeeded('3'))).toBe(state)
  })
})

describe('a returned failure', () => {
  const rejected = () =>
    run([...keys('1'), select(DIVIDE), ...keys('0'), submit, failed('Cannot divide by zero.')])

  it('surfaces the message and ends the calculation', () => {
    const state = rejected()

    expect(state.error).toBe('Cannot divide by zero.')
    expect(state.phase).toEqual({ kind: 'entering' })
  })

  it('keeps the operation and the left operand, and clears the one to retype', () => {
    const state = rejected()

    expect(state.left).toBe('1')
    expect(state.operation).toBe(DIVIDE)
    expect(state.right).toBeNull()
    expect(displayValue(state)).toBe('1')
    expect(canSubmit(state)).toBe(false)
  })

  it('is submittable again once the operand is retyped', () => {
    const state = run(keys('4'), rejected())

    expect(canSubmit(state)).toBe(true)
    expect(state.error).toBeNull()
  })

  it('is ignored when it arrives after a clear', () => {
    const state = run([...keys('1'), select(ADD), ...keys('2'), submit, clear])

    expect(calculatorReducer(state, failed('Cannot divide by zero.'))).toBe(state)
  })
})

describe('no client-side arithmetic (ADR-0009)', () => {
  it('sends what was typed and shows no result until the server returns one', () => {
    const submitted = run([...keys('0.1'), select(ADD), ...keys('0.2'), submit])

    expect(submitted.phase).toEqual({
      kind: 'calculating',
      request: { operation: 'add', operands: [0.1, 0.2] },
    })
    expect(displayValue(submitted)).toBe('0.2')

    expect(displayValue(calculatorReducer(submitted, succeeded('0.3')))).toBe('0.3')
  })

  it('does not normalize an operand as it is entered', () => {
    expect(displayValue(run(keys('1.500')))).toBe('1.500')
  })
})
