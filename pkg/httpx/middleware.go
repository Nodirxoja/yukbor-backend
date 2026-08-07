package httpx

import (
	"log/slog"
	"net/http"
	"runtime/debug"
)

// Recover turns a panic into a contract-shaped 500 instead of a dropped
// connection. On a demo day a single nil map must not take a service down.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				slog.Error("panic", "path", r.URL.Path, "err", v, "stack", string(debug.Stack()))
				WriteError(w, http.StatusInternalServerError, CodeInternal, "internal error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// Wrap applies the standard middleware stack every service router uses.
func Wrap(h http.Handler) http.Handler { return Recover(Logger(h)) }
