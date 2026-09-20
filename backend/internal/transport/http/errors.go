package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/ilgazsenyuz/sezzle-technical-assignment/backend/internal/calc"
)

// apiError is a failure already classified for the wire: the status line, the machine
// code the client switches on, and the human message it never parses (ADR-0004). It is a
// description of a response rather than an error value; the error it came from stays in
// the layer that produced it.
type apiError struct {
	status  int
	code    string
	message string
}

func invalidJSON() *apiError {
	return &apiError{http.StatusBadRequest, "INVALID_JSON", "Request body is not valid JSON"}
}

func invalidOperand(position int) *apiError {
	return &apiError{http.StatusBadRequest, "INVALID_OPERAND",
		fmt.Sprintf("Operand at position %d is not a finite number", position)}
}

func unknownOperation(id string) *apiError {
	return &apiError{http.StatusNotFound, "UNKNOWN_OPERATION",
		fmt.Sprintf("Unknown operation '%s'", id)}
}

func methodNotSupported(method string) *apiError {
	return &apiError{http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED",
		fmt.Sprintf("Method %s is not supported for this resource", method)}
}

func internalError() *apiError {
	return &apiError{http.StatusInternalServerError, "INTERNAL_ERROR", "An unexpected error occurred"}
}

// domainFailures is the whole taxonomy of ADR-0004: one row per domain error, and adding a
// domain error means adding a row rather than inventing a new envelope. Message text is
// written here because calc's error strings are internal and never surfaced.
var domainFailures = []struct {
	sentinel error
	status   int
	code     string
	message  func(operation calc.Operation, operandCount int) string
}{
	{
		calc.ErrWrongOperandCount, http.StatusBadRequest, "WRONG_OPERAND_COUNT",
		func(operation calc.Operation, operandCount int) string {
			return fmt.Sprintf("Operation '%s' requires exactly %d %s, received %d",
				operation.ID, operation.Arity, operandNoun(operation.Arity), operandCount)
		},
	},
	{
		calc.ErrInvalidOperand, http.StatusBadRequest, "INVALID_OPERAND",
		func(calc.Operation, int) string { return "Operand is not a finite number" },
	},
	{
		calc.ErrUnknownOperation, http.StatusNotFound, "UNKNOWN_OPERATION",
		func(operation calc.Operation, _ int) string {
			return fmt.Sprintf("Unknown operation '%s'", operation.ID)
		},
	},
	{
		calc.ErrDivisionByZero, http.StatusUnprocessableEntity, "DIVISION_BY_ZERO",
		func(calc.Operation, int) string { return "Cannot divide by zero" },
	},
	{
		calc.ErrNegativeSqrt, http.StatusUnprocessableEntity, "NEGATIVE_SQRT",
		func(calc.Operation, int) string { return "Cannot take the square root of a negative number" },
	},
	{
		calc.ErrResultOverflow, http.StatusUnprocessableEntity, "RESULT_OVERFLOW",
		func(calc.Operation, int) string { return "Result is too large to represent" },
	},
}

// classify maps a domain error onto its wire form. An error matching no row is a fault in
// this package rather than a rejected request, so it becomes a 500 carrying no detail.
func classify(err error, operation calc.Operation, operandCount int) *apiError {
	for _, failure := range domainFailures {
		if errors.Is(err, failure.sentinel) {
			return &apiError{failure.status, failure.code, failure.message(operation, operandCount)}
		}
	}
	return internalError()
}

func operandNoun(count int) string {
	if count == 1 {
		return "operand"
	}
	return "operands"
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	// The status line is already committed, so a failed write means the client is gone
	// and there is nowhere left to report it.
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, failure *apiError) {
	writeJSON(w, failure.status, errorResponse{Error: errorDetail{
		Code:    failure.code,
		Message: failure.message,
	}})
}
