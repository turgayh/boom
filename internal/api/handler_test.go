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
	"github.com/turgayh/boom/internal/domain"
	"github.com/turgayh/boom/internal/queue"
	"github.com/turgayh/boom/internal/repository"
)

func setupRouter(h *Handler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/v1/notifications", h.CreateNotification)
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
		"priority":       "high",
		"recipient":      "+905551234567",
		"channel":        "sms",
		"content":        "Test message",
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
		"priority":       "normal",
		"recipient":      "user@example.com",
		"channel":        "email",
		"content":        "Duplicate test",
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
