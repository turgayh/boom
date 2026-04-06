// internal/api/handler.go
package api

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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

func (h *Handler) GetNotification(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid notification id"})
		return
	}

	n, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "notification not found"})
		return
	}

	c.JSON(http.StatusOK, n)
}

func (h *Handler) CreateBatchNotification(c *gin.Context) {
	var req model.BatchCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if len(req.Notifications) == 0 || len(req.Notifications) > 1000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "batch size must be between 1 and 1000"})
		return
	}

	batchID := uuid.New()
	notifications := req.ToNotifications(batchID)

	result, err := h.repo.CreateBatch(c.Request.Context(), notifications)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ids := make([]string, 0, len(result.Succeeded))
	for _, n := range result.Succeeded {
		if err := h.publisher.Publish(c.Request.Context(), n); err != nil {
			result.Failed = append(result.Failed, repository.BatchError{
				Index: -1,
				ID:    n.ID.String(),
				Error: "queue error",
			})
			continue
		}
		ids = append(ids, n.ID.String())
	}

	var failed []model.BatchFailedItem
	for _, f := range result.Failed {
		failed = append(failed, model.BatchFailedItem{
			Index: f.Index,
			ID:    f.ID,
			Error: f.Error,
		})
	}

	c.JSON(http.StatusAccepted, model.BatchCreateResponse{
		BatchID:       batchID.String(),
		Notifications: ids,
		Total:         len(ids),
		Failed:        failed,
	})
}

func (h *Handler) GetBatchNotifications(c *gin.Context) {
	batchID, err := uuid.Parse(c.Param("batchId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid batch id"})
		return
	}

	notifications, err := h.repo.GetByBatchID(c.Request.Context(), batchID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"batch_id":      batchID.String(),
		"notifications": notifications,
		"total":         len(notifications),
	})
}

func (h *Handler) ListNotifications(c *gin.Context) {
	page := 1
	pageSize := 20
	if v := c.Query("page"); v != "" {
		fmt.Sscanf(v, "%d", &page)
	}
	if v := c.Query("page_size"); v != "" {
		fmt.Sscanf(v, "%d", &pageSize)
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	filter := repository.ListFilter{
		Status:   c.Query("status"),
		Channel:  c.Query("channel"),
		DateFrom: c.Query("date_from"),
		DateTo:   c.Query("date_to"),
		Page:     page,
		PageSize: pageSize,
	}

	notifications, total, err := h.repo.List(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"notifications": notifications,
		"total":         total,
		"page":          page,
		"page_size":     pageSize,
	})
}

func (h *Handler) CancelNotification(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid notification id"})
		return
	}

	if err := h.repo.Cancel(c.Request.Context(), id); err != nil {
		if errors.Is(err, repository.ErrNotCancellable) {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "cancelled"})
}

func (h *Handler) Metrics(c *gin.Context) {
	metrics, err := h.repo.Metrics(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	depths, err := h.publisher.QueueDepths()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"notifications": metrics,
		"queues":        depths,
	})
}

func (h *Handler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
