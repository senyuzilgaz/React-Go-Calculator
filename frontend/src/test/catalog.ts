// Extracted from docs/API_EXAMPLES.md by script, not retyped.

import type { OperationCatalog } from '../api/types'

export const CATALOG: OperationCatalog = {
  operations: [
    {
      id: 'add',
      name: 'Addition',
      symbol: '+',
      arity: 2,
      parameters: [
        {
          name: 'augend',
          description: 'The value added to.',
        },
        {
          name: 'addend',
          description: 'The value added.',
        },
      ],
      endpoint: '/api/v1/operations/add',
    },
    {
      id: 'subtract',
      name: 'Subtraction',
      symbol: '−',
      arity: 2,
      parameters: [
        {
          name: 'minuend',
          description: 'The value subtracted from.',
        },
        {
          name: 'subtrahend',
          description: 'The value subtracted.',
        },
      ],
      endpoint: '/api/v1/operations/subtract',
    },
    {
      id: 'multiply',
      name: 'Multiplication',
      symbol: '×',
      arity: 2,
      parameters: [
        {
          name: 'multiplicand',
          description: 'The value multiplied.',
        },
        {
          name: 'multiplier',
          description: 'The number of times it is taken.',
        },
      ],
      endpoint: '/api/v1/operations/multiply',
    },
    {
      id: 'divide',
      name: 'Division',
      symbol: '÷',
      arity: 2,
      parameters: [
        {
          name: 'dividend',
          description: 'The value divided.',
        },
        {
          name: 'divisor',
          description: 'The value divided by. Must not be zero.',
        },
      ],
      endpoint: '/api/v1/operations/divide',
    },
    {
      id: 'power',
      name: 'Exponentiation',
      symbol: '^',
      arity: 2,
      parameters: [
        {
          name: 'base',
          description: 'The value raised to a power.',
        },
        {
          name: 'exponent',
          description: 'The power the base is raised to.',
        },
      ],
      endpoint: '/api/v1/operations/power',
    },
    {
      id: 'sqrt',
      name: 'Square root',
      symbol: '√',
      arity: 1,
      parameters: [
        {
          name: 'radicand',
          description: 'The value whose square root is taken. Must not be negative.',
        },
      ],
      endpoint: '/api/v1/operations/sqrt',
    },
    {
      id: 'percent',
      name: 'Percentage',
      symbol: '%',
      arity: 2,
      parameters: [
        {
          name: 'percentage',
          description: 'The percentage to take.',
        },
        {
          name: 'value',
          description: 'The value the percentage is taken of.',
        },
      ],
      endpoint: '/api/v1/operations/percent',
    },
  ],
}
