package chat

import (
	"context"
	"errors"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockClient は messageCreator のモック。
type mockClient struct {
	resp *anthropic.Message
	err  error
	// 直近のリクエストを記録
	lastParams anthropic.MessageNewParams
}

func (m *mockClient) New(_ context.Context, params anthropic.MessageNewParams, _ ...option.RequestOption) (*anthropic.Message, error) {
	m.lastParams = params
	if m.err != nil {
		return nil, m.err
	}
	return m.resp, nil
}

// textMessage はテキストブロック1つを持つレスポンスを作る。
func textMessage(text string) *anthropic.Message {
	return &anthropic.Message{
		Content: []anthropic.ContentBlockUnion{
			{Type: "text", Text: text},
		},
	}
}

func TestExtract_EventStatus(t *testing.T) {
	json := `前置き説明。 {"status":"event","message":"TGSの予定案を作成しました","event":{"title":"東京ゲームショウ2026","starts_at":"2026-09-24T10:00:00+09:00","ends_at":"2026-09-27T17:00:00+09:00","all_day":false,"location":"幕張メッセ","description":"ゲーム展示会","source_url":"https://tgs.example.com"}}`
	mock := &mockClient{resp: textMessage(json)}
	ex := newExtractorWithClient(mock, "", nil)

	res, err := ex.Extract(context.Background(), nil, "TGSの予定を追加して")
	require.NoError(t, err)
	assert.Equal(t, StatusEvent, res.Status)
	require.NotNil(t, res.Event)
	assert.Equal(t, "東京ゲームショウ2026", res.Event.Title)
	assert.Equal(t, "幕張メッセ", res.Event.Location)
	// 複数日イベントの開始/終了保持（UTCに正規化）
	assert.Equal(t, 2026, res.Event.StartsAt.Year())
	assert.True(t, res.Event.EndsAt.After(res.Event.StartsAt))

	// web_search ツールが付与されていること
	require.Len(t, mock.lastParams.Tools, 1)
	assert.NotNil(t, mock.lastParams.Tools[0].OfWebSearchTool20250305)
}

func TestExtract_MultipleCandidates(t *testing.T) {
	json := `{"status":"multiple","message":"候補が複数あります","candidates":[
	  {"title":"A","starts_at":"2026-01-01T10:00:00Z","ends_at":"2026-01-01T12:00:00Z","all_day":false,"location":"X","description":"","source_url":""},
	  {"title":"B","starts_at":"2026-02-01T10:00:00Z","ends_at":"2026-02-01T12:00:00Z","all_day":false,"location":"Y","description":"","source_url":""}
	]}`
	mock := &mockClient{resp: textMessage(json)}
	ex := newExtractorWithClient(mock, "", nil)

	res, err := ex.Extract(context.Background(), nil, "曖昧なイベント")
	require.NoError(t, err)
	assert.Equal(t, StatusMultiple, res.Status)
	require.Len(t, res.Candidates, 2)
	assert.Equal(t, "A", res.Candidates[0].Title)
	assert.Nil(t, res.Event)
}

func TestExtract_NotFound(t *testing.T) {
	json := `{"status":"not_found","message":"特定できませんでした。追加情報をください"}`
	mock := &mockClient{resp: textMessage(json)}
	ex := newExtractorWithClient(mock, "", nil)

	res, err := ex.Extract(context.Background(), nil, "なんかいい感じのやつ")
	require.NoError(t, err)
	assert.Equal(t, StatusNotFound, res.Status)
	assert.Nil(t, res.Event)
	assert.Contains(t, res.Message, "追加情報")
}

func TestExtract_OffTopic(t *testing.T) {
	json := `{"status":"off_topic","message":"予定登録専用です"}`
	mock := &mockClient{resp: textMessage(json)}
	ex := newExtractorWithClient(mock, "", nil)

	res, err := ex.Extract(context.Background(), nil, "今日の天気は？")
	require.NoError(t, err)
	assert.Equal(t, StatusOffTopic, res.Status)
}

func TestExtract_ClientError(t *testing.T) {
	mock := &mockClient{err: errors.New("network down")}
	ex := newExtractorWithClient(mock, "", nil)

	_, err := ex.Extract(context.Background(), nil, "TGS")
	require.Error(t, err)
}

func TestExtract_NoJSON(t *testing.T) {
	mock := &mockClient{resp: textMessage("JSONを含まないテキスト応答")}
	ex := newExtractorWithClient(mock, "", nil)

	_, err := ex.Extract(context.Background(), nil, "TGS")
	require.Error(t, err)
}

func TestExtract_UsesModelAndHistory(t *testing.T) {
	json := `{"status":"off_topic","message":"ok"}`
	mock := &mockClient{resp: textMessage(json)}
	ex := newExtractorWithClient(mock, "claude-opus-4-8", nil)

	history := []Turn{
		{Role: "user", Content: "こんにちは"},
		{Role: "assistant", Content: "予定登録のお手伝いをします"},
	}
	_, err := ex.Extract(context.Background(), history, "TGSを追加")
	require.NoError(t, err)
	assert.Equal(t, anthropic.Model("claude-opus-4-8"), mock.lastParams.Model)
	// history(2) + 今回のユーザーメッセージ(1) = 3
	assert.Len(t, mock.lastParams.Messages, 3)
}
