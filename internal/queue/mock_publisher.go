package queue

import (
	"context"

	"github.com/turgayh/boom/internal/domain"
)

type MockPublisher struct {
	PublishFn func(ctx context.Context, notification *domain.Notification) error
}

func (m *MockPublisher) Publish(ctx context.Context, notification *domain.Notification) error {
	return m.PublishFn(ctx, notification)
}
