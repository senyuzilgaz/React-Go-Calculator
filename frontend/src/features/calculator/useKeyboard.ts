import { useEffect, type Dispatch } from 'react'

import type { Operation } from '../../api/types'
import { actionForKey, type CalculatorAction } from './reducer'

export function useKeyboard(operations: Operation[], dispatch: Dispatch<CalculatorAction>): void {
  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.metaKey || event.ctrlKey || event.altKey) return
      if (belongsToTheFocusedElement(event.target, event.key)) return

      const action = actionForKey(event.key, operations)
      if (action === null) return

      event.preventDefault()
      dispatch(action)
    }

    window.addEventListener('keydown', onKeyDown)

    return () => window.removeEventListener('keydown', onKeyDown)
  }, [operations, dispatch])
}

// Enter and Space activate whichever key already has focus, which is what a button is for;
// stealing them would break keyboard navigation of the keypad. Every other key is the
// calculator's, and `=` submits regardless of what is focused.
function belongsToTheFocusedElement(target: EventTarget | null, key: string): boolean {
  if (!(target instanceof HTMLElement)) return false
  if (target.isContentEditable) return true

  const { tagName } = target
  if (tagName === 'INPUT' || tagName === 'TEXTAREA' || tagName === 'SELECT') return true

  return tagName === 'BUTTON' && (key === 'Enter' || key === ' ')
}
