package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/turgayh/boom/internal/domain"
)

type CreateNotificationRequest struct {
	Priority       domain.Priority `json:"priority" validate:"required,oneof=high normal low"`
	Recipient      string          `json:"recipient"`
	Channel        string          `json:"channel" validate:"required,oneof=email sms push"`
	Content        string          `json:"content"`
	IdempotencyKey string          `json:"idempotency_key" validate:"required,uuid"`
}

type CreateNotificationResponse struct {
	ID string `json:"id"`
}

func (r *CreateNotificationRequest) ToNotification() *domain.Notification {
	return &domain.Notification{
		ID:             uuid.New(),
		Priority:       r.Priority,
		Recipient:      r.Recipient,
		Channel:        r.Channel,
		Content:        r.Content,
		IdempotencyKey: r.IdempotencyKey,
		Status:         domain.StatusPending,
		Attempts:       0,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
}

func (r *CreateNotificationResponse) FromNotification(n *domain.Notification) *CreateNotificationResponse {
	return &CreateNotificationResponse{
		ID: n.ID.String(),
	}
}

type BatchCreateRequest struct {
	Notifications []CreateNotificationRequest `json:"notifications" validate:"required,min=1,max=1000"`
}

type BatchCreateResponse struct {
	BatchID       string   `json:"batch_id"`
	Notifications []string `json:"notification_ids"`
	Total         int      `json:"total"`
}

func (r *BatchCreateRequest) ToNotifications(batchID uuid.UUID) []*domain.Notification {
	notifications := make([]*domain.Notification, len(r.Notifications))
	for i, n := range r.Notifications {
		notif := n.ToNotification()
		notif.BatchID = &batchID
		notifications[i] = notif
	}
	return notifications
}
