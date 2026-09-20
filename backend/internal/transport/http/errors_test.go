package http

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/ilgazsenyuz/sezzle-technical-assignment/backend/internal/calc"
)

// The taxonomy of ADR-0004, asserted directly rather than through a request, so that every
// domain error has a mapping whether or not a request can currently reach it. calc's
// ErrInvalidOperand is one such: the decoder rejects non-numeric operands first, and the
// mapping exists so a change to the decode path degrades to a 400 rather than a 500.
func TestEveryDomainErrorIsClassified(t *testing.T) {
	sqrt, ok := calc.Lookup("sqrt")
	if !ok {
		t.Fatal("sqrt is not registered")
	}

	tests := []struct {
		err     error
		status  int
		code    string
		message string
	}{
		{calc.ErrWrongOperandCount, http.StatusBadRequest, "WRONG_OPERAND_COUNT",
			"Operation 'sqrt' requires exactly 1 operand, received 2"},
		{calc.ErrInvalidOperand, http.StatusBadRequest, "INVALID_OPERAND",
			"Operand is not a finite number"},
		{calc.ErrUnknownOperation, http.StatusNotFound, "UNKNOWN_OPERATION",
			"Unknown operation 'sqrt'"},
		{calc.ErrDivisionByZero, http.StatusUnprocessableEntity, "DIVISION_BY_ZERO",
			"Cannot divide by zero"},
		{calc.ErrNegativeSqrt, http.StatusUnprocessableEntity, "NEGATIVE_SQRT",
			"Cannot take the square root of a negative number"},
		{calc.ErrResultOverflow, http.StatusUnprocessableEntity, "RESULT_OVERFLOW",
			"Result is too large to represent"},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			// Wrapped, as calc returns them, to prove classification survives context.
			wrapped := fmt.Errorf("sqrt(4, 2): %w", tt.err)

			got := classify(wrapped, sqrt, 2)
			if got.status != tt.status {
				t.Errorf("status = %d, want %d", got.status, tt.status)
			}
			if got.code != tt.code {
				t.Errorf("code = %q, want %q", got.code, tt.code)
			}
			if got.message != tt.message {
				t.Errorf("message = %q, want %q", got.message, tt.message)
			}
		})
	}
}

// An error the taxonomy does not cover is a fault in this package, not a rejected request,
// and must never leak its text.
func TestUnmappedErrorBecomesInternalError(t *testing.T) {
	operation, _ := calc.Lookup("add")

	got := classify(errors.New("connection to the flux capacitor failed"), operation, 2)

	if got.status != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", got.status)
	}
	if got.code != "INTERNAL_ERROR" {
		t.Errorf("code = %q, want INTERNAL_ERROR", got.code)
	}
	if got.message != "An unexpected error occurred" {
		t.Errorf("message = %q, want the published message", got.message)
	}
}

// Both arities are published, so both messages have to read correctly.
func TestOperandCountMessageAgreesWithArity(t *testing.T) {
	tests := []struct {
		operation string
		received  int
		want      string
	}{
		{"sqrt", 2, "Operation 'sqrt' requires exactly 1 operand, received 2"},
		{"sqrt", 0, "Operation 'sqrt' requires exactly 1 operand, received 0"},
		{"add", 1, "Operation 'add' requires exactly 2 operands, received 1"},
		{"add", 3, "Operation 'add' requires exactly 2 operands, received 3"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			operation, ok := calc.Lookup(tt.operation)
			if !ok {
				t.Fatalf("%s is not registered", tt.operation)
			}

			got := classify(calc.ErrWrongOperandCount, operation, tt.received)
			if got.message != tt.want {
				t.Errorf("message = %q, want %q", got.message, tt.want)
			}
		})
	}
}
