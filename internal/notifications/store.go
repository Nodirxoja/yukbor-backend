package notifications

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/aventiseld/yukbor-backend/pkg/db"
	"github.com/aventiseld/yukbor-backend/pkg/models"
)

// Store is the notifications schema: the durable feed behind the realtime
// channel. A user who was offline when an event fired still sees it here,
// which is what makes the WS hub safe to keep in memory.
type Store struct{ pool *pgxpool.Pool }

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

const notificationColumns = `id, user_id, type, title, body, related_order_id, is_read, created_at`

func scanNotification(row pgx.Row) (*models.AppNotification, error) {
	var n models.AppNotification
	err := row.Scan(&n.ID, &n.UserID, &n.Type, &n.Title, &n.Body,
		&n.RelatedOrderID, &n.IsRead, &n.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, db.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &n, nil
}

func (s *Store) Create(ctx context.Context, userID string, spec models.NotifySpec) (*models.AppNotification, error) {
	return scanNotification(s.pool.QueryRow(ctx, `
		INSERT INTO notifications.notifications (user_id, type, title, body, related_order_id)
		VALUES ($1,$2,$3,$4,$5)
		RETURNING `+notificationColumns,
		userID, spec.Type, spec.Title, spec.Body, spec.RelatedOrderID))
}

func (s *Store) ListByUser(ctx context.Context, userID string) ([]models.AppNotification, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+notificationColumns+`
		FROM notifications.notifications
		WHERE user_id = $1 ORDER BY created_at DESC LIMIT 200`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []models.AppNotification{}
	for rows.Next() {
		n, err := scanNotification(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *n)
	}
	return out, rows.Err()
}

// MarkRead flips a notification, scoped to its owner so one user cannot mark
// another's feed.
func (s *Store) MarkRead(ctx context.Context, id, userID string) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE notifications.notifications SET is_read = TRUE
		WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return db.ErrNotFound
	}
	return nil
}
