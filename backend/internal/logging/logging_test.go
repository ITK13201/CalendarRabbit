package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestIDRoundTrip(t *testing.T) {
	ctx := WithRequestID(context.Background(), "req-123")
	assert.Equal(t, "req-123", RequestIDFromContext(ctx))
}

func TestRequestIDFromContext_Empty(t *testing.T) {
	assert.Equal(t, "", RequestIDFromContext(context.Background()))
}

func TestLogContext_IncludesRequestIDAndExtra(t *testing.T) {
	var buf bytes.Buffer
	logger := New(&buf, slog.LevelInfo)

	ctx := WithRequestID(context.Background(), "req-abc")
	LogContext(ctx, logger, slog.LevelInfo, "doing work",
		slog.String("entity", "calendar_event"),
		slog.Int("count", 3),
	)

	var out map[string]interface{}
	require.NoError(t, json.Unmarshal(buf.Bytes(), &out))

	assert.Equal(t, "doing work", out["msg"])
	assert.Equal(t, "req-abc", out[RequestIDField])
	assert.Equal(t, "calendar_event", out["entity"])
	assert.Equal(t, float64(3), out["count"])
	assert.Equal(t, "INFO", out["level"])
}

func TestLogContext_WithoutRequestID(t *testing.T) {
	var buf bytes.Buffer
	logger := New(&buf, slog.LevelInfo)

	LogContext(context.Background(), logger, slog.LevelWarn, "no request id")

	var out map[string]interface{}
	require.NoError(t, json.Unmarshal(buf.Bytes(), &out))
	_, hasRequestID := out[RequestIDField]
	assert.False(t, hasRequestID)
	assert.Equal(t, "WARN", out["level"])
}
