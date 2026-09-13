package middleware

import (
	"github.com/ITK13201/CalendarRabbit/backend/internal/logging"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RequestIDHeader はリクエスト/レスポンスで用いる requestId ヘッダ名。
const RequestIDHeader = "X-Request-Id"

// RequestID は requestId を採番（既存ヘッダがあれば引き継ぐ）し、
// gin.Context と request の context 双方に格納、レスポンスヘッダにも付与する。
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader(RequestIDHeader)
		if requestID == "" {
			requestID = uuid.NewString()
		}

		ctx := logging.WithRequestID(c.Request.Context(), requestID)
		c.Request = c.Request.WithContext(ctx)
		c.Set(logging.RequestIDField, requestID)
		c.Writer.Header().Set(RequestIDHeader, requestID)

		c.Next()
	}
}
