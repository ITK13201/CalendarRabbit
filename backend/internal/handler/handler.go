// Package handler は gin ベースの HTTP ハンドラとルーティングを提供する。
package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/ITK13201/CalendarRabbit/backend/internal/domain/derr"
	"github.com/gin-gonic/gin"
)

// timeRFC3339 は API 全体で使う日時フォーマット。
const timeRFC3339 = time.RFC3339

// badRequest はバリデーションエラーを生成するショートハンド。
func badRequest(field, message string) error {
	return derr.NewValidationError(field, message)
}

// Handler は各ユースケースを束ねる HTTP ハンドラ。
type Handler struct {
	calendar CalendarUseCase
	settings SettingsUseCase
	chat     ChatUseCase
	logger   *slog.Logger
}

// Deps は Handler の依存。
type Deps struct {
	Calendar CalendarUseCase
	Settings SettingsUseCase
	Chat     ChatUseCase
	Logger   *slog.Logger
}

// New は Handler を生成する。
func New(deps Deps) *Handler {
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{
		calendar: deps.Calendar,
		settings: deps.Settings,
		chat:     deps.Chat,
		logger:   logger,
	}
}

// errorResponse は API のエラーレスポンス本体。
type errorResponse struct {
	Error string `json:"error"`
	Field string `json:"field,omitempty"`
}

// respondError はドメインエラーを適切な HTTP ステータスにマップして返す。
func respondError(c *gin.Context, err error) {
	var ve *derr.ValidationError
	switch {
	case errors.As(err, &ve):
		c.JSON(http.StatusBadRequest, errorResponse{Error: ve.Message, Field: ve.Field})
	case errors.Is(err, derr.ErrNotFound):
		c.JSON(http.StatusNotFound, errorResponse{Error: "resource not found"})
	case errors.Is(err, derr.ErrConflict):
		c.JSON(http.StatusConflict, errorResponse{Error: err.Error()})
	default:
		_ = c.Error(err)
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "internal server error"})
	}
}
