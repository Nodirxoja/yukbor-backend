// Package config loads service configuration from the environment.
package config

import (
	"crypto/subtle"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Port          string // service listen port
	AppEnv        string // "dev" | "prod" — gates demo affordances (plan §10)
	DatabaseURL   string // postgres://user:pass@host:5432/yukbor?sslmode=disable
	JWTSecret     string // shared HS256 secret for all services
	InternalToken string // shared secret for service-to-service calls

	// Phone numbers that accept TestOTPCode instead of a real SMS code, so the
	// mobile app can be built against production before an SMS gateway exists.
	TestPhones  string // comma-separated, or "*" for every number
	TestOTPCode string // the code those numbers accept

	// OTPRateLimit is how many codes a single phone may request per 10-minute
	// window. 0 disables the limit entirely — useful while testing a flow by
	// hand, where the normal limit locks you out after three tries.
	OTPRateLimit int

	// Dashboard sign-in. The back office is a different audience from the app:
	// operators are not customers, do not necessarily have a handset to hand,
	// and should not have to wait for an SMS to read a chart. So it has its own
	// username/password rather than reusing the phone flow.
	AdminUsername string
	AdminPassword string

	// Downstream service base URLs (used by orders → wallet/notifications).
	AuthURL          string
	OrdersURL        string
	WalletURL        string
	NotificationsURL string
	ReviewsURL       string
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return n
		}
	}
	return def
}

// Load reads config with sane docker-compose defaults.
func Load(defaultPort string) Config {
	return Config{
		Port:          env("PORT", defaultPort),
		AppEnv:        env("APP_ENV", "dev"),
		DatabaseURL:   env("DATABASE_URL", "postgres://yukbor:yukbor@localhost:5432/yukbor?sslmode=disable"),
		JWTSecret:     env("JWT_SECRET", "dev-secret-change-me"),
		InternalToken: env("INTERNAL_TOKEN", "dev-internal-token"),
		TestPhones:    env("TEST_PHONES", ""),
		TestOTPCode:   env("TEST_OTP_CODE", "0000"),
		OTPRateLimit:  envInt("OTP_RATE_LIMIT", 3),
		AdminUsername: env("ADMIN_USERNAME", "admin"),
		AdminPassword: env("ADMIN_PASSWORD", ""),

		AuthURL:          env("AUTH_URL", "http://localhost:8081"),
		OrdersURL:        env("ORDERS_URL", "http://localhost:8082"),
		WalletURL:        env("WALLET_URL", "http://localhost:8083"),
		NotificationsURL: env("NOTIFICATIONS_URL", "http://localhost:8084"),
		ReviewsURL:       env("REVIEWS_URL", "http://localhost:8085"),
	}
}

// IsProd reports whether demo affordances (OTP master code, simulated
// upstreams' deterministic triggers) must be disabled.
func (c Config) IsProd() bool { return c.AppEnv == "prod" }

// TestPhonesAreUniversal reports whether TEST_PHONES is the "*" wildcard, i.e.
// TestOTPCode is accepted for EVERY number.
//
// That makes the OTP prove nothing: anyone can sign in as any phone. It exists
// so the product can be demonstrated end to end before an SMS gateway is wired,
// and it is a single config value precisely so it can be revoked in one line
// without a rebuild. The auth service logs a warning on every start while it is
// on, so it cannot be running unnoticed.
func (c Config) TestPhonesAreUniversal() bool {
	return strings.TrimSpace(c.TestPhones) == "*"
}

// AdminPasswordMatches compares in constant time, so a wrong password takes the
// same time to reject regardless of how much of it was right.
func (c Config) AdminPasswordMatches(username, password string) bool {
	if c.AdminPassword == "" {
		return false // no password configured: dashboard sign-in is closed
	}
	userOK := subtle.ConstantTimeCompare([]byte(username), []byte(c.AdminUsername)) == 1
	passOK := subtle.ConstantTimeCompare([]byte(password), []byte(c.AdminPassword)) == 1
	return userOK && passOK
}

// TestPhoneSet parses TEST_PHONES into a lookup set.
//
// These numbers accept TestOTPCode instead of the real SMS code, so the mobile
// app can be developed against production before an SMS gateway is wired.
func (c Config) TestPhoneSet() map[string]bool {
	out := map[string]bool{}
	for _, p := range strings.Split(c.TestPhones, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out[p] = true
		}
	}
	return out
}

// Secret returns the HS256 signing key as bytes.
func (c Config) Secret() []byte { return []byte(c.JWTSecret) }
