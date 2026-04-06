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
	Cancel(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, filter ListFilter) ([]*domain.Notification, int, error)
	Metrics(ctx context.Context) (*NotificationMetrics, error)
}

type NotificationMetrics struct {
	Total     int            `json:"total"`
	ByStatus  map[string]int `json:"by_status"`
	ByChannel map[string]int `json:"by_channel"`
}

type ListFilter struct {
	Status   string
	Channel  string
	DateFrom string
	DateTo   string
	Page     int
	PageSize int
}

type postgresNotificationRepository struct {
	db  *pgxpool.Pool
	log *slog.Logger
}

func NewNotificationRepository(db *pgxpool.Pool, log *slog.Logger) NotificationRepository {
	return &postgresNotificationRepository{db: db, log: log}
}

var (
	ErrDuplicateIdempotencyKey = fmt.Errorf("duplicate idempotency key")
	ErrNotCancellable          = fmt.Errorf("only pending notifications can be cancelled")
)

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

func (r *postgresNotificationRepository) Cancel(ctx context.Context, id uuid.UUID) error {
	result, err := r.db.Exec(ctx, `
        UPDATE notifications
        SET status = $2, updated_at = NOW()
        WHERE id = $1 AND status = 'pending'`,
		id, domain.StatusCancelled,
	)
	if err != nil {
		r.log.Error("failed to cancel notification", "error", err)
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotCancellable
	}
	return nil
}

func (r *postgresNotificationRepository) List(ctx context.Context, filter ListFilter) ([]*domain.Notification, int, error) {
	where := "WHERE 1=1"
	args := []any{}
	argIdx := 1

	if filter.Status != "" {
		where += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, filter.Status)
		argIdx++
	}
	if filter.Channel != "" {
		where += fmt.Sprintf(" AND channel = $%d", argIdx)
		args = append(args, filter.Channel)
		argIdx++
	}
	if filter.DateFrom != "" {
		where += fmt.Sprintf(" AND created_at >= $%d", argIdx)
		args = append(args, filter.DateFrom)
		argIdx++
	}
	if filter.DateTo != "" {
		where += fmt.Sprintf(" AND created_at <= $%d", argIdx)
		args = append(args, filter.DateTo)
		argIdx++
	}

	var total int
	countQuery := "SELECT COUNT(*) FROM notifications " + where
	if err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	offset := (filter.Page - 1) * filter.PageSize
	query := fmt.Sprintf(`
		SELECT id, batch_id, priority, status, idempotency_key,
		       recipient, channel, content, attempts, created_at, updated_at
		FROM notifications %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d`, where, argIdx, argIdx+1)
	args = append(args, filter.PageSize, offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var notifications []*domain.Notification
	for rows.Next() {
		var n domain.Notification
		if err := rows.Scan(&n.ID, &n.BatchID, &n.Priority, &n.Status,
			&n.IdempotencyKey, &n.Recipient, &n.Channel, &n.Content, &n.Attempts,
			&n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, 0, err
		}
		notifications = append(notifications, &n)
	}
	return notifications, total, nil
}

func (r *postgresNotificationRepository) Metrics(ctx context.Context) (*NotificationMetrics, error) {
	metrics := &NotificationMetrics{
		ByStatus:  make(map[string]int),
		ByChannel: make(map[string]int),
	}

	// total
	if err := r.db.QueryRow(ctx, "SELECT COUNT(*) FROM notifications").Scan(&metrics.Total); err != nil {
		return nil, err
	}

	// by status
	rows, err := r.db.Query(ctx, "SELECT status, COUNT(*) FROM notifications GROUP BY status")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		metrics.ByStatus[status] = count
	}

	// by channel
	rows2, err := r.db.Query(ctx, "SELECT channel, COUNT(*) FROM notifications GROUP BY channel")
	if err != nil {
		return nil, err
	}
	defer rows2.Close()
	for rows2.Next() {
		var channel string
		var count int
		if err := rows2.Scan(&channel, &count); err != nil {
			return nil, err
		}
		metrics.ByChannel[channel] = count
	}

	return metrics, nil
}
