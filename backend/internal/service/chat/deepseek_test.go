package chat

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mustTime は RFC3339 文字列を time.Time にパースする（テスト用）。
func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, s)
	require.NoError(t, err)
	return parsed
}

// mockCompleter は chatCompleter のモック。responses を呼び出し順に返す。
type mockCompleter struct {
	responses []*openai.ChatCompletion
	err       error
	calls     []openai.ChatCompletionNewParams
}

func (m *mockCompleter) New(_ context.Context, params openai.ChatCompletionNewParams, _ ...option.RequestOption) (*openai.ChatCompletion, error) {
	m.calls = append(m.calls, params)
	if m.err != nil {
		return nil, m.err
	}
	idx := len(m.calls) - 1
	if idx >= len(m.responses) {
		idx = len(m.responses) - 1
	}
	return m.responses[idx], nil
}

// openAIMessage はテキスト1つを持つ Chat Completion レスポンスを作る。
func openAIMessage(text string) *openai.ChatCompletion {
	return &openai.ChatCompletion{
		Choices: []openai.ChatCompletionChoice{
			{Message: openai.ChatCompletionMessage{Content: text}},
		},
	}
}

// mockSearcher は Searcher のモック。
type mockSearcher struct {
	results []SearchResult
	err     error
	queries []string
}

func (m *mockSearcher) Search(_ context.Context, query string) ([]SearchResult, error) {
	m.queries = append(m.queries, query)
	if m.err != nil {
		return nil, m.err
	}
	return m.results, nil
}

func sampleResults() []SearchResult {
	return []SearchResult{
		{Title: "東京ゲームショウ2026 公式", URL: "https://tgs.example.com", Content: "2026年9月24日〜27日 幕張メッセ"},
	}
}

func TestDeepSeekExtract_EventStatus(t *testing.T) {
	json := `{"status":"event","message":"TGSの予定案を作成しました","event":{"title":"東京ゲームショウ2026","starts_at":"2026-09-24T10:00:00+09:00","ends_at":"2026-09-27T17:00:00+09:00","all_day":false,"location":"幕張メッセ","description":"ゲーム展示会","source_url":"https://tgs.example.com"}}`
	comp := &mockCompleter{responses: []*openai.ChatCompletion{openAIMessage(json)}}
	search := &mockSearcher{results: sampleResults()}
	ex := newDeepSeekExtractorWithClient(comp, search, "", nil)

	res, err := ex.Extract(context.Background(), nil, "TGSの予定を追加して")
	require.NoError(t, err)
	assert.Equal(t, StatusEvent, res.Status)
	require.NotNil(t, res.Event)
	assert.Equal(t, "東京ゲームショウ2026", res.Event.Title)
	assert.Equal(t, "幕張メッセ", res.Event.Location)
	assert.Equal(t, 2026, res.Event.StartsAt.Year())
	assert.True(t, res.Event.EndsAt.After(res.Event.StartsAt))

	// 検索は 1 回のみ、クエリに文脈キーワードが補われている
	require.Len(t, search.queries, 1)
	assert.Contains(t, search.queries[0], searchContextKeywords)
	// LLM 呼び出しは 1 回、検索結果がユーザーメッセージへ注入されている
	require.Len(t, comp.calls, 1)
	assert.Contains(t, lastUserContent(t, comp.calls[0]), "幕張メッセ")
}

func TestDeepSeekExtract_InjectsJSTAndJapaneseRules(t *testing.T) {
	json := `{"status":"off_topic","message":"ok"}`
	comp := &mockCompleter{responses: []*openai.ChatCompletion{openAIMessage(json)}}
	ex := newDeepSeekExtractorWithClient(comp, &mockSearcher{results: sampleResults()}, "", nil)

	_, err := ex.Extract(context.Background(), nil, "TGSを追加")
	require.NoError(t, err)
	require.Len(t, comp.calls, 1)

	// system メッセージに JST・日本語出力の規則が含まれる
	require.NotNil(t, comp.calls[0].Messages[0].OfSystem)
	sys := comp.calls[0].Messages[0].OfSystem.Content.OfString
	require.True(t, sys.Valid())
	assert.Contains(t, sys.Value, "+09:00")
	assert.Contains(t, sys.Value, "日本標準時")

	// user メッセージに現在日時（JST）が注入される
	assert.Contains(t, lastUserContent(t, comp.calls[0]), "現在の日時")

	// JSON 強制出力が有効
	assert.NotNil(t, comp.calls[0].ResponseFormat.OfJSONObject)
}

