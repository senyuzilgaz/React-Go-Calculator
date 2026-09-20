import { Display } from './features/calculator/Display'
import { Keypad } from './features/calculator/Keypad'
import { useCalculator } from './features/calculator/useCalculator'
import { useKeyboard } from './features/calculator/useKeyboard'
import { useOperations } from './features/calculator/useOperations'
import './App.css'
import './features/calculator/calculator.css'

export default function App() {
  const { dispatch, status, display, pending, canSubmit, error } = useCalculator()
  const catalog = useOperations()

  useKeyboard(catalog.operations, dispatch)

  const calculating = status === 'calculating'

  return (
    <main className="app">
      <h1 className="app__title">Calculator</h1>

      <Display value={display} pending={pending} error={error} busy={calculating} />

      <Keypad
        operations={catalog.operations}
        loading={catalog.loading}
        error={catalog.error}
        onRetry={catalog.reload}
        disabled={calculating}
        canSubmit={canSubmit}
        onDigit={(digit) => dispatch({ type: 'digitPressed', digit })}
        onDecimal={() => dispatch({ type: 'decimalPressed' })}
        onSign={() => dispatch({ type: 'signToggled' })}
        onClear={() => dispatch({ type: 'cleared' })}
        onOperation={(operation) => dispatch({ type: 'operationSelected', operation })}
        onSubmit={() => dispatch({ type: 'submitted' })}
      />
    </main>
  )
}
