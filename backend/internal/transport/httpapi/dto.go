package httpapi

import "encoding/json"

// Operands decode one element at a time rather than into []float64, so a bad one can be
// reported with its position (ADR-0014).
type calculationRequest struct {
	Operands []json.RawMessage `json:"operands"`
}

type calculationResponse struct {
	Operation string    `json:"operation"`
	Operands  []float64 `json:"operands"`
	Result    string    `json:"result"`
}

type healthResponse struct {
	Status string `json:"status"`
}

type parameterPayload struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type operationPayload struct {
	ID         string             `json:"id"`
	Name       string             `json:"name"`
	Symbol     string             `json:"symbol"`
	Arity      int                `json:"arity"`
	Parameters []parameterPayload `json:"parameters"`
	Endpoint   string             `json:"endpoint"`
}

type catalogResponse struct {
	Operations []operationPayload `json:"operations"`
}

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type errorResponse struct {
	Error errorDetail `json:"error"`
}
