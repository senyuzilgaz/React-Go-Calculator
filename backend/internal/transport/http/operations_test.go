package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// The pairs published in docs/API_EXAMPLES.md, asserted end to end. These are the
// executable half of the contract (ADR-0014).
func TestExecuteReturnsPublishedExamples(t *testing.T) {
	server := newServer(t, Config{})

	tests := []struct {
		operation string
		body      string
		want      string
	}{
		{"add", `{"operands":[0.1,0.2]}`, `{"operation":"add","operands":[0.1,0.2],"result":"0.3"}`},
		{"subtract", `{"operands":[0.3,0.1]}`, `{"operation":"subtract","operands":[0.3,0.1],"result":"0.2"}`},
		{"multiply", `{"operands":[1.1,1.1]}`, `{"operation":"multiply","operands":[1.1,1.1],"result":"1.21"}`},
		{"divide", `{"operands":[10,4]}`, `{"operation":"divide","operands":[10,4],"result":"2.5"}`},
		{"divide", `{"operands":[1,3]}`, `{"operation":"divide","operands":[1,3],"result":"0.333333333333"}`},
		{"power", `{"operands":[2,10]}`, `{"operation":"power","operands":[2,10],"result":"1024"}`},
		{"sqrt", `{"operands":[2]}`, `{"operation":"sqrt","operands":[2],"result":"1.41421356237"}`},
		{"percent", `{"operands":[15,200]}`, `{"operation":"percent","operands":[15,200],"result":"30"}`},
		{"multiply", `{"operands":[123456789,987654321]}`, `{"operation":"multiply","operands":[123456789,987654321],"result":"1.21932631113e+17"}`},
		{"divide", `{"operands":[1,1000000000]}`, `{"operation":"divide","operands":[1,1000000000],"result":"0.000000001"}`},
		{"divide", `{"operands":[1,10000000000]}`, `{"operation":"divide","operands":[1,10000000000],"result":"1e-10"}`},
		{"multiply", `{"operands":[0,-5]}`, `{"operation":"multiply","operands":[0,-5],"result":"0"}`},
	}

	for _, tt := range tests {
		t.Run(tt.operation+" "+tt.body, func(t *testing.T) {
			got := post(t, server, operationEndpoint(tt.operation), tt.body)

			if got.status != http.StatusOK {
				t.Fatalf("status = %d, want 200\nbody: %s", got.status, got.body)
			}
			if strings.TrimSpace(got.body) != tt.want {
				t.Errorf("body = %s, want %s", strings.TrimSpace(got.body), tt.want)
			}
		})
	}
}

// result is a string on the wire, not a number. Serialized as a number the browser would
// re-parse it as binary64 and reintroduce exactly the error the rounding policy removes
// (ADR-0003 clause 6).
func TestResultIsCarriedAsAString(t *testing.T) {
	server := newServer(t, Config{})

	got := post(t, server, "/api/v1/operations/add", `{"operands":[0.1,0.2]}`)

	if !strings.Contains(got.body, `"result":"0.3"`) {
		t.Errorf("body %s does not carry result as a quoted string", strings.TrimSpace(got.body))
	}

	var raw map[string]json.RawMessage
	got.decode(t, &raw)

	var result any
	if err := json.Unmarshal(raw["result"], &result); err != nil {
		t.Fatalf("decoding result: %v", err)
	}
	if _, ok := result.(string); !ok {
		t.Errorf("result decoded as %T, want string", result)
	}
}

func TestExecuteEchoesOperandsAsReceived(t *testing.T) {
	server := newServer(t, Config{})

	got := post(t, server, "/api/v1/operations/subtract", `{"operands":[-2.5,0.25]}`)

	var payload calculationResponse
	got.decode(t, &payload)

	if payload.Operation != "subtract" {
		t.Errorf("operation = %q, want subtract", payload.Operation)
	}
	if len(payload.Operands) != 2 || payload.Operands[0] != -2.5 || payload.Operands[1] != 0.25 {
		t.Errorf("operands = %v, want [-2.5 0.25]", payload.Operands)
	}
}

// Every operation in the catalog is reachable at the endpoint the catalog publishes. A
// registry entry that routing did not pick up would fail here rather than in the frontend.
func TestEveryPublishedOperationIsExecutable(t *testing.T) {
	server := newServer(t, Config{})

	var catalog catalogResponse
	get(t, server, "/api/v1/operations").decode(t, &catalog)

	if len(catalog.Operations) == 0 {
		t.Fatal("catalog is empty")
	}

	for _, operation := range catalog.Operations {
		t.Run(operation.ID, func(t *testing.T) {
			operands := make([]string, operation.Arity)
			for i := range operands {
				operands[i] = "4"
			}
			body := fmt.Sprintf(`{"operands":[%s]}`, strings.Join(operands, ","))

			got := post(t, server, operation.Endpoint, body)
			if got.status != http.StatusOK {
				t.Fatalf("POST %s = %d, want 200\nbody: %s", operation.Endpoint, got.status, got.body)
			}
		})
	}
}

// Content-Type is not enforced; the body is parsed as JSON regardless (ADR-0014).
func TestExecuteIgnoresContentType(t *testing.T) {
	server := newServer(t, Config{})

	for _, contentType := range []string{"text/plain", "application/xml", ""} {
		t.Run("content-type "+contentType, func(t *testing.T) {
			headers := map[string]string{}
			if contentType != "" {
				headers["Content-Type"] = contentType
			}

			got := sendWithHeaders(t, server, http.MethodPost, "/api/v1/operations/add", `{"operands":[1,2]}`, headers)
			if got.status != http.StatusOK {
				t.Errorf("status = %d, want 200\nbody: %s", got.status, got.body)
			}
		})
	}
}

