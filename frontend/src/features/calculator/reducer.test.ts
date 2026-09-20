import { describe, expect, it } from 'vitest'

import type { Operation } from '../../api/types'
import { operationById } from '../../test/operations'
import { CATALOG } from '../../test/catalog'
import {
  actionForKey,
  calculatorReducer,
  canSubmit,
  displayValue,
  initialState,
  pendingExpression,
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

describe('an operand carried into a unary operation', () => {
  const calculating = (request: { operation: string; operands: number[] }) => ({
    kind: 'calculating',
    request,
  })

  it('is replaced by the digits typed after the operation', () => {
    const state = run([...keys('5'), select(SQRT), ...keys('3')])

    expect(state.left).toBe('3')
    expect(displayValue(state)).toBe('3')
  })

  it('keeps collecting the digits of that new entry', () => {
    expect(displayValue(run([...keys('5'), select(SQRT), ...keys('12')]))).toBe('12')
  })

  it('is replaced even by a digit it already reads as', () => {
    expect(displayValue(run([...keys('5'), select(SQRT), ...keys('55')]))).toBe('55')
  })

  it('is replaced by a decimal point starting a new entry', () => {
    expect(displayValue(run([...keys('5'), select(SQRT), ...keys('.5')]))).toBe('0.5')
  })

  it('is computed on when the operation is submitted untouched', () => {
    expect(run([...keys('12'), select(SQRT), submit]).phase).toEqual(
      calculating({ operation: 'sqrt', operands: [12] }),
    )
  })

  it('is computed on with its sign toggled rather than replaced', () => {
    expect(displayValue(run([...keys('5'), select(SQRT), sign]))).toBe('-5')
  })

  it('never joins the operand typed after it into one number', () => {
    expect(run([...keys('5'), select(SQRT), ...keys('3'), submit]).phase).toEqual(
      calculating({ operation: 'sqrt', operands: [3] }),
    )
  })

  it('applies to the last operand entered, one operation at a time (ADR-0009)', () => {
    const state = run([select(SQRT), ...keys('2'), select(ADD), select(SQRT), ...keys('2'), submit])

    expect(state.phase).toEqual(calculating({ operation: 'sqrt', operands: [2] }))
  })
})

describe('the pending expression', () => {
  it('is absent until an operation is chosen', () => {
    expect(pendingExpression(initialState)).toBeNull()
    expect(pendingExpression(run(keys('12')))).toBeNull()
  })

  it('follows the operand for a binary operation', () => {
    expect(pendingExpression(run([...keys('12'), select(DIVIDE)]))).toBe('12 ÷')
  })

  it('stays visible while the second operand is entered', () => {
    expect(pendingExpression(run([...keys('12'), select(DIVIDE), ...keys('4')]))).toBe('12 ÷')
  })

  it('precedes the operand for a unary operation', () => {
    expect(pendingExpression(run([...keys('9'), select(SQRT)]))).toBe('√ 9')
  })

  it('is gone once a result returns', () => {
    const state = run([...keys('1'), select(DIVIDE), ...keys('4'), submit, succeeded('0.25')])

    expect(pendingExpression(state)).toBeNull()
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

  const rejectedRoot = () =>
    run([...keys('4'), sign, select(SQRT), submit, failed('Cannot take the square root…')])

  it('keeps a unary operand on display, since there is no second one to clear', () => {
    const state = rejectedRoot()

    expect(state.left).toBe('-4')
    expect(state.operation).toBe(SQRT)
    expect(displayValue(state)).toBe('-4')
  })

  it('cannot be resent unchanged when the rejected operand is the one on display', () => {
    expect(canSubmit(rejectedRoot())).toBe(false)
  })

  it('is submittable again once a unary operand is corrected in place', () => {
    const state = calculatorReducer(rejectedRoot(), sign)

    expect(displayValue(state)).toBe('4')
    expect(canSubmit(state)).toBe(true)
    expect(state.error).toBeNull()
  })

  it('is submittable again once a unary operand is retyped', () => {
    const state = run(keys('9'), rejectedRoot())

    expect(displayValue(state)).toBe('9')
    expect(canSubmit(state)).toBe(true)
  })
})

describe('the action a keystroke maps to', () => {
  const OPERATIONS = CATALOG.operations
  const keyed = (key: string) => actionForKey(key, OPERATIONS)

  it('maps every digit', () => {
    for (const digit of ['0', '1', '2', '3', '4', '5', '6', '7', '8', '9']) {
      expect(keyed(digit)).toEqual({ type: 'digitPressed', digit })
    }
  })

  it('maps both decimal separators', () => {
    expect(keyed('.')).toEqual({ type: 'decimalPressed' })
    expect(keyed(',')).toEqual({ type: 'decimalPressed' })
  })

  it('maps enter and equals to submitting', () => {
    expect(keyed('Enter')).toEqual({ type: 'submitted' })
    expect(keyed('=')).toEqual({ type: 'submitted' })
  })

  it('maps escape and c to clearing', () => {
    expect(keyed('Escape')).toEqual({ type: 'cleared' })
    expect(keyed('c')).toEqual({ type: 'cleared' })
    expect(keyed('C')).toEqual({ type: 'cleared' })
  })

  it('maps a typed operator onto the operation whose symbol it stands for', () => {
    expect(keyed('+')).toEqual({ type: 'operationSelected', operation: ADD })
    expect(keyed('-')).toEqual({ type: 'operationSelected', operation: operationById('subtract') })
    expect(keyed('*')).toEqual({ type: 'operationSelected', operation: operationById('multiply') })
    expect(keyed('x')).toEqual({ type: 'operationSelected', operation: operationById('multiply') })
    expect(keyed('/')).toEqual({ type: 'operationSelected', operation: DIVIDE })
    expect(keyed('^')).toEqual({ type: 'operationSelected', operation: operationById('power') })
    expect(keyed('r')).toEqual({ type: 'operationSelected', operation: SQRT })
    expect(keyed('%')).toEqual({ type: 'operationSelected', operation: operationById('percent') })
  })

  it('maps a catalog symbol that was typed directly', () => {
    expect(keyed('÷')).toEqual({ type: 'operationSelected', operation: DIVIDE })
  })

  it('maps an operation this frontend has never heard of, by its symbol', () => {
    const modulo = { ...ADD, id: 'modulo', name: 'Modulo', symbol: '#' }

    expect(actionForKey('#', [...OPERATIONS, modulo])).toEqual({
      type: 'operationSelected',
      operation: modulo,
    })
  })

  it('maps nothing for a key the calculator has no use for', () => {
    for (const key of ['a', 'Shift', 'F1', 'ArrowLeft', 'Backspace']) {
      expect(keyed(key)).toBeNull()
    }
  })

  it('maps no operator while the catalog is empty', () => {
    expect(actionForKey('/', [])).toBeNull()
    expect(actionForKey('7', [])).toEqual({ type: 'digitPressed', digit: '7' })
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
