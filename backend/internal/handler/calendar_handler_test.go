package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ITK13201/CalendarRabbit/backend/internal/handler"
	"github.com/ITK13201/CalendarRabbit/backend/internal/usecase/calendar"
	"github.com/ITK13201/CalendarRabbit/backend/internal/usecase/settings"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() { gin.SetMode(gin.TestMode) }

func newTestRouter() *gin.Engine {
	uc := calendar.New(newMemProvider(), nil)
	setUC := settings.New(&memSettingsRepo{}, nil)
	h := handler.New(handler.Deps{Calendar: uc, Settings: setUC})
	return handler.NewRouter(h, handler.RouterConfig{AllowedOrigins: []string{"*"}})
}

func doJSON(t *testing.T, r http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&buf).Encode(body))
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestCalendarHandler_CRUDAndPeriod(t *testing.T) {
	r := newTestRouter()
	start := time.Date(2026, 9, 26, 1, 0, 0, 0, time.UTC)

	// create
	w := doJSON(t, r, http.MethodPost, "/api/calendar/events", map[string]any{
		"title":     "TGS",
		"starts_at": start.Format(time.RFC3339),
		"ends_at":   start.Add(time.Hour).Format(time.RFC3339),
		"location":  "Makuhari",
	})
	require.Equal(t, http.StatusCreated, w.Code)
	var created map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	id := int(created["id"].(float64))
	assert.Equal(t, "TGS", created["title"])

	// get
	w = doJSON(t, r, http.MethodGet, "/api/calendar/events/"+itoa(id), nil)
	require.Equal(t, http.StatusOK, w.Code)

	// update
	w = doJSON(t, r, http.MethodPut, "/api/calendar/events/"+itoa(id), map[string]any{
		"title":     "TGS Updated",
		"starts_at": start.Format(time.RFC3339),
		"ends_at":   start.Add(2 * time.Hour).Format(time.RFC3339),
	})
	require.Equal(t, http.StatusOK, w.Code)

	// period list
	from := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	w = doJSON(t, r, http.MethodGet, "/api/calendar/events?from="+from.Format(time.RFC3339)+"&to="+to.Format(time.RFC3339), nil)
	require.Equal(t, http.StatusOK, w.Code)
	var list []map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &list))
	assert.Len(t, list, 1)

	// delete
	w = doJSON(t, r, http.MethodDelete, "/api/calendar/events/"+itoa(id), nil)
	require.Equal(t, http.StatusNoContent, w.Code)

	// get after delete -> 404
	w = doJSON(t, r, http.MethodGet, "/api/calendar/events/"+itoa(id), nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestCalendarHandler_ValidationError(t *testing.T) {
	r := newTestRouter()
	// missing title
	w := doJSON(t, r, http.MethodPost, "/api/calendar/events", map[string]any{
		"starts_at": time.Now().UTC().Format(time.RFC3339),
		"ends_at":   time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
	})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCalendarHandler_InvalidTimeFormat(t *testing.T) {
	r := newTestRouter()
	w := doJSON(t, r, http.MethodPost, "/api/calendar/events", map[string]any{
		"title":     "X",
		"starts_at": "not-a-time",
		"ends_at":   time.Now().Format(time.RFC3339),
	})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}
