package http

import (
	"io"
	"log/slog"
	"net/http"
	"strings"
)

// Config is everything the transport needs from its environment. Reading it is main's job,
// so the router stays a pure function of its inputs.
type Config struct {
	// AllowedOrigin enables the CORS middleware for exactly that origin. Empty disables
	// it, which is the development default (ADR-0012).
	AllowedOrigin string

	// Logger receives one line per request. Defaults to a discarding logger.
	Logger *slog.Logger
}

// NewRouter builds the handler for the whole API.
//
// Each path is registered twice: once with its method, and once without. Go's ServeMux
// prefers the more specific pattern, so a supported method reaches the handler and every
// other method falls through to the second registration, which answers 405 in the
// documented envelope. Left to itself ServeMux would answer 405 in plain text, which the
// contract does not describe (ADR-0014).
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

	// Innermost first: the correlation id must exist before anything logs, and the
	// logger's defer must outlive the recovery so a panicking request is still recorded.
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

// handleUnroutable answers any path the contract does not describe. UNKNOWN_OPERATION is
// the only 404 in the closed error enum (ADR-0004), and answering in the envelope keeps
// the promise that every response from this API is JSON.
func handleUnroutable(w http.ResponseWriter, _ *http.Request) {
	writeError(w, &apiError{
		status:  http.StatusNotFound,
		code:    "UNKNOWN_OPERATION",
		message: "The requested resource does not exist",
	})
}
