// Package reviews owns post-completion reviews and the aggregated rating
// that feeds User.rating / User.ratingsCount.
package reviews

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/aventiseld/yukbor-backend/pkg/config"
	"github.com/aventiseld/yukbor-backend/pkg/httpx"
	"github.com/aventiseld/yukbor-backend/pkg/models"
	"github.com/aventiseld/yukbor-backend/pkg/svc"
)

type Handler struct {
	cfg    config.Config
	store  *Store
	orders *svc.Client
	auth   *svc.Client
	notify *svc.Client
}

func NewHandler(cfg config.Config, pool *pgxpool.Pool) *Handler {
	return &Handler{
		cfg:    cfg,
		store:  NewStore(pool),
		orders: svc.New(cfg.OrdersURL, cfg.InternalToken),
		auth:   svc.New(cfg.AuthURL, cfg.InternalToken),
		notify: svc.New(cfg.NotificationsURL, cfg.InternalToken),
	}
}

// Routes wires every endpoint from the API contract (§6).
func (h *Handler) Routes() http.Handler {
	secret := h.cfg.Secret()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", httpx.Health("reviews"))

	mux.HandleFunc("POST /reviews", httpx.Authed(secret, h.create))
	mux.HandleFunc("GET /reviews/rating", httpx.Authed(secret, h.rating))
	mux.HandleFunc("GET /reviews", httpx.Authed(secret, h.list))

	return httpx.Wrap(mux)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		OrderID    string  `json:"orderId"`
		ReviewerID string  `json:"reviewerId"`
		RevieweeID string  `json:"revieweeId"`
		Rating     int     `json:"rating"`
		Comment    *string `json:"comment,omitempty"`
	}
	if err := httpx.ReadJSONLoose(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid body")
		return
	}

	claims := httpx.Claims(r)
	if req.ReviewerID == "" {
		req.ReviewerID = claims.Sub
	}
	if req.ReviewerID != claims.Sub && claims.Role != string(models.RoleAdmin) {
		httpx.WriteError(w, http.StatusForbidden, httpx.CodeForbidden,
			"cannot review on behalf of another user")
		return
	}
	if req.Rating < 1 || req.Rating > 5 {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeValidation, "rating must be 1..5")
		return
	}
	if req.RevieweeID == "" || req.RevieweeID == req.ReviewerID {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeValidation,
			"revieweeId is required and cannot be yourself")
		return
	}

	// A review is only meaningful once the work is done (contract §6).
	var order models.Order
	if err := h.orders.Get(r.Context(), "/internal/orders/"+req.OrderID, &order); err != nil {
		if svc.CodeOf(err) == httpx.CodeOrderNotFound {
			httpx.WriteError(w, http.StatusNotFound, httpx.CodeOrderNotFound, "заказ не найден")
			return
		}
		h.fail(w, "load order", err)
		return
	}
	if !completed(order) {
		httpx.WriteError(w, http.StatusConflict, httpx.CodeOrderNotCompleted,
			"оставить отзыв можно только после завершения заказа")
		return
	}

	review, err := h.store.Create(r.Context(), models.Review{
		OrderID:    req.OrderID,
		ReviewerID: req.ReviewerID,
		RevieweeID: req.RevieweeID,
		Rating:     req.Rating,
		Comment:    req.Comment,
	})
	if errors.Is(err, ErrDuplicate) {
		httpx.WriteError(w, http.StatusConflict, httpx.CodeReviewAlreadyExists,
			"вы уже оставили отзыв по этому заказу")
		return
	}
	if err != nil {
		h.fail(w, "create review", err)
		return
	}

	h.propagate(r, review)
	httpx.WriteJSON(w, http.StatusCreated, review)
}

// propagate recomputes the reviewee's aggregate and pushes it into auth, which
// owns User.rating. Failures are logged, not surfaced: the review itself is
// already durable, and a stale rating is a smaller problem than a 500 on a
// review the user can see was saved.
func (h *Handler) propagate(r *http.Request, review *models.Review) {
	agg, err := h.store.Aggregate(r.Context(), review.RevieweeID)
	if err != nil {
		slog.Error("aggregate rating failed", "reviewee", review.RevieweeID, "err", err)
		return
	}
	if err := h.auth.Post(r.Context(), "/internal/users/"+review.RevieweeID+"/rating", agg, nil); err != nil {
		slog.Error("rating push failed", "reviewee", review.RevieweeID, "err", err)
	}

	h.notify.Fire("/internal/events", models.EmitEventRequest{
		UserIDs: []string{review.RevieweeID},
		Event:   models.EventNotificationCreated,
		Data:    review,
		Notify: &models.NotifySpec{
			Type:           models.NotifReviewReceived,
			Title:          "Новый отзыв",
			Body:           ratingText(review.Rating),
			RelatedOrderID: &review.OrderID,
		},
	})
}

func (h *Handler) rating(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("userId")
	if userID == "" {
		userID = httpx.Claims(r).Sub
	}
	agg, err := h.store.Aggregate(r.Context(), userID)
	if err != nil {
		h.fail(w, "aggregate rating", err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, agg)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("userId")
	if userID == "" {
		userID = httpx.Claims(r).Sub
	}
	items, err := h.store.ListByReviewee(r.Context(), userID)
	if err != nil {
		h.fail(w, "list reviews", err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, items)
}

func (h *Handler) fail(w http.ResponseWriter, op string, err error) {
	slog.Error("reviews", "op", op, "err", err)
	httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal error")
}

// completed checks every leg the order has, not just the top-level status —
// a combo order is only done when all of its legs are.
func completed(o models.Order) bool {
	if o.Status != models.StatusCompleted {
		return false
	}
	if o.EquipmentStatus != nil && *o.EquipmentStatus != models.StatusCompleted {
		return false
	}
	if o.LaborStatus != nil && *o.LaborStatus != models.StatusCompleted {
		return false
	}
	return true
}

func ratingText(rating int) string {
	return "Вам поставили оценку " + strconv.Itoa(rating) + " из 5"
}
