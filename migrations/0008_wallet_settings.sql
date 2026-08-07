-- The platform commission belongs to the wallet, not to orders.
--
-- 0007 seeded platform_commission_pct into orders.tariffs, which would have
-- made two services able to disagree about the same number. Commission is
-- computed server-side by exactly one owner (plan §12: money drift), so it
-- moves into the wallet's own schema — still a table, so it stays adjustable
-- live during Q&A like the tariffs.
CREATE TABLE IF NOT EXISTS wallet.settings (
    key         TEXT PRIMARY KEY,
    value       NUMERIC(14,2) NOT NULL,
    description TEXT NOT NULL
);

INSERT INTO wallet.settings (key, value, description) VALUES
    ('platform_commission_pct', 10, 'Комиссия платформы, % от суммы заказа')
ON CONFLICT (key) DO NOTHING;

DELETE FROM orders.tariffs WHERE key = 'platform_commission_pct';
