package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/ilgazsenyuz/sezzle-technical-assignment/backend/internal/calc"
)

// apiError is a failure already classified for the wire: the status line, the machine code
// the client switches on, and the human message it never parses (ADR-0004).
type apiError struct {
	status  int
	code    string
	message string
}

func invalidJSON() *apiError {
	return &apiError{
		status:  http.StatusBadRequest,
		code:    "INVALID_JSON",
		message: "Request body is not valid JSON",
	}
}

func invalidOperand(position int) *apiError {
	return &apiError{
		status:  http.StatusBadRequest,
		code:    "INVALID_OPERAND",
		message: fmt.Sprintf("Operand at position %d is not a finite number", position),
	}
}

func unknownOperation(id string) *apiError {
	return &apiError{
		status:  http.StatusNotFound,
		code:    "UNKNOWN_OPERATION",
		message: fmt.Sprintf("Unknown operation '%s'", id),
	}
}

// unknownResource answers a path outside the contract. UNKNOWN_OPERATION is the only 404 in
// the closed enum of ADR-0004.
func unknownResource() *apiError {
	return &apiError{
		status:  http.StatusNotFound,
		code:    "UNKNOWN_OPERATION",
		message: "The requested resource does not exist",
	}
}

func methodNotSupported(method string) *apiError {
	return &apiError{
		status:  http.StatusMethodNotAllowed,
		code:    "METHOD_NOT_ALLOWED",
		message: fmt.Sprintf("Method %s is not supported for this resource", method),
	}
}

func internalError() *apiError {
	return &apiError{
		status:  http.StatusInternalServerError,
		code:    "INTERNAL_ERROR",
		message: "An unexpected error occurred",
	}
}

// classify maps a domain error onto its wire form — the taxonomy of ADR-0004. Message text
// lives here because calc's error strings are internal and never surfaced. An error matching
// no case is a fault in this package rather than a rejected request, so it becomes a 500
// carrying no detail.
//
// calc.ErrUnknownOperation is absent deliberately: handleExecute resolves existence before
// calling calc, so it cannot reach here.
func classify(err error, operation calc.Operation, operandCount int) *apiError {
	switch {
	case errors.Is(err, calc.ErrWrongOperandCount):
		return &apiError{
			status: http.StatusBadRequest,
			code:   "WRONG_OPERAND_COUNT",
			message: fmt.Sprintf("Operation '%s' requires exactly %d %s, received %d",
				operation.ID, operation.Arity, operandNoun(operation.Arity), operandCount),
		}

	// Unreachable while the decoder rejects non-numeric operands first; kept so a change to
	// the decode path degrades to a 400 rather than a 500 (ADR-0017).
	case errors.Is(err, calc.ErrInvalidOperand):
		return &apiError{
			status:  http.StatusBadRequest,
			code:    "INVALID_OPERAND",
			message: "Operand is not a finite number",
		}

	case errors.Is(err, calc.ErrDivisionByZero):
		return &apiError{
			status:  http.StatusUnprocessableEntity,
			code:    "DIVISION_BY_ZERO",
			message: "Cannot divide by zero",
		}

	case errors.Is(err, calc.ErrNegativeSqrt):
		return &apiError{
			status:  http.StatusUnprocessableEntity,
			code:    "NEGATIVE_SQRT",
			message: "Cannot take the square root of a negative number",
		}

	case errors.Is(err, calc.ErrResultOverflow):
		return &apiError{
			status:  http.StatusUnprocessableEntity,
			code:    "RESULT_OVERFLOW",
			message: "Result is too large to represent",
		}

	default:
		return internalError()
	}
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

	// The status line is already committed, so a failed write means the client is gone and
	// there is nowhere left to report it.
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, failure *apiError) {
	writeJSON(w, failure.status, errorResponse{Error: errorDetail{
		Code:    failure.code,
		Message: failure.message,
	}})
}
