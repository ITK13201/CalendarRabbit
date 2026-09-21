package handler

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"

	"github.com/ITK13201/CalendarRabbit/backend/internal/usecase/googlesync"
	"github.com/gin-gonic/gin"
)

// GoogleUseCase は Google 連携ハンドラが依存するユースケース（未設定時は nil）。
type GoogleUseCase interface {
	AuthURL(state string) string
	Status(ctx context.Context) (*googlesync.Status, error)
	Connect(ctx context.Context, code string) error
	Resync(ctx context.Context) (int, error)
	Disconnect(ctx context.Context, deleteCalendar bool) error
}

// googleStatusResponse は連携状態のレスポンス。
type googleStatusResponse struct {
	Configured bool `json:"configured"`
	Connected  bool `json:"connected"`
	// NeedsReconnect はトークン失効やカレンダー消失で未連携相当へ落ちており再連携が必要な状態。
	NeedsReconnect bool `json:"needs_reconnect"`
	PendingCount   int  `json:"pending_count"`
}

// googleAuthResponse は認可URLのレスポンス。
type googleAuthResponse struct {
	AuthURL string `json:"auth_url"`
}

// googleResyncResponse は再同期結果のレスポンス。
type googleResyncResponse struct {
	PendingCount int `json:"pending_count"`
}

// googleReady は Google 連携が設定済み（利用可能）かどうかを返す。
func (h *Handler) googleReady() bool {
	return h.google != nil
}

// respondNotConfigured は連携未設定時の応答を返す。
func respondNotConfigured(c *gin.Context) {
	c.JSON(http.StatusServiceUnavailable, errorResponse{Error: "google sync is not configured"})
}

// newState は CSRF 対策の state（ランダム文字列）を生成する。
func newState() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// GoogleAuth godoc
// @Summary  Google 連携の認可URLを返す
// @Tags     google
// @Produce  json
// @Success  200 {object} googleAuthResponse
// @Failure  503 {object} errorResponse
// @Router   /api/google/auth [get]
func (h *Handler) GoogleAuth(c *gin.Context) {
	if !h.googleReady() {
		respondNotConfigured(c)
		return
	}
	state, err := newState()
	if err != nil {
		respondError(c, err)
		return
	}
	h.mu.Lock()
	h.googleState = state
	h.mu.Unlock()

	c.JSON(http.StatusOK, googleAuthResponse{AuthURL: h.google.AuthURL(state)})
}

// GoogleCallback godoc
// @Summary  Google 認可コールバック（トークン交換して連携を確立し設定画面へ戻す）
// @Tags     google
// @Param    state query string true "CSRF state"
// @Param    code query string false "認可コード"
// @Success  302
// @Router   /api/google/callback [get]
func (h *Handler) GoogleCallback(c *gin.Context) {
	if !h.googleReady() {
		respondNotConfigured(c)
		return
	}

	// state 検証（CSRF 対策）。使い捨てにする。
	state := c.Query("state")
	h.mu.Lock()
	expected := h.googleState
	h.googleState = ""
	h.mu.Unlock()
	if state == "" || expected == "" || state != expected {
		c.Redirect(http.StatusFound, h.googleRedirect+"?google=error")
		return
	}

	// 同意拒否・エラー時は連携を有効にせず戻す。
	if errParam := c.Query("error"); errParam != "" {
		c.Redirect(http.StatusFound, h.googleRedirect+"?google=error")
		return
	}
	code := c.Query("code")
	if code == "" {
		c.Redirect(http.StatusFound, h.googleRedirect+"?google=error")
		return
	}

	if err := h.google.Connect(c.Request.Context(), code); err != nil {
		h.logger.Error("google connect failed", "error", err.Error())
		c.Redirect(http.StatusFound, h.googleRedirect+"?google=error")
		return
	}
	c.Redirect(http.StatusFound, h.googleRedirect+"?google=connected")
}

// GoogleStatus godoc
// @Summary  Google 連携状態を取得する（未同期件数を含む）
// @Tags     google
// @Produce  json
// @Success  200 {object} googleStatusResponse
// @Router   /api/google/status [get]
func (h *Handler) GoogleStatus(c *gin.Context) {
	if !h.googleReady() {
		// 未設定でも 200 を返し、フロントは configured=false で UI を隠す。
		c.JSON(http.StatusOK, googleStatusResponse{Configured: false})
		return
	}
	st, err := h.google.Status(c.Request.Context())
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, googleStatusResponse{
		Configured:     true,
		Connected:      st.Connected,
		NeedsReconnect: st.NeedsReconnect,
		PendingCount:   st.PendingCount,
	})
}

// GoogleDisconnect godoc
// @Summary  Google 連携を解除する（専用カレンダーの削除/保持を選択）
// @Tags     google
// @Produce  json
// @Param    delete_calendar query bool false "専用カレンダーごと削除するか（既定 false=残す）"
// @Success  204
// @Failure  503 {object} errorResponse
// @Router   /api/google/connection [delete]
func (h *Handler) GoogleDisconnect(c *gin.Context) {
	if !h.googleReady() {
		respondNotConfigured(c)
		return
	}
	deleteCalendar := c.Query("delete_calendar") == "true"
	if err := h.google.Disconnect(c.Request.Context(), deleteCalendar); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// GoogleResync godoc
// @Summary  未同期イベントを手動で再同期する
// @Tags     google
// @Produce  json
// @Success  200 {object} googleResyncResponse
// @Failure  503 {object} errorResponse
// @Router   /api/google/resync [post]
func (h *Handler) GoogleResync(c *gin.Context) {
	if !h.googleReady() {
		respondNotConfigured(c)
		return
	}
	remaining, err := h.google.Resync(c.Request.Context())
	if err != nil {
		// 失効/カレンダー消失は再連携が必要。status の needs_reconnect と揃えて 409 を返す。
		if errors.Is(err, googlesync.ErrReconnectRequired) || errors.Is(err, googlesync.ErrNotConnected) {
			c.JSON(http.StatusConflict, errorResponse{Error: err.Error()})
			return
		}
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, googleResyncResponse{PendingCount: remaining})
}
