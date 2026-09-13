package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/ITK13201/CalendarRabbit/backend/internal/domain/entity"
	"github.com/ITK13201/CalendarRabbit/backend/internal/logging"
	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// DefaultModel は既定の Claude モデル（design.md D3）。
const DefaultModel = "claude-sonnet-4-6"

// defaultMaxWebSearches は web_search ツールの利用回数上限。
const defaultMaxWebSearches = 5

// systemPrompt は予定登録特化のシステムプロンプト。
const systemPrompt = `あなたはカレンダーへの予定登録に特化したアシスタントです。
ユーザーのメッセージからイベント（例: 展示会、コンサート、大会など）を特定し、web_searchツールで最新の開催情報（名称・開始/終了日時・場所・概要・情報源URL）を調べてください。

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

判定ルール:
- イベントを1件特定できた場合: status="event" とし "event" を埋める。
- 複数のイベント候補が該当する場合: status="multiple" とし "candidates" に候補を列挙し、"message" でどれか確認を求める。
- 対象イベントを特定できない/情報が不十分な場合: status="not_found" とし、"message" で追加情報を求める。"event" は出力しない。
- 予定登録と無関係なメッセージの場合: status="off_topic" とし、"message" で予定登録専用である旨を案内する。
複数日開催のイベントは starts_at と ends_at の両方を必ず設定してください。`

// messageCreator は Anthropic Messages API の New を抽象化する（テストでモック化）。
type messageCreator interface {
	New(ctx context.Context, params anthropic.MessageNewParams, opts ...option.RequestOption) (*anthropic.Message, error)
}

// ClaudeExtractor は Claude API を用いた Extractor 実装。
type ClaudeExtractor struct {
	client         messageCreator
	model          string
	maxWebSearches int64
	logger         *slog.Logger
}

var _ Extractor = (*ClaudeExtractor)(nil)

// NewClaudeExtractor は API キーとモデルから ClaudeExtractor を生成する。
func NewClaudeExtractor(apiKey, model string, logger *slog.Logger) *ClaudeExtractor {
	if model == "" {
		model = DefaultModel
	}
	client := anthropic.NewClient(option.WithAPIKey(apiKey))
	return &ClaudeExtractor{
		client:         &client.Messages,
		model:          model,
		maxWebSearches: defaultMaxWebSearches,
		logger:         logger,
	}
}

// newExtractorWithClient はテスト用にモッククライアントを注入する。
func newExtractorWithClient(client messageCreator, model string, logger *slog.Logger) *ClaudeExtractor {
	if model == "" {
		model = DefaultModel
	}
	return &ClaudeExtractor{client: client, model: model, maxWebSearches: defaultMaxWebSearches, logger: logger}
}

// extractionDTO は Claude が返す JSON のパース用。
type extractionDTO struct {
	Status     string     `json:"status"`
	Message    string     `json:"message"`
	Event      *eventDTO  `json:"event"`
	Candidates []eventDTO `json:"candidates"`
}

type eventDTO struct {
	Title       string `json:"title"`
	StartsAt    string `json:"starts_at"`
	EndsAt      string `json:"ends_at"`
	AllDay      bool   `json:"all_day"`
	Location    string `json:"location"`
	Description string `json:"description"`
	SourceURL   string `json:"source_url"`
}

// Extract は会話履歴とユーザーメッセージを送り、イベント抽出結果を返す。
func (e *ClaudeExtractor) Extract(ctx context.Context, history []Turn, userMessage string) (*ExtractionResult, error) {
	messages := make([]anthropic.MessageParam, 0, len(history)+1)
	for _, t := range history {
		block := anthropic.NewTextBlock(t.Content)
		if t.Role == entity.RoleAssistant {
			messages = append(messages, anthropic.NewAssistantMessage(block))
		} else {
			messages = append(messages, anthropic.NewUserMessage(block))
		}
	}
	messages = append(messages, anthropic.NewUserMessage(anthropic.NewTextBlock(userMessage)))

	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(e.model),
		MaxTokens: 2048,
		System: []anthropic.TextBlockParam{
			{Text: systemPrompt, CacheControl: anthropic.CacheControlEphemeralParam{}},
		},
		Messages: messages,
		Tools: []anthropic.ToolUnionParam{
			{OfWebSearchTool20250305: &anthropic.WebSearchTool20250305Param{
				MaxUses: anthropic.Int(e.maxWebSearches),
			}},
		},
	}

	resp, err := e.client.New(ctx, params)
	if err != nil {
		logging.LogContext(ctx, e.logger, slog.LevelError, "chat.claude.request_failed", slog.String("error", err.Error()))
		return nil, fmt.Errorf("claude request failed: %w", err)
	}

	text := collectText(resp)
	result, err := parseExtraction(text)
	if err != nil {
		logging.LogContext(ctx, e.logger, slog.LevelError, "chat.claude.parse_failed", slog.String("error", err.Error()))
		return nil, err
	}
	logging.LogContext(ctx, e.logger, slog.LevelInfo, "chat.claude.extracted", slog.String("status", string(result.Status)))
	return result, nil
}

// collectText はレスポンスの text ブロックを連結する。
func collectText(resp *anthropic.Message) string {
	var sb strings.Builder
	for _, block := range resp.Content {
		if block.Type == "text" {
			sb.WriteString(block.Text)
		}
	}
	return sb.String()
}

// parseExtraction は Claude の出力テキストから JSON を取り出して結果に変換する。
func parseExtraction(text string) (*ExtractionResult, error) {
	raw := extractJSON(text)
	if raw == "" {
		return nil, errors.New("no JSON object found in Claude response")
	}
	var dto extractionDTO
	if err := json.Unmarshal([]byte(raw), &dto); err != nil {
		return nil, fmt.Errorf("failed to parse Claude JSON: %w", err)
	}

	result := &ExtractionResult{
		Status:  ExtractionStatus(dto.Status),
		Message: dto.Message,
	}
	switch result.Status {
	case StatusEvent:
		if dto.Event == nil {
			return nil, errors.New("status=event but event is missing")
		}
		ev, err := toExtractedEvent(*dto.Event)
		if err != nil {
			return nil, err
		}
		result.Event = &ev
	case StatusMultiple:
		for _, c := range dto.Candidates {
			ev, err := toExtractedEvent(c)
			if err != nil {
				return nil, err
			}
			result.Candidates = append(result.Candidates, ev)
		}
	case StatusNotFound, StatusOffTopic:
		// event/candidates は不要
	default:
		return nil, fmt.Errorf("unknown status: %q", dto.Status)
	}
	return result, nil
}

// extractJSON はテキスト中の最初の '{' から最後の '}' までを取り出す。
func extractJSON(text string) string {
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start < 0 || end < 0 || end < start {
		return ""
	}
	return text[start : end+1]
}

func toExtractedEvent(d eventDTO) (ExtractedEvent, error) {
	start, err := parseRFC3339("starts_at", d.StartsAt)
	if err != nil {
		return ExtractedEvent{}, err
	}
	end, err := parseRFC3339("ends_at", d.EndsAt)
	if err != nil {
		return ExtractedEvent{}, err
	}
	return ExtractedEvent{
		Title:       d.Title,
		StartsAt:    start,
		EndsAt:      end,
		AllDay:      d.AllDay,
		Location:    d.Location,
		Description: d.Description,
		SourceURL:   d.SourceURL,
	}, nil
}

func parseRFC3339(field, raw string) (time.Time, error) {
	if raw == "" {
		return time.Time{}, fmt.Errorf("%s is empty", field)
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("%s is not RFC3339: %w", field, err)
	}
	return t.UTC(), nil
}
