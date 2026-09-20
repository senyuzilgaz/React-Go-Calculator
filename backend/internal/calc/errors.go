package calc

import "errors"

// The domain's failure modes, each mapping to exactly one wire code (ADR-0004), so callers
// classify with errors.Is. This text is for logs; a client never sees it.
var (
	ErrUnknownOperation  = errors.New("unknown operation")
	ErrWrongOperandCount = errors.New("wrong operand count")
	ErrInvalidOperand    = errors.New("operand is not a finite number")
	ErrDivisionByZero    = errors.New("division by zero")
	ErrNegativeSqrt      = errors.New("square root of a negative number")
	ErrResultOverflow    = errors.New("result is not a finite number")
)
