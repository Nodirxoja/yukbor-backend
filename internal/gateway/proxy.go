// Package gateway is a thin reverse proxy giving iOS a single base URL.
// No business logic lives here — just path routing, CORS, and logging.
package gateway

import (
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/aventiseld/yukbor-backend/pkg/config"
	"github.com/aventiseld/yukbor-backend/pkg/httpx"
)

func proxyTo(rawURL string) http.Handler {
	target, err := url.Parse(rawURL)
	if err != nil {
		panic("gateway: bad upstream url " + rawURL)
	}
	return httputil.NewSingleHostReverseProxy(target)
}

// healthOf proxies to an upstream's /health regardless of the incoming path,
// so /health/<service> reaches a service that is not published on a host port.
func healthOf(rawURL string) http.Handler {
	p := proxyTo(rawURL)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = "/health"
		p.ServeHTTP(w, r)
	})
}

// Routes maps public path prefixes to their owning service.
func Routes(cfg config.Config) http.Handler {
	auth := proxyTo(cfg.AuthURL)
	orders := proxyTo(cfg.OrdersURL)
	wallet := proxyTo(cfg.WalletURL)
	notifs := proxyTo(cfg.NotificationsURL)
	reviews := proxyTo(cfg.ReviewsURL)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", httpx.Health("gateway"))

	// Aggregate health: only the gateway is published on a host port, so
	// /health/<service> is how scripts/smoke.sh proves the whole stack is up.
	mux.Handle("GET /health/auth", healthOf(cfg.AuthURL))
	mux.Handle("GET /health/orders", healthOf(cfg.OrdersURL))
	mux.Handle("GET /health/wallet", healthOf(cfg.WalletURL))
	mux.Handle("GET /health/notifications", healthOf(cfg.NotificationsURL))
	mux.Handle("GET /health/reviews", healthOf(cfg.ReviewsURL))

	mux.Handle("/auth/", auth)
	mux.Handle("/users/", auth)
	mux.Handle("/orders", orders)
	mux.Handle("/orders/", orders)
	mux.Handle("/wallet/", wallet)
	mux.Handle("/notifications", notifs)
	mux.Handle("/notifications/", notifs)
	mux.Handle("/ws", notifs)
	mux.Handle("/reviews", reviews)
	mux.Handle("/reviews/", reviews)

	// Admin surface for the dashboard (plan §11). Each endpoint is owned by
	// the service that holds the data and is guarded there by an admin-role
	// JWT — the gateway stays a dumb router.
	mux.Handle("/admin/users", auth)
	mux.Handle("/admin/orders", orders)
	mux.Handle("/admin/stats", wallet)
	mux.Handle("/admin/transactions", wallet)

	return httpx.Wrap(cors(mux))
}

// cors is permissive for the hackathon; tighten before production.
func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
