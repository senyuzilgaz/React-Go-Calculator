package http

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequestIDIsGeneratedWhenAbsent(t *testing.T) {
	server := newServer(t, Config{})

	first := get(t, server, "/healthz").header.Get("X-Request-Id")
	second := get(t, server, "/healthz").header.Get("X-Request-Id")

	if first == "" || second == "" {
		t.Fatalf("request ids = %q and %q, want both generated", first, second)
	}
	if first == second {
		t.Errorf("both requests were correlated as %q; ids must be per-request", first)
	}
}

func TestRequestIDIsEchoedWhenSupplied(t *testing.T) {
	server := newServer(t, Config{})

	const supplied = "01J9ZC8N2K4QWERTY0123456789"
	got := sendWithHeaders(t, server, http.MethodGet, "/healthz", "", map[string]string{
		"X-Request-Id": supplied,
	})

	if echoed := got.header.Get("X-Request-Id"); echoed != supplied {
		t.Errorf("X-Request-Id = %q, want the supplied %q", echoed, supplied)
	}
}

// The header is attacker-controlled and lands in every log line for the request, so an
// unreasonable value is replaced rather than carried into the logs.
func TestUnreasonableRequestIDIsReplaced(t *testing.T) {
	server := newServer(t, Config{})

	tests := []struct {
		name     string
		supplied string
	}{
		{"too long", strings.Repeat("a", maxRequestIDLength+1)},
		{"contains a space", "id with spaces"},
		{"contains a tab", "id\twith\ttab"},
		{"non-ascii", "idé"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sendWithHeaders(t, server, http.MethodGet, "/healthz", "", map[string]string{
				"X-Request-Id": tt.supplied,
			})

			echoed := got.header.Get("X-Request-Id")
			if echoed == "" {
				t.Fatal("no X-Request-Id on the response")
			}
			if echoed == tt.supplied {
				t.Errorf("X-Request-Id = %q, want a generated replacement", echoed)
			}
		})
	}
}

// A panic cannot be provoked through the router, since no handler panics. The middleware
// is therefore exercised around a handler that does, still over a real server.
func TestPanicBecomesInternalError(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	panicking := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	})

	server := httptest.NewServer(requestID(recoverPanics(logger)(panicking)))
	t.Cleanup(server.Close)

	got := get(t, server, "/healthz")

	if got.status != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", got.status)
	}
	if got.header.Get("X-Request-Id") == "" {
		t.Error("no X-Request-Id to correlate the fault with its log entry")
	}

	detail := got.errorDetail(t)
	if detail.Code != "INTERNAL_ERROR" {
		t.Errorf("code = %q, want INTERNAL_ERROR", detail.Code)
	}
	if detail.Message != "An unexpected error occurred" {
		t.Errorf("message = %q, want the published message", detail.Message)
	}
	if strings.Contains(got.body, "boom") {
		t.Errorf("body %q leaks the panic value", got.body)
	}
}

// A panic after the response is committed cannot be replaced by a 500; the middleware must
// leave the committed status alone rather than trying to write a second one.
func TestPanicAfterResponseStartedLeavesTheStatusAlone(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, healthResponse{Status: "ok"})
		panic("boom")
	})

	server := httptest.NewServer(recoverPanics(logger)(handler))
	t.Cleanup(server.Close)

	got := get(t, server, "/healthz")
	if got.status != http.StatusOK {
		t.Errorf("status = %d, want the already-committed 200", got.status)
	}
}

func TestCORSIsAbsentUntilAnOriginIsConfigured(t *testing.T) {
	server := newServer(t, Config{})

	got := sendWithHeaders(t, server, http.MethodGet, "/healthz", "", map[string]string{
		"Origin": "https://calculator.example",
	})

	if allow := got.header.Get("Access-Control-Allow-Origin"); allow != "" {
		t.Errorf("Access-Control-Allow-Origin = %q, want none in the development default", allow)
	}
}

