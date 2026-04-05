package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/turgayh/boom/internal/domain"
)

type MockNotificationRepository struct {
	CreateFn       func(ctx context.Context, n *domain.Notification) error
	CreateBatchFn  func(ctx context.Context, notifications []*domain.Notification) (*BatchResult, error)
	GetByIDFn      func(ctx context.Context, id uuid.UUID) (*domain.Notification, error)
	GetByBatchIDFn func(ctx context.Context, batchID uuid.UUID) ([]*domain.Notification, error)
	UpdateStatusFn func(ctx context.Context, id uuid.UUID, status domain.Status, providerMsgID *string) error
	CancelFn       func(ctx context.Context, id uuid.UUID) error
}

func (m *MockNotificationRepository) Create(ctx context.Context, n *domain.Notification) error {
	return m.CreateFn(ctx, n)
}

func (m *MockNotificationRepository) CreateBatch(ctx context.Context, notifications []*domain.Notification) (*BatchResult, error) {
	return m.CreateBatchFn(ctx, notifications)
}

func (m *MockNotificationRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Notification, error) {
	return m.GetByIDFn(ctx, id)
}

func (m *MockNotificationRepository) GetByBatchID(ctx context.Context, batchID uuid.UUID) ([]*domain.Notification, error) {
	return m.GetByBatchIDFn(ctx, batchID)
}

func (m *MockNotificationRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.Status, providerMsgID *string) error {
	return m.UpdateStatusFn(ctx, id, status, providerMsgID)
}

func (m *MockNotificationRepository) Cancel(ctx context.Context, id uuid.UUID) error {
	return m.CancelFn(ctx, id)
}
