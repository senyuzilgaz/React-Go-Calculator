import { useEffect, useReducer, type Dispatch } from 'react'

import { calculate } from '../../api/client'
import { isAbortError, uiMessage } from '../../api/errors'
import {
  calculatorReducer,
  canSubmit,
  displayValue,
  initialState,
  type CalculatorAction,
  type CalculatorPhase,
  type CalculatorState,
} from './reducer'

export interface Calculator {
  state: CalculatorState
  dispatch: Dispatch<CalculatorAction>
  status: CalculatorPhase['kind']
  display: string
  canSubmit: boolean
  error: string | null
}

export function useCalculator(): Calculator {
  const [state, dispatch] = useReducer(calculatorReducer, initialState)
  const { phase } = state

  // The phase is the dependency because it changes identity exactly once per calculation:
  // the reducer returns the state it was given for anything it ignores, so a key pressed
  // mid-flight cannot re-run this effect and send a second request. The cleanup cancels a
  // request the user walked away from, by clearing or by unmounting.
  useEffect(() => {
    if (phase.kind !== 'calculating') return

    const controller = new AbortController()
    const { operation, operands } = phase.request

    void (async () => {
      try {
        const { result } = await calculate(operation, operands, controller.signal)
        dispatch({ type: 'calculationSucceeded', result })
      } catch (error) {
        if (isAbortError(error)) return
        dispatch({ type: 'calculationFailed', message: uiMessage(error) })
      }
    })()

    return () => controller.abort()
  }, [phase])

  return {
    state,
    dispatch,
    status: phase.kind,
    display: displayValue(state),
    canSubmit: canSubmit(state),
    error: state.error,
  }
}
