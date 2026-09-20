package http

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/ilgazsenyuz/sezzle-technical-assignment/backend/internal/calc"
)

func handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, healthResponse{Status: "ok"})
}

// handleCatalog projects the registry onto the wire. Nothing here is hand-maintained: the
// operations, their order, and their metadata all come from calc, so the catalog cannot
// drift from what the server actually executes (ADR-0002).
func handleCatalog(w http.ResponseWriter, _ *http.Request) {
	operations := calc.Catalog()

	payload := make([]operationPayload, len(operations))
	for i, operation := range operations {
		parameters := make([]parameterPayload, len(operation.Parameters))
		for j, parameter := range operation.Parameters {
			parameters[j] = parameterPayload{Name: parameter.Name, Description: parameter.Description}
		}

		payload[i] = operationPayload{
			ID:         operation.ID,
			Name:       operation.Name,
			Symbol:     operation.Symbol,
			Arity:      operation.Arity,
			Parameters: parameters,
			Endpoint:   operationEndpoint(operation.ID),
		}
	}

	writeJSON(w, http.StatusOK, catalogResponse{Operations: payload})
}

// handleExecute applies one operation. Existence is settled before the body is read: the
// path segment names a resource, and a request for one that does not exist is a 404
// whatever its body contains (ADR-0002).
func handleExecute(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("op")

	operation, ok := calc.Lookup(id)
	if !ok {
		writeError(w, unknownOperation(id))
		return
	}

	operands, failure := decodeOperands(w, r)
	if failure != nil {
		writeError(w, failure)
		return
	}

	result, err := calc.Apply(operation.ID, operands)
	if err != nil {
		writeError(w, classify(err, operation, len(operands)))
		return
	}

	writeJSON(w, http.StatusOK, calculationResponse{
		Operation: operation.ID,
		Operands:  operands,
		Result:    calc.Format(result),
	})
}

var jsonNull = []byte("null")

// decodeOperands reads the request body into operands, reporting a malformed body and a
// malformed operand as the different failures they are: the body either parses or it does
// not, whereas an operand that parses but is not a number has a known position.
func decodeOperands(w http.ResponseWriter, r *http.Request) ([]float64, *apiError) {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxRequestBytes))
	decoder.DisallowUnknownFields()

	var request calculationRequest
	if err := decoder.Decode(&request); err != nil {
		return nil, invalidJSON()
	}
	if decoder.More() {
		return nil, invalidJSON()
	}

	operands := make([]float64, len(request.Operands))
	for i, raw := range request.Operands {
		// Positions are 1-based in the published messages. JSON null is rejected
		// explicitly because it unmarshals into a float64 without error, silently
		// becoming zero.
		if bytes.Equal(raw, jsonNull) {
			return nil, invalidOperand(i + 1)
		}
		if err := json.Unmarshal(raw, &operands[i]); err != nil {
			return nil, invalidOperand(i + 1)
		}
	}

	return operands, nil
}

func operationEndpoint(id string) string {
	return operationsPath + "/" + id
}
