package chat

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/ITK13201/CalendarRabbit/backend/internal/domain/entity"
	"github.com/ITK13201/CalendarRabbit/backend/internal/logging"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/shared"
)

// deepSeekMaxTokens は completion のトークン上限。reasoning 系モデルでも最終JSONが
// 途中で切れないよう十分に確保する。
const deepSeekMaxTokens = 8192

// DefaultDeepSeekModel は既定の DeepSeek モデル。
// 公式 API が受け付けるモデル名は deepseek-flash / deepseek-v4-pro（design.md の DeepSeek-V4-Pro-0813 は API 側で不可）。
const DefaultDeepSeekModel = "deepseek-v4-pro"

// DefaultDeepSeekBaseURL は DeepSeek OpenAI 互換 API の既定エンドポイント。
const DefaultDeepSeekBaseURL = "https://api.deepseek.com"

// maxDeepSeekResearch は not_found 時の再検索回数の上限（design.md D5）。初回に加え最大 1 回。
const maxDeepSeekResearch = 1

// deepSeekRequestTimeout は 1 回の Chat Completions 呼び出しの上限。reasoning 系は
// 数十秒かかるため長めに取るが、2 パス合計がプロキシ側のタイムアウトを超えないよう抑える。
const deepSeekRequestTimeout = 80 * time.Second

// deepSeekMaxRetries は 1 呼び出しあたりの再試行回数。長時間応答での多重化を避けるため抑える。
const deepSeekMaxRetries = 1

// jst は日本標準時（Asia/Tokyo, UTC+09:00）。distroless でも tzdata に依存しないよう固定オフセットで定義する。
var jst = time.FixedZone("JST", 9*60*60)

// deepSeekSystemPrompt は事前検索＋注入方式に合わせたシステムプロンプト。
// Claude のネイティブ web_search とは異なり、アプリが渡す検索結果のみを根拠に抽出する。
const deepSeekSystemPrompt = `あなたはカレンダーへの予定登録に特化した日本語アシスタントです。
ユーザーのメッセージからイベント（例: 展示会、コンサート、大会など）を特定し、与えられた「検索結果」を根拠に最新の開催情報（名称・開始/終了日時・場所・概要・情報源URL）を判断してください。検索結果に無い事実を創作してはいけません。

必ず最後に、次のJSONオブジェクトのみを1つ、コードブロックなしで出力してください（前後に説明文を付けない）:
{
  "status": "event" | "multiple" | "not_found" | "off_topic",
  "message": "ユーザーへの日本語の応答",
  "event": {
    "title": "イベント名",
    "starts_at": "RFC3339形式の開始日時(例: 2026-09-24T10:00:00+09:00)",
    "ends_at": "RFC3339形式の終了日時",
    "all_day": true または false,
    "location": "開催場所",
    "description": "概要",
    "source_url": "情報源URL"
  },
  "candidates": [ 上記eventと同じ形式のオブジェクトの配列 ]
}

日時の規則（重要）:
- すべての日時は日本標準時（Asia/Tokyo, UTC+09:00）で解釈・出力すること。RFC3339 のオフセットは必ず "+09:00" を付け、UTC の "Z" は使わないこと。
- 「今日」「今年」などの相対表現は、ユーザーメッセージで与えられる「現在の日時」を基準に解釈すること。
- ユーザーが年を明示していない場合は、現在の日時以降で最も近い開催回（今年または来年）を優先して特定すること。既に今年の開催が終了している場合は来年の開催を採用する。
- 終日イベント（all_day=true）の場合、starts_at は初日の "T00:00:00+09:00"、ends_at は最終開催日（当日）の "T00:00:00+09:00" とすること。ends_at を翌日にずらさないこと。例: 4/30〜5/2 開催なら ends_at は "2027-05-02T00:00:00+09:00"。

出力言語の規則:
- "message" と "description"（候補含む）は必ず日本語で記述すること。検索結果が英語でも日本語に要約・翻訳して記述する。

判定ルール:
- イベントを1件特定できた場合: status="event" とし "event" を埋める。
- 複数のイベント候補が該当する場合: status="multiple" とし "candidates" に候補を列挙し、"message" でどれか確認を求める。
- 検索結果からは対象イベントを特定できない/情報が不十分な場合: status="not_found" とし、"message" で追加情報を求める。"event" は出力しない。
- 予定登録と無関係なメッセージの場合: status="off_topic" とし、"message" で予定登録専用である旨を案内する。
複数日開催のイベントは starts_at と ends_at の両方を必ず設定してください。`

// chatCompleter は OpenAI 互換 Chat Completions API の New を抽象化する（テストでモック化）。
type chatCompleter interface {
	New(ctx context.Context, params openai.ChatCompletionNewParams, opts ...option.RequestOption) (*openai.ChatCompletion, error)
}

// DeepSeekExtractor は DeepSeek（OpenAI 互換 API）＋事前検索注入による Extractor 実装。
type DeepSeekExtractor struct {
	client   chatCompleter
	searcher Searcher
	model    string
	now      func() time.Time
	logger   *slog.Logger
}

var _ Extractor = (*DeepSeekExtractor)(nil)

// NewDeepSeekExtractor は API キー・モデル・base_url と Searcher から DeepSeekExtractor を生成する。
func NewDeepSeekExtractor(apiKey, model, baseURL string, searcher Searcher, logger *slog.Logger) *DeepSeekExtractor {
	if model == "" {
		model = DefaultDeepSeekModel
	}
	if baseURL == "" {
		baseURL = DefaultDeepSeekBaseURL
	}
	client := openai.NewClient(
		option.WithAPIKey(apiKey),
		option.WithBaseURL(baseURL),
		option.WithRequestTimeout(deepSeekRequestTimeout),
		option.WithMaxRetries(deepSeekMaxRetries),
	)
	return &DeepSeekExtractor{
		client:   &client.Chat.Completions,
		searcher: searcher,
		model:    model,
		now:      time.Now,
		logger:   logger,
	}
}

