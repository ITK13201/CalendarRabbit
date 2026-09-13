package middleware

import (
	"log/slog"
	"time"

	"github.com/ITK13201/CalendarRabbit/backend/internal/logging"
	"github.com/gin-gonic/gin"
)

// Logger は slog(JSON) でアクセスログを出力する。requestId を含める。
// RequestID ミドルウェアより後に登録すること。
func Logger(logger *slog.Logger) gin.HandlerFunc {
	if logger == nil {
		logger = logging.Default()
	}
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		ctx := c.Request.Context()

		attrs := []slog.Attr{
			slog.String("method", c.Request.Method),
			slog.String("path", path),
			slog.String("query", query),
			slog.Int("status", c.Writer.Status()),
			slog.Int("bytes", c.Writer.Size()),
			slog.Duration("latency", latency),
			slog.String("clientIp", c.ClientIP()),
		}
		if len(c.Errors) > 0 {
			attrs = append(attrs, slog.String("errors", c.Errors.String()))
		}

		level := slog.LevelInfo
		if c.Writer.Status() >= 500 {
			level = slog.LevelError
		} else if c.Writer.Status() >= 400 {
			level = slog.LevelWarn
		}

		logging.LogContext(ctx, logger, level, "access", attrs...)
	}
}
