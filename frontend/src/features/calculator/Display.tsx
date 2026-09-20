interface DisplayProps {
  value: string
  pending: string | null
  error: string | null
  busy: boolean
}

export function Display({ value, pending, error, busy }: DisplayProps) {
  return (
    <div className="display">
      <p className="display__pending">{pending}</p>

      <output className="display__value" aria-busy={busy}>
        {value}
      </output>

      {error !== null && (
        <p className="display__error" role="alert">
          {error}
        </p>
      )}
    </div>
  )
}
