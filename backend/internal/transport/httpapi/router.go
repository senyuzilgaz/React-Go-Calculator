// Package httpapi exposes the calculator over HTTP: routing, request decoding, error
// classification, and middleware. It owns the wire; internal/calc owns the arithmetic
// (ADR-0007).
package httpapi

import (
	"io"
	"log/slog"
	"net/http"
	"strings"
)

// The router and the endpoint published in the catalog are both built from this, so they
// cannot disagree.
const operationsPath = "/api/v1/operations"

// Config is what the transport needs from its environment. Reading it is main's job.
type Config struct {
	// AllowedOrigin enables CORS for exactly that origin. Empty disables it, which is the
	// development default (ADR-0012).
	AllowedOrigin string

	// Logger receives one line per request. Defaults to a discarding logger.
	Logger *slog.Logger
}

// NewRouter builds the handler for the whole API.
//
// Each path is registered twice, once with its method and once without: ServeMux prefers the
// more specific pattern, so every other method falls through to the second registration and
// answers 405 in the envelope. ServeMux's own 405 is plain text (ADR-0014).
func NewRouter(config Config) http.Handler {
	logger := config.Logger
	if logger == nil {
		logger = slog.New(slog.NewJSONHandler(io.Discard, nil))
	}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", handleHealth)
	mux.HandleFunc("/healthz", methodNotAllowed(http.MethodGet))

	mux.HandleFunc("GET "+operationsPath, handleCatalog)
	mux.HandleFunc(operationsPath, methodNotAllowed(http.MethodGet))

	mux.HandleFunc("POST "+operationsPath+"/{op}", handleExecute)
	mux.HandleFunc(operationsPath+"/{op}", methodNotAllowed(http.MethodPost))

	mux.HandleFunc("/", handleUnroutable)

	// Innermost first: the correlation id must exist before anything logs, and the logger's
	// defer must outlive the recovery so a panicking request is still recorded.
	var handler http.Handler = mux
	handler = corsPolicy(config.AllowedOrigin)(handler)
	handler = recoverPanics(logger)(handler)
	handler = logRequests(logger)(handler)
	handler = requestID(handler)

	return handler
}

func methodNotAllowed(supported ...string) http.HandlerFunc {
	allow := strings.Join(supported, ", ")

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Allow", allow)
		writeError(w, methodNotSupported(r.Method))
	}
}

// Paths outside the contract answer in the envelope too, so every response is JSON.
func handleUnroutable(w http.ResponseWriter, _ *http.Request) {
	writeError(w, unknownResource())
}
