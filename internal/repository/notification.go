package repository

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/turgayh/boom/internal/domain"
)

type BatchResult struct {
	Succeeded []*domain.Notification
	Failed    []BatchError
}

type BatchError struct {
	Index int    `json:"index"`
	ID    string `json:"id"`
	Error string `json:"error"`
}

type NotificationRepository interface {
	Create(ctx context.Context, n *domain.Notification) error
	CreateBatch(ctx context.Context, notifications []*domain.Notification) (*BatchResult, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Notification, error)
	GetByBatchID(ctx context.Context, batchID uuid.UUID) ([]*domain.Notification, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status domain.Status, providerMsgID *string) error
}

type postgresNotificationRepository struct {
	db  *pgxpool.Pool
	log *slog.Logger
}

func NewNotificationRepository(db *pgxpool.Pool, log *slog.Logger) NotificationRepository {
	return &postgresNotificationRepository{db: db, log: log}
}

var ErrDuplicateIdempotencyKey = fmt.Errorf("duplicate idempotency key")

func (r *postgresNotificationRepository) Create(ctx context.Context, n *domain.Notification) error {
	result, err := r.db.Exec(ctx, `
        INSERT INTO notifications
            (id, batch_id, priority, status, idempotency_key, recipient, channel, content, attempts)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
        ON CONFLICT (idempotency_key) DO NOTHING`,
		n.ID, n.BatchID, n.Priority, n.Status,
		n.IdempotencyKey, n.Recipient, n.Channel, n.Content, n.Attempts,
	)
	if err != nil {
		r.log.Error("failed to create notification", "error", err)
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrDuplicateIdempotencyKey
	}
	return nil
}

func (r *postgresNotificationRepository) CreateBatch(ctx context.Context, notifications []*domain.Notification) (*BatchResult, error) {
	result := &BatchResult{}

	for i, n := range notifications {
		_, err := r.db.Exec(ctx, `
            INSERT INTO notifications
                (id, batch_id, priority, status, idempotency_key, recipient, channel, content, attempts)
            VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
            ON CONFLICT (idempotency_key) DO NOTHING`,
			n.ID, n.BatchID, n.Priority, n.Status,
			n.IdempotencyKey, n.Recipient, n.Channel, n.Content, n.Attempts,
		)
		if err != nil {
			r.log.Error("failed to insert notification in batch", "error", err, "id", n.ID, "index", i)
			result.Failed = append(result.Failed, BatchError{
				Index: i,
				ID:    n.ID.String(),
				Error: err.Error(),
			})
			continue
		}
		result.Succeeded = append(result.Succeeded, n)
	}

	return result, nil
}

func (r *postgresNotificationRepository) GetByBatchID(ctx context.Context, batchID uuid.UUID) ([]*domain.Notification, error) {
	rows, err := r.db.Query(ctx, `
        SELECT id, batch_id, priority, status, idempotency_key,
               recipient, channel, content, attempts, created_at, updated_at
        FROM notifications WHERE batch_id = $1
        ORDER BY created_at`, batchID)
	if err != nil {
		r.log.Error("failed to get notifications by batch id", "error", err)
		return nil, err
	}
	defer rows.Close()

	var notifications []*domain.Notification
	for rows.Next() {
		var n domain.Notification
		if err := rows.Scan(&n.ID, &n.BatchID, &n.Priority, &n.Status,
			&n.IdempotencyKey, &n.Recipient, &n.Channel, &n.Content, &n.Attempts,
			&n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, err
		}
		notifications = append(notifications, &n)
	}
	return notifications, nil
}

func (r *postgresNotificationRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Notification, error) {
	var n domain.Notification
	err := r.db.QueryRow(ctx, `
        SELECT id, batch_id, priority, status, idempotency_key,
               recipient, channel, content, attempts, created_at, updated_at
        FROM notifications WHERE id = $1`, id).
		Scan(&n.ID, &n.BatchID, &n.Priority, &n.Status,
			&n.IdempotencyKey, &n.Recipient, &n.Channel, &n.Content, &n.Attempts,
			&n.CreatedAt, &n.UpdatedAt)
	if err != nil {
		r.log.Error("failed to get notification by id", "error", err)
		return nil, err
	}
	return &n, nil
}

func (r *postgresNotificationRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.Status, providerMsgID *string) error {
	_, err := r.db.Exec(ctx, `
        UPDATE notifications
        SET status = $2, provider_msg_id = $3, attempts = attempts + 1, updated_at = NOW()
        WHERE id = $1`,
		id, status, providerMsgID,
	)
	if err != nil {
		r.log.Error("failed to update notification status", "error", err)
		return err
	}
	return nil
}
