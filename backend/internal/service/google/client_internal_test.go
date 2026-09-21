package google

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
)

func TestToGoogleEvent_AllDay(t *testing.T) {
	// ends_at は最終日を含む（inclusive）。09/17〜09/21 の複数日終日イベント。
	ev := EventData{
		Title:    "Holiday",
		AllDay:   true,
		StartsAt: time.Date(2026, 9, 17, 0, 0, 0, 0, time.UTC),
		EndsAt:   time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC),
		TimeZone: "Asia/Tokyo",
	}
	g := toGoogleEvent(ev)
	require.NotNil(t, g.Start)
	assert.Equal(t, "2026-09-17", g.Start.Date)
	// Google の end.date は排他的なので最終日(09/21)+1 の 09/22 になる。
	assert.Equal(t, "2026-09-22", g.End.Date)
	// 終日は dateTime を用いない。
	assert.Empty(t, g.Start.DateTime)
	assert.Empty(t, g.End.DateTime)
}

func TestToGoogleEvent_AllDaySingleDay(t *testing.T) {
	// 単日の終日イベント（starts_at == ends_at）は end.date が翌日になる。
	ev := EventData{
		Title:    "Single",
		AllDay:   true,
		StartsAt: time.Date(2026, 9, 17, 0, 0, 0, 0, time.UTC),
		EndsAt:   time.Date(2026, 9, 17, 0, 0, 0, 0, time.UTC),
	}
	g := toGoogleEvent(ev)
	assert.Equal(t, "2026-09-17", g.Start.Date)
	assert.Equal(t, "2026-09-18", g.End.Date)
}

func TestToGoogleEvent_Timed(t *testing.T) {
	ev := EventData{
		Title:    "Meeting",
		AllDay:   false,
		StartsAt: time.Date(2026, 9, 21, 1, 0, 0, 0, time.UTC),
		EndsAt:   time.Date(2026, 9, 21, 2, 0, 0, 0, time.UTC),
		TimeZone: "Asia/Tokyo",
	}
	g := toGoogleEvent(ev)
	assert.Equal(t, "2026-09-21T01:00:00Z", g.Start.DateTime)
	assert.Equal(t, "2026-09-21T02:00:00Z", g.End.DateTime)
	assert.Equal(t, "Asia/Tokyo", g.Start.TimeZone)
	assert.Equal(t, "Asia/Tokyo", g.End.TimeZone)
	// 時刻付きは date を用いない。
	assert.Empty(t, g.Start.Date)
}

func TestClassifyErr_RevokedOn401(t *testing.T) {
	err := classifyErr(&googleapi.Error{Code: http.StatusUnauthorized, Message: "invalid credentials"})
	assert.ErrorIs(t, err, ErrTokenRevoked)
}

func TestClassifyErr_RevokedOnRetrieveError(t *testing.T) {
	err := classifyErr(&oauth2.RetrieveError{ErrorCode: "invalid_grant"})
	assert.ErrorIs(t, err, ErrTokenRevoked)
}

func TestClassifyErr_PassesThroughOther(t *testing.T) {
	other := errors.New("boom")
	err := classifyErr(other)
	assert.Equal(t, other, err)
	assert.NotErrorIs(t, err, ErrTokenRevoked)
}

// TestService_CreateEvent はモック HTTP エンドポイントに対して CreateEvent が
// 生成イベントIDを返すことを確認する。
func TestService_CreateEvent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"evt-123"}`))
	}))
	defer ts.Close()

	client := newTestService(t, ts)
	id, err := client.CreateEvent(context.Background(), "cal-1", EventData{Title: "x"})
	require.NoError(t, err)
	assert.Equal(t, "evt-123", id)
}

// TestService_RevokedOn401 はモックが 401 を返すと ErrTokenRevoked に正規化されることを確認する。
func TestService_RevokedOn401(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"code":401,"message":"invalid credentials"}}`))
	}))
	defer ts.Close()

	client := newTestService(t, ts)
	_, err := client.CreateEvent(context.Background(), "cal-1", EventData{Title: "x"})
	assert.ErrorIs(t, err, ErrTokenRevoked)
}

func newTestService(t *testing.T, ts *httptest.Server) CalendarClient {
	t.Helper()
	client, err := newService(context.Background(),
		option.WithHTTPClient(ts.Client()),
		option.WithEndpoint(ts.URL),
	)
	require.NoError(t, err)
	return client
}
