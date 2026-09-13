package handler_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSettingsHandler_GetDefaultThenUpdate(t *testing.T) {
	r := newTestRouter()

	// 未初期化: 既定 TZ が返る
	w := doJSON(t, r, http.MethodGet, "/api/settings", nil)
	require.Equal(t, http.StatusOK, w.Code)
	var got map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, "Asia/Tokyo", got["timezone"])

	// 更新
	w = doJSON(t, r, http.MethodPut, "/api/settings", map[string]any{"timezone": "UTC"})
	require.Equal(t, http.StatusOK, w.Code)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, "UTC", got["timezone"])

	// 後続 GET に反映
	w = doJSON(t, r, http.MethodGet, "/api/settings", nil)
	require.Equal(t, http.StatusOK, w.Code)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, "UTC", got["timezone"])
}

func TestSettingsHandler_InvalidTimezone(t *testing.T) {
	r := newTestRouter()
	w := doJSON(t, r, http.MethodPut, "/api/settings", map[string]any{"timezone": "Mars/Phobos"})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}
