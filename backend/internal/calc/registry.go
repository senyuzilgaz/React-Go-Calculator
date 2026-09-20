// Package calc holds the arithmetic domain: the operation registry, the operations
// themselves, and the rounding policy that turns a result into its display form.
//
// It knows nothing of HTTP, JSON, or status codes. Failures are reported as domain errors
// for the transport layer to classify (ADR-0004, ADR-0007).
package calc

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Parameter is one positional operand, described by the role it plays in the operation.
type Parameter struct {
	Name        string
	Description string
}

// Operation is a single entry in the registry: the metadata the catalog publishes and the
// arithmetic behind it.
type Operation struct {
	ID         string
	Name       string
	Symbol     string
	Arity      int
	Parameters []Parameter

	// Apply assumes len(operands) == Arity and that every operand is finite; it indexes
	// operands directly and does not re-check. Apply is the guarded entry point that
	// establishes both, and is what callers outside this package should use.
	Apply func(operands []float64) (float64, error)
}

// catalog is the single source of truth for routing, arity validation, and the catalog
// response (ADR-0002). Adding an operation is one entry here and no other change; order is
// the order the catalog publishes.
var catalog = []Operation{
	{
		ID:     "add",
		Name:   "Addition",
		Symbol: "+",
		Arity:  2,
		Parameters: []Parameter{
			{Name: "augend", Description: "The value added to."},
			{Name: "addend", Description: "The value added."},
		},
		Apply: func(operands []float64) (float64, error) {
			return operands[0] + operands[1], nil
		},
	},
	{
		ID:     "subtract",
		Name:   "Subtraction",
		Symbol: "−",
		Arity:  2,
		Parameters: []Parameter{
			{Name: "minuend", Description: "The value subtracted from."},
			{Name: "subtrahend", Description: "The value subtracted."},
		},
		Apply: func(operands []float64) (float64, error) {
			return operands[0] - operands[1], nil
		},
	},
	{
		ID:     "multiply",
		Name:   "Multiplication",
		Symbol: "×",
		Arity:  2,
		Parameters: []Parameter{
			{Name: "multiplicand", Description: "The value multiplied."},
			{Name: "multiplier", Description: "The number of times it is taken."},
		},
		Apply: func(operands []float64) (float64, error) {
			return operands[0] * operands[1], nil
		},
	},
	{
		ID:     "divide",
		Name:   "Division",
		Symbol: "÷",
		Arity:  2,
		Parameters: []Parameter{
			{Name: "dividend", Description: "The value divided."},
			{Name: "divisor", Description: "The value divided by. Must not be zero."},
		},
		Apply: func(operands []float64) (float64, error) {
			// Catches negative zero too, which would otherwise divide to -Inf.
			if operands[1] == 0 {
				return 0, ErrDivisionByZero
			}
			return operands[0] / operands[1], nil
		},
	},
	{
		ID:     "power",
		Name:   "Exponentiation",
		Symbol: "^",
		Arity:  2,
		Parameters: []Parameter{
			{Name: "base", Description: "The value raised to a power."},
			{Name: "exponent", Description: "The power the base is raised to."},
		},
		Apply: func(operands []float64) (float64, error) {
			return math.Pow(operands[0], operands[1]), nil
		},
	},
	{
		ID:     "sqrt",
		Name:   "Square root",
		Symbol: "√",
		Arity:  1,
		Parameters: []Parameter{
			{Name: "radicand", Description: "The value whose square root is taken. Must not be negative."},
		},
		Apply: func(operands []float64) (float64, error) {
			// Negative zero is not a negative number: math.Sqrt(-0) is -0, which Format
			// normalizes. A sign-bit test here would reject it.
			if operands[0] < 0 {
				return 0, ErrNegativeSqrt
			}
			return math.Sqrt(operands[0]), nil
		},
	},
	{
		ID:     "percent",
		Name:   "Percentage",
		Symbol: "%",
		Arity:  2,
		Parameters: []Parameter{
			{Name: "percentage", Description: "The percentage to take."},
			{Name: "value", Description: "The value the percentage is taken of."},
		},
		Apply: func(operands []float64) (float64, error) {
			// a x b / 100, "a percent of b" (ADR-0010). The product is formed first, so a
			// large enough pair overflows even where the true result is representable.
			return operands[0] * operands[1] / 100, nil
		},
	},
}

// registry is the only dispatch in the package: an operation is found by identifier or it
// does not exist. There is no branch anywhere that names an operation.
var registry = func() map[string]Operation {
	byID := make(map[string]Operation, len(catalog))
	for _, operation := range catalog {
		byID[operation.ID] = operation
	}
	return byID
}()

// Catalog returns every registered operation, in published order. The returned slice is
// the caller's own, so reordering or editing it cannot corrupt the registry.
func Catalog() []Operation {
	return append([]Operation(nil), catalog...)
}

// Lookup resolves an operation identifier exactly; there is no normalization or near-match.
func Lookup(id string) (Operation, bool) {
	operation, ok := registry[id]
	return operation, ok
}

// Apply resolves an operation and runs it against operands.
//
// The guards run in a fixed order — existence, arity, operand finiteness, then the
// computation — because each failure carries a different meaning, and the transport layer
// maps each to a different status (ADR-0004). A request that fails several at once is
// reported by the first.
func Apply(id string, operands []float64) (float64, error) {
	operation, ok := Lookup(id)
	if !ok {
		return 0, fmt.Errorf("%q: %w", id, ErrUnknownOperation)
	}

	if len(operands) != operation.Arity {
		return 0, fmt.Errorf("%s: arity %d, received %d operands: %w",
			operation.ID, operation.Arity, len(operands), ErrWrongOperandCount)
	}

	for i, operand := range operands {
		// Positions are 1-based, matching the published message "Operand at position 1 is
		// not a finite number" for the first operand (api/openapi.yaml).
		if math.IsNaN(operand) || math.IsInf(operand, 0) {
			return 0, fmt.Errorf("%s: position %d is %v: %w",
				operation.ID, i+1, operand, ErrInvalidOperand)
		}
	}

	result, err := operation.Apply(operands)
	if err != nil {
		return 0, fmt.Errorf("%s(%s): %w", operation.ID, joinOperands(operands), err)
	}

	// Non-finite values never reach the wire. Finite operands can still produce Inf or NaN
	// — overflow, a pole, an undefined form — and all of them are one condition here.
	if math.IsNaN(result) || math.IsInf(result, 0) {
		return 0, fmt.Errorf("%s(%s) = %v: %w",
			operation.ID, joinOperands(operands), result, ErrResultOverflow)
	}

	return result, nil
}

func joinOperands(operands []float64) string {
	parts := make([]string, len(operands))
	for i, operand := range operands {
		parts[i] = strconv.FormatFloat(operand, 'g', -1, 64)
	}
	return strings.Join(parts, ", ")
}
