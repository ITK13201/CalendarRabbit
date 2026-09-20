package chat_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/ITK13201/CalendarRabbit/backend/ent"
	"github.com/ITK13201/CalendarRabbit/backend/internal/domain/derr"
	"github.com/ITK13201/CalendarRabbit/backend/internal/domain/entity"
	chatservice "github.com/ITK13201/CalendarRabbit/backend/internal/service/chat"
	"github.com/ITK13201/CalendarRabbit/backend/internal/service/persistence"
	"github.com/ITK13201/CalendarRabbit/backend/internal/testsupport"
	"github.com/ITK13201/CalendarRabbit/backend/internal/usecase/chat"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestUC は単一の Extractor を既定プロバイダに割り当てて UseCase を構築するテストヘルパ。
// 設定リーダは nil のため、常に defaultProvider(deepseek) の Extractor が使われる。
func newTestUC(client *ent.Client, ext chatservice.Extractor, logger *slog.Logger) *chat.UseCase {
	return chat.New(client, map[string]chatservice.Extractor{"deepseek": ext}, nil, "deepseek", logger)
}

// mockExtractor は chatservice.Extractor のモック。
type mockExtractor struct {
	result *chatservice.ExtractionResult
	err    error
}

func (m *mockExtractor) Extract(_ context.Context, _ []chatservice.Turn, _ string) (*chatservice.ExtractionResult, error) {
	return m.result, m.err
}

// capturingExtractor は Extract に渡された会話履歴を記録する。
type capturingExtractor struct {
	result    *chatservice.ExtractionResult
	lastTurns []chatservice.Turn
}

func (c *capturingExtractor) Extract(_ context.Context, turns []chatservice.Turn, _ string) (*chatservice.ExtractionResult, error) {
	c.lastTurns = turns
	return c.result, nil
}

