package reviews

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/aventiseld/yukbor-backend/pkg/db"
	"github.com/aventiseld/yukbor-backend/pkg/models"
)

// Store is the reviews schema plus the rating aggregate that feeds
// User.rating / User.ratingsCount (contract §6).
type Store struct{ pool *pgxpool.Pool }

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// ErrDuplicate is the UNIQUE(order_id, reviewer_id, reviewee_id) violation:
// one review per person per counterparty per order.
var ErrDuplicate = errors.New("review already exists")

const reviewColumns = `id, order_id, reviewer_id, reviewee_id, rating, comment, created_at`

func scanReview(row pgx.Row) (*models.Review, error) {
	var rv models.Review
	err := row.Scan(&rv.ID, &rv.OrderID, &rv.ReviewerID, &rv.RevieweeID,
		&rv.Rating, &rv.Comment, &rv.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, db.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &rv, nil
}

func (s *Store) Create(ctx context.Context, rv models.Review) (*models.Review, error) {
	out, err := scanReview(s.pool.QueryRow(ctx, `
		INSERT INTO reviews.reviews (order_id, reviewer_id, reviewee_id, rating, comment)
		VALUES ($1,$2,$3,$4,$5)
		RETURNING `+reviewColumns,
		rv.OrderID, rv.ReviewerID, rv.RevieweeID, rv.Rating, rv.Comment))
	if err != nil && strings.Contains(err.Error(), "duplicate key") {
		return nil, ErrDuplicate
	}
	return out, err
}

// Aggregate is the value pushed into auth.users after every new review.
func (s *Store) Aggregate(ctx context.Context, revieweeID string) (models.RatingUpdate, error) {
	var agg models.RatingUpdate
	err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(ROUND(AVG(rating)::numeric, 2), 0)::float8, count(*)
		FROM reviews.reviews WHERE reviewee_id = $1`, revieweeID).Scan(&agg.Rating, &agg.Count)
	return agg, err
}

func (s *Store) ListByReviewee(ctx context.Context, revieweeID string) ([]models.Review, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+reviewColumns+`
		FROM reviews.reviews WHERE reviewee_id = $1 ORDER BY created_at DESC`, revieweeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []models.Review{}
	for rows.Next() {
		rv, err := scanReview(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *rv)
	}
	return out, rows.Err()
}
