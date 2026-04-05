package repository

import (
	"context"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/turgayh/boom/internal/domain"
)

type NotificationRepository struct {
	db  *pgxpool.Pool
	log *slog.Logger
}

type INotificationRepository interface {
	Create(ctx context.Context, n *domain.Notification) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Notification, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status domain.Status, providerMsgID *string) error
}

func NewNotificationRepository(db *pgxpool.Pool, log *slog.Logger) INotificationRepository {
	return &NotificationRepository{db: db, log: log}
}

func (r *NotificationRepository) Create(ctx context.Context, n *domain.Notification) error {
	_, err := r.db.Exec(ctx, `
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
	return nil
}

func (r *NotificationRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Notification, error) {
	var n domain.Notification
	err := r.db.QueryRow(ctx, `
        SELECT id, batch_id, priority, status, idempotency_key,
               recipient, channel, content, attempts
        FROM notifications WHERE id = $1`, id).
		Scan(&n.ID, &n.BatchID, &n.Priority, &n.Status,
			&n.IdempotencyKey, &n.Recipient, &n.Channel, &n.Content, &n.Attempts)
	if err != nil {
		r.log.Error("failed to get notification by id", "error", err)
		return nil, err
	}
	return &n, nil
}

func (r *NotificationRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.Status, providerMsgID *string) error {
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
