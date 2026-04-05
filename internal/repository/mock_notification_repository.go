package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/turgayh/boom/internal/domain"
)

type MockNotificationRepository struct {
	CreateFn       func(ctx context.Context, n *domain.Notification) error
	GetByIDFn      func(ctx context.Context, id uuid.UUID) (*domain.Notification, error)
	UpdateStatusFn func(ctx context.Context, id uuid.UUID, status domain.Status, providerMsgID *string) error
}

func (m *MockNotificationRepository) Create(ctx context.Context, n *domain.Notification) error {
	return m.CreateFn(ctx, n)
}

func (m *MockNotificationRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Notification, error) {
	return m.GetByIDFn(ctx, id)
}

func (m *MockNotificationRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.Status, providerMsgID *string) error {
	return m.UpdateStatusFn(ctx, id, status, providerMsgID)
}
