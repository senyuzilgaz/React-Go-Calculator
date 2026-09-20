// Package calc holds the arithmetic: the operation registry and the rounding policy. It
// knows nothing of HTTP or JSON, and reports failures as domain errors (ADR-0007).
package calc

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Parameter is one positional operand, named for the role it plays.
type Parameter struct {
	Name        string
	Description string
}

// Operation is one registry entry: what the catalog publishes and the arithmetic behind it.
type Operation struct {
	ID         string
	Name       string
	Symbol     string
	Arity      int
	Parameters []Parameter

	// Assumes len(operands) == Arity and every operand finite. The package-level Apply is
	// the guarded entry point that establishes both.
	Apply func(operands []float64) (float64, error)
}

// The single source of truth for routing, arity validation and the catalog response
// (ADR-0002). Adding an operation is one entry here. Order is published order.
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
			// A sign-bit test would reject negative zero, which is not negative.
			// math.Sqrt(-0) is -0, which Format normalizes.
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
			// "a percent of b" (ADR-0010). The product is formed first, so a large
			// enough pair overflows where the true result would be representable.
			return operands[0] * operands[1] / 100, nil
		},
	},
}

// The only dispatch in the package; no branch anywhere names an operation.
var registry = func() map[string]Operation {
	byID := make(map[string]Operation, len(catalog))
	for _, operation := range catalog {
		byID[operation.ID] = operation
	}
	return byID
}()

// Catalog returns every operation in published order.
func Catalog() []Operation {
	operations := make([]Operation, len(catalog))
	for i, operation := range catalog {
		operations[i] = operation.clone()
	}
	return operations
}

// Lookup resolves an identifier exactly, with no normalization or near-match.
func Lookup(id string) (Operation, bool) {
	operation, ok := registry[id]
	if !ok {
		return Operation{}, false
	}
	return operation.clone(), true
}

// Detaches an operation from the registry. A bare struct copy would leave Parameters
// aliasing registry state, which a caller could edit and corrupt every later request.
func (o Operation) clone() Operation {
	o.Parameters = append([]Parameter(nil), o.Parameters...)
	return o
}

// Apply resolves an operation and runs it against operands. The guards run in a fixed
// order (existence, arity, finiteness, computation) because the transport layer maps each
// to a different status, and the first one to fail is what gets reported (ADR-0004).
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
		// Positions are 1-based in the published messages.
		if math.IsNaN(operand) || math.IsInf(operand, 0) {
			return 0, fmt.Errorf("%s: position %d is %v: %w",
				operation.ID, i+1, operand, ErrInvalidOperand)
		}
	}

	result, err := operation.Apply(operands)
	if err != nil {
		return 0, fmt.Errorf("%s(%s): %w", operation.ID, joinOperands(operands), err)
	}

	// Finite operands can still produce Inf or NaN, which never reach the wire.
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
