import { useEffect, useState } from 'react'

import { getOperations } from './api/client'
import { isAbortError, uiMessage } from './api/errors'
import type { Operation } from './api/types'
import './App.css'

// Operation keys come from the catalog, so a new operation needs no change here (ADR-0009).
// The keys stay inert until the reducer that drives them exists.
export default function App() {
  const [operations, setOperations] = useState<Operation[]>([])
  const [failure, setFailure] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    const controller = new AbortController()

    void (async () => {
      try {
        setOperations(await getOperations(controller.signal))
      } catch (error) {
        if (isAbortError(error)) return
        setFailure(uiMessage(error))
      } finally {
        if (!controller.signal.aborted) setLoading(false)
      }
    })()

    return () => controller.abort()
  }, [])

  return (
    <main className="app">
      <h1>Calculator</h1>

      {failure !== null && (
        <p className="banner" role="alert">
          {failure}
        </p>
      )}

      <section className="operations" aria-busy={loading} aria-label="Operations">
        {loading && <p className="status">Loading operations…</p>}
        {operations.map((operation) => (
          <button key={operation.id} type="button" aria-label={operation.name} disabled>
            {operation.symbol}
          </button>
        ))}
      </section>
    </main>
  )
}
