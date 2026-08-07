// Package httpx contains the small shared HTTP toolkit: contract-compliant
// error responses, JSON helpers, and common middleware.
package httpx

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

// APIError is the single error envelope from the API contract:
//
//	{ "error": { "code": "OTP_INVALID", "message": "..." } }
type APIError struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

func WriteError(w http.ResponseWriter, status int, code, message string) {
	var e APIError
	e.Error.Code = code
	e.Error.Message = message
	WriteJSON(w, status, e)
}

// NotImplemented is the placeholder handler used by skeleton endpoints.
func NotImplemented(w http.ResponseWriter, _ *http.Request) {
	WriteError(w, http.StatusNotImplemented, "NOT_IMPLEMENTED", "endpoint is scaffolded but not implemented yet")
}

// ReadJSON decodes the request body into v, rejecting unknown fields early so
// contract drift is caught in development instead of production.
func ReadJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

// Health returns a handler for GET /health.
func Health(service string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		WriteJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": service})
	}
}

// Logger is a minimal request-logging middleware.
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		slog.Info("http", "method", r.Method, "path", r.URL.Path, "dur", time.Since(start).String())
	})
}

// InternalOnly guards service-to-service endpoints with a shared secret.
func InternalOnly(token string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if token == "" || r.Header.Get("X-Internal-Token") != token {
			WriteError(w, http.StatusForbidden, "FORBIDDEN", "internal endpoint")
			return
		}
		next(w, r)
	}
}
