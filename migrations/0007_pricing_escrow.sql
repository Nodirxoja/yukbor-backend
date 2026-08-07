-- Per-leg pricing, server-side tariffs, and payment plumbing.
--
-- A combo order carries ONE priceEstimate but pays 2-3 executors separately,
-- so each leg needs its own amount: the escrow row for (orderId, payeeId) is
-- created from orders.order_legs.price, never from the order total.
ALTER TABLE orders.order_legs ADD COLUMN IF NOT EXISTS price NUMERIC(14,0) NOT NULL DEFAULT 0;

-- The contract's Order has no paymentMethod, but wallet needs one when orders
-- opens escrow on accept. Accepted as an optional field on POST /orders and
-- defaulted here, so iOS sends nothing new.
ALTER TABLE orders.orders ADD COLUMN IF NOT EXISTS payment_method TEXT NOT NULL DEFAULT 'payme'
    CHECK (payment_method IN ('payme','click','uzcard'));

-- Cancellation reason, so a cancelled order can explain itself in the dashboard.
ALTER TABLE orders.orders ADD COLUMN IF NOT EXISTS cancelled_at TIMESTAMPTZ;

-- Tariffs live in a table rather than in code so they can be "updated live"
-- during Q&A (plan §10) without a rebuild. Values are UZS.
CREATE TABLE IF NOT EXISTS orders.tariffs (
    key         TEXT PRIMARY KEY,
    value       NUMERIC(14,2) NOT NULL,
    description TEXT NOT NULL
);

INSERT INTO orders.tariffs (key, value, description) VALUES
    ('transport_per_ton',      80000,  'Перевозка: ставка за тонну груза'),
    ('equipment_hour_crane',   450000, 'Спецтехника: кран, за час'),
    ('equipment_hour_excavator', 400000, 'Спецтехника: экскаватор, за час'),
    ('equipment_hour_loader',  250000, 'Спецтехника: погрузчик, за час'),
    ('equipment_hour_forklift', 200000, 'Спецтехника: вилочный погрузчик, за час'),
    ('labor_hour_per_worker',  45000,  'Рабочая сила: за час за одного рабочего'),
    ('minimum_order',          100000, 'Минимальная стоимость заказа'),
    ('platform_commission_pct', 10,    'Комиссия платформы, %')
ON CONFLICT (key) DO NOTHING;

-- Provider reference from the simulated PSP charge (payme_txn_...), so a
-- transaction can be traced the way a real one would be.
ALTER TABLE wallet.transactions ADD COLUMN IF NOT EXISTS provider_ref TEXT;
ALTER TABLE wallet.transactions ADD COLUMN IF NOT EXISTS refunded_at  TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS transactions_order_idx ON wallet.transactions (order_id);
