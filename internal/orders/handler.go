// Package orders owns the order lifecycle: creation, per-leg accept/status,
// cancellation, client confirmation (which triggers per-payee escrow release
// in the wallet service), and backhaul search.
package orders

import (
	"net/http"

	"github.com/aventiseld/yukbor-backend/pkg/config"
	"github.com/aventiseld/yukbor-backend/pkg/httpx"
)

type Handler struct {
	cfg config.Config
	// store *Store // TODO(day-1/2): postgres store; legs stored as rows,
	//                 flattened to the contract's Order shape on the way out.
}

func NewHandler(cfg config.Config) *Handler { return &Handler{cfg: cfg} }

// Routes wires every endpoint from the API contract (§3).
func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", httpx.Health("orders"))

	mux.HandleFunc("POST /orders", h.create)                          // TODO(day-1)
	mux.HandleFunc("GET /orders", h.listByClient)                     // TODO(day-1)  ?clientId=
	mux.HandleFunc("GET /orders/available", h.available)              // TODO(day-2)  ?leg=
	mux.HandleFunc("GET /orders/backhaul", h.backhaul)                // TODO(day-3)  ?dropoffLat=&dropoffLng=&excludeOrderId=
	mux.HandleFunc("GET /orders/{id}", h.get)                         // TODO(day-1)
	mux.HandleFunc("POST /orders/{id}/accept", h.accept)              // TODO(day-2)  atomic per-leg claim
	mux.HandleFunc("PATCH /orders/{id}/status", h.updateStatus)       // TODO(day-2)  per-leg, forward-only
	mux.HandleFunc("POST /orders/{id}/cancel", h.cancel)              // TODO(day-2)
	mux.HandleFunc("POST /orders/{id}/confirm-completion", h.confirm) // TODO(day-2)  → wallet release per payee
	mux.HandleFunc("POST /orders/estimate", h.estimate)               // TODO(day-2)  server-side price formula

	return httpx.Logger(mux)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request)       { httpx.NotImplemented(w, r) }
func (h *Handler) listByClient(w http.ResponseWriter, r *http.Request) { httpx.NotImplemented(w, r) }
func (h *Handler) available(w http.ResponseWriter, r *http.Request)    { httpx.NotImplemented(w, r) }
func (h *Handler) backhaul(w http.ResponseWriter, r *http.Request)     { httpx.NotImplemented(w, r) }
func (h *Handler) get(w http.ResponseWriter, r *http.Request)          { httpx.NotImplemented(w, r) }
func (h *Handler) accept(w http.ResponseWriter, r *http.Request)       { httpx.NotImplemented(w, r) }
func (h *Handler) updateStatus(w http.ResponseWriter, r *http.Request) { httpx.NotImplemented(w, r) }
func (h *Handler) cancel(w http.ResponseWriter, r *http.Request)       { httpx.NotImplemented(w, r) }
func (h *Handler) confirm(w http.ResponseWriter, r *http.Request)      { httpx.NotImplemented(w, r) }
func (h *Handler) estimate(w http.ResponseWriter, r *http.Request)     { httpx.NotImplemented(w, r) }
