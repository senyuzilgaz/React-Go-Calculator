// Package http exposes the calculator over HTTP: routing, request decoding, error
// classification, and middleware.
//
// It owns everything about the wire — status codes, JSON shapes, the error envelope, URL
// paths — and internal/calc owns the arithmetic. The two meet only at calc's exported
// functions and domain errors (ADR-0007).
package http

import "encoding/json"

// operandsPath is the prefix the operation resources live under. The router and the
// endpoint published in the catalog are both built from it, so they cannot disagree.
const operationsPath = "/api/v1/operations"

// maxRequestBytes caps the request body. A body beyond it is reported as INVALID_JSON,
// as documented in api/openapi.yaml.
const maxRequestBytes = 4 << 10

// Operands are decoded one element at a time rather than straight into []float64, so that
// a bad element can be reported with its position (ADR-0014).
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
