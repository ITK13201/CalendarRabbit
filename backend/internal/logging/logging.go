// Package logging は slog(JSON) ベースの構造化ロギングと、
// context に載せた requestId を横断的に扱うためのヘルパを提供する。
package logging

import (
	"context"
	"io"
	"log/slog"
	"os"
)

// ctxKey は context に値を格納するための非公開キー型。
type ctxKey struct{ name string }

var requestIDKey = ctxKey{name: "requestID"}

// RequestIDField はログに出力する requestId のフィールド名。
const RequestIDField = "requestId"

// New は JSON 形式で出力する slog.Logger を生成する。
func New(w io.Writer, level slog.Level) *slog.Logger {
	handler := slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level})
	return slog.New(handler)
}

// Default は標準出力に JSON を出力する既定ロガーを生成する。
func Default() *slog.Logger {
	return New(os.Stdout, slog.LevelInfo)
}

// WithRequestID は requestId を context に格納する。
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

// RequestIDFromContext は context から requestId を取り出す。存在しなければ空文字。
func RequestIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDKey).(string); ok {
		return v
	}
	return ""
}

// FromContext は context の requestId を付与した logger を返す。
func FromContext(ctx context.Context, logger *slog.Logger) *slog.Logger {
	if logger == nil {
		logger = Default()
	}
	if id := RequestIDFromContext(ctx); id != "" {
		return logger.With(slog.String(RequestIDField, id))
	}
	return logger
}

// LogContext は context の requestId と任意の extra field を付与して構造化ログを出力する。
// service 層はこのヘルパを通じて requestId 追跡付きのログを出力する（design.md D6）。
func LogContext(ctx context.Context, logger *slog.Logger, level slog.Level, msg string, extra ...slog.Attr) {
	if logger == nil {
		logger = Default()
	}
	attrs := make([]slog.Attr, 0, len(extra)+1)
	if id := RequestIDFromContext(ctx); id != "" {
		attrs = append(attrs, slog.String(RequestIDField, id))
	}
	attrs = append(attrs, extra...)
	logger.LogAttrs(ctx, level, msg, attrs...)
}
