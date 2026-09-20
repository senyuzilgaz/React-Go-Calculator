package calc

import "errors"

// Each maps to one wire code (ADR-0004); callers classify with errors.Is. The text is for
// logs and never reaches a client.
var (
	ErrUnknownOperation  = errors.New("unknown operation")
	ErrWrongOperandCount = errors.New("wrong operand count")
	ErrInvalidOperand    = errors.New("operand is not a finite number")
	ErrDivisionByZero    = errors.New("division by zero")
	ErrNegativeSqrt      = errors.New("square root of a negative number")
	ErrResultOverflow    = errors.New("result is not a finite number")
)
