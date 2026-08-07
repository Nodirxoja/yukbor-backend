package httpx

import (
	"context"
	"net/http"

	"github.com/aventiseld/yukbor-backend/pkg/jwtx"
)

type ctxKey int

const claimsKey ctxKey = iota

// Authed requires a valid, non-expired access token. JWT validation lives in
// each service (plan §2) rather than the gateway, so this wraps individual
// routes — the OTP and health endpoints stay open.
func Authed(secret []byte, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, err := jwtx.FromRequest(secret, r)
		if err != nil {
			WriteError(w, http.StatusUnauthorized, CodeUnauthorized, "valid access token required")
			return
		}
		if claims.Typ != "access" {
			WriteError(w, http.StatusUnauthorized, CodeUnauthorized, "refresh token cannot be used for access")
			return
		}
		next(w, r.WithContext(WithClaims(r.Context(), claims)))
	}
}

// AuthedRole additionally restricts a route to the listed roles — used by the
// /admin/* endpoints the dashboard consumes (plan §11).
func AuthedRole(secret []byte, roles []string, next http.HandlerFunc) http.HandlerFunc {
	return Authed(secret, func(w http.ResponseWriter, r *http.Request) {
		got := Claims(r).Role
		for _, want := range roles {
			if got == want {
				next(w, r)
				return
			}
		}
		WriteError(w, http.StatusForbidden, CodeForbidden, "insufficient role")
	})
}

// AuthedOrInternal accepts either a user's access token or the shared internal
// token. The escrow endpoints need this: the contract exposes them publicly
// (§4), but in practice the orders service is what calls them, on behalf of a
// user whose token it does not hold.
func AuthedOrInternal(secret []byte, internalToken string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if internalToken != "" && r.Header.Get("X-Internal-Token") == internalToken {
			next(w, r)
			return
		}
		Authed(secret, next)(w, r)
	}
}

// WithClaims stores verified claims on a context (also used by the WS handler,
// which authenticates via a query parameter rather than a header).
func WithClaims(ctx context.Context, c jwtx.Claims) context.Context {
	return context.WithValue(ctx, claimsKey, c)
}

// Claims returns the verified claims of an authenticated request. It returns
// the zero value on unauthenticated routes, so only call it behind Authed.
func Claims(r *http.Request) jwtx.Claims {
	c, _ := r.Context().Value(claimsKey).(jwtx.Claims)
	return c
}
