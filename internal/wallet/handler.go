// Package wallet owns escrow: money is held when a leg is accepted and
// released per (orderId, payeeId) after client confirmation. Amounts are
// decimal strings (UZS) end-to-end — never floats.
package wallet

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/aventiseld/yukbor-backend/pkg/config"
	"github.com/aventiseld/yukbor-backend/pkg/db"
	"github.com/aventiseld/yukbor-backend/pkg/httpx"
	"github.com/aventiseld/yukbor-backend/pkg/models"
	"github.com/aventiseld/yukbor-backend/pkg/svc"
)

// PaymentProvider is the seam for Payme/Click/Uzcard. MVP binds
// SimulatedPaymentProvider — the hold/release/refund ledger is fully real,
// only the charge against the provider is imitated (plan §10).
type PaymentProvider interface {
	// Charge authorizes the payment and returns a provider reference
	// (e.g. "payme_txn_9f2c..."). Demo failure trigger: amount
	// "999999999" → PAYMENT_DECLINED.
	Charge(ctx context.Context, method models.PaymentMethod, amount string) (providerRef string, err error)
}

var ErrPaymentDeclined = errors.New("PAYMENT_DECLINED")

// SimulatedPaymentProvider imitates a PSP with realistic latency and a
// plausible transaction reference.
type SimulatedPaymentProvider struct{}

func (SimulatedPaymentProvider) Charge(ctx context.Context, method models.PaymentMethod, amount string) (string, error) {
	select {
	case <-time.After(1500 * time.Millisecond):
	case <-ctx.Done():
		return "", ctx.Err()
	}
	if amount == "999999999" {
		return "", ErrPaymentDeclined
	}
	h := sha256.Sum256([]byte(string(method) + amount))
	return string(method) + "_txn_" + hex.EncodeToString(h[:6]), nil
}

type Handler struct {
	cfg      config.Config
	store    *Store
	provider PaymentProvider
	orders   *svc.Client
	auth     *svc.Client
}

func NewHandler(cfg config.Config, pool *pgxpool.Pool) *Handler {
	return &Handler{
		cfg:      cfg,
		store:    NewStore(pool),
		provider: SimulatedPaymentProvider{},
		orders:   svc.New(cfg.OrdersURL, cfg.InternalToken),
		auth:     svc.New(cfg.AuthURL, cfg.InternalToken),
	}
}

// Routes wires every endpoint from the API contract (§4).
func (h *Handler) Routes() http.Handler {
	secret := h.cfg.Secret()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", httpx.Health("wallet"))

	mux.HandleFunc("POST /wallet/transactions",
		httpx.AuthedOrInternal(secret, h.cfg.InternalToken, h.create))
	mux.HandleFunc("POST /wallet/transactions/release",
		httpx.AuthedOrInternal(secret, h.cfg.InternalToken, h.release))
	mux.HandleFunc("GET /wallet/transactions", httpx.Authed(secret, h.listByPayee))

	// Dashboard (plan §11)
	mux.HandleFunc("GET /admin/stats",
		httpx.AuthedRole(secret, []string{string(models.RoleAdmin)}, h.adminStats))

	// Internal: cancellation returns held money to the payer.
	mux.HandleFunc("POST /internal/transactions/refund",
		httpx.InternalOnly(h.cfg.InternalToken, h.refund))
	mux.HandleFunc("GET /internal/transactions",
		httpx.InternalOnly(h.cfg.InternalToken, h.listByOrder))

	return httpx.Wrap(mux)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		OrderID       string               `json:"orderId"`
		OrderTitle    string               `json:"orderTitle"`
		PayerID       string               `json:"payerId"`
		PayeeID       string               `json:"payeeId"`
		Amount        string               `json:"amount"`
		PaymentMethod models.PaymentMethod `json:"paymentMethod"`
	}
	if err := httpx.ReadJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid body")
		return
	}

	amount, err := strconv.ParseInt(req.Amount, 10, 64)
	if err != nil || amount < 0 {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeValidation,
			"amount must be a whole number of UZS as a string")
		return
	}
	if !validMethod(req.PaymentMethod) {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeValidation, "unknown paymentMethod")
		return
	}

	// Idempotency first: a retried accept must not charge the card twice.
	if existing, err := h.store.Existing(r.Context(), req.OrderID, req.PayeeID); err == nil {
		httpx.WriteJSON(w, http.StatusOK, existing)
		return
	} else if !errors.Is(err, db.ErrNotFound) {
		h.fail(w, "lookup transaction", err)
		return
	}

	providerRef, err := h.provider.Charge(r.Context(), req.PaymentMethod, req.Amount)
	if errors.Is(err, ErrPaymentDeclined) {
		httpx.WriteError(w, http.StatusPaymentRequired, httpx.CodePaymentDeclined,
			"платёж отклонён провайдером")
		return
	}
	if err != nil {
		h.fail(w, "charge", err)
		return
	}

	pct, err := h.store.CommissionPct(r.Context())
	if err != nil {
		h.fail(w, "read commission", err)
		return
	}

	rec, created, err := h.store.Create(r.Context(), NewTx{
		OrderID:       req.OrderID,
		OrderTitle:    req.OrderTitle,
		PayerID:       req.PayerID,
		PayeeID:       req.PayeeID,
		Amount:        amount,
		Commission:    amount * pct / 100, // computed server-side only (§12)
		PaymentMethod: req.PaymentMethod,
		ProviderRef:   providerRef,
	})
	if err != nil {
		h.fail(w, "create transaction", err)
		return
	}

	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	httpx.WriteJSON(w, status, rec)
}

