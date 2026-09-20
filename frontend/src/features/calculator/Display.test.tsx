import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { Display } from './Display'

describe('Display', () => {
  it('renders the value it is given, verbatim', () => {
    render(<Display value="1.21932631113e+17" pending={null} error={null} busy={false} />)

    expect(screen.getByRole('status')).toHaveTextContent('1.21932631113e+17')
  })

  it('renders the pending expression above the value', () => {
    render(<Display value="4" pending="12 ÷" error={null} busy={false} />)

    expect(screen.getByText('12 ÷')).toBeInTheDocument()
    expect(screen.getByRole('status')).toHaveTextContent('4')
  })

  it('announces an error without replacing the value', () => {
    render(<Display value="1" pending="1 ÷" error="Cannot divide by zero." busy={false} />)

    expect(screen.getByRole('alert')).toHaveTextContent('Cannot divide by zero.')
    expect(screen.getByRole('status')).toHaveTextContent('1')
  })

  it('has no alert when there is no error', () => {
    render(<Display value="0" pending={null} error={null} busy={false} />)

    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('marks the value busy while a calculation is in flight', () => {
    render(<Display value="2" pending="1 +" error={null} busy />)

    expect(screen.getByRole('status')).toHaveAttribute('aria-busy', 'true')
  })
})
