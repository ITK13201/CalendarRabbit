package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
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

// splitJSONLines は改行区切りの JSON ログを1件ずつパースする。
func splitJSONLines(t *testing.T, b []byte) []map[string]any {
	t.Helper()
	var out []map[string]any
	for line := range strings.SplitSeq(strings.TrimSpace(string(b)), "\n") {
		if line == "" {
			continue
		}
		var m map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &m))
		out = append(out, m)
	}
	return out
}

// Logger は http.request / http.response の2本を出力し、
// 予約キーをトップレベル、その他を extra へネストする。
func TestLogger_EmitsRequestAndResponse(t *testing.T) {
	var buf bytes.Buffer
	logger := logging.New(&buf, slog.LevelInfo)

	r := gin.New()
	r.Use(RequestID())
	r.Use(Logger(logger))
	r.GET("/ping", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"pong": true}) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ping?x=1", nil)
	req.Header.Set(RequestIDHeader, "log-req-1")
	r.ServeHTTP(w, req)

	lines := splitJSONLines(t, buf.Bytes())
	require.Len(t, lines, 2)

	reqLog := lines[0]
	assert.Equal(t, "http.request", reqLog["msg"])
	assert.Equal(t, "log-req-1", reqLog[logging.RequestIDField])
	assert.Equal(t, http.MethodGet, reqLog[logging.MethodField])
	assert.Equal(t, "/ping", reqLog[logging.PathField])
	assert.Equal(t, "x=1", reqLog[logging.QueryField])
	reqExtra, ok := reqLog[logging.ExtraGroup].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, reqExtra, "headers")
	assert.Contains(t, reqExtra, "body")

	respLog := lines[1]
	assert.Equal(t, "http.response", respLog["msg"])
	assert.Equal(t, "log-req-1", respLog[logging.RequestIDField])
	respExtra, ok := respLog[logging.ExtraGroup].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(200), respExtra["status"])
	assert.Contains(t, respExtra, "latency_ms")
	assert.Contains(t, respExtra["body"].(string), "pong")
}

// 機密ヘッダーはマスクされる。
func TestLogger_MasksSensitiveHeaders(t *testing.T) {
	var buf bytes.Buffer
	logger := logging.New(&buf, slog.LevelInfo)

	r := gin.New()
	r.Use(RequestID())
	r.Use(Logger(logger))
	r.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer super-secret")
	r.ServeHTTP(w, req)

	lines := splitJSONLines(t, buf.Bytes())
	require.GreaterOrEqual(t, len(lines), 1)
	headers := lines[0][logging.ExtraGroup].(map[string]any)["headers"].(map[string]any)
	auth := headers["Authorization"].([]any)
	assert.Equal(t, logging.RedactedValue, auth[0])
	assert.NotContains(t, buf.String(), "super-secret")
}

// リクエストボディのログ化は後続ハンドラの読み取りを妨げない。
func TestLogger_RequestBodyReadableByHandler(t *testing.T) {
	var buf bytes.Buffer
	logger := logging.New(&buf, slog.LevelInfo)

	r := gin.New()
	r.Use(RequestID())
	r.Use(Logger(logger))
	var received string
	r.POST("/echo", func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		require.NoError(t, err)
		received = string(body)
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/echo", strings.NewReader(`{"hello":"world"}`))
	r.ServeHTTP(w, req)

	assert.Equal(t, `{"hello":"world"}`, received, "handler must read the same request body")
	lines := splitJSONLines(t, buf.Bytes())
	require.GreaterOrEqual(t, len(lines), 1)
	reqBody := lines[0][logging.ExtraGroup].(map[string]any)["body"].(string)
	assert.Equal(t, `{"hello":"world"}`, reqBody)
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