func TestDeepSeekExtract_MultipleCandidates(t *testing.T) {
	json := `{"status":"multiple","message":"候補が複数あります","candidates":[
	  {"title":"A","starts_at":"2026-01-01T10:00:00Z","ends_at":"2026-01-01T12:00:00Z","all_day":false,"location":"X","description":"","source_url":""},
	  {"title":"B","starts_at":"2026-02-01T10:00:00Z","ends_at":"2026-02-01T12:00:00Z","all_day":false,"location":"Y","description":"","source_url":""}
	]}`
	comp := &mockCompleter{responses: []*openai.ChatCompletion{openAIMessage(json)}}
	ex := newDeepSeekExtractorWithClient(comp, &mockSearcher{results: sampleResults()}, "", nil)

	res, err := ex.Extract(context.Background(), nil, "曖昧なイベント")
	require.NoError(t, err)
	assert.Equal(t, StatusMultiple, res.Status)
	require.Len(t, res.Candidates, 2)
	assert.Equal(t, "A", res.Candidates[0].Title)
	assert.Nil(t, res.Event)
}

func TestDeepSeekExtract_NotFoundThenResearchSucceeds(t *testing.T) {
	notFound := `{"status":"not_found","message":"特定できませんでした"}`
	found := `{"status":"event","message":"見つかりました","event":{"title":"E","starts_at":"2026-05-01T10:00:00+09:00","ends_at":"2026-05-01T12:00:00+09:00","all_day":false,"location":"Z","description":"","source_url":"https://e.example.com"}}`
	comp := &mockCompleter{responses: []*openai.ChatCompletion{openAIMessage(notFound), openAIMessage(found)}}
	search := &mockSearcher{results: sampleResults()}
	ex := newDeepSeekExtractorWithClient(comp, search, "", nil)

	res, err := ex.Extract(context.Background(), nil, "何かのイベント")
	require.NoError(t, err)
	assert.Equal(t, StatusEvent, res.Status)
	// 初回 + 再検索1回 = 2 回
	require.Len(t, search.queries, 2)
	require.Len(t, comp.calls, 2)
	// 再検索クエリは見直されている
	assert.NotEqual(t, search.queries[0], search.queries[1])
	assert.Contains(t, search.queries[1], "公式サイト")
}

func TestDeepSeekExtract_NotFoundStopsAtLimit(t *testing.T) {
	notFound := `{"status":"not_found","message":"特定できませんでした。追加情報をください"}`
	comp := &mockCompleter{responses: []*openai.ChatCompletion{openAIMessage(notFound)}}
	search := &mockSearcher{results: sampleResults()}
	ex := newDeepSeekExtractorWithClient(comp, search, "", nil)

	res, err := ex.Extract(context.Background(), nil, "存在しないイベント")
	require.NoError(t, err)
	assert.Equal(t, StatusNotFound, res.Status)
	// 初回 + 上限までの再検索1回 = 2 回で打ち切る
	assert.Len(t, search.queries, 2)
	assert.Len(t, comp.calls, 2)
	assert.Contains(t, res.Message, "追加情報")
}

