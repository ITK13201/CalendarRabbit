package handler

import (
	"context"
	"net/http"

	"github.com/ITK13201/CalendarRabbit/backend/internal/domain/entity"
	"github.com/gin-gonic/gin"
)

// SettingsUseCase は設定ハンドラが依存するユースケース。
type SettingsUseCase interface {
	Get(ctx context.Context) (*entity.AppSetting, error)
	Update(ctx context.Context, timezone string) (*entity.AppSetting, error)
}

type settingsRequest struct {
	Timezone string `json:"timezone"`
}

type settingsResponse struct {
	Timezone  string `json:"timezone"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

func toSettingsResponse(s *entity.AppSetting) settingsResponse {
	resp := settingsResponse{Timezone: s.Timezone}
	if !s.UpdatedAt.IsZero() {
		resp.UpdatedAt = s.UpdatedAt.UTC().Format(timeRFC3339)
	}
	return resp
}

// GetSettings godoc
// @Summary  アプリ設定を取得する
// @Tags     settings
// @Produce  json
// @Success  200 {object} settingsResponse
// @Router   /api/settings [get]
func (h *Handler) GetSettings(c *gin.Context) {
	s, err := h.settings.Get(c.Request.Context())
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toSettingsResponse(s))
}

// UpdateSettings godoc
// @Summary  アプリ設定を更新する
// @Tags     settings
// @Accept   json
// @Produce  json
// @Param    body body settingsRequest true "設定"
// @Success  200 {object} settingsResponse
// @Failure  400 {object} errorResponse
// @Router   /api/settings [put]
func (h *Handler) UpdateSettings(c *gin.Context) {
	var req settingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, badRequest("body", "invalid request body"))
		return
	}
	s, err := h.settings.Update(c.Request.Context(), req.Timezone)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toSettingsResponse(s))
}
