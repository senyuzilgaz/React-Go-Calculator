import { useCallback, useEffect, useState } from 'react'

import { getOperations } from '../../api/client'
import { isAbortError, uiMessage } from '../../api/errors'
import type { Operation } from '../../api/types'

export interface OperationCatalogState {
  operations: Operation[]
  loading: boolean
  error: string | null
  reload: () => void
}

export function useOperations(): OperationCatalogState {
  const [attempt, setAttempt] = useState(0)
  const [operations, setOperations] = useState<Operation[]>([])
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    const controller = new AbortController()

    void (async () => {
      try {
        setOperations(await getOperations(controller.signal))
      } catch (failure) {
        if (isAbortError(failure)) return
        setError(uiMessage(failure))
      } finally {
        if (!controller.signal.aborted) setLoading(false)
      }
    })()

    return () => controller.abort()
  }, [attempt])

  const reload = useCallback(() => {
    setError(null)
    setLoading(true)
    setAttempt((attempted) => attempted + 1)
  }, [])

  return { operations, loading, error, reload }
}