func TestDeepSeekExtract_OffTopic(t *testing.T) {
	json := `{"status":"off_topic","message":"予定登録専用です"}`
	comp := &mockCompleter{responses: []*openai.ChatCompletion{openAIMessage(json)}}
	search := &mockSearcher{results: sampleResults()}
	ex := newDeepSeekExtractorWithClient(comp, search, "", nil)

	res, err := ex.Extract(context.Background(), nil, "今日の天気は？")
	require.NoError(t, err)
	assert.Equal(t, StatusOffTopic, res.Status)
	// off_topic でも常に検索する（design.md D3）
	assert.Len(t, search.queries, 1)
}

func TestDeepSeekExtract_SearchError(t *testing.T) {
	comp := &mockCompleter{responses: []*openai.ChatCompletion{openAIMessage(`{"status":"off_topic","message":"ok"}`)}}
	search := &mockSearcher{err: errors.New("search down")}
	ex := newDeepSeekExtractorWithClient(comp, search, "", nil)

	_, err := ex.Extract(context.Background(), nil, "TGS")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "search failed")
}

func TestDeepSeekExtract_ClientError(t *testing.T) {
	comp := &mockCompleter{err: errors.New("network down")}
	ex := newDeepSeekExtractorWithClient(comp, &mockSearcher{results: sampleResults()}, "", nil)

	_, err := ex.Extract(context.Background(), nil, "TGS")
	require.Error(t, err)
}

func TestDeepSeekExtract_UsesModelAndHistory(t *testing.T) {
	json := `{"status":"off_topic","message":"ok"}`
	comp := &mockCompleter{responses: []*openai.ChatCompletion{openAIMessage(json)}}
	ex := newDeepSeekExtractorWithClient(comp, &mockSearcher{results: sampleResults()}, "DeepSeek-V4-Pro-9999", nil)

	history := []Turn{
		{Role: "user", Content: "こんにちは"},
		{Role: "assistant", Content: "予定登録のお手伝いをします"},
	}
	_, err := ex.Extract(context.Background(), history, "TGSを追加")
	require.NoError(t, err)
	require.Len(t, comp.calls, 1)
	assert.Equal(t, openai.ChatModel("DeepSeek-V4-Pro-9999"), comp.calls[0].Model)
	// system(1) + history(2) + 今回のユーザーメッセージ(1) = 4
	assert.Len(t, comp.calls[0].Messages, 4)
}

// lastUserContent は最後の user メッセージのテキストを取り出す（検索結果注入の検証用）。
func lastUserContent(t *testing.T, params openai.ChatCompletionNewParams) string {
	t.Helper()
	require.NotEmpty(t, params.Messages)
	last := params.Messages[len(params.Messages)-1]
	require.NotNil(t, last.OfUser)
	content := last.OfUser.Content.OfString
	require.True(t, content.Valid())
	return content.Value
}

func TestBuildSearchQuery(t *testing.T) {
	now := mustTime(t, "2026-09-20T00:00:00Z")

	// 年が含まれない発話には今年と来年の両方を補う
	q := buildSearchQuery("TGSの予定を追加して", now)
	assert.Contains(t, q, "TGS")
	assert.Contains(t, q, "2026年")
	assert.Contains(t, q, "2027年")
	assert.Contains(t, q, searchContextKeywords)

	// 既に年が含まれる場合は重複補完しない
	q2 := buildSearchQuery("2025年のコミケ", now)
	assert.NotContains(t, q2, "2026年")
	assert.NotContains(t, q2, "2027年")
	assert.Contains(t, q2, "2025年")

	// 空発話はそのまま
	assert.Equal(t, "", buildSearchQuery("   ", now))
}

func TestRefineSearchQuery(t *testing.T) {
	refined := refineSearchQuery("TGS 2026年 開催日程")
	assert.Contains(t, refined, "公式サイト")
	assert.Contains(t, refined, "最新情報")
}

func TestFormatSearchResults(t *testing.T) {
	assert.Contains(t, formatSearchResults(nil), "検索結果は得られませんでした")

	out := formatSearchResults(sampleResults())
	assert.Contains(t, out, "https://tgs.example.com")
	assert.Contains(t, out, "幕張メッセ")
	assert.True(t, strings.HasPrefix(out, "[1]"))
}
