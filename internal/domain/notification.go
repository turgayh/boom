package domain

import (
	"time"

	"github.com/google/uuid"
)

type Priority string

const (
	PriorityHigh   Priority = "high"
	PriorityNormal Priority = "normal"
	PriorityLow    Priority = "low"
)

type Status string

const (
	StatusPending    Status = "pending"
	StatusProcessing Status = "processing"
	StatusDelivered  Status = "delivered"
	StatusFailed     Status = "failed"
	StatusCancelled  Status = "cancelled"
)

type Notification struct {
	ID             uuid.UUID  `json:"id" db:"id"`
	BatchID        *uuid.UUID `json:"batch_id,omitempty" db:"batch_id"`
	Priority       Priority   `json:"priority" db:"priority"`
	Status         Status     `json:"status" db:"status"`
	IdempotencyKey string     `json:"idempotency_key" db:"idempotency_key"`
	Recipient      string     `json:"recipient" db:"recipient"`
	Channel        string     `json:"channel" db:"channel"`
	Content        string     `json:"content" db:"content"`
	Attempts       int        `json:"attempts" db:"attempts"`
	CreatedAt      time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at" db:"updated_at"`
}

type NotificationBatch struct {
	NotificationID uuid.UUID `json:"notification_id" db:"notification_id"`
	Priority       Priority  `json:"priority" db:"priority"`
}
