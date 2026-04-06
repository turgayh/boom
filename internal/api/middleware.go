package api

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const CorrelationIDHeader = "X-Correlation-ID"

func CorrelationIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		correlationID := c.GetHeader(CorrelationIDHeader)
		if correlationID == "" {
			correlationID = uuid.New().String()
		}

		c.Set("correlation_id", correlationID)
		c.Header(CorrelationIDHeader, correlationID)

		slog.Info("request started",
			"correlation_id", correlationID,
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
		)

		c.Next()

		slog.Info("request completed",
			"correlation_id", correlationID,
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
		)
	}
}
