-- orders schema: orders + legs-as-rows (atomic per-leg claims, per-leg status)
CREATE SCHEMA IF NOT EXISTS orders;

CREATE TABLE IF NOT EXISTS orders.orders (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id      UUID NOT NULL,
    client_name    TEXT NOT NULL,
    type           TEXT NOT NULL CHECK (type IN ('transportOnly','transportWithOptions','equipmentOnly','laborOnly')),

    -- cargo (nullable unless transport leg exists)
    cargo_type             TEXT,
    weight_tons            NUMERIC(8,2),
    requires_refrigeration BOOLEAN,
    required_vehicle_type  TEXT,
    special_instructions   TEXT,

    -- equipment request (nullable)
    equipment_type           TEXT,
    equipment_duration_hours INTEGER,
    equipment_notes          TEXT,

    -- labor request (nullable)
    labor_workers_count    INTEGER,
    labor_duration_hours   INTEGER,
    labor_task_description TEXT,

    pickup_address  TEXT NOT NULL,
    pickup_lat      DOUBLE PRECISION NOT NULL,
    pickup_lng      DOUBLE PRECISION NOT NULL,
    dropoff_address TEXT NOT NULL,
    dropoff_lat     DOUBLE PRECISION NOT NULL,
    dropoff_lng     DOUBLE PRECISION NOT NULL,

    scheduled_date TIMESTAMPTZ NOT NULL,
    price_estimate NUMERIC(14,0) NOT NULL, -- UZS, serialized as string
    currency       TEXT NOT NULL DEFAULT 'UZS',

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS orders_client_idx ON orders.orders (client_id, created_at DESC);

CREATE TABLE IF NOT EXISTS orders.order_legs (
    order_id      UUID NOT NULL REFERENCES orders.orders(id) ON DELETE CASCADE,
    leg           TEXT NOT NULL CHECK (leg IN ('transport','equipment','labor')),
    status        TEXT NOT NULL DEFAULT 'published' CHECK (status IN
        ('draft','published','matched','accepted','inProgress','loadingInProgress',
         'inTransit','delivered','completed','cancelled','disputed')),
    executor_id   UUID,
    executor_name TEXT,
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (order_id, leg)
);
-- Atomic claim (LEG_ALREADY_TAKEN protection):
--   UPDATE orders.order_legs SET executor_id=$1, executor_name=$2, status='accepted'
--   WHERE order_id=$3 AND leg=$4 AND executor_id IS NULL AND status='published'
--   RETURNING order_id;  -- zero rows ⇒ 409 LEG_ALREADY_TAKEN / ORDER_NOT_PUBLISHED

CREATE INDEX IF NOT EXISTS order_legs_open_idx
    ON orders.order_legs (leg, status) WHERE executor_id IS NULL;
