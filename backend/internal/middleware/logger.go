package middleware

import (
	"bytes"
	"io"
	"log/slog"
	"time"

	"github.com/ITK13201/CalendarRabbit/backend/internal/logging"
	"github.com/gin-gonic/gin"
)

// logBodyMaxBytes は HTTP リクエスト/レスポンスボディをログへ捕捉する上限バイト数
// （design D5・MoneyRabbit 準拠）。超過分は捨て、末尾に truncatedSuffix を付す。
const logBodyMaxBytes = 4 * 1024

// truncatedSuffix はボディ丸め時に付与する接尾辞。
const truncatedSuffix = "...(truncated)"

// bodyWriter はレスポンスボディを logBodyMaxBytes まで捕捉する gin.ResponseWriter ラッパ。
type bodyWriter struct {
	gin.ResponseWriter
	buf       bytes.Buffer
	truncated bool
}

func (w *bodyWriter) capture(b []byte) {
	if w.buf.Len() >= logBodyMaxBytes {
		if len(b) > 0 {
			w.truncated = true
		}
		return
	}
	remain := logBodyMaxBytes - w.buf.Len()
	if len(b) > remain {
		w.buf.Write(b[:remain])
		w.truncated = true
		return
	}
	w.buf.Write(b)
}

func (w *bodyWriter) Write(b []byte) (int, error) {
	w.capture(b)
	return w.ResponseWriter.Write(b)
}

func (w *bodyWriter) WriteString(s string) (int, error) {
	w.capture([]byte(s))
	return w.ResponseWriter.WriteString(s)
}

// capturedBody は捕捉済みボディを丸め接尾辞付きで返す。
func (w *bodyWriter) capturedBody() string {
	if w.truncated {
		return w.buf.String() + truncatedSuffix
	}
	return w.buf.String()
}

// Logger は HTTP リクエスト/レスポンスを2本立て（http.request / http.response）で
// 構造化出力する（design D5）。method/path/query を context へ格納し（1.2 のヘルパ）、
// ヘッダーには機密マスク（D6）を適用する。RequestID ミドルウェアより後に登録すること。
func Logger(logger *slog.Logger) gin.HandlerFunc {
	if logger == nil {
		logger = logging.Default()
	}
	return func(c *gin.Context) {
		start := time.Now()

		method := c.Request.Method
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		// method/path/query を context へ格納し、以降の全ログのトップレベルへ昇格させる。
		ctx := logging.WithHTTPInfo(c.Request.Context(), method, path, query)
		c.Request = c.Request.WithContext(ctx)

		// リクエストボディを読み取り、後続ハンドラのために復元する（design D5）。
		reqBody := ""
		if c.Request.Body != nil {
			raw, err := io.ReadAll(c.Request.Body)
			_ = c.Request.Body.Close()
			if err == nil {
				c.Request.Body = io.NopCloser(bytes.NewReader(raw))
				reqBody = truncateBody(raw)
			}
		}

		logger.LogAttrs(ctx, slog.LevelInfo, "http.request",
			slog.String("query", query),
			slog.Any("headers", logging.MaskHeader(c.Request.Header)),
			slog.String("body", reqBody),
		)

		bw := &bodyWriter{ResponseWriter: c.Writer}
		c.Writer = bw

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()

		attrs := []slog.Attr{
			slog.Int("status", status),
			slog.Any("headers", logging.MaskHeader(c.Writer.Header())),
			slog.String("body", bw.capturedBody()),
			slog.Float64("latency_ms", logging.DurationMillis(latency)),
		}
		if len(c.Errors) > 0 {
			attrs = append(attrs, slog.String("errors", c.Errors.String()))
		}

		level := slog.LevelInfo
		switch {
		case status >= 500:
			level = slog.LevelError
		case status >= 400:
			level = slog.LevelWarn
		}

		logger.LogAttrs(ctx, level, "http.response", attrs...)
	}
}

// truncateBody はボディを logBodyMaxBytes まで丸め、超過時は接尾辞を付す。
func truncateBody(raw []byte) string {
	if len(raw) > logBodyMaxBytes {
		return string(raw[:logBodyMaxBytes]) + truncatedSuffix
	}
	return string(raw)
}
