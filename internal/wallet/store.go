package wallet

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/aventiseld/yukbor-backend/pkg/db"
	"github.com/aventiseld/yukbor-backend/pkg/models"
)

// Store is the wallet schema. One row per (orderId, payeeId): a combo order
// settles with each executor independently (contract §4).
type Store struct{ pool *pgxpool.Pool }

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// TxRecord is a transaction row. models.Transaction is the wire projection.
type TxRecord struct {
	models.Transaction
	ProviderRef *string    `json:"providerRef"`
	RefundedAt  *time.Time `json:"refundedAt"`
}

const txColumns = `
	id, order_id, order_title, payer_id, payee_id,
	amount::bigint, platform_commission::bigint, payment_method, status,
	provider_ref, created_at, released_at, refunded_at`

func scanTx(row pgx.Row) (*TxRecord, error) {
	var (
		t                  TxRecord
		amount, commission int64
	)
	err := row.Scan(&t.ID, &t.OrderID, &t.OrderTitle, &t.PayerID, &t.PayeeID,
		&amount, &commission, &t.PaymentMethod, &t.Status,
		&t.ProviderRef, &t.CreatedAt, &t.ReleasedAt, &t.RefundedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, db.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	// Money crosses the wire as decimal strings, never floats (plan §12).
	t.Amount = strconv.FormatInt(amount, 10)
	t.PlatformCommission = strconv.FormatInt(commission, 10)
	return &t, nil
}

// NewTx is a validated escrow request with the commission already computed.
type NewTx struct {
	OrderID       string
	OrderTitle    string
	PayerID       string
	PayeeID       string
	Amount        int64
	Commission    int64
	PaymentMethod models.PaymentMethod
	ProviderRef   string
}

// Existing returns the transaction for a pair, if any.
func (s *Store) Existing(ctx context.Context, orderID, payeeID string) (*TxRecord, error) {
	return scanTx(s.pool.QueryRow(ctx, `SELECT `+txColumns+`
		FROM wallet.transactions WHERE order_id = $1 AND payee_id = $2`, orderID, payeeID))
}

// Create opens escrow. It is idempotent on (order_id, payee_id): a retried
// accept returns the existing transaction rather than double-charging, and
// `created` tells the caller which happened.
func (s *Store) Create(ctx context.Context, n NewTx) (rec *TxRecord, created bool, err error) {
	rec, err = scanTx(s.pool.QueryRow(ctx, `
		INSERT INTO wallet.transactions
			(order_id, order_title, payer_id, payee_id, amount,
			 platform_commission, payment_method, provider_ref)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (order_id, payee_id) DO NOTHING
		RETURNING `+txColumns,
		n.OrderID, n.OrderTitle, n.PayerID, n.PayeeID, n.Amount,
		n.Commission, n.PaymentMethod, n.ProviderRef))

	if errors.Is(err, db.ErrNotFound) {
		// The conflict fired — somebody already opened escrow for this pair.
		rec, err = s.Existing(ctx, n.OrderID, n.PayeeID)
		return rec, false, err
	}
	return rec, err == nil, err
}

// Release pays out one executor. Idempotent: releasing an already-released
// transaction returns it unchanged, so a retried confirm-completion is safe
// (plan §6).
func (s *Store) Release(ctx context.Context, orderID, payeeID string) (*TxRecord, error) {
	rec, err := scanTx(s.pool.QueryRow(ctx, `
		UPDATE wallet.transactions SET status = 'released', released_at = now()
		WHERE order_id = $1 AND payee_id = $2 AND status = 'held'
		RETURNING `+txColumns, orderID, payeeID))
	if errors.Is(err, db.ErrNotFound) {
		return s.Existing(ctx, orderID, payeeID) // already released, or absent
	}
	return rec, err
}

// RefundOrder returns every still-held transaction of an order to the payer —
// what a cancellation after acceptance has to do.
func (s *Store) RefundOrder(ctx context.Context, orderID string) ([]TxRecord, error) {
	rows, err := s.pool.Query(ctx, `
		UPDATE wallet.transactions SET status = 'refunded', refunded_at = now()
		WHERE order_id = $1 AND status = 'held'
		RETURNING `+txColumns, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collect(rows)
}

func (s *Store) ListByPayee(ctx context.Context, payeeID string) ([]TxRecord, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+txColumns+`
		FROM wallet.transactions WHERE payee_id = $1 ORDER BY created_at DESC`, payeeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collect(rows)
}

func (s *Store) ListByOrder(ctx context.Context, orderID string) ([]TxRecord, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+txColumns+`
		FROM wallet.transactions WHERE order_id = $1 ORDER BY created_at`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collect(rows)
}

func (s *Store) ListAll(ctx context.Context) ([]TxRecord, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+txColumns+`
		FROM wallet.transactions ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collect(rows)
}

func collect(rows pgx.Rows) ([]TxRecord, error) {
	out := []TxRecord{}
	for rows.Next() {
		t, err := scanTx(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *t)
	}
	return out, rows.Err()
}

// CommissionPct reads the platform's cut from wallet.settings. Reading it per
// request rather than caching is what makes "we can change the commission live"
// true during Q&A (plan §10) — a back-office table update takes effect on the
// next transaction with no restart.
func (s *Store) CommissionPct(ctx context.Context) (int64, error) {
	var pct int64
	err := s.pool.QueryRow(ctx,
		`SELECT value::bigint FROM wallet.settings WHERE key = 'platform_commission_pct'`).Scan(&pct)
	if errors.Is(err, pgx.ErrNoRows) {
		return 10, nil
	}
	return pct, err
}

// MoneyStats aggregates the dashboard's money row in SQL (plan §11):
// credited to executors, service fees earned, and what is still in escrow.
func (s *Store) MoneyStats(ctx context.Context) (credited, fees, held int64, err error) {
	err = s.pool.QueryRow(ctx, `
		SELECT
			COALESCE(SUM(amount - platform_commission) FILTER (WHERE status = 'released'), 0)::bigint,
			COALESCE(SUM(platform_commission)          FILTER (WHERE status = 'released'), 0)::bigint,
			COALESCE(SUM(amount)                       FILTER (WHERE status = 'held'),     0)::bigint
		FROM wallet.transactions`).Scan(&credited, &fees, &held)
	return credited, fees, held, err
}
