package middleware

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ITK13201/CalendarRabbit/backend/internal/logging"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestRequestID_SetsContextAndHeader(t *testing.T) {
	r := gin.New()
	r.Use(RequestID())

	var capturedFromCtx string
	r.GET("/", func(c *gin.Context) {
		capturedFromCtx = logging.RequestIDFromContext(c.Request.Context())
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	r.ServeHTTP(w, req)

	headerID := w.Header().Get(RequestIDHeader)
	assert.NotEmpty(t, headerID)
	assert.Equal(t, headerID, capturedFromCtx, "context requestId must match response header")
}

func TestRequestID_HonorsIncomingHeader(t *testing.T) {
	r := gin.New()
	r.Use(RequestID())
	r.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(RequestIDHeader, "incoming-id")
	r.ServeHTTP(w, req)

	assert.Equal(t, "incoming-id", w.Header().Get(RequestIDHeader))
}

func TestLogger_OutputsJSONWithRequestID(t *testing.T) {
	var buf bytes.Buffer
	logger := logging.New(&buf, slog.LevelInfo)

	r := gin.New()
	r.Use(RequestID())
	r.Use(Logger(logger))
	r.GET("/ping", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ping?x=1", nil)
	req.Header.Set(RequestIDHeader, "log-req-1")
	r.ServeHTTP(w, req)

	var out map[string]interface{}
	require.NoError(t, json.Unmarshal(buf.Bytes(), &out))
	assert.Equal(t, "access", out["msg"])
	assert.Equal(t, "log-req-1", out[logging.RequestIDField])
	assert.Equal(t, "/ping", out["path"])
	assert.Equal(t, float64(200), out["status"])
	assert.Equal(t, "GET", out["method"])
}

func TestCORS_AllowsConfiguredOrigin(t *testing.T) {
	r := gin.New()
	r.Use(CORS([]string{"http://localhost:5173"}))
	r.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	r.ServeHTTP(w, req)

	assert.Equal(t, "http://localhost:5173", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Contains(t, w.Header().Get("Access-Control-Allow-Methods"), "POST")
}

func TestCORS_RejectsUnknownOrigin(t *testing.T) {
	r := gin.New()
	r.Use(CORS([]string{"http://localhost:5173"}))
	r.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "http://evil.example.com")
	r.ServeHTTP(w, req)

	assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_PreflightReturns204(t *testing.T) {
	r := gin.New()
	r.Use(CORS([]string{"http://localhost:5173"}))
	r.POST("/", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, "http://localhost:5173", w.Header().Get("Access-Control-Allow-Origin"))
}
