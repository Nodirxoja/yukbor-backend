// Package notifications owns the notification feed and the realtime channel.
// Other services emit events via POST /internal/events; the hub fans them out
// to connected users and persists notification rows.
package notifications

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/aventiseld/yukbor-backend/pkg/config"
	"github.com/aventiseld/yukbor-backend/pkg/db"
	"github.com/aventiseld/yukbor-backend/pkg/httpx"
	"github.com/aventiseld/yukbor-backend/pkg/jwtx"
	"github.com/aventiseld/yukbor-backend/pkg/models"
)

// pingInterval keeps idle connections alive through proxies; iOS falls back to
// polling if the socket drops anyway (contract §7).
const pingInterval = 30 * time.Second

type Handler struct {
	cfg   config.Config
	store *Store
	hub   *Hub
}

func NewHandler(cfg config.Config, pool *pgxpool.Pool) *Handler {
	return &Handler{cfg: cfg, store: NewStore(pool), hub: NewHub()}
}

// Routes wires every endpoint from the API contract (§5, §7).
func (h *Handler) Routes() http.Handler {
	secret := h.cfg.Secret()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", httpx.Health("notifications"))

	mux.HandleFunc("GET /notifications", httpx.Authed(secret, h.list))
	mux.HandleFunc("PATCH /notifications/{id}/read", httpx.Authed(secret, h.markRead))

	// Realtime channel (§7): one connection per user, events
	// order.updated | order.created | transaction.updated | notification.created
	mux.HandleFunc("GET /ws", h.websocket)

	// Internal: other services emit events here (fire-and-forget callers).
	mux.HandleFunc("POST /internal/events",
		httpx.InternalOnly(h.cfg.InternalToken, h.emit))

	return httpx.Wrap(mux)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	claims := httpx.Claims(r)
	userID := r.URL.Query().Get("userId")
	if userID == "" {
		userID = claims.Sub
	}
	if userID != claims.Sub && claims.Role != string(models.RoleAdmin) {
		httpx.WriteError(w, http.StatusForbidden, httpx.CodeForbidden,
			"cannot read another user's notifications")
		return
	}

	items, err := h.store.ListByUser(r.Context(), userID)
	if err != nil {
		h.fail(w, "list notifications", err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, items)
}

func (h *Handler) markRead(w http.ResponseWriter, r *http.Request) {
	err := h.store.MarkRead(r.Context(), r.PathValue("id"), httpx.Claims(r).Sub)
	if errors.Is(err, db.ErrNotFound) {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "notification not found")
		return
	}
	if err != nil {
		h.fail(w, "mark read", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// websocket upgrades and streams this user's events until they disconnect.
//
// Auth is by JWT, taken from ?token= or the Authorization header — a plain
// ?userId= would let anyone subscribe to anyone's stream. The plan sketched
// ?userId=; that is the one place this implementation deliberately differs.
func (h *Handler) websocket(w http.ResponseWriter, r *http.Request) {
	claims, err := h.wsClaims(r)
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthorized,
			"valid access token required (?token= or Authorization header)")
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		// The dashboard and the iOS app are not same-origin with the gateway.
		InsecureSkipVerify: true,
	})
	if err != nil {
		slog.Warn("ws upgrade failed", "err", err)
		return
	}
	defer conn.CloseNow()

	events, unsubscribe := h.hub.Subscribe(claims.Sub)
	defer unsubscribe()

	ctx := conn.CloseRead(r.Context()) // we never read; this detects disconnects
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	slog.Info("ws connected", "user", claims.Sub, "connections", h.hub.Count(claims.Sub))

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			err := conn.Ping(pingCtx)
			cancel()
			if err != nil {
				return
			}
		case ev, ok := <-events:
			if !ok {
				return
			}
			writeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			err := wsjson.Write(writeCtx, conn, ev)
			cancel()
			if err != nil {
				slog.Warn("ws write failed", "user", claims.Sub, "err", err)
				return
			}
		}
	}
}

// wsClaims accepts the token from a query parameter as well as the header:
// browser WebSocket clients cannot set headers.
func (h *Handler) wsClaims(r *http.Request) (jwtx.Claims, error) {
	if token := r.URL.Query().Get("token"); token != "" {
		return jwtx.Verify(h.cfg.Secret(), token)
	}
	return jwtx.FromRequest(h.cfg.Secret(), r)
}

// emit fans an event out to connected users and, when the caller asked for it,
// persists a notification row so the feed survives a disconnected client.
func (h *Handler) emit(w http.ResponseWriter, r *http.Request) {
	var req models.EmitEventRequest
	if err := httpx.ReadJSONLoose(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid body")
		return
	}

	if req.Event != "" && req.Data != nil {
		h.hub.Publish(req.UserIDs, models.WSEvent{Event: req.Event, Data: req.Data})
	}

	if req.Notify != nil {
		for _, userID := range req.NotifyRecipients() {
			note, err := h.store.Create(r.Context(), userID, *req.Notify)
			if err != nil {
				slog.Error("could not persist notification", "user", userID, "err", err)
				continue
			}
			h.hub.Publish([]string{userID},
				models.WSEvent{Event: models.EventNotificationCreated, Data: note})
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) fail(w http.ResponseWriter, op string, err error) {
	slog.Error("notifications", "op", op, "err", err)
	httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal error")
}
