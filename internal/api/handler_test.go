package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"context"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/turgayh/boom/internal/api/model"
	"github.com/turgayh/boom/internal/domain"
	"github.com/turgayh/boom/internal/queue"
	"github.com/turgayh/boom/internal/repository"
)

func setupRouter(h *Handler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/v1/notifications", h.CreateNotification)
	r.POST("/v1/notifications/batch", h.CreateBatchNotification)
	r.GET("/v1/notifications/:id", h.GetNotification)
	r.GET("/v1/notifications/batch/:batchId", h.GetBatchNotifications)
	return r
}

func TestCreateNotification_Success(t *testing.T) {
	mockRepo := &repository.MockNotificationRepository{
		CreateFn: func(ctx context.Context, n *domain.Notification) error {
			return nil
		},
	}
	mockPub := &queue.MockPublisher{
		PublishFn: func(ctx context.Context, n *domain.Notification) error {
			return nil
		},
	}

	h := NewHandler(mockRepo, mockPub)
	router := setupRouter(h)

	body := map[string]string{
		"priority":        "high",
		"recipient":       "+905551234567",
		"channel":         "sms",
		"content":         "Test message",
		"idempotency_key": uuid.New().String(),
	}
	jsonBody, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/v1/notifications", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)

	var resp map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.NotEmpty(t, resp["id"])
}

func TestCreateNotification_DuplicateIdempotencyKey(t *testing.T) {
	mockRepo := &repository.MockNotificationRepository{
		CreateFn: func(ctx context.Context, n *domain.Notification) error {
			return repository.ErrDuplicateIdempotencyKey
		},
	}
	mockPub := &queue.MockPublisher{
		PublishFn: func(ctx context.Context, n *domain.Notification) error {
			return nil
		},
	}

	h := NewHandler(mockRepo, mockPub)
	router := setupRouter(h)

	body := map[string]string{
		"priority":        "normal",
		"recipient":       "user@example.com",
		"channel":         "email",
		"content":         "Duplicate test",
		"idempotency_key": uuid.New().String(),
	}
	jsonBody, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/v1/notifications", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)

	var resp map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "idempotency key already exists", resp["error"])
}

func TestGetNotification_Success(t *testing.T) {
	notificationID := uuid.New()

	mockRepo := &repository.MockNotificationRepository{
		GetByIDFn: func(ctx context.Context, id uuid.UUID) (*domain.Notification, error) {
			return &domain.Notification{
				ID:        notificationID,
				Priority:  domain.PriorityHigh,
				Status:    domain.StatusDelivered,
				Recipient: "+905551234567",
				Channel:   "sms",
				Content:   "Test message",
			}, nil
		},
	}
	mockPub := &queue.MockPublisher{
		PublishFn: func(ctx context.Context, n *domain.Notification) error {
			return nil
		},
	}

	h := NewHandler(mockRepo, mockPub)
	router := setupRouter(h)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/v1/notifications/"+notificationID.String(), nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp domain.Notification
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, notificationID, resp.ID)
	assert.Equal(t, domain.StatusDelivered, resp.Status)
	assert.Equal(t, "sms", resp.Channel)
}

func TestCreateBatchNotification_Success(t *testing.T) {
	mockRepo := &repository.MockNotificationRepository{
		CreateBatchFn: func(ctx context.Context, notifications []*domain.Notification) (*repository.BatchResult, error) {
			return &repository.BatchResult{Succeeded: notifications}, nil
		},
	}
	mockPub := &queue.MockPublisher{
		PublishFn: func(ctx context.Context, n *domain.Notification) error {
			return nil
		},
	}

	h := NewHandler(mockRepo, mockPub)
	router := setupRouter(h)

	body := map[string]any{
		"notifications": []map[string]string{
			{
				"priority":        "high",
				"recipient":       "+905551234567",
				"channel":         "sms",
				"content":         "Batch message 1",
				"idempotency_key": uuid.New().String(),
			},
			{
				"priority":        "normal",
				"recipient":       "user@example.com",
				"channel":         "email",
				"content":         "Batch message 2",
				"idempotency_key": uuid.New().String(),
			},
		},
	}
	jsonBody, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/v1/notifications/batch", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)

	var resp model.BatchCreateResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.NotEmpty(t, resp.BatchID)
	assert.Equal(t, 2, resp.Total)
	assert.Len(t, resp.Notifications, 2)
}

func BenchmarkCreateNotification(b *testing.B) {
	gin.SetMode(gin.TestMode)

	mockRepo := &repository.MockNotificationRepository{
		CreateFn: func(ctx context.Context, n *domain.Notification) error {
			return nil
		},
	}
	mockPub := &queue.MockPublisher{
		PublishFn: func(ctx context.Context, n *domain.Notification) error {
			return nil
		},
	}

	h := NewHandler(mockRepo, mockPub)
	router := setupRouter(h)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		body := map[string]string{
			"priority":        "high",
			"recipient":       "+905551234567",
			"channel":         "sms",
			"content":         "Benchmark message",
			"idempotency_key": uuid.New().String(),
		}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/v1/notifications", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)
	}
}

func BenchmarkCreateBatchNotification(b *testing.B) {
	gin.SetMode(gin.TestMode)

	mockRepo := &repository.MockNotificationRepository{
		CreateBatchFn: func(ctx context.Context, notifications []*domain.Notification) (*repository.BatchResult, error) {
			return &repository.BatchResult{Succeeded: notifications}, nil
		},
	}
	mockPub := &queue.MockPublisher{
		PublishFn: func(ctx context.Context, n *domain.Notification) error {
			return nil
		},
	}

	h := NewHandler(mockRepo, mockPub)
	router := setupRouter(h)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		notifications := make([]map[string]string, 70)
		for j := 0; j < 70; j++ {
			notifications[j] = map[string]string{
				"priority":        "normal",
				"recipient":       "+905551234567",
				"channel":         "sms",
				"content":         "Batch bench message",
				"idempotency_key": uuid.New().String(),
			}
		}
		body := map[string]any{"notifications": notifications}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/v1/notifications/batch", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)
	}
}
