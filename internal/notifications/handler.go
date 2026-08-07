// Package notifications owns the notification feed and the realtime channel.
// Other services emit events via POST /internal/events; the hub fans them out
// to connected users and persists notification rows.
package notifications

import (
	"net/http"

	"github.com/aventiseld/yukbor-backend/pkg/config"
	"github.com/aventiseld/yukbor-backend/pkg/httpx"
)

type Handler struct {
	cfg config.Config
	hub *Hub
	// store *Store // TODO(day-3): postgres store
}

func NewHandler(cfg config.Config) *Handler {
	return &Handler{cfg: cfg, hub: NewHub()}
}

// Routes wires every endpoint from the API contract (§5, §7).
func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", httpx.Health("notifications"))

	mux.HandleFunc("GET /notifications", h.list)                 // TODO(day-3) ?userId=
	mux.HandleFunc("PATCH /notifications/{id}/read", h.markRead) // TODO(day-3)

	// Realtime channel (§7): one WS connection per user,
	// events: order.updated | order.created | transaction.updated | notification.created
	mux.HandleFunc("GET /ws", h.websocket) // TODO(day-3): upgrade via nhooyr.io/websocket

	// Internal: other services emit events here (fire-and-forget callers).
	mux.HandleFunc("POST /internal/events",
		httpx.InternalOnly(h.cfg.InternalToken, h.emit)) // TODO(day-3)

	return httpx.Logger(mux)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request)      { httpx.NotImplemented(w, r) }
func (h *Handler) markRead(w http.ResponseWriter, r *http.Request)  { httpx.NotImplemented(w, r) }
func (h *Handler) websocket(w http.ResponseWriter, r *http.Request) { httpx.NotImplemented(w, r) }
func (h *Handler) emit(w http.ResponseWriter, r *http.Request)      { httpx.NotImplemented(w, r) }
