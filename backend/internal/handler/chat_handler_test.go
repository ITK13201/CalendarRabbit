package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/ITK13201/CalendarRabbit/backend/internal/handler"
	"github.com/ITK13201/CalendarRabbit/backend/internal/service/calendarprovider"
	chatservice "github.com/ITK13201/CalendarRabbit/backend/internal/service/chat"
	"github.com/ITK13201/CalendarRabbit/backend/internal/service/persistence"
	"github.com/ITK13201/CalendarRabbit/backend/internal/testsupport"
	"github.com/ITK13201/CalendarRabbit/backend/internal/usecase/calendar"
	chatuc "github.com/ITK13201/CalendarRabbit/backend/internal/usecase/chat"
	"github.com/ITK13201/CalendarRabbit/backend/internal/usecase/settings"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubExtractor は chatservice.Extractor のスタブ。
type stubExtractor struct {
	result *chatservice.ExtractionResult
}

func (s *stubExtractor) Extract(_ context.Context, _ []chatservice.Turn, _ string) (*chatservice.ExtractionResult, error) {
	return s.result, nil
}

// newDBRouter は DB を用いたフルスタックのルータを構築する。
func newDBRouter(t *testing.T, extraction *chatservice.ExtractionResult) *gin.Engine {
	client := testsupport.NewClient(t)

	calUC := calendar.New(calendarprovider.NewDBProvider(persistence.NewCalendarEventRepository(client)), nil)
	setUC := settings.New(persistence.NewAppSettingRepository(client), nil)
	extractors := map[string]chatservice.Extractor{"deepseek": &stubExtractor{result: extraction}}
	chatUC := chatuc.New(client, extractors, setUC, "deepseek", nil)

	h := handler.New(handler.Deps{Calendar: calUC, Settings: setUC, Chat: chatUC})
	return handler.NewRouter(h, handler.RouterConfig{AllowedOrigins: []string{"*"}})
}

func eventExtraction() *chatservice.ExtractionResult {
	start := time.Date(2026, 9, 24, 1, 0, 0, 0, time.UTC)
	return &chatservice.ExtractionResult{
		Status:  chatservice.StatusEvent,
		Message: "TGSの予定案を作成しました",
		Event: &chatservice.ExtractedEvent{
			Title:    "東京ゲームショウ2026",
			StartsAt: start,
			EndsAt:   start.Add(72 * time.Hour),
			Location: "幕張メッセ",
		},
	}
}

func TestChatHandler_ApprovalGatesRegistration(t *testing.T) {
	r := newDBRouter(t, eventExtraction())

	// メッセージ送信 → 予定案が返る
	w := doJSON(t, r, http.MethodPost, "/api/chat/messages", map[string]any{"content": "TGSの予定を追加して"})
	require.Equal(t, http.StatusOK, w.Code)
	var send map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &send))
	proposal, ok := send["proposal"].(map[string]any)
	require.True(t, ok, "proposal should be present")
	proposalID := int(proposal["id"].(float64))
	assert.Equal(t, "pending", proposal["status"])

	// 承認前: カレンダーは空
	w = doJSON(t, r, http.MethodGet, "/api/calendar/events", nil)
	require.Equal(t, http.StatusOK, w.Code)
	var before []map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &before))
	assert.Empty(t, before, "no event registered before approval")

	// 承認
	w = doJSON(t, r, http.MethodPost, "/api/chat/proposals/"+itoa(proposalID)+"/approve", nil)
	require.Equal(t, http.StatusOK, w.Code)

	// 承認後: カレンダーに1件登録される
	w = doJSON(t, r, http.MethodGet, "/api/calendar/events", nil)
	require.Equal(t, http.StatusOK, w.Code)
	var after []map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &after))
	require.Len(t, after, 1)
	assert.Equal(t, "東京ゲームショウ2026", after[0]["title"])

	// 再承認は 409
	w = doJSON(t, r, http.MethodPost, "/api/chat/proposals/"+itoa(proposalID)+"/approve", nil)
	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestChatHandler_RejectDoesNotRegister(t *testing.T) {
	r := newDBRouter(t, eventExtraction())

	w := doJSON(t, r, http.MethodPost, "/api/chat/messages", map[string]any{"content": "TGSを追加"})
	require.Equal(t, http.StatusOK, w.Code)
	var send map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &send))
	proposalID := int(send["proposal"].(map[string]any)["id"].(float64))

	// 却下
	w = doJSON(t, r, http.MethodPost, "/api/chat/proposals/"+itoa(proposalID)+"/reject", nil)
	require.Equal(t, http.StatusOK, w.Code)
	var rejected map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &rejected))
	assert.Equal(t, "rejected", rejected["status"])

	// カレンダーは空のまま
	w = doJSON(t, r, http.MethodGet, "/api/calendar/events", nil)
	var after []map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &after))
	assert.Empty(t, after)
}

func TestChatHandler_OffTopicNoProposal(t *testing.T) {
	r := newDBRouter(t, &chatservice.ExtractionResult{
		Status: chatservice.StatusOffTopic, Message: "予定登録専用です",
	})

	w := doJSON(t, r, http.MethodPost, "/api/chat/messages", map[string]any{"content": "今日の天気は?"})
	require.Equal(t, http.StatusOK, w.Code)
	var send map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &send))
	_, hasProposal := send["proposal"]
	assert.False(t, hasProposal, "off_topic must not produce a proposal")
}

func TestChatHandler_ConversationHistory(t *testing.T) {
	r := newDBRouter(t, eventExtraction())

	_ = doJSON(t, r, http.MethodPost, "/api/chat/messages", map[string]any{"content": "TGSを追加"})

	w := doJSON(t, r, http.MethodGet, "/api/chat/conversations", nil)
	require.Equal(t, http.StatusOK, w.Code)
	var conv map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &conv))
	messages := conv["messages"].([]any)
	assert.Len(t, messages, 2)
	proposals := conv["proposals"].([]any)
	assert.Len(t, proposals, 1)
}

func TestChatHandler_ClearConversation(t *testing.T) {
	r := newDBRouter(t, eventExtraction())

	// メッセージ送信で履歴・予定案を作成する
	w := doJSON(t, r, http.MethodPost, "/api/chat/messages", map[string]any{"content": "TGSを追加"})
	require.Equal(t, http.StatusOK, w.Code)

	// クリアは 204 を返す
	w = doJSON(t, r, http.MethodDelete, "/api/chat/conversations", nil)
	require.Equal(t, http.StatusNoContent, w.Code)

	// クリア後の履歴取得は空
	w = doJSON(t, r, http.MethodGet, "/api/chat/conversations", nil)
	require.Equal(t, http.StatusOK, w.Code)
	var conv map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &conv))
	assert.Empty(t, conv["messages"].([]any))
	assert.Empty(t, conv["proposals"].([]any))
}

func TestChatHandler_EmptyContent(t *testing.T) {
	r := newDBRouter(t, eventExtraction())
	w := doJSON(t, r, http.MethodPost, "/api/chat/messages", map[string]any{"content": ""})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}
