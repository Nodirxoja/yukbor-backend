-- notifications schema
CREATE SCHEMA IF NOT EXISTS notifications;

CREATE TABLE IF NOT EXISTS notifications.notifications (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id          UUID NOT NULL,
    type             TEXT NOT NULL CHECK (type IN
        ('newOrderMatch','orderStatusChanged','paymentReleased','backhaulSuggestion','reviewReceived')),
    title            TEXT NOT NULL,
    body             TEXT NOT NULL,
    related_order_id UUID,
    is_read          BOOLEAN NOT NULL DEFAULT FALSE,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS notifications_user_idx
    ON notifications.notifications (user_id, created_at DESC);