func eventResult() *chatservice.ExtractionResult {
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

func TestSendMessage_EventCreatesPendingProposalNotRegistered(t *testing.T) {
	client := testsupport.NewClient(t)
	uc := newTestUC(client, &mockExtractor{result: eventResult()}, nil)
	ctx := context.Background()

	res, err := uc.SendMessage(ctx, "TGSの予定を追加して")
	require.NoError(t, err)

	// ユーザー/アシスタントメッセージが永続化される
	require.NotNil(t, res.UserMessage)
	require.NotNil(t, res.AssistantMessage)
	assert.Equal(t, entity.RoleUser, res.UserMessage.Role)
	assert.Equal(t, entity.RoleAssistant, res.AssistantMessage.Role)

	// pending 予定案が作成される
	require.NotNil(t, res.Proposal)
	assert.Equal(t, entity.ProposalStatusPending, res.Proposal.Status)
	assert.Equal(t, "東京ゲームショウ2026", res.Proposal.Title)

	// 承認前はカレンダーに登録されない
	events, err := persistence.NewCalendarEventRepository(client, nil).List(ctx)
	require.NoError(t, err)
	assert.Empty(t, events)
}

func TestSendMessage_OffTopicNoProposal(t *testing.T) {
	client := testsupport.NewClient(t)
	uc := newTestUC(client, &mockExtractor{result: &chatservice.ExtractionResult{
		Status: chatservice.StatusOffTopic, Message: "予定登録専用です",
	}}, nil)

	res, err := uc.SendMessage(context.Background(), "今日の天気は？")
	require.NoError(t, err)
	assert.Nil(t, res.Proposal)
	assert.NotNil(t, res.AssistantMessage)
}

func TestSendMessage_NotFoundNoProposal(t *testing.T) {
	client := testsupport.NewClient(t)
	uc := newTestUC(client, &mockExtractor{result: &chatservice.ExtractionResult{
		Status: chatservice.StatusNotFound, Message: "特定できません。追加情報をください",
	}}, nil)

	res, err := uc.SendMessage(context.Background(), "なんかいい感じのイベント")
	require.NoError(t, err)
	assert.Nil(t, res.Proposal)
}

func TestSendMessage_MultipleCandidates(t *testing.T) {
	client := testsupport.NewClient(t)
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	uc := newTestUC(client, &mockExtractor{result: &chatservice.ExtractionResult{
		Status:  chatservice.StatusMultiple,
		Message: "候補が複数あります",
		Candidates: []chatservice.ExtractedEvent{
			{Title: "A", StartsAt: start, EndsAt: start.Add(time.Hour)},
			{Title: "B", StartsAt: start, EndsAt: start.Add(time.Hour)},
		},
	}}, nil)

	res, err := uc.SendMessage(context.Background(), "曖昧なやつ")
	require.NoError(t, err)
	assert.Nil(t, res.Proposal)
	require.Len(t, res.Candidates, 2)
	assert.Equal(t, "A", res.Candidates[0].Title)
}

func TestApproveProposal_RegistersEvent(t *testing.T) {
	client := testsupport.NewClient(t)
	uc := newTestUC(client, &mockExtractor{result: eventResult()}, nil)
	ctx := context.Background()

	sent, err := uc.SendMessage(ctx, "TGSを追加")
	require.NoError(t, err)

	event, err := uc.ApproveProposal(ctx, sent.Proposal.ID, nil)
	require.NoError(t, err)
	assert.Equal(t, "東京ゲームショウ2026", event.Title)

	// カレンダーに登録される
	events, err := persistence.NewCalendarEventRepository(client, nil).List(ctx)
	require.NoError(t, err)
	require.Len(t, events, 1)

	// 予定案が approved になり event に紐付く
	prop, err := persistence.NewEventProposalRepository(client, nil).Get(ctx, sent.Proposal.ID)
	require.NoError(t, err)
	assert.Equal(t, entity.ProposalStatusApproved, prop.Status)
	require.NotNil(t, prop.CalendarEventID)
	assert.Equal(t, event.ID, *prop.CalendarEventID)
}

func TestApproveProposal_WithEdit(t *testing.T) {
	client := testsupport.NewClient(t)
	uc := newTestUC(client, &mockExtractor{result: eventResult()}, nil)
	ctx := context.Background()

	sent, err := uc.SendMessage(ctx, "TGSを追加")
	require.NoError(t, err)

	newStart := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	event, err := uc.ApproveProposal(ctx, sent.Proposal.ID, &chat.ProposalEdit{
		Title:    "編集後タイトル",
		StartsAt: newStart,
		EndsAt:   newStart.Add(2 * time.Hour),
		Location: "別会場",
	})
	require.NoError(t, err)
	assert.Equal(t, "編集後タイトル", event.Title)
	assert.Equal(t, "別会場", event.Location)
	assert.True(t, newStart.Equal(event.StartsAt))
}

func TestApproveProposal_InvalidEdit(t *testing.T) {
	client := testsupport.NewClient(t)
	uc := newTestUC(client, &mockExtractor{result: eventResult()}, nil)
	ctx := context.Background()

	sent, err := uc.SendMessage(ctx, "TGSを追加")
	require.NoError(t, err)

	// 終了 < 開始
	_, err = uc.ApproveProposal(ctx, sent.Proposal.ID, &chat.ProposalEdit{
		Title:    "X",
		StartsAt: time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC),
		EndsAt:   time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC),
	})
	assert.True(t, derr.IsValidation(err))
}

func TestApproveProposal_AlreadyProcessed(t *testing.T) {
	client := testsupport.NewClient(t)
	uc := newTestUC(client, &mockExtractor{result: eventResult()}, nil)
	ctx := context.Background()

	sent, err := uc.SendMessage(ctx, "TGSを追加")
	require.NoError(t, err)
	_, err = uc.ApproveProposal(ctx, sent.Proposal.ID, nil)
	require.NoError(t, err)

	// 再承認は不可（重複登録しない）
	_, err = uc.ApproveProposal(ctx, sent.Proposal.ID, nil)
	assert.ErrorIs(t, err, derr.ErrConflict)

	events, err := persistence.NewCalendarEventRepository(client, nil).List(ctx)
	require.NoError(t, err)
	assert.Len(t, events, 1, "no duplicate registration")
}

