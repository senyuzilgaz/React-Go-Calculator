import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { ComponentProps } from 'react'
import { describe, expect, it, vi } from 'vitest'

import { CATALOG } from '../../test/catalog'
import { operationById } from '../../test/operations'
import { Keypad } from './Keypad'

type KeypadProps = ComponentProps<typeof Keypad>

function renderKeypad(overrides: Partial<KeypadProps> = {}) {
  const props: KeypadProps = {
    operations: CATALOG.operations,
    loading: false,
    error: null,
    onRetry: vi.fn(),
    disabled: false,
    canSubmit: false,
    onDigit: vi.fn(),
    onDecimal: vi.fn(),
    onSign: vi.fn(),
    onClear: vi.fn(),
    onOperation: vi.fn(),
    onSubmit: vi.fn(),
    ...overrides,
  }

  render(<Keypad {...props} />)

  return { props, user: userEvent.setup() }
}

const operationKeys = () =>
  within(screen.getByRole('region', { name: 'Operations' })).getAllByRole('button')

describe('Keypad', () => {
  it('renders one key per operation in the catalog it is given', () => {
    renderKeypad()

    expect(operationKeys().map((key) => key.textContent)).toEqual(
      CATALOG.operations.map((operation) => operation.symbol),
    )
    expect(operationKeys().map((key) => key.getAttribute('aria-label'))).toEqual(
      CATALOG.operations.map((operation) => operation.name),
    )
  })

  it('renders only the operations it is given', () => {
    renderKeypad({ operations: [operationById('add'), operationById('sqrt')] })

    expect(operationKeys().map((key) => key.textContent)).toEqual(['+', '√'])
  })

  it('reports the operation that was pressed, not its identifier', async () => {
    const { props, user } = renderKeypad()

    await user.click(screen.getByRole('button', { name: 'Division' }))

    expect(props.onOperation).toHaveBeenCalledWith(operationById('divide'))
  })

  it('reports each digit that was pressed', async () => {
    const { props, user } = renderKeypad()

    for (const digit of ['0', '1', '2', '3', '4', '5', '6', '7', '8', '9']) {
      await user.click(screen.getByRole('button', { name: digit }))
    }

    expect(props.onDigit).toHaveBeenCalledTimes(10)
    expect(props.onDigit).toHaveBeenNthCalledWith(1, '0')
    expect(props.onDigit).toHaveBeenLastCalledWith('9')
  })

  it('reports the control keys', async () => {
    const { props, user } = renderKeypad()

    await user.click(screen.getByRole('button', { name: 'Decimal point' }))
    await user.click(screen.getByRole('button', { name: 'Toggle sign' }))
    await user.click(screen.getByRole('button', { name: 'Clear' }))

    expect(props.onDecimal).toHaveBeenCalledOnce()
    expect(props.onSign).toHaveBeenCalledOnce()
    expect(props.onClear).toHaveBeenCalledOnce()
  })

  it('offers equals only when the computation can be submitted', async () => {
    const { props, user } = renderKeypad({ canSubmit: false })

    expect(screen.getByRole('button', { name: 'Equals' })).toBeDisabled()

    await user.click(screen.getByRole('button', { name: 'Equals' }))
    expect(props.onSubmit).not.toHaveBeenCalled()
  })

  it('submits when the computation is complete', async () => {
    const { props, user } = renderKeypad({ canSubmit: true })

    await user.click(screen.getByRole('button', { name: 'Equals' }))

    expect(props.onSubmit).toHaveBeenCalledOnce()
  })

  it('disables every key except clear while a calculation is in flight', () => {
    renderKeypad({ disabled: true })

    expect(screen.getByRole('button', { name: '7' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Division' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Toggle sign' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Clear' })).toBeEnabled()
  })

  it('renders the digits while the catalog is still loading', () => {
    renderKeypad({ operations: [], loading: true })

    expect(screen.getByRole('button', { name: '7' })).toBeEnabled()
    expect(screen.getByRole('button', { name: 'Clear' })).toBeEnabled()
    expect(screen.getByText('Loading operations…')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Division' })).not.toBeInTheDocument()
  })

  it('offers a retry when the catalog could not be loaded', async () => {
    const { props, user } = renderKeypad({
      operations: [],
      error: 'Cannot reach the calculator service.',
    })

    expect(screen.getByRole('alert')).toHaveTextContent('Cannot reach the calculator service.')

    await user.click(screen.getByRole('button', { name: 'Try again' }))

    expect(props.onRetry).toHaveBeenCalledOnce()
  })
})
