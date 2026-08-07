-- reviews schema
CREATE SCHEMA IF NOT EXISTS reviews;

CREATE TABLE IF NOT EXISTS reviews.reviews (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id    UUID NOT NULL,
    reviewer_id UUID NOT NULL,
    reviewee_id UUID NOT NULL,
    rating      SMALLINT NOT NULL CHECK (rating BETWEEN 1 AND 5),
    comment     TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (order_id, reviewer_id, reviewee_id)
);
CREATE INDEX IF NOT EXISTS reviews_reviewee_idx ON reviews.reviews (reviewee_id);

-- Aggregate for GET /reviews/rating:
--   SELECT COALESCE(ROUND(AVG(rating)::numeric, 2), 0) AS rating, COUNT(*) AS count
--   FROM reviews.reviews WHERE reviewee_id = $1;
