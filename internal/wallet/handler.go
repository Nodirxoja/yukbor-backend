// Package wallet owns escrow: money is held when a leg is accepted and
// released per (orderId, payeeId) after client confirmation. Amounts are
// decimal strings (UZS) end-to-end — never floats.
package wallet

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"time"

	"github.com/aventiseld/yukbor-backend/pkg/config"
	"github.com/aventiseld/yukbor-backend/pkg/httpx"
	"github.com/aventiseld/yukbor-backend/pkg/models"
)

type Handler struct {
	cfg config.Config
	// store *Store // TODO(day-2): postgres store with UNIQUE(order_id, payee_id)
}

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

func NewHandler(cfg config.Config) *Handler { return &Handler{cfg: cfg} }

// Routes wires every endpoint from the API contract (§4).
func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", httpx.Health("wallet"))

	mux.HandleFunc("POST /wallet/transactions", h.create)          // TODO(day-2) commission = 10%, computed server-side
	mux.HandleFunc("POST /wallet/transactions/release", h.release) // TODO(day-2) idempotent: already released → 200
	mux.HandleFunc("GET /wallet/transactions", h.listByPayee)      // TODO(day-2) ?payeeId=

	return httpx.Logger(mux)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request)      { httpx.NotImplemented(w, r) }
func (h *Handler) release(w http.ResponseWriter, r *http.Request)     { httpx.NotImplemented(w, r) }
func (h *Handler) listByPayee(w http.ResponseWriter, r *http.Request) { httpx.NotImplemented(w, r) }
