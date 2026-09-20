import type { Operation } from '../../api/types'
import type { Digit } from './reducer'

const DIGITS: Digit[] = ['7', '8', '9', '4', '5', '6', '1', '2', '3']

interface KeypadProps {
  operations: Operation[]
  loading: boolean
  error: string | null
  onRetry: () => void
  disabled: boolean
  canSubmit: boolean
  onDigit: (digit: Digit) => void
  onDecimal: () => void
  onSign: () => void
  onClear: () => void
  onOperation: (operation: Operation) => void
  onSubmit: () => void
}

export function Keypad({
  operations,
  loading,
  error,
  onRetry,
  disabled,
  canSubmit,
  onDigit,
  onDecimal,
  onSign,
  onClear,
  onOperation,
  onSubmit,
}: KeypadProps) {
  return (
    <div className="keypad">
      <section className="keypad__operations" aria-label="Operations" aria-busy={loading}>
        {loading && <p className="keypad__status">Loading operations…</p>}

        {error !== null && (
          <div className="keypad__status keypad__status--error" role="alert">
            <p>{error}</p>
            <button type="button" className="keypad__retry" onClick={onRetry}>
              Try again
            </button>
          </div>
        )}

        {operations.map((operation) => (
          <button
            key={operation.id}
            type="button"
            className="key key--operation"
            aria-label={operation.name}
            disabled={disabled}
            onClick={() => onOperation(operation)}
          >
            {operation.symbol}
          </button>
        ))}
      </section>

      <section className="keypad__keys" aria-label="Digits and controls">
        {DIGITS.map((digit) => (
          <button
            key={digit}
            type="button"
            className="key"
            disabled={disabled}
            onClick={() => onDigit(digit)}
          >
            {digit}
          </button>
        ))}

        <button
          type="button"
          className="key"
          aria-label="Decimal point"
          disabled={disabled}
          onClick={onDecimal}
        >
          .
        </button>
        <button
          type="button"
          className="key"
          disabled={disabled}
          onClick={() => onDigit('0')}
        >
          0
        </button>
        <button
          type="button"
          className="key"
          aria-label="Toggle sign"
          disabled={disabled}
          onClick={onSign}
        >
          ±
        </button>

        <button type="button" className="key key--clear" aria-label="Clear" onClick={onClear}>
          C
        </button>
        <button
          type="button"
          className="key key--submit"
          aria-label="Equals"
          disabled={!canSubmit}
          onClick={onSubmit}
        >
          =
        </button>
      </section>
    </div>
  )
}