func TestRejectProposal(t *testing.T) {
	client := testsupport.NewClient(t)
	uc := newTestUC(client, &mockExtractor{result: eventResult()}, nil)
	ctx := context.Background()

	sent, err := uc.SendMessage(ctx, "TGSを追加")
	require.NoError(t, err)

	rejected, err := uc.RejectProposal(ctx, sent.Proposal.ID)
	require.NoError(t, err)
	assert.Equal(t, entity.ProposalStatusRejected, rejected.Status)

	// 登録されない
	events, err := persistence.NewCalendarEventRepository(client, nil).List(ctx)
	require.NoError(t, err)
	assert.Empty(t, events)

	// 却下済みは再度却下不可
	_, err = uc.RejectProposal(ctx, sent.Proposal.ID)
	assert.ErrorIs(t, err, derr.ErrConflict)
}

func TestApproveProposal_NotFound(t *testing.T) {
	client := testsupport.NewClient(t)
	uc := newTestUC(client, &mockExtractor{result: eventResult()}, nil)
	_, err := uc.ApproveProposal(context.Background(), 999999, nil)
	assert.ErrorIs(t, err, derr.ErrNotFound)
}

func TestSendMessage_HistoryTruncatedToRecentTurns(t *testing.T) {
	client := testsupport.NewClient(t)
	ext := &capturingExtractor{result: &chatservice.ExtractionResult{
		Status: chatservice.StatusNotFound, Message: "ok",
	}}
	uc := newTestUC(client, ext, nil)
	ctx := context.Background()

	// maxHistoryTurns=10。各送信で user+assistant の2件が永続化される。
	// 7回送信すると7回目送信前の履歴は12件 → 直近10件に切り詰められる。
	const expectedLimit = 10
	for range 7 {
		_, err := uc.SendMessage(ctx, "予定を追加して")
		require.NoError(t, err)
	}
	assert.Len(t, ext.lastTurns, expectedLimit)
}

func TestSendMessage_HistoryWithinLimitReturnsAll(t *testing.T) {
	client := testsupport.NewClient(t)
	ext := &capturingExtractor{result: &chatservice.ExtractionResult{
		Status: chatservice.StatusNotFound, Message: "ok",
	}}
	uc := newTestUC(client, ext, nil)
	ctx := context.Background()

	// 2回送信 → 3回目送信前の履歴は4件（上限10以内）なので全件渡す。
	for range 2 {
		_, err := uc.SendMessage(ctx, "予定を追加して")
		require.NoError(t, err)
	}
	_, err := uc.SendMessage(ctx, "予定を追加して")
	require.NoError(t, err)
	assert.Len(t, ext.lastTurns, 4)
}

func TestClearConversation_RemovesMessagesAndProposals(t *testing.T) {
	client := testsupport.NewClient(t)
	uc := newTestUC(client, &mockExtractor{result: eventResult()}, nil)
	ctx := context.Background()

	_, err := uc.SendMessage(ctx, "TGSを追加")
	require.NoError(t, err)

	// クリア前は履歴と予定案が存在する
	view, err := uc.GetConversation(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, view.Messages)
	require.NotEmpty(t, view.Proposals)

	require.NoError(t, uc.ClearConversation(ctx))

	// クリア後はメッセージ・予定案が空
	view, err = uc.GetConversation(ctx)
	require.NoError(t, err)
	assert.Empty(t, view.Messages)
	assert.Empty(t, view.Proposals)
}

func TestGetConversation_History(t *testing.T) {
	client := testsupport.NewClient(t)
	uc := newTestUC(client, &mockExtractor{result: eventResult()}, nil)
	ctx := context.Background()

	_, err := uc.SendMessage(ctx, "TGSを追加")
	require.NoError(t, err)

	view, err := uc.GetConversation(ctx)
	require.NoError(t, err)
	require.NotNil(t, view.Conversation)
	// user + assistant
	require.Len(t, view.Messages, 2)
	assert.Equal(t, entity.RoleUser, view.Messages[0].Role)
	assert.Equal(t, entity.RoleAssistant, view.Messages[1].Role)
	// 1件の予定案
	require.Len(t, view.Proposals, 1)
}
