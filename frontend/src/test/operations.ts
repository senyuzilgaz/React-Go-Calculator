import type { KnownOperationId, Operation } from '../api/types'
import { CATALOG } from './catalog'

export function operationById(id: KnownOperationId): Operation {
  const operation = CATALOG.operations.find((candidate) => candidate.id === id)
  if (!operation) throw new Error(`the catalog fixture has no operation '${id}'`)
  return operation
}
