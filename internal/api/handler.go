// internal/api/handler.go
package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/turgayh/boom/internal/api/model"
	"github.com/turgayh/boom/internal/queue"
	"github.com/turgayh/boom/internal/repository"
)

type Handler struct {
	repo      repository.NotificationRepository
	publisher queue.Publisher
}

func NewHandler(repo repository.NotificationRepository, publisher queue.Publisher) *Handler {
	return &Handler{repo: repo, publisher: publisher}
}

func (h *Handler) CreateNotification(c *gin.Context) {
	var req model.CreateNotificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	n := req.ToNotification()
	if err := h.repo.Create(c.Request.Context(), n); err != nil {
		if errors.Is(err, repository.ErrDuplicateIdempotencyKey) {
			c.JSON(http.StatusConflict, gin.H{"error": "idempotency key already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := h.publisher.Publish(c.Request.Context(), n); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "queue error"})
		return
	}

	c.JSON(http.StatusAccepted, model.CreateNotificationResponse{ID: n.ID.String()})
}

func (h *Handler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
