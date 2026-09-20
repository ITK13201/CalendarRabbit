// Package logging は slog(JSON) ベースの構造化ロギングと、
// context に載せた requestId / HTTP 情報（method/path/query）を
// 横断的に扱うためのヘルパを提供する。
//
// 出力は contextHandler により、requestId/method/path/query を
// トップレベルへ昇格し、その他の属性を extra 配下へネストする（design D1/D2）。
package logging

import (
	"context"
	"io"
	"log/slog"
	"os"
)

// ctxKey は context に値を格納するための非公開キー型。
type ctxKey struct{ name string }

var (
	requestIDKey = ctxKey{name: "requestID"}
	httpInfoKey  = ctxKey{name: "httpInfo"}
)

// RequestIDField はログに出力する requestId のフィールド名。
const RequestIDField = "requestId"

// New は contextHandler で包んだ JSON 形式の slog.Logger を生成する。
func New(w io.Writer, level slog.Level) *slog.Logger {
	base := slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level})
	return slog.New(NewContextHandler(base))
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

// HTTPInfo は context に伝播する HTTP リクエスト情報（トップレベルへ昇格する対象）。
type HTTPInfo struct {
	Method string
	Path   string
	Query  string
}

// WithHTTPInfo は HTTP メソッド/パス/クエリを context に格納する（design D2）。
func WithHTTPInfo(ctx context.Context, method, path, query string) context.Context {
	return context.WithValue(ctx, httpInfoKey, HTTPInfo{Method: method, Path: path, Query: query})
}

// HTTPInfoFromContext は context から HTTP 情報を取り出す。存在しなければゼロ値。
func HTTPInfoFromContext(ctx context.Context) HTTPInfo {
	if v, ok := ctx.Value(httpInfoKey).(HTTPInfo); ok {
		return v
	}
	return HTTPInfo{}
}

// LogContext は任意の extra field を付与して構造化ログを出力する。
// requestId/method/path/query は contextHandler が context から解決してトップレベルへ
// 昇格するため、ここで明示的に付与する必要はない。
func LogContext(ctx context.Context, logger *slog.Logger, level slog.Level, msg string, extra ...slog.Attr) {
	if logger == nil {
		logger = Default()
	}
	logger.LogAttrs(ctx, level, msg, extra...)
}
