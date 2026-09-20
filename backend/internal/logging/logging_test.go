package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

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

func TestHTTPInfoRoundTrip(t *testing.T) {
	ctx := WithHTTPInfo(context.Background(), http.MethodPost, "/chat", "x=1")
	info := HTTPInfoFromContext(ctx)
	assert.Equal(t, http.MethodPost, info.Method)
	assert.Equal(t, "/chat", info.Path)
	assert.Equal(t, "x=1", info.Query)
}

func TestHTTPInfoFromContext_Empty(t *testing.T) {
	assert.Equal(t, HTTPInfo{}, HTTPInfoFromContext(context.Background()))
}

// contextHandler は予約キーをトップレベルへ、その他を extra へネストする。
func TestContextHandler_PromotesReservedAndNestsExtra(t *testing.T) {
	var buf bytes.Buffer
	logger := New(&buf, slog.LevelInfo)

	ctx := WithRequestID(context.Background(), "req-abc")
	ctx = WithHTTPInfo(ctx, http.MethodGet, "/events", "from=1")
	LogContext(ctx, logger, slog.LevelInfo, "doing work",
		slog.String("entity", "calendar_event"),
		slog.Int("count", 3),
	)

	var out map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &out))

	// トップレベル: 予約キー。
	assert.Equal(t, "doing work", out["msg"])
	assert.Equal(t, "INFO", out["level"])
	assert.Equal(t, "req-abc", out[RequestIDField])
	assert.Equal(t, http.MethodGet, out[MethodField])
	assert.Equal(t, "/events", out[PathField])
	assert.Equal(t, "from=1", out[QueryField])

	// 任意属性はトップレベルに出ず extra 配下へ。
	_, hasTop := out["entity"]
	assert.False(t, hasTop)
	extra, ok := out[ExtraGroup].(map[string]any)
	require.True(t, ok, "extra group must exist")
	assert.Equal(t, "calendar_event", extra["entity"])
	assert.Equal(t, float64(3), extra["count"])
}

// リクエスト文脈を持たないログでは予約キーを省略する。
func TestContextHandler_WithoutContext(t *testing.T) {
	var buf bytes.Buffer
	logger := New(&buf, slog.LevelInfo)

	LogContext(context.Background(), logger, slog.LevelWarn, "no request id")

	var out map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &out))
	_, hasRequestID := out[RequestIDField]
	assert.False(t, hasRequestID)
	_, hasMethod := out[MethodField]
	assert.False(t, hasMethod)
	assert.Equal(t, "WARN", out["level"])
}

func TestTruncate(t *testing.T) {
	assert.Equal(t, "abc", Truncate("abc", 1000))
	// 1000文字境界: ちょうど1000はそのまま。
	exactly := strings.Repeat("あ", 1000)
	assert.Equal(t, exactly, Truncate(exactly, 1000))
	// 1001文字は先頭1000＋省略記号。
	over := strings.Repeat("あ", 1001)
	got := Truncate(over, 1000)
	assert.Equal(t, strings.Repeat("あ", 1000)+"…", got)
	assert.Equal(t, 1001, len([]rune(got)))
}

func TestMask(t *testing.T) {
	assert.True(t, IsSensitiveKey("Authorization"))
	assert.True(t, IsSensitiveKey("authorization"))
	assert.True(t, IsSensitiveKey("X-Api-Key"))
	assert.True(t, IsSensitiveKey("Set-Cookie"))
	assert.False(t, IsSensitiveKey("Content-Type"))

	h := http.Header{}
	h.Set("Authorization", "Bearer secret")
	h.Set("Content-Type", "application/json")
	masked := MaskHeader(h)
	assert.Equal(t, []string{RedactedValue}, masked["Authorization"])
	assert.Equal(t, []string{"application/json"}, masked["Content-Type"])
	// 元のヘッダーは不変。
	assert.Equal(t, "Bearer secret", h.Get("Authorization"))
}

// Trace は started/finished の2本を出力し、result/error を反映する。
func TestTrace_StartedAndFinished(t *testing.T) {
	var buf bytes.Buffer
	logger := New(&buf, slog.LevelInfo)
	ctx := WithRequestID(context.Background(), "req-trace")

	func() (res string, err error) {
		defer Trace(ctx, logger, "pkg.T.Do", Args{"in": "hi"}, &res, &err)()
		time.Sleep(2 * time.Millisecond)
		res = "done"
		return res, nil
	}()

	lines := splitJSONLines(t, buf.Bytes())
	require.Len(t, lines, 2)

	started := lines[0]
	assert.Equal(t, "pkg.T.Do started", started["msg"])
	assert.Equal(t, "req-trace", started[RequestIDField])
	startedExtra := started[ExtraGroup].(map[string]any)
	startedArgs := startedExtra["args"].(map[string]any)
	assert.Equal(t, "hi", startedArgs["in"])

	finished := lines[1]
	assert.Equal(t, "pkg.T.Do finished", finished["msg"])
	finishedExtra := finished[ExtraGroup].(map[string]any)
	assert.Equal(t, "done", finishedExtra["result"])
	_, hasErr := finishedExtra["error"]
	assert.False(t, hasErr)
	// latency_ms はサブミリ秒精度の float で、実測値（>0）が入る。
	latency, ok := finishedExtra["latency_ms"].(float64)
	require.True(t, ok, "latency_ms must be a float")
	assert.Positive(t, latency)
}

// error 時の finished は Error レベルで error を含める。
func TestTrace_Error(t *testing.T) {
	var buf bytes.Buffer
	logger := New(&buf, slog.LevelInfo)

	func() (res *string, err error) {
		defer Trace(context.Background(), logger, "pkg.T.Fail", nil, &res, &err)()
		err = errors.New("boom")
		return nil, err
	}()

	lines := splitJSONLines(t, buf.Bytes())
	require.Len(t, lines, 2)
	finished := lines[1]
	assert.Equal(t, "ERROR", finished["level"])
	finishedExtra := finished[ExtraGroup].(map[string]any)
	assert.Equal(t, "boom", finishedExtra["error"])
}

// Trace は args/result の長い文字列を丸め、機密キーをマスクする。
func TestTrace_TruncatesAndMasks(t *testing.T) {
	var buf bytes.Buffer
	logger := New(&buf, slog.LevelInfo)

	long := strings.Repeat("a", 2000)
	type payload struct {
		Body          string `json:"body"`
		Authorization string `json:"authorization"`
	}
	func() (res *payload, err error) {
		defer Trace(context.Background(), logger, "pkg.T.Big",
			Args{"authorization": "Bearer top-secret", "note": long}, &res, &err)()
		res = &payload{Body: long, Authorization: "Bearer top-secret"}
		return res, nil
	}()

	lines := splitJSONLines(t, buf.Bytes())
	require.Len(t, lines, 2)

	startedArgs := lines[0][ExtraGroup].(map[string]any)["args"].(map[string]any)
	assert.Equal(t, RedactedValue, startedArgs["authorization"])
	assert.Equal(t, MaxTruncateRunes+1, len([]rune(startedArgs["note"].(string))))

	finishedExtra := lines[1][ExtraGroup].(map[string]any)
	result := finishedExtra["result"].(map[string]any)
	assert.Equal(t, RedactedValue, result["authorization"])
	assert.Equal(t, MaxTruncateRunes+1, len([]rune(result["body"].(string))))
}

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
