// Package middleware provides HTTP middleware for request tracing, structured
// logging, and panic recovery for the booking-service API.
package middleware

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// ctxKey is an unexported type used as the key for context-scoped values.
type ctxKey string

// requestIDKey is the context key under which the current request id is stored.
const requestIDKey ctxKey = "request-id"

// statusRecorder wraps an http.ResponseWriter so downstream handlers can
// capture the status code that was ultimately written.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

// WriteHeader records the status code before delegating to the wrapped writer.
func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

// RequestID ensures every request has a request id and echoes it back to the
// client in the X-Request-ID response header. If the request already carries an
// X-Request-ID header, that value is reused; otherwise a new id is generated.
// The id is also stored in the request context for downstream middlewares and
// handlers to read.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = uuid.NewString()
		}
		w.Header().Set("X-Request-ID", requestID)
		ctx := context.WithValue(r.Context(), requestIDKey, requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequestLogger emits a structured log line for each completed request,
// including the request id, method, path, response status, duration, and remote
// address. It should be placed inside RequestID so the request id is available.
func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)

		requestID, _ := r.Context().Value(requestIDKey).(string)
		slog.Info("request",
			"request_id", requestID,
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
			"remote_addr", r.RemoteAddr,
		)
	})
}

// Recoverer is a recovery middleware that intercepts panics from downstream
// handlers, logs them, and writes a generic 500 response using the standard
// error envelope so clients always receive a well-formed JSON body. It should
// run inside both RequestID and RequestLogger so recovered requests are still
// traced and logged.
func Recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				requestID, _ := r.Context().Value(requestIDKey).(string)
				slog.Error("panic recovered", "request_id", requestID, "panic", rec)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"error": map[string]any{
						"code":    "internal_error",
						"message": "internal server error",
					},
				})
			}
		}()
		next.ServeHTTP(w, r)
	})
}
