package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"github.com/ITK13201/CalendarRabbit/backend/internal/handler"
	"github.com/ITK13201/CalendarRabbit/backend/internal/usecase/calendar"
	"github.com/ITK13201/CalendarRabbit/backend/internal/usecase/googlesync"
	"github.com/ITK13201/CalendarRabbit/backend/internal/usecase/settings"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeGoogleUC は handler テスト用の GoogleUseCase フェイク。
type fakeGoogleUC struct {
	lastState        string
	status           *googlesync.Status
	statusErr        error
	connectCalled    bool
	connectCode      string
	connectErr       error
	resyncRemaining  int
	resyncErr        error
	disconnectCalled bool
	disconnectDelete bool
}

func (f *fakeGoogleUC) AuthURL(state string) string {
	f.lastState = state
	return "https://accounts.google.com/o/oauth2/auth?state=" + url.QueryEscape(state)
}

func (f *fakeGoogleUC) Status(context.Context) (*googlesync.Status, error) {
	if f.statusErr != nil {
		return nil, f.statusErr
	}
	if f.status == nil {
		return &googlesync.Status{}, nil
	}
	return f.status, nil
}

func (f *fakeGoogleUC) Connect(_ context.Context, code string) error {
	f.connectCalled = true
	f.connectCode = code
	return f.connectErr
}

func (f *fakeGoogleUC) Resync(context.Context) (int, error) {
	return f.resyncRemaining, f.resyncErr
}

func (f *fakeGoogleUC) Disconnect(_ context.Context, deleteCalendar bool) error {
	f.disconnectCalled = true
	f.disconnectDelete = deleteCalendar
	return nil
}

func newGoogleRouter(g handler.GoogleUseCase) *gin.Engine {
	uc := calendar.New(newMemProvider(), nil)
	setUC := settings.New(&memSettingsRepo{}, nil)
	h := handler.New(handler.Deps{Calendar: uc, Settings: setUC, Google: g, GoogleRedirect: "/app"})
	return handler.NewRouter(h, handler.RouterConfig{AllowedOrigins: []string{"*"}})
}

// 6.1 認可URL取得 → コールバックで state 検証しトークン交換する。
func TestGoogleHandler_AuthThenCallback(t *testing.T) {
	g := &fakeGoogleUC{}
	r := newGoogleRouter(g)

	// auth
	w := doJSON(t, r, http.MethodGet, "/api/google/auth", nil)
	require.Equal(t, http.StatusOK, w.Code)
	var authResp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &authResp))
	assert.Contains(t, authResp["auth_url"], "state=")
	require.NotEmpty(t, g.lastState)

	// callback（正しい state）
	w = doJSON(t, r, http.MethodGet, "/api/google/callback?state="+url.QueryEscape(g.lastState)+"&code=auth-code", nil)
	require.Equal(t, http.StatusFound, w.Code)
	assert.Equal(t, "/app?google=connected", w.Header().Get("Location"))
	assert.True(t, g.connectCalled)
	assert.Equal(t, "auth-code", g.connectCode)
}

// 6.1 state 不一致（CSRF）はトークン交換せずエラーへ戻す。
func TestGoogleHandler_CallbackStateMismatch(t *testing.T) {
	g := &fakeGoogleUC{}
	r := newGoogleRouter(g)

	_ = doJSON(t, r, http.MethodGet, "/api/google/auth", nil)
	w := doJSON(t, r, http.MethodGet, "/api/google/callback?state=wrong&code=auth-code", nil)
	require.Equal(t, http.StatusFound, w.Code)
	assert.Equal(t, "/app?google=error", w.Header().Get("Location"))
	assert.False(t, g.connectCalled)
}

// 6.1 同意拒否（error パラメータ）は連携を有効にしない。
func TestGoogleHandler_CallbackConsentDenied(t *testing.T) {
	g := &fakeGoogleUC{}
	r := newGoogleRouter(g)

	_ = doJSON(t, r, http.MethodGet, "/api/google/auth", nil)
	w := doJSON(t, r, http.MethodGet, "/api/google/callback?state="+url.QueryEscape(g.lastState)+"&error=access_denied", nil)
	require.Equal(t, http.StatusFound, w.Code)
	assert.Equal(t, "/app?google=error", w.Header().Get("Location"))
	assert.False(t, g.connectCalled)
}

// 6.1 status は連携状態と未同期件数を返す。
func TestGoogleHandler_Status(t *testing.T) {
	g := &fakeGoogleUC{status: &googlesync.Status{Connected: true, PendingCount: 3}}
	r := newGoogleRouter(g)

	w := doJSON(t, r, http.MethodGet, "/api/google/status", nil)
	require.Equal(t, http.StatusOK, w.Code)
	var got map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, true, got["configured"])
	assert.Equal(t, true, got["connected"])
	assert.Equal(t, float64(3), got["pending_count"])
}

// 6.1 disconnect の delete_calendar パラメータが反映される。
func TestGoogleHandler_DisconnectDeleteParam(t *testing.T) {
	g := &fakeGoogleUC{}
	r := newGoogleRouter(g)

	w := doJSON(t, r, http.MethodDelete, "/api/google/connection?delete_calendar=true", nil)
	require.Equal(t, http.StatusNoContent, w.Code)
	assert.True(t, g.disconnectCalled)
	assert.True(t, g.disconnectDelete)

	g2 := &fakeGoogleUC{}
	r2 := newGoogleRouter(g2)
	w = doJSON(t, r2, http.MethodDelete, "/api/google/connection", nil)
	require.Equal(t, http.StatusNoContent, w.Code)
	assert.True(t, g2.disconnectCalled)
	assert.False(t, g2.disconnectDelete)
}

// 6.1 resync は残りの未同期件数を返す。
func TestGoogleHandler_Resync(t *testing.T) {
	g := &fakeGoogleUC{resyncRemaining: 2}
	r := newGoogleRouter(g)

	w := doJSON(t, r, http.MethodPost, "/api/google/resync", nil)
	require.Equal(t, http.StatusOK, w.Code)
	var got map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, float64(2), got["pending_count"])
}

// 6.2 未設定時は連携系が「未設定」を返し、DB CRUD は動作する。
func TestGoogleHandler_NotConfigured(t *testing.T) {
	r := newGoogleRouter(nil) // Google 未設定

	// status は configured=false を返す。
	w := doJSON(t, r, http.MethodGet, "/api/google/status", nil)
	require.Equal(t, http.StatusOK, w.Code)
	var got map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, false, got["configured"])

	// アクション系は 503。
	assert.Equal(t, http.StatusServiceUnavailable, doJSON(t, r, http.MethodGet, "/api/google/auth", nil).Code)
	assert.Equal(t, http.StatusServiceUnavailable, doJSON(t, r, http.MethodPost, "/api/google/resync", nil).Code)
	assert.Equal(t, http.StatusServiceUnavailable, doJSON(t, r, http.MethodDelete, "/api/google/connection", nil).Code)

	// DB CRUD は連携未設定でも動作する。
	cw := doJSON(t, r, http.MethodPost, "/api/calendar/events", map[string]any{
		"title":     "Local only",
		"starts_at": "2026-09-21T01:00:00Z",
		"ends_at":   "2026-09-21T02:00:00Z",
	})
	require.Equal(t, http.StatusCreated, cw.Code)
}
