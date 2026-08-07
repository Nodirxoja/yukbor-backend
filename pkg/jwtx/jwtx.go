// Package jwtx is a dependency-free HS256 JWT implementation sufficient for
// the hackathon MVP: issue and verify access/refresh tokens with a shared
// secret. Swap for a full library (e.g. golang-jwt) if claims grow.
package jwtx

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Claims struct {
	Sub  string `json:"sub"`  // user id
	Role string `json:"role"` // UserRole
	Typ  string `json:"typ"`  // "access" | "refresh"
	Exp  int64  `json:"exp"`  // unix seconds
	Iat  int64  `json:"iat"`  // unix seconds
}

var (
	ErrInvalidToken = errors.New("invalid token")
	ErrExpiredToken = errors.New("expired token")
)

func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

// Sign issues an HS256 JWT for the given claims.
func Sign(secret []byte, c Claims) (string, error) {
	header := b64([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payload, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	signingInput := header + "." + b64(payload)
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(signingInput))
	return signingInput + "." + b64(mac.Sum(nil)), nil
}

// Verify checks signature and expiry and returns the claims.
func Verify(secret []byte, token string) (Claims, error) {
	var c Claims
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return c, ErrInvalidToken
	}
	signingInput := parts[0] + "." + parts[1]
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(signingInput))
	want := b64(mac.Sum(nil))
	if !hmac.Equal([]byte(want), []byte(parts[2])) {
		return c, ErrInvalidToken
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return c, ErrInvalidToken
	}
	if err := json.Unmarshal(payload, &c); err != nil {
		return c, ErrInvalidToken
	}
	if time.Now().Unix() >= c.Exp {
		return c, ErrExpiredToken
	}
	return c, nil
}

// NewAccess / NewRefresh build standard claim sets.
func NewAccess(userID, role string, ttl time.Duration) Claims {
	now := time.Now()
	return Claims{Sub: userID, Role: role, Typ: "access", Iat: now.Unix(), Exp: now.Add(ttl).Unix()}
}

func NewRefresh(userID, role string, ttl time.Duration) Claims {
	now := time.Now()
	return Claims{Sub: userID, Role: role, Typ: "refresh", Iat: now.Unix(), Exp: now.Add(ttl).Unix()}
}

// FromRequest extracts and verifies the bearer token of an incoming request.
func FromRequest(secret []byte, r *http.Request) (Claims, error) {
	h := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if !strings.HasPrefix(h, prefix) {
		return Claims{}, fmt.Errorf("%w: missing bearer token", ErrInvalidToken)
	}
	return Verify(secret, strings.TrimPrefix(h, prefix))
}
