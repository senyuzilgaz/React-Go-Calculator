package httpapi

import (
	"context"
	"crypto/rand"
	"log/slog"
	"net/http"
	"time"
)

const requestIDHeader = "X-Request-Id"

// maxRequestIDLength bounds an echoed correlation id. The value is attacker-controlled and
// reaches every log line for the request, so an unreasonable one is replaced rather than
// carried.
const maxRequestIDLength = 128

type contextKey int

const requestIDContextKey contextKey = iota

// RequestIDFrom returns the correlation id carried by a request.
func RequestIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(requestIDContextKey).(string)
	return id
}

// requestID puts a correlation id on every response, echoed from the request when one was
// supplied and generated otherwise (ADR-0014).
func requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := acceptableRequestID(r.Header.Get(requestIDHeader))
		if id == "" {
			id = rand.Text()
		}

		w.Header().Set(requestIDHeader, id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDContextKey, id)))
	})
}

func acceptableRequestID(value string) string {
	if value == "" || len(value) > maxRequestIDLength {
		return ""
	}
	for _, character := range value {
		if character < '!' || character > '~' {
			return ""
		}
	}
	return value
}

// logRequests records one line per request. It reports through a defer so a request that
// panics is still logged, with the status the recovery middleware wrote.
func logRequests(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			started := time.Now()
			recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

			defer func() {
				logger.Info("request",
					"method", r.Method,
					"path", r.URL.Path,
					"status", recorder.status,
					"duration_ms", time.Since(started).Milliseconds(),
					"request_id", RequestIDFrom(r.Context()),
				)
			}()

			next.ServeHTTP(recorder, r)
		})
	}
}

// recoverPanics turns an unexpected fault into the documented 500, carrying no internal
// detail; the correlation id ties the response to the log entry that has it.
func recoverPanics(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

			defer func() {
				fault := recover()
				if fault == nil {
					return
				}

				logger.Error("panic recovered",
					"panic", fault,
					"method", r.Method,
					"path", r.URL.Path,
					"request_id", RequestIDFrom(r.Context()),
				)

				// Once the status line is out the response cannot be replaced; the
				// truncated body is all the client will get.
				if !recorder.wroteHeader {
					writeError(recorder, internalError())
				}
			}()

			next.ServeHTTP(recorder, r)
		})
	}
}

// corsPolicy is a no-op until an origin is configured, which is the development case: the
// Vite dev server proxies the API, so no cross-origin request is ever made (ADR-0012).
func corsPolicy(allowedOrigin string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if allowedOrigin == "" {
			return next
		}

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("Vary", "Origin")

			if r.Header.Get("Origin") != allowedOrigin {
				next.ServeHTTP(w, r)
				return
			}

			w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)

			// Only a genuine preflight is answered here. Any other OPTIONS is an
			// unsupported method on a real path, and the router owns that answer — 405 in
			// the envelope, the same as it gives with CORS disabled (ADR-0022).
			if r.Method != http.MethodOptions || r.Header.Get("Access-Control-Request-Method") == "" {
				next.ServeHTTP(w, r)
				return
			}

			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, "+requestIDHeader)
			w.Header().Set("Access-Control-Max-Age", "600")
			w.WriteHeader(http.StatusNoContent)
		})
	}
}

// statusRecorder remembers the status line so the logger can report it and the recovery
// middleware can tell whether the response has already been committed.
type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (s *statusRecorder) WriteHeader(status int) {
	if s.wroteHeader {
		return
	}
	s.status = status
	s.wroteHeader = true
	s.ResponseWriter.WriteHeader(status)
}

func (s *statusRecorder) Write(body []byte) (int, error) {
	if !s.wroteHeader {
		s.WriteHeader(http.StatusOK)
	}
	return s.ResponseWriter.Write(body)
}
