package calc

import "errors"

// The domain's failure modes. Each maps to exactly one wire code in the transport layer
// (ADR-0004), so callers classify with errors.Is. Message text is written for logs and is
// never surfaced to a client, which sees only the code the transport layer chooses.
var (
	ErrUnknownOperation  = errors.New("unknown operation")
	ErrWrongOperandCount = errors.New("wrong operand count")
	ErrInvalidOperand    = errors.New("operand is not a finite number")
	ErrDivisionByZero    = errors.New("division by zero")
	ErrNegativeSqrt      = errors.New("square root of a negative number")
	ErrResultOverflow    = errors.New("result is not a finite number")
)