// newDeepSeekExtractorWithClient はテスト用にモッククライアント／Searcher を注入する。
func newDeepSeekExtractorWithClient(client chatCompleter, searcher Searcher, model string, logger *slog.Logger) *DeepSeekExtractor {
	if model == "" {
		model = DefaultDeepSeekModel
	}
	return &DeepSeekExtractor{
		client:   client,
		searcher: searcher,
		model:    model,
		now:      time.Now,
		logger:   logger,
	}
}

// Extract は事前検索→結果注入→LLM 1 回呼び出しでイベントを抽出する。
// 初回が not_found の場合のみ、クエリを見直して最大 1 回だけ再検索・再抽出する（design.md D5）。
func (e *DeepSeekExtractor) Extract(ctx context.Context, history []Turn, userMessage string) (res *ExtractionResult, err error) {
	defer logging.Trace(ctx, e.logger, "chat.DeepSeekExtractor.Extract",
		logging.Args{"userMessage": userMessage, "historyLen": len(history)}, &res, &err)()

	query := buildSearchQuery(userMessage, e.now().In(jst))

	result, err := e.searchAndExtract(ctx, history, userMessage, query)
	if err != nil {
		return nil, err
	}

	// not_found の場合のみ限定的 2 パス（再検索は最大 maxDeepSeekResearch 回）。
	// 再検索の attempt/query は started/finished では表せない固有の診断情報のため残す（design D4）。
	for attempt := 0; result.Status == StatusNotFound && attempt < maxDeepSeekResearch; attempt++ {
		query = refineSearchQuery(query)
		logging.LogContext(ctx, e.logger, slog.LevelInfo, "chat.deepseek.research",
			slog.Int("attempt", attempt+1), slog.String("query", query))
		result, err = e.searchAndExtract(ctx, history, userMessage, query)
		if err != nil {
			return nil, err
		}
	}

	// extracted（status）は Trace の finished（result）へ集約したため撤去。
	return result, nil
}

// searchAndExtract は 1 パス分（検索→注入→LLM 呼び出し→パース）を実行する。
func (e *DeepSeekExtractor) searchAndExtract(ctx context.Context, history []Turn, userMessage, query string) (*ExtractionResult, error) {
	results, err := e.searcher.Search(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	messages := make([]openai.ChatCompletionMessageParamUnion, 0, len(history)+2)
	messages = append(messages, openai.SystemMessage(deepSeekSystemPrompt))
	for _, t := range history {
		if t.Role == entity.RoleAssistant {
			messages = append(messages, openai.AssistantMessage(t.Content))
		} else {
			messages = append(messages, openai.UserMessage(t.Content))
		}
	}
	nowJST := e.now().In(jst)
	userContent := fmt.Sprintf(
		"現在の日時（日本標準時 Asia/Tokyo, UTC+09:00）: %s\n\nユーザーの依頼:\n%s\n\n--- 検索結果 ---\n%s",
		nowJST.Format("2006-01-02 15:04 (Mon)"),
		userMessage,
		formatSearchResults(results),
	)
	messages = append(messages, openai.UserMessage(userContent))

	callStart := time.Now()
	resp, err := e.client.New(ctx, openai.ChatCompletionNewParams{
		Model:     openai.ChatModel(e.model),
		MaxTokens: openai.Int(deepSeekMaxTokens),
		Messages:  messages,
		// JSON オブジェクトのみを返すよう強制し、抽出の安定性を高める。
		ResponseFormat: openai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONObject: &shared.ResponseFormatJSONObjectParam{},
		},
	})
	callLatency := time.Since(callStart)
	if err != nil {
		// request 失敗は Trace（Extract）の finished error へ集約したため個別ログは撤去。
		return nil, fmt.Errorf("deepseek request failed: %w", err)
	}

	text := collectOpenAIText(resp)
	// レスポンスボディ（LLM 生応答）を extra.responseBody として丸め、外部API実行の latency_ms を出力する（design D4）。
	logging.LogContext(ctx, e.logger, slog.LevelInfo, "chat.deepseek.response",
		slog.Float64("latency_ms", logging.DurationMillis(callLatency)),
		slog.String("responseBody", logging.Truncate(text, logging.MaxTruncateRunes)))

	result, err := parseExtraction(text)
	if err != nil {
		// parse 失敗時の finish_reason は started/finished では表せない固有の診断情報のため残す
		// （raw 抜粋は上の responseBody に含まれる。design D4）。
		logging.LogContext(ctx, e.logger, slog.LevelError, "chat.deepseek.parse_failed",
			slog.String("error", err.Error()),
			slog.String("finish_reason", finishReason(resp)),
			slog.Int("text_len", len(text)),
		)
		return nil, err
	}
	return result, nil
}

// collectOpenAIText はレスポンスの最初の choice の本文を返す。
func collectOpenAIText(resp *openai.ChatCompletion) string {
	if resp == nil || len(resp.Choices) == 0 {
		return ""
	}
	return resp.Choices[0].Message.Content
}

// finishReason は最初の choice の finish_reason を返す（デバッグ用）。
func finishReason(resp *openai.ChatCompletion) string {
	if resp == nil || len(resp.Choices) == 0 {
		return ""
	}
	return resp.Choices[0].FinishReason
}
