-- wallet schema: escrow transactions, one per (order, payee)
CREATE SCHEMA IF NOT EXISTS wallet;

CREATE TABLE IF NOT EXISTS wallet.transactions (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id            UUID NOT NULL,
    order_title         TEXT NOT NULL,
    payer_id            UUID NOT NULL,
    payee_id            UUID NOT NULL,
    amount              NUMERIC(14,0) NOT NULL CHECK (amount >= 0),              -- UZS
    platform_commission NUMERIC(14,0) NOT NULL CHECK (platform_commission >= 0), -- UZS
    payment_method      TEXT NOT NULL CHECK (payment_method IN ('payme','click','uzcard')),
    status              TEXT NOT NULL DEFAULT 'held' CHECK (status IN ('held','released','refunded')),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    released_at         TIMESTAMPTZ,
    UNIQUE (order_id, payee_id) -- combo orders: several rows per order, one per executor
);
CREATE INDEX IF NOT EXISTS transactions_payee_idx ON wallet.transactions (payee_id, created_at DESC);
