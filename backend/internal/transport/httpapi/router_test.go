package httpapi

import (
	"net/http"
	"testing"
)

// The 404/405 split is contract: it tells a client "no such resource" apart from "not that
// way" (ADR-0014).
func TestRoutingAndMethodMatching(t *testing.T) {
	server := newServer(t, Config{})

	tests := []struct {
		name   string
		method string
		path   string
		body   string
		status int
		allow  string
	}{
		{"health", http.MethodGet, "/healthz", "", http.StatusOK, ""},
		// ServeMux matches HEAD against a registered GET.
		{"health accepts HEAD", http.MethodHead, "/healthz", "", http.StatusOK, ""},
		{"health rejects POST", http.MethodPost, "/healthz", "", http.StatusMethodNotAllowed, "GET"},
		{"health rejects PUT", http.MethodPut, "/healthz", "", http.StatusMethodNotAllowed, "GET"},
		{"health rejects DELETE", http.MethodDelete, "/healthz", "", http.StatusMethodNotAllowed, "GET"},

		{"catalog", http.MethodGet, "/api/v1/operations", "", http.StatusOK, ""},
		{"catalog rejects POST", http.MethodPost, "/api/v1/operations", `{"operands":[1,2]}`, http.StatusMethodNotAllowed, "GET"},
		{"catalog rejects DELETE", http.MethodDelete, "/api/v1/operations", "", http.StatusMethodNotAllowed, "GET"},

		{"execute", http.MethodPost, "/api/v1/operations/add", `{"operands":[1,2]}`, http.StatusOK, ""},
		{"execute rejects GET", http.MethodGet, "/api/v1/operations/add", "", http.StatusMethodNotAllowed, "POST"},
		{"execute rejects PUT", http.MethodPut, "/api/v1/operations/add", `{"operands":[1,2]}`, http.StatusMethodNotAllowed, "POST"},
		{"execute rejects DELETE", http.MethodDelete, "/api/v1/operations/add", "", http.StatusMethodNotAllowed, "POST"},
		{"execute rejects PATCH", http.MethodPatch, "/api/v1/operations/add", "", http.StatusMethodNotAllowed, "POST"},

		{"unknown operation", http.MethodPost, "/api/v1/operations/modulo", `{"operands":[10,3]}`, http.StatusNotFound, ""},
		{"empty operation segment", http.MethodPost, "/api/v1/operations/", `{"operands":[1,2]}`, http.StatusNotFound, ""},
		{"operation with trailing path", http.MethodPost, "/api/v1/operations/add/extra", `{"operands":[1,2]}`, http.StatusNotFound, ""},
		{"unversioned path", http.MethodPost, "/api/operations/add", `{"operands":[1,2]}`, http.StatusNotFound, ""},
		{"root", http.MethodGet, "/", "", http.StatusNotFound, ""},
		{"unrelated path", http.MethodGet, "/metrics", "", http.StatusNotFound, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := send(t, server, tt.method, tt.path, tt.body)

			if got.status != tt.status {
				t.Errorf("%s %s = %d, want %d\nbody: %s", tt.method, tt.path, got.status, tt.status, got.body)
			}
			if allow := got.header.Get("Allow"); allow != tt.allow {
				t.Errorf("%s %s Allow = %q, want %q", tt.method, tt.path, allow, tt.allow)
			}
		})
	}
}

// Every response is JSON and correlated, including the ones no handler produced.
func TestEveryResponseIsCorrelatedJSON(t *testing.T) {
	server := newServer(t, Config{})

	requests := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/healthz", ""},
		{http.MethodGet, "/api/v1/operations", ""},
		{http.MethodPost, "/api/v1/operations/add", `{"operands":[1,2]}`},
		{http.MethodPost, "/api/v1/operations/divide", `{"operands":[1,0]}`},
		{http.MethodPost, "/api/v1/operations/modulo", `{"operands":[1,2]}`},
		{http.MethodPost, "/api/v1/operations/add", `{"operands":[1,2`},
		{http.MethodGet, "/api/v1/operations/add", ""},
		{http.MethodGet, "/nothing/here", ""},
	}

	for _, request := range requests {
		t.Run(request.method+" "+request.path, func(t *testing.T) {
			got := send(t, server, request.method, request.path, request.body)

			if contentType := got.header.Get("Content-Type"); contentType != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", contentType)
			}
			if got.header.Get("X-Request-Id") == "" {
				t.Error("no X-Request-Id header")
			}

			var anyJSON any
			got.decode(t, &anyJSON)
		})
	}
}

func TestMethodNotAllowedUsesTheErrorEnvelope(t *testing.T) {
	server := newServer(t, Config{})

	got := send(t, server, http.MethodGet, "/api/v1/operations/add", "")

	if got.status != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", got.status)
	}
	if allow := got.header.Get("Allow"); allow != "POST" {
		t.Errorf("Allow = %q, want POST", allow)
	}

	detail := got.errorDetail(t)
	if detail.Code != "METHOD_NOT_ALLOWED" {
		t.Errorf("code = %q, want METHOD_NOT_ALLOWED", detail.Code)
	}
	if detail.Message != "Method GET is not supported for this resource" {
		t.Errorf("message = %q, want the message published in api/openapi.yaml", detail.Message)
	}
}

func TestHealthReportsOK(t *testing.T) {
	server := newServer(t, Config{})

	got := get(t, server, "/healthz")
	if got.status != http.StatusOK {
		t.Fatalf("status = %d, want 200", got.status)
	}

	var health healthResponse
	got.decode(t, &health)
	if health.Status != "ok" {
		t.Errorf("status = %q, want ok", health.Status)
	}
}
