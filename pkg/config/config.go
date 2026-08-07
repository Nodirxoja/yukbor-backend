// Package config loads service configuration from the environment.
package config

import (
	"os"
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
	TestPhones  string // comma-separated, e.g. "+998900000001,+998900000002"
	TestOTPCode string // the code those numbers accept

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

// TestPhoneSet parses TEST_PHONES into a lookup set.
//
// These numbers accept TestOTPCode instead of the real SMS code, so the mobile
// app can be developed against production before an SMS gateway is wired. The
// allowlist is deliberately explicit and per-number: it is not a mode, so it
// cannot be left switched on by accident, and it never weakens login for a
// real user's phone.
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
