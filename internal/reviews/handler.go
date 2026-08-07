// Package reviews owns post-completion reviews and the aggregated rating
// that feeds User.rating / User.ratingsCount.
package reviews

import (
	"net/http"

	"github.com/aventiseld/yukbor-backend/pkg/config"
	"github.com/aventiseld/yukbor-backend/pkg/httpx"
)

type Handler struct {
	cfg config.Config
	// store *Store // TODO(day-3): postgres store, UNIQUE(order_id, reviewer_id, reviewee_id)
}

func NewHandler(cfg config.Config) *Handler { return &Handler{cfg: cfg} }

// Routes wires every endpoint from the API contract (§6).
func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", httpx.Health("reviews"))

	mux.HandleFunc("POST /reviews", h.create)       // TODO(day-3) only after order completed
	mux.HandleFunc("GET /reviews/rating", h.rating) // TODO(day-3) ?userId= → { rating, count }

	return httpx.Logger(mux)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) { httpx.NotImplemented(w, r) }
func (h *Handler) rating(w http.ResponseWriter, r *http.Request) { httpx.NotImplemented(w, r) }
