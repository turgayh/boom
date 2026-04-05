package repository_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/turgayh/boom/internal/domain"
	"github.com/turgayh/boom/internal/repository"
)

func TestNotificationRepository_Create(t *testing.T) {
	var captured *domain.Notification

	repo := &repository.MockNotificationRepository{
		CreateFn: func(ctx context.Context, n *domain.Notification) error {
			captured = n
			return nil
		},
	}

	id := uuid.MustParse("00000000-0000-0000-0000-000000000000")
	notification := &domain.Notification{
		ID:             id,
		BatchID:        nil,
		Priority:       domain.PriorityNormal,
		Status:         domain.StatusPending,
		IdempotencyKey: fmt.Sprintf("%s-%s", t.Name(), uuid.NewString()),
		Recipient:      "test@example.com",
		Channel:        "email",
		Content:        "test",
		Attempts:       0,
	}

	err := repo.Create(context.Background(), notification)
	require.NoError(t, err)

	assert.Equal(t, id, captured.ID)
	assert.Nil(t, captured.BatchID)
	assert.Equal(t, domain.PriorityNormal, captured.Priority)
	assert.Equal(t, domain.StatusPending, captured.Status)
	assert.Equal(t, "test@example.com", captured.Recipient)
	assert.Equal(t, "email", captured.Channel)
	assert.Equal(t, "test", captured.Content)
	assert.Equal(t, 0, captured.Attempts)
}

func TestNotificationRepository_GetByID(t *testing.T) {
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	expected := &domain.Notification{
		ID:             id,
		Priority:       domain.PriorityHigh,
		Status:         domain.StatusDelivered,
		IdempotencyKey: "test-key",
		Recipient:      "user@example.com",
		Channel:        "sms",
		Content:        "hello",
		Attempts:       1,
	}

	repo := &repository.MockNotificationRepository{
		GetByIDFn: func(ctx context.Context, reqID uuid.UUID) (*domain.Notification, error) {
			assert.Equal(t, id, reqID)
			return expected, nil
		},
	}

	result, err := repo.GetByID(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, expected, result)
}

func TestNotificationRepository_UpdateStatus(t *testing.T) {
	id := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	msgID := "provider-123"

	var calledWith struct {
		id            uuid.UUID
		status        domain.Status
		providerMsgID *string
	}

	repo := &repository.MockNotificationRepository{
		UpdateStatusFn: func(ctx context.Context, reqID uuid.UUID, status domain.Status, providerMsgID *string) error {
			calledWith.id = reqID
			calledWith.status = status
			calledWith.providerMsgID = providerMsgID
			return nil
		},
	}

	err := repo.UpdateStatus(context.Background(), id, domain.StatusDelivered, &msgID)
	require.NoError(t, err)

	assert.Equal(t, id, calledWith.id)
	assert.Equal(t, domain.StatusDelivered, calledWith.status)
	assert.Equal(t, &msgID, calledWith.providerMsgID)
}