// One case per code in the taxonomy of ADR-0014, asserting status, code, and the exact
// message published in api/openapi.yaml and docs/API_EXAMPLES.md.
func TestExecuteFailures(t *testing.T) {
	server := newServer(t, Config{})

	tests := []struct {
		name    string
		path    string
		body    string
		status  int
		code    string
		message string
	}{
		{
			"unparseable body", "/api/v1/operations/add", `{"operands":[1,2`,
			http.StatusBadRequest, "INVALID_JSON", "Request body is not valid JSON",
		},
		{
			"empty body", "/api/v1/operations/add", "",
			http.StatusBadRequest, "INVALID_JSON", "Request body is not valid JSON",
		},
		{
			"body is not an object", "/api/v1/operations/add", `[1,2]`,
			http.StatusBadRequest, "INVALID_JSON", "Request body is not valid JSON",
		},
		{
			"unknown field", "/api/v1/operations/add", `{"operands":[1,2],"precision":3}`,
			http.StatusBadRequest, "INVALID_JSON", "Request body is not valid JSON",
		},
		{
			"trailing content after the object", "/api/v1/operations/add", `{"operands":[1,2]}{"operands":[3,4]}`,
			http.StatusBadRequest, "INVALID_JSON", "Request body is not valid JSON",
		},
		{
			"string operand", "/api/v1/operations/add", `{"operands":["abc",2]}`,
			http.StatusBadRequest, "INVALID_OPERAND", "Operand at position 1 is not a finite number",
		},
		{
			"null operand", "/api/v1/operations/add", `{"operands":[1,null]}`,
			http.StatusBadRequest, "INVALID_OPERAND", "Operand at position 2 is not a finite number",
		},
		{
			"boolean operand", "/api/v1/operations/add", `{"operands":[1,true]}`,
			http.StatusBadRequest, "INVALID_OPERAND", "Operand at position 2 is not a finite number",
		},
		{
			// Not representable in binary64, so it never becomes an operand at all.
			"operand beyond float64", "/api/v1/operations/add", `{"operands":[1e999,2]}`,
			http.StatusBadRequest, "INVALID_OPERAND", "Operand at position 1 is not a finite number",
		},
		{
			"two operands to a unary operation", "/api/v1/operations/sqrt", `{"operands":[9,16]}`,
			http.StatusBadRequest, "WRONG_OPERAND_COUNT", "Operation 'sqrt' requires exactly 1 operand, received 2",
		},
		{
			"one operand to a binary operation", "/api/v1/operations/add", `{"operands":[1]}`,
			http.StatusBadRequest, "WRONG_OPERAND_COUNT", "Operation 'add' requires exactly 2 operands, received 1",
		},
		{
			"no operands field", "/api/v1/operations/add", `{}`,
			http.StatusBadRequest, "WRONG_OPERAND_COUNT", "Operation 'add' requires exactly 2 operands, received 0",
		},
		{
			"empty operands", "/api/v1/operations/sqrt", `{"operands":[]}`,
			http.StatusBadRequest, "WRONG_OPERAND_COUNT", "Operation 'sqrt' requires exactly 1 operand, received 0",
		},
		{
			"unknown operation", "/api/v1/operations/modulo", `{"operands":[10,3]}`,
			http.StatusNotFound, "UNKNOWN_OPERATION", "Unknown operation 'modulo'",
		},
		{
			"divide by zero", "/api/v1/operations/divide", `{"operands":[10,0]}`,
			http.StatusUnprocessableEntity, "DIVISION_BY_ZERO", "Cannot divide by zero",
		},
		{
			"square root of a negative", "/api/v1/operations/sqrt", `{"operands":[-4]}`,
			http.StatusUnprocessableEntity, "NEGATIVE_SQRT", "Cannot take the square root of a negative number",
		},
		{
			"result overflows", "/api/v1/operations/power", `{"operands":[1e308,2]}`,
			http.StatusUnprocessableEntity, "RESULT_OVERFLOW", "Result is too large to represent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := post(t, server, tt.path, tt.body)

			if got.status != tt.status {
				t.Fatalf("status = %d, want %d\nbody: %s", got.status, tt.status, got.body)
			}

			detail := got.errorDetail(t)
			if detail.Code != tt.code {
				t.Errorf("code = %q, want %q", detail.Code, tt.code)
			}
			if detail.Message != tt.message {
				t.Errorf("message = %q, want %q", detail.Message, tt.message)
			}
		})
	}
}

// An oversized body is INVALID_JSON rather than 413, as the contract states (ADR-0014).
func TestOversizedBodyIsRejected(t *testing.T) {
	server := newServer(t, Config{})

	padding := strings.Repeat("0", maxRequestBytes)
	body := fmt.Sprintf(`{"operands":[1,2.%s]}`, padding)

	got := post(t, server, "/api/v1/operations/add", body)

	if got.status != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400\nbody: %s", got.status, got.body)
	}
	if detail := got.errorDetail(t); detail.Code != "INVALID_JSON" {
		t.Errorf("code = %q, want INVALID_JSON", detail.Code)
	}
}

// Existence is settled before the body is read: a request for an operation that does not
// exist is a 404 whatever it carries.
func TestUnknownOperationOutranksAMalformedBody(t *testing.T) {
	server := newServer(t, Config{})

	got := post(t, server, "/api/v1/operations/modulo", `{"operands":[1,2`)

	if got.status != http.StatusNotFound {
		t.Fatalf("status = %d, want 404\nbody: %s", got.status, got.body)
	}
	if detail := got.errorDetail(t); detail.Code != "UNKNOWN_OPERATION" {
		t.Errorf("code = %q, want UNKNOWN_OPERATION", detail.Code)
	}
}