func TestCORSAllowsTheConfiguredOrigin(t *testing.T) {
	const origin = "https://calculator.example"
	server := newServer(t, Config{AllowedOrigin: origin})

	got := sendWithHeaders(t, server, http.MethodPost, "/api/v1/operations/add", `{"operands":[1,2]}`,
		map[string]string{"Origin": origin})

	if got.status != http.StatusOK {
		t.Fatalf("status = %d, want 200\nbody: %s", got.status, got.body)
	}
	if allow := got.header.Get("Access-Control-Allow-Origin"); allow != origin {
		t.Errorf("Access-Control-Allow-Origin = %q, want %q", allow, origin)
	}
	if vary := got.header.Get("Vary"); !strings.Contains(vary, "Origin") {
		t.Errorf("Vary = %q, want it to include Origin", vary)
	}
}

func TestCORSIgnoresOtherOrigins(t *testing.T) {
	server := newServer(t, Config{AllowedOrigin: "https://calculator.example"})

	got := sendWithHeaders(t, server, http.MethodGet, "/healthz", "", map[string]string{
		"Origin": "https://attacker.example",
	})

	if allow := got.header.Get("Access-Control-Allow-Origin"); allow != "" {
		t.Errorf("Access-Control-Allow-Origin = %q, want none for an unlisted origin", allow)
	}
}

func TestCORSAnswersPreflight(t *testing.T) {
	const origin = "https://calculator.example"
	server := newServer(t, Config{AllowedOrigin: origin})

	got := sendWithHeaders(t, server, http.MethodOptions, "/api/v1/operations/add", "", map[string]string{
		"Origin":                        origin,
		"Access-Control-Request-Method": "POST",
	})

	if got.status != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", got.status)
	}
	if allow := got.header.Get("Access-Control-Allow-Methods"); !strings.Contains(allow, "POST") {
		t.Errorf("Access-Control-Allow-Methods = %q, want it to include POST", allow)
	}
}

// Without CORS configured there is no preflight to answer, so OPTIONS is just an
// unsupported method on a real path.
func TestOptionsIsMethodNotAllowedWithoutCORS(t *testing.T) {
	server := newServer(t, Config{})

	got := send(t, server, http.MethodOptions, "/api/v1/operations/add", "")

	if got.status != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", got.status)
	}
	if allow := got.header.Get("Allow"); allow != "POST" {
		t.Errorf("Allow = %q, want POST", allow)
	}
}

func TestRequestsAreLogged(t *testing.T) {
	var logged strings.Builder
	logger := slog.New(slog.NewJSONHandler(&logged, nil))

	server := newServer(t, Config{Logger: logger})
	got := post(t, server, "/api/v1/operations/divide", `{"operands":[1,0]}`)

	line := logged.String()
	for _, want := range []string{`"method":"POST"`, `"path":"/api/v1/operations/divide"`, `"status":422`} {
		if !strings.Contains(line, want) {
			t.Errorf("log line %s does not contain %s", line, want)
		}
	}
	if id := got.header.Get("X-Request-Id"); !strings.Contains(line, id) {
		t.Errorf("log line %s does not carry the response's request id %q", line, id)
	}
}

// A handler that writes a body without setting a status has still committed the response.
// The recorder has to notice, or the recovery middleware would try to write a 500 over a
// response already on the wire.
func TestPanicAfterAnImplicitStatusLeavesTheBodyAlone(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"status":"ok"}`))
		panic("boom")
	})

	server := httptest.NewServer(recoverPanics(logger)(handler))
	t.Cleanup(server.Close)

	got := get(t, server, "/healthz")

	if got.status != http.StatusOK {
		t.Errorf("status = %d, want the implicitly committed 200", got.status)
	}
	if !strings.Contains(got.body, `"status":"ok"`) {
		t.Errorf("body = %q, want the already-written body preserved", got.body)
	}
}

// Only the first status line reaches the client, so only the first is what the log should
// report the request as having returned.
func TestOnlyTheFirstStatusIsRecorded(t *testing.T) {
	var logged strings.Builder
	logger := slog.New(slog.NewJSONHandler(&logged, nil))

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.WriteHeader(http.StatusInternalServerError)
	})

	server := httptest.NewServer(logRequests(logger)(handler))
	t.Cleanup(server.Close)

	got := get(t, server, "/healthz")

	if got.status != http.StatusCreated {
		t.Errorf("status = %d, want 201", got.status)
	}
	if !strings.Contains(logged.String(), `"status":201`) {
		t.Errorf("log line %s does not report the committed 201", logged.String())
	}
}
