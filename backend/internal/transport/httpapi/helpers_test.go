package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newServer starts the real router over a real listener. Every test goes through it rather
// than calling a handler directly, so routing, method matching, and middleware are covered.
func newServer(t *testing.T, config Config) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(NewRouter(config))
	t.Cleanup(server.Close)
	return server
}

type response struct {
	status int
	header http.Header
	body   string
}

func (r response) decode(t *testing.T, target any) {
	t.Helper()
	if err := json.Unmarshal([]byte(r.body), target); err != nil {
		t.Fatalf("response body is not JSON: %v\nbody: %s", err, r.body)
	}
}

func (r response) errorDetail(t *testing.T) errorDetail {
	t.Helper()
	var envelope errorResponse
	r.decode(t, &envelope)
	return envelope.Error
}

func send(t *testing.T, server *httptest.Server, method, path, body string) response {
	t.Helper()
	return sendWithHeaders(t, server, method, path, body, nil)
}

func sendWithHeaders(t *testing.T, server *httptest.Server, method, path, body string, headers map[string]string) response {
	t.Helper()

	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}

	request, err := http.NewRequest(method, server.URL+path, reader)
	if err != nil {
		t.Fatalf("building %s %s: %v", method, path, err)
	}
	for name, value := range headers {
		request.Header.Set(name, value)
	}

	result, err := server.Client().Do(request)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer result.Body.Close()

	payload, err := io.ReadAll(result.Body)
	if err != nil {
		t.Fatalf("reading %s %s body: %v", method, path, err)
	}

	return response{status: result.StatusCode, header: result.Header, body: string(payload)}
}

func post(t *testing.T, server *httptest.Server, path, body string) response {
	t.Helper()
	return send(t, server, http.MethodPost, path, body)
}

func get(t *testing.T, server *httptest.Server, path string) response {
	t.Helper()
	return send(t, server, http.MethodGet, path, "")
}