func (h *Handler) release(w http.ResponseWriter, r *http.Request) {
	var req struct {
		OrderID string `json:"orderId"`
		PayeeID string `json:"payeeId"`
	}
	if err := httpx.ReadJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid body")
		return
	}

	rec, err := h.store.Release(r.Context(), req.OrderID, req.PayeeID)
	if errors.Is(err, db.ErrNotFound) {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeTransactionNotFound,
			"escrow transaction not found")
		return
	}
	if err != nil {
		h.fail(w, "release", err)
		return
	}
	// Already-released returns 200 with current state, so retries are safe.
	httpx.WriteJSON(w, http.StatusOK, rec)
}

func (h *Handler) listByPayee(w http.ResponseWriter, r *http.Request) {
	claims := httpx.Claims(r)
	payeeID := r.URL.Query().Get("payeeId")
	if payeeID == "" {
		payeeID = claims.Sub
	}
	// An executor may only read their own ledger.
	if payeeID != claims.Sub && claims.Role != string(models.RoleAdmin) {
		httpx.WriteError(w, http.StatusForbidden, httpx.CodeForbidden,
			"cannot read another user's transactions")
		return
	}

	txs, err := h.store.ListByPayee(r.Context(), payeeID)
	if err != nil {
		h.fail(w, "list transactions", err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, txs)
}

func (h *Handler) listByOrder(w http.ResponseWriter, r *http.Request) {
	txs, err := h.store.ListByOrder(r.Context(), r.URL.Query().Get("orderId"))
	if err != nil {
		h.fail(w, "list transactions", err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, txs)
}

func (h *Handler) refund(w http.ResponseWriter, r *http.Request) {
	var req struct {
		OrderID string `json:"orderId"`
	}
	if err := httpx.ReadJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid body")
		return
	}
	txs, err := h.store.RefundOrder(r.Context(), req.OrderID)
	if err != nil {
		h.fail(w, "refund", err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, txs)
}

// adminStats merges wallet's own money aggregates with counts owned by orders
// and auth. A failure upstream degrades a number to zero rather than failing
// the whole dashboard.
func (h *Handler) adminStats(w http.ResponseWriter, r *http.Request) {
	credited, fees, held, err := h.store.MoneyStats(r.Context())
	if err != nil {
		h.fail(w, "money stats", err)
		return
	}

	stats := models.AdminStats{
		CreditedToExecutors: strconv.FormatInt(credited, 10),
		ServiceFeesCharged:  strconv.FormatInt(fees, 10),
		HeldInEscrow:        strconv.FormatInt(held, 10),
	}

	var orderStats models.OrderStats
	if err := h.orders.Get(r.Context(), "/internal/orders/stats", &orderStats); err != nil {
		slog.Warn("order stats unavailable", "err", err)
	} else {
		stats.TotalOrders = orderStats.Total
		stats.ActiveOrders = orderStats.Active
		stats.CompletedOrders = orderStats.Completed
	}

	var userStats models.UserStats
	if err := h.auth.Get(r.Context(), "/internal/users/stats", &userStats); err != nil {
		slog.Warn("user stats unavailable", "err", err)
	} else {
		stats.RegisteredUsers = userStats.Total
	}

	httpx.WriteJSON(w, http.StatusOK, stats)
}

func (h *Handler) fail(w http.ResponseWriter, op string, err error) {
	slog.Error("wallet", "op", op, "err", err)
	httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal error")
}

func validMethod(m models.PaymentMethod) bool {
	switch m {
	case models.PaymentPayme, models.PaymentClick, models.PaymentUzcard:
		return true
	}
	return false
}
